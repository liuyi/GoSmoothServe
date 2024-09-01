package service

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"go_service_core/core/log"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"path/filepath"
	"smoothserver/config"
	"strings"
	"sync"
	"time"
)

const (
	StatusNone = iota
	StatusStopped
	StatusStopping
	StatusWillRunning
	StatusWaitingStop
	StatusRunning
)

type Instance struct {
	Pid         string
	Port        int
	Status      int
	Host        string
	NeedRestart bool
}

type Service struct {
	Name          string
	Data          config.ServiceData
	Instances     []*Instance
	instanceIndex int
	initialized   bool //服务已经被初始化，创建了实例、文件监听等
	mutex         sync.Mutex
	watcher       *fsnotify.Watcher
	restartTimer  *time.Timer     //重启时的定时器，等待几秒后，如果时间没有被刷新，则正式开始重启
	stopWg        *sync.WaitGroup //当服务的实例等待停止时，要设定完成以便于在停止所有实例时，能够安全退出serve
}

func New(serviceData config.ServiceData) *Service {
	service := Service{Name: serviceData.Name, Data: serviceData}
	return &service
}
func (service *Service) CreateAndListen() {
	// 创建反向代理服务器
	// 启动 HTTP 服务器并监听指定端口

	//
	//if service.initialized {
	//	return
	//}
	//service.initialized = true
	go service.initWatcher()

	//检测是否有多个名字,给每一个域名都做反向代理

	go func() {
		serverNameArr := strings.Split(service.Data.ServerName, ",")
		for _, serverName := range serverNameArr {
			serverName := strings.TrimSpace(serverName)
			http.HandleFunc(serverName+"/", service.handleRequest)
		}

		log.Info("Start service ", zap.String("name", service.Name), zap.Int("port", service.Data.Port))

		err := http.ListenAndServe(fmt.Sprintf(":%d", service.Data.Port), nil)
		if err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Error("port  is in use,connect closed.", zap.Int("port", service.Data.Port))
			} else {
				log.Error("Failed to start Service", zap.String("name", service.Name), zap.Error(err))
			}
		}

	}()

}

func (service *Service) Start() {
	// 根据配置启动服务实例，并添加到 ServicesMap 中
	if service.Instances == nil {
		instances := make([]*Instance, service.Data.InstanceCount)
		service.Instances = instances
	} else {
		//如果已经存在，就不要继续了
		//fmt.Println("Service Started already!")
		//return
	}

	for i := 0; i < service.Data.InstanceCount; i++ {
		if service.Instances[i] == nil {
			//create new instance
			port := service.Data.StartInstancePort + i
			// 启动服务实例
			pid, err := service.StartInstance(port, service.Data.ExecutablePath) //启动了一个web实例以后 被堵塞 无法继续，应该如何解决
			if err != nil {
				log.Error("Failed to start service instance:", zap.Error(err))
				continue
			}
			instance := &Instance{Pid: pid, Port: port, Status: StatusRunning}
			service.Instances[i] = instance
		} else {
			//已经存在老的实例
			instance := service.Instances[i]
			if instance.Status == StatusStopped && instance.NeedRestart != true {
				//如果已被停止，而且没有处于自动重启的状态
				newPid, err := service.StartInstance(instance.Port, service.Data.ExecutablePath)
				if err != nil {

					log.Error("start instance failed", zap.Error(err), zap.Int("port", instance.Port), zap.String("path", service.Data.ExecutablePath))
					return
				}
				instance.Pid = newPid
				//启动后等几秒钟再使其进入可服务状态，没有监听instance的cmd输出内容来判断，因为不希望那么耦合
				instance.Status = StatusWillRunning
				time.Sleep(time.Duration(service.Data.DelayRunningTime) * time.Second)
				instance.Status = StatusRunning
			}
		}

	}

}
func (service *Service) SelectInstance() *Instance {
	if len(service.Instances) == 0 {
		return nil
	}

	instanceCount := len(service.Instances)
	service.instanceIndex = (service.instanceIndex) % instanceCount // 更新索引，实现轮询

	for i := 0; i < instanceCount; i++ {
		instance := service.Instances[service.instanceIndex]
		service.instanceIndex = (service.instanceIndex + 1) % instanceCount // 更新索引，实现轮询
		if instance.Status >= StatusWaitingStop {
			return instance
		}
	}

	return nil
}

