package main

import (
	"flag"
	"fmt"
	"go.uber.org/zap"
	"go_service_core/core/log"
	"net/http"
	"os"
	"os/signal"
	"smoothserver/config"
	"smoothserver/service"
	"syscall"
)

var configPath = "./smoothserve.yaml"

var ServicesMap map[string]*service.Service = make(map[string]*service.Service)

var debug bool = false

//var proxyMap map[string]*reverse_proxy.ReverseProxy = make(map[string]*reverse_proxy.ReverseProxy)

func main() {

	flag.BoolVar(&debug, "debug", false, "debug = false")
	workDirectory, _ := os.Getwd()

	/**
	  file_path: ./log/app.log
	  max_size: 100
	  max_backups: 10
	  max_age: 100
	  compress: true
	  debug: true
	*/

	config.LoadConfig(configPath)

	log.Init(config.ConfigData.Log)
	log.Info("Start SmoothServe service,work directory:", zap.String("word_directory", workDirectory))
	log.Info("Load config", zap.String("config", configPath))

	config.LoadServerMap(config.ConfigData.SubConfigDir)

	//// 打印加载的服务配置

	go listenCommand()
	go handleSysSig()
	//默认启动时直接启动所有服务
	createAnStartAllService()

	// 阻塞主 goroutine
	<-make(chan struct{})
}
func createAnStartAllService() {
	if config.ServicesDataMap == nil {
		log.Info("Should create config for services, before start it")
		return
	}
	for _, serviceData := range config.ServicesDataMap {
		//fmt.Printf("Servcie %s will start, and listen at port: %d\n", name, serviceData.Port)
		// 在这里启动服务实例
		go createProxy(serviceData)
	}
}
func createProxy(serviceData config.ServiceData) {
	// 启动反向代理服务器
	srv := service.New(serviceData)
	ServicesMap[serviceData.Name] = srv
	srv.CreateAndListen()

	go srv.Start()
}

func startAllService() {
	for name := range ServicesMap {
		srv := ServicesMap[name]
		srv.Start()
	}
}

func restartAllService() {
	for name := range ServicesMap {
		srv := ServicesMap[name]
		srv.RestartOneByOne()
	}
}

func listenCommand() {

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {

		action := string(request.PostFormValue("action"))
		serviceName := string(request.PostFormValue("service_name"))

		if action == "stop" {

			if serviceName != "" {
				mService := ServicesMap[serviceName]
				if mService == nil {
					_, err := writer.Write([]byte("Can't find the service:" + serviceName))
					if err != nil {
						return
					}
				} else {
					_, err := writer.Write([]byte(fmt.Sprintf("service %s will be stop  ", serviceName)))
					if err != nil {
						return
					}
					mService.Stop()
					_, err = writer.Write([]byte(fmt.Sprintf("service  %s stopped safety. ", serviceName)))
					if err != nil {
						return
					}
				}
			} else {
				exitServe()
			}
			return
		}

		if action == "start" {

			if serviceName != "" {
				mService := ServicesMap[serviceName]
				if mService == nil {
					_, err := writer.Write([]byte("Can't find the service:" + serviceName))
					if err != nil {
						return
					}
				} else {
					mService.Start()
					_, err := writer.Write([]byte(fmt.Sprintf("service  %s  all instance started. ", serviceName)))
					if err != nil {
						return
					}
				}
			} else {
				//todo start all
				//一般不要这样干 启动代理服务器的时候会自动启动所有的服务
				startAllService()
			}

			return
		}

		if action == "restart" {
			if serviceName != "" {
				mService := ServicesMap[serviceName]
				if mService == nil {
					_, err := writer.Write([]byte("Can't find the service:" + serviceName))
					if err != nil {
						return
					}
				} else {
					mService.RestartOneByOne()
					_, err := writer.Write([]byte(fmt.Sprintf("service  %s  restart. ", serviceName)))
					if err != nil {
						return
					}
				}
			} else {
				//todo start all
				//一般不要这样干 重启代理服务器就行了
				_, err := writer.Write([]byte(fmt.Sprintf("do restart all service.")))
				if err != nil {
					return
				}
				restartAllService()

			}

			return
		}
	})
	address := fmt.Sprintf("%s:%d", config.ConfigData.ProxyAddr, config.ConfigData.CommandPort)
	err := http.ListenAndServe(address, nil)
	if err != nil {
		return
	}
}

func handleSysSig() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	log.Info("smoothserve 开始侦听系统信号")
	go func() {
		for {

			sig := <-sigCh

			switch sig {
			case syscall.SIGTERM:
				exitServe()
			default:
				log.Info("收到来自系统信号:", zap.String("sig", sig.String()))
			}

		}
	}()
}

func exitServe() {
	log.Info("smoothserve will exit")
	for _, mService := range ServicesMap {
		log.Info("stopping service", zap.String("name", mService.Name))
		mService.Stop()
		log.Info("service stopped", zap.String("name", mService.Name))
	}
	log.Info("All service are stopped, exit serve")
	os.Exit(0)
}
