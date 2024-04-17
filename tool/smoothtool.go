package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"server/config"
	"strconv"
	"strings"
)

var (
	configPath string = ""

	action          string
	start           string
	stop            string
	smoothServeName string = "smoothserve"
	smoothServePath string = "./smoothserve"
)

func main() {
	fmt.Println("Start SmoothServe Tool...")
	wordDirectory, _ := os.Getwd()
	fmt.Println("GoSmoothServe tool work directory:", wordDirectory)
	flag.StringVar(&start, "start", "", "启动GoSmoothServe")
	flag.StringVar(&stop, "stop", "", "停止GoSmoothServe")
	flag.StringVar(&action, "action", "", "执行动作")
	flag.StringVar(&configPath, "config", "./smoothserve.yaml", "smoothserve的配置文件，一般不要设置")

	flag.Parse()

	config.LoadConfig(configPath)

	startAction := hasArg("start")
	stopAction := hasArg("stop")

	if startAction && stopAction {
		fmt.Println("The two command can only set one of them.")
		os.Exit(1)
	}
	if startAction {
		if start != "" {
			startService(start)
		} else {
			startServe()
		}

		return
	}

	if stopAction {
		if stop != "" {
			stopService(stop)
		} else {
			stopServe()
		}

		return
	}

}
func hasArg(name string) bool {
	args := flag.Args()
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}

func getProcessPid(processName string) (int, error) {
	// 使用 ps 命令查找进程
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// 检查输出中是否包含指定的进程名
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, processName) {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				pidStr := fields[1]
				pid, err := strconv.Atoi(pidStr)
				if err != nil {
					return 0, fmt.Errorf("failed to parse PID")
				}
				return pid, nil
			}
		}
	}

	return 0, nil
}

func isProcessRunning(processName string) (bool, error) {
	// 使用 ps 命令查找进程
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	// 检查输出中是否包含指定的进程名
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, processName) {
			return true, nil
		}
	}

	return false, nil
}

func startServe() {

	runningPid, err := getProcessPid(smoothServeName)
	if err != nil {
		fmt.Println("Got error when start GoSmoothServe", err)
		return
	}

	if runningPid != 0 {
		fmt.Println("GoSmoothServe is runing already")
		return
	}

	//cmd := exec.Command(smoothServePath, "&")
	cmd := exec.Command("nohup", smoothServePath, "&")
	cmd.Dir = filepath.Dir(smoothServePath)
	//stdout, err := cmd.StdoutPipe()
	//// 启动命令
	if err = cmd.Start(); err != nil {
		fmt.Println("run cmd error:", cmd, err)
		return
	}
	//
	//Screen := bufio.NewScanner(stdout)
	//
	//go func() {
	//	for Screen.Scan() {
	//		fmt.Println(Screen.Text())
	//	}
	//}()

	//go func() {
	//if err := cmd.Wait(); err != nil {
	//
	//	fmt.Println("GoSmoothServe Process exited with error:", err)
	//}
	//}()
	// 获取进程 ID
	pid := fmt.Sprintf("%d", cmd.Process.Pid)
	fmt.Println("GoSmoothServe is running, pid is " + pid)
}

func stopServe() {
	pid, err := getProcessPid(smoothServeName)

	if err != nil {
		fmt.Println("Find process pid failed,error:", err)
		return
	}

	// 如果找不到进程，则直接返回
	if pid == 0 {
		fmt.Println("No process pid found, GoSmoothServe not running.")
		return
	}

	// 使用 kill 命令终止进程
	cmd := exec.Command("kill", strconv.Itoa(pid), "-TERM")
	err = cmd.Run()
	if err != nil {
		// 检查是否是 exec.ExitError 类型
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 获取进程的退出状态
			exitCode := exitErr.ExitCode()

			fmt.Println("exit code:", exitCode, " status:", exitErr.ProcessState.String())
		} else {
			fmt.Println("kill GoSmoothServe failed:", err)
		}

		return
	}

	// 等待进程完全退出
	//err = cmd.Wait()
	//if err != nil {
	//	fmt.Println("Wating GoSmoothServe  exit, got error: ", err)
	//	return
	//}

	fmt.Println("GoSmoothServe service stopped safety.")
}

func startService(serviceName string) {
	formData := url.Values{}
	formData.Set("action", "start")
	formData.Set("service_name", serviceName)

	post(formData)
}

func stopService(serviceName string) {
	formData := url.Values{}
	formData.Set("action", "stop")
	formData.Set("service_name", serviceName)

	post(formData)
}

func restartService(serviceName string) {
	formData := url.Values{}
	formData.Set("action", "restart")
	formData.Set("service_name", serviceName)

	post(formData)
}

func post(formData url.Values) (string, error) {
	url := fmt.Sprintf("http://", config.ConfigData.ProxyAddr, ":", config.ConfigData.CommandPort)
	response, err := http.PostForm(url, formData)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(response.Body)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
