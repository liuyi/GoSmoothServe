package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"server/config"
	"strconv"
	"sync"
)

var configPath = "./smoothserve.yaml"

type ServiceInstance struct {
	Pid    string
	Port   int
	Status int
	Host   string
}

type Service struct {
	Name      string
	Data      config.ServiceData
	Instances []ServiceInstance
}

var ServicesMap map[string]Service = make(map[string]Service)

func main() {

	wordDirectory, _ := os.Getwd()
	fmt.Println("Start SmoothServe server,work directory:", wordDirectory)
	fmt.Println("Load config", configPath)

	config.LoadConfig(configPath)

	config.LoadServerMap(config.ConfigData.SubConfigDir)

	//// 打印加载的服务配置
	fmt.Println("Loaded services:")
	for name, service := range config.ServicesDataMap {
		fmt.Printf("ServiceData Name: %s, Port: %d\n", name, service.Port)
		// 在这里启动服务实例
		startService(name, service)
	}

	//test()

	// 阻塞主 goroutine
	<-make(chan struct{})
}

func test() {

	cmdPath := "/data/www/zyaps/build/zyaps_linux"
	cmd := exec.Command(cmdPath, "--port", "8086")
	cmd.Dir = filepath.Dir(cmdPath)

	// 获取命令的标准输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("无法获取标准输出管道:", err)
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		fmt.Println("无法启动命令:", err)
		return
	}

	// 使用 scanner 读取输出
	scanner := bufio.NewScanner(stdout)

	go func() {
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()
	go func() {
		// 检查命令是否执行完毕
		if err := cmd.Wait(); err != nil {
			fmt.Println("命令执行失败:", err)
			return
		}
	}()

	// 检查是否有错误
	if err := scanner.Err(); err != nil {
		fmt.Println("读取输出时发生错误:", err)
	}
}

func startService(name string, serviceData config.ServiceData) {
	// 根据配置启动服务实例，并添加到 ServicesMap 中
	instances := make([]ServiceInstance, serviceData.InstanceCount)

	fmt.Println("InstanceCount:", serviceData.InstanceCount)
	for i := 0; i < serviceData.InstanceCount; i++ {
		port := serviceData.StartInstancePort + i
		// 启动服务实例
		pid, err := startInstance(name, port, serviceData.ExecutablePath) //启动了一个web实例以后 被堵塞 无法继续，应该如何解决
		if err != nil {
			fmt.Println("Failed to start service instance:", err)
			continue
		}
		instance := ServiceInstance{Pid: pid, Port: port, Status: 1}
		instances[i] = instance
	}
	fmt.Println("InstanceCount2:", serviceData.InstanceCount, len(instances))
	service := Service{Name: name, Data: serviceData, Instances: instances}
	ServicesMap[name] = service

	// 启动反向代理服务器
	go startReverseProxyServer(service)
}
func startInstance(name string, port int, executablePath string) (string, error) {
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

func startReverseProxyServer(service Service) {
	// 创建反向代理服务器
	proxy := NewReverseProxy(service)

	// 启动 HTTP 服务器并监听指定端口
	http.HandleFunc("/", proxy.HandleRequest)
	fmt.Printf("Reverse Proxy Server for Service %s started on port %d\n", service.Name, service.Data.Port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", service.Data.Port), nil)
	if err != nil {
		fmt.Printf("Failed to start Reverse Proxy Server for Service %s: %s\n", service.Name, err)
	}
}

type ReverseProxy struct {
	service Service
	mutex   sync.Mutex
}

func NewReverseProxy(service Service) *ReverseProxy {
	return &ReverseProxy{
		service: service,
		mutex:   sync.Mutex{},
	}
}

func (rp *ReverseProxy) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// 选择一个服务实例处理请求
	rp.mutex.Lock()
	defer rp.mutex.Unlock()
	instance := rp.selectInstance()
	if instance == nil {
		http.Error(w, "No available instance", http.StatusServiceUnavailable)
		return
	}

	// 构建代理地址
	proxyURL := fmt.Sprintf("127.0.0.1:%d", instance.Port)

	//fmt.Println("proxy instance.Port:", instance.Port)
	fmt.Println("proxy url:", proxyURL)
	// 创建反向代理
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = proxyURL
			req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
			req.Host = instance.Host // 更新 Host，确保正确代理
		},
	}

	// 执行反向代理
	proxy.ServeHTTP(w, r)
}

var instanceIndex int = 0 // 使用 int 类型保存索引
func (rp *ReverseProxy) selectInstance() *ServiceInstance {
	// 选择一个可用的服务实例，使用轮询算法
	//rp.mutex.Lock()
	//defer rp.mutex.Unlock() // 在函数结尾处释放互斥锁

	instances := rp.service.Instances
	if len(instances) == 0 {
		return nil
	}

	instanceCount := len(instances)
	instanceIndex = (instanceIndex) % instanceCount // 更新索引，实现轮询

	for i := 0; i < instanceCount; i++ {
		instance := &instances[instanceIndex]
		instanceIndex = (instanceIndex + 1) % instanceCount // 更新索引，实现轮询
		if instance.Status == 1 {
			fmt.Println("selected index", instanceIndex, "instanceCount:", instanceCount)
			return instance
		}
	}

	return nil
}
