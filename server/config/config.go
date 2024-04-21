package config

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type ServiceData struct {
	Name              string   `yaml:"name"`
	ServerName        string   `yaml:"server_name"`
	ServerIp          string   `yaml:"server_ip"`
	Port              int      `yaml:"port"`
	StartInstancePort int      `yaml:"start_instance_port"`
	InstanceCount     int      `yaml:"instance_count"`
	ExecutablePath    string   `yaml:"executable_path"`
	AutoRestart       bool     `yaml:"auto_restart"`
	DelayRunningTime  int      `yaml:"delay_running_time"` //启动后等几秒进入可服务状态
	DelayUpdateTime   int      `yaml:"delay_update_time"`  //有文件更新后等几秒开始重启实例
	WatchFiles        []string `yaml:"watch_files"`
}

type SmoothServeConfig struct {
	CommandPort int
	ProxyAddr   string

	SubConfigDir string
}

var ServicesDataMap map[string]ServiceData = make(map[string]ServiceData)
var ConfigData SmoothServeConfig

func LoadConfig(configPath string) {
	viper.SetConfigFile(configPath)
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("Read config file got server_error:"+configPath+". ", err)
		return
	}

	err = viper.Unmarshal(&ConfigData)
	if err != nil {
		return
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		fmt.Println("config file changed,reload it.")
		err := viper.ReadInConfig()
		if err != nil {
			return
		}
		err = viper.Unmarshal(&ConfigData)
		if err != nil {
			return
		}
	})
}

func LoadServerMap(configDir string) {
	//读取主配置文件指定的服务配置文件，为了不容易出错，这里一个配置文件里只写一个服务
	files, err := filepath.Glob(filepath.Join(configDir, "*.yaml"))
	if err != nil {
		fmt.Printf("Failed to read directory: %s\n", err)
		return
	}

	for _, file := range files {
		// 读取配置文件
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var serviceData ServiceData
		err = yaml.Unmarshal(data, &serviceData)
		if err != nil {
			fmt.Println("Parse serviceData config failed, skip  config file :", file)
			fmt.Println(err)
			continue
		}

		if serviceData.Port == 0 || serviceData.ServerName == "" || serviceData.InstanceCount == 0 {
			fmt.Println("Parse serviceData config failed, skip config file :", file)
			fmt.Println(err)
			continue
		}

		if serviceData.ServerIp == "" {
			serviceData.ServerIp = "127.0.0.1"
		}
		ServicesDataMap[serviceData.Name] = serviceData

	}
}