func (service *Service) StartInstance(port int, executablePath string) (string, error) {
	// 启动指定的 HTTP 服务器进程，并传递端口号作为参数
	cmd := exec.Command(executablePath, fmt.Sprintf("-port=%d", port))
	cmd.Dir = filepath.Dir(executablePath)

	// 设置合适的环境变量等

	stdout, err := cmd.StdoutPipe()

	// 启动命令
	if err = cmd.Start(); err != nil {
		log.Error("start instance failed", zap.String("cmd", cmd.String()), zap.Error(err))
		return "", err
	}

	Screen := bufio.NewScanner(stdout)

	go func() {
		for Screen.Scan() {
			fmt.Println(Screen.Text())
		}
	}()

	// 获取进程 ID
	pid := fmt.Sprintf("%d", cmd.Process.Pid)

	// 等待进程退出
	go func() {
		if err := cmd.Wait(); err != nil {

			log.Error("Service Instance process exited with error:", zap.Error(err))
		}

		//查看当前是不是实例是不是需要重启
		instance := service.getInstance(pid)

		if instance.Status == StatusStopping {
			//被彻底停止，现在需要被重启
			instance.Status = StatusStopped

			if instance.NeedRestart {
				newPid, err := service.StartInstance(instance.Port, service.Data.ExecutablePath)
				if err != nil {

					log.Error("start instance error ", zap.Error(err))
					return
				}
				instance.Pid = newPid
				//启动后等几秒钟再使其进入可服务状态，没有监听instance的cmd输出内容来判断，因为不希望那么耦合
				instance.Status = StatusWillRunning
				time.Sleep(time.Duration(service.Data.DelayRunningTime) * time.Second)
				instance.Status = StatusRunning
				instance.NeedRestart = false
				//启动以后再开始停止下一个
				service.stopOne()
			} else {
				if service.stopWg != nil {
					service.stopWg.Done()
				}
			}

		} else {
			if service.stopWg != nil {
				service.stopWg.Done()
			}
		}
	}()

	// 返回进程 ID
	return pid, nil
}
func (service *Service) handleRequest(w http.ResponseWriter, r *http.Request) {
	// 选择一个服务实例处理请求
	service.mutex.Lock()
	defer service.mutex.Unlock()
	instance := service.SelectInstance()
	if instance == nil {
		http.Error(w, "No available instance", http.StatusServiceUnavailable)
		return
	}

	// 构建代理地址 目前仅支持本地的ip,因为服务启动过的方式就是通过调用本地命令行执行的，如果要支持代理到不同的服务器，则还需要增加远程启动服务的方式，目前没有这个需求就不加了。
	proxyURL := fmt.Sprintf("%s:%d", service.Data.ServerIp, instance.Port)

	// 创建反向代理
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = proxyURL
			req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
			req.Host = instance.Host // 更新 Host，确保正确代理
		},
	}

	// 执行反向代理
	rp.ServeHTTP(w, r)
}
func (service *Service) initWatcher() {
	// 创建新的fsnotify watcher
	if service.watcher != nil {
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("Error creating watcher:", zap.Error(err))
		return
	}
	service.watcher = watcher

	rootDir := filepath.Dir(service.Data.ExecutablePath)

	// 向Watcher添加要监视的文件和文件夹路径
	for _, path := range service.Data.WatchFiles {
		absFilePath := filepath.Join(rootDir, path)
		err = watcher.Add(absFilePath)
		if err != nil {
			log.Error("Error adding path to watcher", zap.String("path", absFilePath), zap.Error(err))
			return
		}
	}

	// 启动协程监视文件更改
	go func() {
		defer func(watcher *fsnotify.Watcher) {
			log.Info("Close watcher")
			err := watcher.Close()
			if err != nil {
				log.Error("close watcher error", zap.Error(err))
			}
		}(watcher)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Info("Event:", zap.String("name", event.Name), zap.String("Op", event.Op.String()))

				//
				//if event.Op&fsnotify.Write == fsnotify.Write ||
				//	event.Op&fsnotify.Create == fsnotify.Create ||
				//	event.Op&fsnotify.Remove == fsnotify.Remove ||
				//	event.Op&fsnotify.Rename == fsnotify.Rename {
				//
				//	// 如果发生了写入、创建或删除事件，则重新加载服务实例
				//	service.RestartInstances()
				//}
				// 检测到文件写入事件，启动定时器
				log.Info("有文件更新", zap.String("service", service.Name))
				service.mutex.Lock()
				if service.restartTimer != nil {
					// 如果定时器已存在，则停止它
					//fmt.Println("有已经存在的定时器，停止它")
					service.restartTimer.Stop()
				}
				//fmt.Println("启动一个定时器，到时间后去重启")
				// 设置新的定时器
				service.restartTimer = time.AfterFunc(time.Duration(service.Data.DelayRunningTime)*time.Second, service.RestartOneByOne)
				service.mutex.Unlock()
				//service.RestartOneByOne()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("watch error", zap.Error(err))
			}
		}
	}()
}

func (service *Service) getInstance(pid string) *Instance {
	for _, instance := range service.Instances {
		if instance != nil && instance.Pid == pid {
			return instance
		}
	}
	return nil
}

func (service *Service) RestartOneByOne() {
	log.Info("Restart service instance one by one.")
	for _, instance := range service.Instances {
		//先标记为都需要停止
		instance.Status = StatusWaitingStop
		instance.NeedRestart = true
	}

	//开始停止第一个
	service.stopOne()

}

func (service *Service) stopOne() {
	var selectedInstance *Instance
	for _, instance := range service.Instances {
		//先标记为都需要停止
		if instance.Status == StatusWaitingStop {
			selectedInstance = instance
			break
		}

	}

	if selectedInstance == nil {
		//已经没有需要停止的了

		log.Info("all instance stopped ")

		return
	}

	//开始停止第一个
	selectedInstance.Status = StatusStopping
	err := service.StopInstance(selectedInstance.Pid)
	if err != nil {
		log.Error("stop instance failed,pid:", zap.String("pid", selectedInstance.Pid), zap.Error(err))
		return
	}

}

func (service *Service) StopInstance(pid string) error {
	// 停止指定进程
	cmd := exec.Command("kill", "-TERM", pid)
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {

			log.Error("exit instance error", zap.String("error", exitErr.ProcessState.String()))
		}
		if service.stopWg != nil {
			service.stopWg.Done()
		}
		return err
	}
	return nil
}

// Stop
// 停止所有的实例
func (service *Service) Stop() {
	service.stopWg = new(sync.WaitGroup)
	for _, instance := range service.Instances {
		instance.Status = StatusStopping
		service.stopWg.Add(1)
		err := service.StopInstance(instance.Pid)
		if err != nil {
			log.Error("Stop instance got error", zap.Error(err))
			return
		}
	}
	service.stopWg.Wait()
}
