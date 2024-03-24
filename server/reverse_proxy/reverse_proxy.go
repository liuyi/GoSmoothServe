package reverse_proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"server/config"
	"server/service"
	"sync"
)

type ReverseProxy struct {
	Name    string
	service *service.Service
	mutex   sync.Mutex
}

func New(serviceData config.ServiceData) *ReverseProxy {
	srv := service.New(serviceData)
	return &ReverseProxy{
		Name:    serviceData.Name,
		service: srv,
		mutex:   sync.Mutex{},
	}
}

func (proxy *ReverseProxy) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// 选择一个服务实例处理请求
	proxy.mutex.Lock()
	defer proxy.mutex.Unlock()
	instance := proxy.service.SelectInstance()
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

func (proxy *ReverseProxy) Start() {
	// 创建反向代理服务器
	// 启动 HTTP 服务器并监听指定端口
	srv := proxy.service
	go srv.Start()

	http.HandleFunc("/", proxy.HandleRequest)
	fmt.Printf("Reverse Proxy Server for Service %s started on port %d\n", srv.Name, srv.Data.Port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", srv.Data.Port), nil)
	if err != nil {
		fmt.Printf("Failed to start Reverse Proxy Server for Service %s: %s\n", srv.Name, err)
	}

}
