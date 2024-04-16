# GoSmoothServe

### Introduction
GoSmoothServe is a reverse proxy service designed to seamlessly hot-reload web services on a single server, enabling smooth restarts and shutdowns without losing requests.

+ It facilitates updating services without restarts, maintaining continuity of existing connections to ensure users enjoy stable and uninterrupted service seamlessly.
+ With GoSmoothServe, managing and scaling your web services becomes effortless, offering efficient load balancing and a seamless updating experience.

+ If there are no specific requirements, GoSmoothServe can directly replace nginx as a web proxy service.
  > However, if firewall requirements exist, it is recommended to use nginx in front and then pass requests to GoSmoothServe. GoSmoothServe focuses solely on seamless hot-reloading and smooth stopping and restarting.

### Usage

##### Start the GoSmoothServe reverse proxy service
Starting the reverse proxy service will read all configurations under ./service and start all services.
````shell
./smoothtool start
````

##### Stop the GoSmoothServe reverse proxy service
This will stop all services, but it will wait until all connections are completed before completely interrupting the service.
````shell
./smoothtool stop
````

##### Stop a running web service
This will stop the service named example_service_name specified in the configuration file, no longer accepting new requests. However, it will wait until all connections are completed before completely interrupting the service.
````shell
./smoothtool stop example_service_name
````

##### Restart a running web service
This will restart the service named example_service_name specified in the configuration file, allowing new requests. It will restart multiple instances of the service one by one. Incoming requests will be distributed to the working instances, ensuring no request loss or interruption. Instances processing requests will not be assigned new requests. The service will be restarted once all requests are processed.
````shell
./smoothtool stop example_service_name
````

### Run as a system service
Running the following code under the bin directory will automatically install smoothserve as a system service, starting with the system.
````shell
sudo bash install_smoothserve.sh
````

### Folder Structure

#### Structure List:
- bin
    - smooth_tool_linux
    - smooth_serve_linux
    - smoothserve.yaml
    - services
        + example.com.yaml

#### File Descriptions:
+ bin: Root directory of executable files, can be placed anywhere.
+ smooth_serve_linux: Runs in the background, accepts HTTP requests from users or passed through nginx proxy, and reverse proxies to various proxied service instances.
+ smooth_tool_linux: Command-line tool to control the reverse proxy service.
+ smoothserve.yaml: Configuration file for reverse proxy (similar to nginx.conf).
+ services: Contains configuration files for proxied services (similar to nginx vhost).

### Configuration Files

#### smoothserve.yaml
````yaml
CommandPort: 8080 # Port for smooth_tool_linux to send commands to the reverse proxy service
ProxyAddr: "127.0.0.1" # IP address of the reverse proxy service
ProxyPort: 8085 # Port where requests from the nginx server are forwarded. If not using nginx, it can be set to port 80.
SubConfigDir: ./services
````

#### services/service_config.yaml
````yaml
name: service_name
server_name: "service_domain, service2_domain"
server_ip:  127.0.0.1 # IP address of the instance
port: 8085 # Port for reverse proxy requests
start_instance_port: 8086 # Starting port for multiple instances of this service. If there are 3 instances, they would be 8086, 8087, 8088.
instance_count: 3 # Number of instances of this service to start on this server
executable_path: your/web/service/bin/file # Entry file of the service
auto_restart: true # Whether to listen for file changes. If true, the service will automatically hot-reload when changes occur in this configuration file or in the files or directories listed in watch_files.
delay_running_time: 3 # Delay in seconds before restarting each instance of the service. It's recommended to set this based on how long it takes for your service to become operational after startup.
watch_files: # List of files and directories to monitor for automatic restarts.
  - ./your/view/folder
  - ./your/web/files
  - ./files/in/your/web/service/folder
````
