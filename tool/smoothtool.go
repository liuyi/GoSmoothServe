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
	"smoothserver/config"
	"smoothserver/quicktool"
	"strconv"
	"strings"
)

var (
	configPath      string = ""
	force           bool
	start           string
	stop            string
	smoothServeName string = "smoothserve"
	smoothServePath string = "./smoothserve"
)

func main() {
	//todo 修改成统一的ubuntu参数
	fmt.Println("Start SmoothServe Tool...")
	wordDirectory, _ := os.Getwd()
	fmt.Println("GoSmoothServe tool work directory:", wordDirectory)
	flag.StringVar(&start, "start", "", "-start service_name #启动某一个服务, 如果为all的话，启动全部")
	flag.StringVar(&stop, "stop", "", "-stop service_name #停止某一个服务, 如果为all的话，停止全部")
	flag.BoolVar(&force, "force", false, "-force 强制执行停止时使用，会直接杀死进程 -stop all -force true")
	flag.StringVar(&configPath, "config", "./smoothserve.yaml", "smoothserve的配置文件，一般不要设置")

	flag.Parse()

	fmt.Println("cmd start", start, "stop", stop)

	config.LoadConfig(configPath)

	if len(start) > 0 && len(stop) > 0 {
		fmt.Println("The two command can only set one of them.")
		os.Exit(1)
	}

	if len(start) > 0 {
		if start != "all" {
			startService(start)
		} else {
			startServe()
		}
		return
	}

	if len(stop) > 0 {
		if stop != "all" {
			stopService(stop)
		} else {
			stopServe(force)
		}
		return
	}

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
				fmt.Println("found pid line", fields)
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

func stopServe(force bool) {

	processes, err := quicktool.GetProcessInfo(smoothServeName)
	//
	if err != nil {
		fmt.Println("Find process pid failed,error:", err)
		return
	}
	for _, psInfo := range processes {

		fmt.Println("ps PID:", psInfo.PID, "program", psInfo.ExecutablePath, "args:", psInfo.Arguments)
	}
	// 如果找不到进程，则直接返回
	psInfo := processes[0]
	pid := psInfo.PID

	var cmd *exec.Cmd
	// 使用 kill 命令终止进程
	if force {
		fmt.Println("Kill process immediately!")
		cmd = exec.Command("kill", strconv.Itoa(pid), "-9")
	} else {
		cmd = exec.Command("kill", strconv.Itoa(pid), "-TERM")

	}
	err = cmd.Run()
	if err != nil {
		// 检查是否是 exec.ExitError 类型
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 获取进程的退出状态
			exitCode := exitErr.ExitCode()

			fmt.Println("wait exit,code:", exitCode, " status:", exitErr.ProcessState)
			fmt.Println("If smoothserve is start as service, stop it use command: sudo systemctl stop smoothserve")
			return
		} else {
			fmt.Println("kill smoothserve failed:", err)
		}

	}

	// 等待进程完全退出
	err = cmd.Wait()
	if err != nil {
		fmt.Println("Wating smoothserve  exit, got error: ", err)
		return
	}

	fmt.Println("GoSmoothServe service stopped safety.")
}

func startService(serviceName string) {
	formData := url.Values{}
	formData.Set("action", "start")
	formData.Set("service_name", serviceName)

	post(formData)
}

func stopService(serviceName string) {

	fmt.Println("stop service ", serviceName)
	formData := url.Values{}
	formData.Set("action", "stop")
	formData.Set("service_name", serviceName)

	_, err := post(formData)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("stop service done")

}

func restartService(serviceName string) {
	formData := url.Values{}
	formData.Set("action", "restart")
	formData.Set("service_name", serviceName)

	post(formData)
}

func post(data url.Values) (string, error) {
	url := fmt.Sprintf("http://%s:%d", config.ConfigData.ProxyAddr, config.ConfigData.CommandPort)
	fmt.Println("url:", url)
	// 构建POST请求
	resp, err := http.PostForm(url, data)
	if err != nil {
		fmt.Println(err)
		return "", fmt.Errorf("failed to send POST request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("close body ", err)
		}
	}(resp.Body) // 确保响应体关闭

	// 检查响应状态码
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response body: %v", err)
		}
		str := string(body)
		fmt.Println("resp:", str)
		return str, nil
	}

	fmt.Println(resp.Status)
	// 如果状态码不是2xx，返回错误
	return "", fmt.Errorf("received non-2xx status code: %v", resp.StatusCode)
}
