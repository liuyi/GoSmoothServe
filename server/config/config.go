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
	Port              int      `yaml:"port"`
	StartInstancePort int      `yaml:"start_instance_port"`
	InstanceCount     int      `yaml:"instance_count"`
	ExecutablePath    string   `yaml:"executable_path"`
	AutoRestart       bool     `yaml:"auto_restart"`
	DelayRunningTime  int      `yaml:"delay_running_time"` //启动后等几秒进入可服务状态
	WatchFiles        []string `yaml:"watch_files"`
}

type SmoothServeConfig struct {
	CommandPort  int
	ProxyAddr    string
	ProxyPort    int
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

	fmt.Println("smoothserve config:", ConfigData.CommandPort, ConfigData.SubConfigDir)

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
		var service ServiceData
		err = yaml.Unmarshal(data, &service)
		if err != nil {
			fmt.Println("Parse service config failed, skip  config file :", file)
			fmt.Println(err)
			continue
		}

		if service.Port == 0 || service.ServerName == "" || service.InstanceCount == 0 {
			fmt.Println("Parse service config failed, skip config file :", file)
			fmt.Println(err)
			continue
		}
		ServicesDataMap[service.Name] = service

	}
}
