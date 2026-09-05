//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"kvcache/nefsagent"
)

// detachedProcAttr Windows 下用 DETACHED_PROCESS 让 daemon 脱离父进程终端。
func detachedProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}

// killPID 按 PID 强杀进程（taskkill 比 os.Process.Kill 更可靠，避免句柄 Access denied）
func killPID(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill %d: %v (%s)", pid, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// findIPCListener 通过 netstat 查找监听 IPC 端口的进程 PID。
// 不依赖可能过时的 PID 文件。
func findIPCListener() int {
	out, err := exec.Command("netstat", "-ano").CombinedOutput()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, nefsagent.IPCAddr) {
			continue
		}
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			if pid, err := strconv.Atoi(fields[len(fields)-1]); err == nil {
				return pid
			}
		}
	}
	return 0
}
