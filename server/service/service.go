package service

import (
	"bufio"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"path/filepath"
	"server/config"
	"strconv"
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

type ServiceInstance struct {
	Pid    string
	Port   int
	Status int
	Host   string
}

type Service struct {
	Name          string
	Data          config.ServiceData
	Instances     []*ServiceInstance
	instanceIndex int
	mutex         sync.Mutex
	watcher       *fsnotify.Watcher
}

func New(serviceData config.ServiceData) *Service {
	service := Service{Name: serviceData.Name, Data: serviceData}
	return &service
}
func (service *Service) Start() {
	// 创建反向代理服务器
	// 启动 HTTP 服务器并监听指定端口
	go service.startAllInstance()
	go service.initWatcher()

	//http.HandleFunc("/", service.handleRequest)
	//检测是否有多个名字
	serverNameArr := strings.Split(service.Data.ServerName, ",")
	for _, serverName := range serverNameArr {
		serverName := strings.TrimSpace(serverName)
		http.HandleFunc(serverName+"/", service.handleRequest)
	}

	fmt.Printf("Reverse Proxy Server for Service %s started on port %d\n", service.Name, service.Data.Port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", service.Data.Port), nil)
	if err != nil {
		fmt.Printf("Failed to start Reverse Proxy Server for Service %s: %s\n", service.Name, err)
	}

}
func (service *Service) startAllInstance() {
	// 根据配置启动服务实例，并添加到 ServicesMap 中
	if service.Instances == nil {
		instances := make([]*ServiceInstance, service.Data.InstanceCount)
		service.Instances = instances
	} else {
		//如果已经存在，就不要继续了
		fmt.Println("Service Started already!")
		return
	}

	for i := 0; i < service.Data.InstanceCount; i++ {
		port := service.Data.StartInstancePort + i
		// 启动服务实例
		pid, err := service.StartInstance(service.Data.Name, port, service.Data.ExecutablePath) //启动了一个web实例以后 被堵塞 无法继续，应该如何解决
		if err != nil {
			fmt.Println("Failed to start service instance:", err)
			continue
		}
		instance := &ServiceInstance{Pid: pid, Port: port, Status: StatusRunning}
		service.Instances[i] = instance
	}

}
func (service *Service) SelectInstance() *ServiceInstance {
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

func (service *Service) StartInstance(name string, port int, executablePath string) (string, error) {
	// 启动指定的 HTTP 服务器进程，并传递端口号作为参数
	cmd := exec.Command(executablePath, "--port="+strconv.Itoa(port))
	cmd.Dir = filepath.Dir(executablePath)

	// 设置合适的环境变量等

	stdout, err := cmd.StdoutPipe()

	// 启动命令
	if err = cmd.Start(); err != nil {
		fmt.Println("run cmd error:", cmd, err)
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

			fmt.Println("Process exited with error:", err)
		}

		//查看当前是不是实例是不是需要重启
		instance := service.getInstance(pid)

		if instance.Status == StatusStopping {
			//被彻底停止，现在需要被重启
			instance.Status = StatusStopped
			newPid, err := service.StartInstance("", instance.Port, service.Data.ExecutablePath)
			if err != nil {

				fmt.Println("start instance error ", err)
				return
			}
			instance.Pid = newPid
			//启动后等几秒钟再使其进入可服务状态，没有监听instance的cmd输出内容来判断，因为不希望那么耦合
			instance.Status = StatusWillRunning
			time.Sleep(time.Duration(service.Data.DelayRunningTime) * time.Second)
			instance.Status = StatusRunning

			//启动以后再开始停止下一个
			service.stopOne()
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Error creating watcher:", err)
		return
	}
	service.watcher = watcher

	rootDir := filepath.Dir(service.Data.ExecutablePath)

	// 向Watcher添加要监视的文件和文件夹路径
	for _, path := range service.Data.WatchFiles {
		absFilePath := filepath.Join(rootDir, path)
		err = watcher.Add(absFilePath)
		if err != nil {
			fmt.Printf("Error adding path %s to watcher: %s\n", absFilePath, err)
			return
		}
	}

	// 启动协程监视文件更改
	go func() {
		defer func(watcher *fsnotify.Watcher) {
			fmt.Println("Close watcher")
			err := watcher.Close()
			if err != nil {
				fmt.Println("close watcher error", err)
			}
		}(watcher)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				fmt.Println("Event:", event.Name, "event.Op:", event.Op, event.Op&fsnotify.Write, fsnotify.Write)

				//
				//if event.Op&fsnotify.Write == fsnotify.Write ||
				//	event.Op&fsnotify.Create == fsnotify.Create ||
				//	event.Op&fsnotify.Remove == fsnotify.Remove ||
				//	event.Op&fsnotify.Rename == fsnotify.Rename {
				//
				//	// 如果发生了写入、创建或删除事件，则重新加载服务实例
				//	service.RestartInstances()
				//}

				service.RestartOneByOne()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("Error:", err)
			}
		}
	}()
}

func (service *Service) getInstance(pid string) *ServiceInstance {
	for _, instance := range service.Instances {
		if instance.Pid == pid {
			return instance
		}
	}
	return nil
}

func (service *Service) RestartOneByOne() {
	for _, instance := range service.Instances {
		//先标记为都需要停止
		instance.Status = StatusWaitingStop

	}

	//开始停止第一个
	service.stopOne()

}

func (service *Service) stopOne() {
	var selectedInstance *ServiceInstance
	for _, instance := range service.Instances {
		//先标记为都需要停止
		if instance.Status == StatusWaitingStop {
			selectedInstance = instance
			break
		}

	}

	if selectedInstance == nil {
		//已经没有需要停止的了

		fmt.Println("no selectedInstance,all instance stopped ")

		return
	}

	//开始停止第一个
	selectedInstance.Status = StatusStopping
	err := service.StopInstance(selectedInstance.Pid)
	if err != nil {
		fmt.Println("stop instance failed,pid:", selectedInstance.Pid+", error:", err)
		return
	}

}

func (service *Service) RestartAllInstances() {
	// 停止并重新启动所有服务实例
	for i, instance := range service.Instances {
		instance.Status = StatusWaitingStop
		fmt.Printf("Stopping instance %d\n", i)
		err := service.StopInstance(instance.Pid)
		if err != nil {
			fmt.Printf("Error stopping instance %d: %s\n", i, err)
			continue
		}

		fmt.Printf("Starting instance %d\n", i)
		pid, err := service.StartInstance(service.Data.Name, instance.Port, service.Data.ExecutablePath)
		if err != nil {
			fmt.Printf("Error starting instance %d: %s\n", i, err)
			continue
		}
		service.Instances[i].Pid = pid
		//重启完毕后 如果不重新设置watcher 那么如果更新正在执行的go二进制文件，则不会有事件，系统有缓存机制。
		watcherErr := service.watcher.Close()
		if watcherErr != nil {
			return
		}
		go service.initWatcher()
	}
}

func (service *Service) StopInstance(pid string) error {
	// 停止指定进程
	cmd := exec.Command("kill", "-TERM", pid)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
