**# GoSmoothServe

### 介绍
GoSmoothServe 是一个用于为单台服务器上Web服务提供无缝热更的反向代理服务，可以平滑重启和停止，不丢失请求。

+ 它可以实现在更新服务时无需重启，同时保持现有连接的连续性，从而确保用户无感知地享受到服务的稳定和持续性。
+ 通过 GoSmoothServe，您可以轻松管理和扩展您的 Web 服务，提供高效的负载均衡和无缝的更新体验。

+ 如果没有特别的需要，可以直接使用GoSmoothServe代替nginx 的web代理服务。
  > 不过如果有防火墙需求，还是建议先用nginx在前面，然后将请求传递到GoSmoothServe。GoSmoothServe仅关注无缝热更和平滑停止和重启。
### 用法

##### 启动GoSmoothServe反向代理服务
 启动反向代理服务将会读取./service下的所有配置，并启动所有服务
````shell
./smoothtool start
````
##### 停止GoSmoothServe反向代理服务
将会停止所有的服务，但是会等待所有链接全部完成后才会完全中断服务
````shell
./smoothtool stop
````
#### 停止正在运行的web服务
将会停止配置文件里名字为example_service_name的服务，不再接受新的请求，但是会等待所有链接全部完成后才会完全中断服务
````shell
./smoothtool stop example_service_name
````

#### 重启正在运行的web服务
将会重启配置文件里名字为example_service_name的服务，同时可以接受新的请求，会对该服务的多个实例依次进行重启，进来的请求会分发给正在工作的实例，不会有请求丢失和中断，有请求正在处理的实例不会被分配新的请求，当所有请求被处理完后会被重启。
````shell
./smoothtool stop example_service_name
````

### 以系统服务的方式随系统运行
在bin目录下运行下面代码将自动将smoothserve安装为系统服务，随系统自动启动 
````shell
sudo bash install_smoothserve.sh
````
### 文件夹结构
#### 结构列表：
- bin 
    - smooth_tool_linux
    - smooth_serve_linux
    - smoothserve.yaml
    - services
       + example.com.yaml

#### 文件说明:
 + bin : 是可执行文件的根目录，可以放在任意位置
 + smooth_serve_linux:在后台运行，接受来自用户或nginx代理传递过来的http请求并反向代理给各被代理的服务实例。
 + smooth_tool_linux: 对反向代理服务进行控制的命令行工具
 + smoothserve.yaml: 反向代理的配置文件(类似nginx的nginx.conf)
 + services 放置被代理的服务配置文件(类似nginx的vhost)

### 配置文件
#### smoothserve.yaml
````yaml
CommandPort: 8080 #smooth_tool_linux给反向代理服务发送命令的端口
ProxyAddr: "127.0.0.1" #反向代理服务的ip
ProxyPort: 8085 #nginx 服务器的请求传递过来的端口 如果不使用nginx，则可以设置为80端口。
SubConfigDir: ./services
````

#### services/服务配置.yaml
````yaml
name: service_name
server_name: "service_domain , service2_domain"
server_ip:  127.0.0.1 #实例的ip地址
port: 8085 #反向代理请求的端口
start_instance_port: 8086 #该服务的多个实例的开始端口，如有3个实例，则依次为8086,8087,8088
instance_count: 3 #在这台服务器上启动几个该服务的实例
executable_path: your/web/service/bin/file #服务的入口文件
auto_restart: true #是否监听文件的修改变化，如果是则在本配置文件和watch_files里配置的文件或文件夹有修改时会自动无缝重启
delay_running_time: 3 #重启服务的实例时，延后几秒再重启下一个实例，这里建议根据自己的服务启动后要多久可以正常工作来设置
watch_files: #自动重启时，监听的文件和文件夹列表
  - ./your/view/folder
  - ./your/web/files
  - ./files/in/your/web/service/folder

````
        
### 被代理的Web服务注意事项
+ 为了实现多个实例同时运行，需要在启动的时候读取命令行参数port，动态设置http请求的端口号
  可以使用flag标准库来实现:
  ````go
    var servicePort int =0
    flag.IntVar(&servicePort, "port", 8081, "启动服务的端口")
  ````**
 
