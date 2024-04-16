package main

import (
	"fmt"
	"os"
	"smoothserver/config"
	"smoothserver/service"
)

var configPath = "./smoothserve.yaml"

var ServicesMap map[string]*service.Service = make(map[string]*service.Service)

//var proxyMap map[string]*reverse_proxy.ReverseProxy = make(map[string]*reverse_proxy.ReverseProxy)

func main() {

	wordDirectory, _ := os.Getwd()
	fmt.Println("Start SmoothServe service,work directory:", wordDirectory)
	fmt.Println("Load config", configPath)

	config.LoadConfig(configPath)
	config.LoadServerMap(config.ConfigData.SubConfigDir)

	//// 打印加载的服务配置
	fmt.Println("Loaded services:")
	for name, serviceData := range config.ServicesDataMap {
		fmt.Printf("ServiceData Name: %s, Port: %d\n", name, serviceData.Port)
		// 在这里启动服务实例
		go createProxy(serviceData)
	}

	// 阻塞主 goroutine
	<-make(chan struct{})
}

func createProxy(serviceData config.ServiceData) {
	// 启动反向代理服务器
	srv := service.New(serviceData)
	ServicesMap[serviceData.Name] = srv
	srv.Start()
}
