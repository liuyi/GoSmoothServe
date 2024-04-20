package quicktool

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Process struct {
	User           string
	PID            int
	CPU            string
	Memory         string
	VSZ            string
	RSS            string
	TTY            string
	Stat           string
	Start          string
	Time           string
	Command        string
	ExecutablePath string
	Arguments      []string
	FileName       string
}

func GetProcessInfo(processFileName string) ([]Process, error) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error running ps aux command: %v", err)
	}
	lines := strings.Split(string(output), "\n")
	var processes []Process

	for _, line := range lines[1:] { // skip header line
		fields := strings.Fields(line)
		if len(fields) >= 11 {
			command := fields[10]
			executablePath, arguments := parseCommand(command)
			FileName := filepath.Base(executablePath)
			pid, _ := strconv.Atoi(fields[1])
			p := Process{
				User:           fields[0],
				PID:            pid,
				CPU:            fields[2],
				Memory:         fields[3],
				VSZ:            fields[4],
				RSS:            fields[5],
				TTY:            fields[6],
				Stat:           fields[7],
				Start:          fields[8],
				Time:           fields[9],
				Command:        command,
				ExecutablePath: executablePath,
				Arguments:      arguments,
				FileName:       FileName,
			}

			//这里请根据 processFileName 提供的进程的二进制文件名，来将符合要求的进行信息加入切片
			//规则 找到executablePath里的文件名，如果名字完全相等则符合要求
			// 检查可执行文件路径的基本名称是否与提供的进程文件名完全匹配

			if FileName == processFileName {
				processes = append(processes, p)
			}

		}
	}

	return processes, nil
}

func parseCommand(command string) (string, []string) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}
