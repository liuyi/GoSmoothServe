package service

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"path/filepath"
	"server/config"
	"strconv"
	"sync"
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
	Instances     []ServiceInstance
	instanceIndex int
	mutex         sync.Mutex
}

func New(serviceData config.ServiceData) *Service {
	service := Service{Name: serviceData.Name, Data: serviceData}
	return &service
}
func (service *Service) Start() {
	// 创建反向代理服务器
	// 启动 HTTP 服务器并监听指定端口
	go service.startAllInstance()

	http.HandleFunc("/", service.handleRequest)
	fmt.Printf("Reverse Proxy Server for Service %s started on port %d\n", service.Name, service.Data.Port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", service.Data.Port), nil)
	if err != nil {
		fmt.Printf("Failed to start Reverse Proxy Server for Service %s: %s\n", service.Name, err)
	}

}
func (service *Service) startAllInstance() {
	// 根据配置启动服务实例，并添加到 ServicesMap 中
	if service.Instances == nil {
		instances := make([]ServiceInstance, service.Data.InstanceCount)
		service.Instances = instances
	} else {
		//如果已经存在，就不要继续了
		fmt.Println("Service Started already!")
		return
	}

	fmt.Println("InstanceCount:", service.Data.InstanceCount)
	for i := 0; i < service.Data.InstanceCount; i++ {
		port := service.Data.StartInstancePort + i
		// 启动服务实例
		pid, err := service.StartInstance(service.Data.Name, port, service.Data.ExecutablePath) //启动了一个web实例以后 被堵塞 无法继续，应该如何解决
		if err != nil {
			fmt.Println("Failed to start service instance:", err)
			continue
		}
		instance := ServiceInstance{Pid: pid, Port: port, Status: 1}
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
		instance := &service.Instances[service.instanceIndex]
		service.instanceIndex = (service.instanceIndex + 1) % instanceCount // 更新索引，实现轮询
		if instance.Status == 1 {
			fmt.Println("selected index", service.instanceIndex, "instanceCount:", instanceCount)
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
	fmt.Println("cmd:", cmd)
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

	fmt.Println("Start service on pid:", pid)

	// 等待进程退出
	go func() {
		if err := cmd.Wait(); err != nil {

			fmt.Println("Process exited with error:", err)
		}
		fmt.Println("wait cmd finish")
	}()

	fmt.Println("return cmd pid:", pid)
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

	// 构建代理地址
	proxyURL := fmt.Sprintf("127.0.0.1:%d", instance.Port)

	//fmt.Println("proxy instance.Port:", instance.Port)
	fmt.Println("proxy url:", proxyURL)
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
