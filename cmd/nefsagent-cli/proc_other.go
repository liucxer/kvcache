//go:build !windows

package main

import "syscall"

// detachedProcAttr 非 Windows 平台用 Setpgid 让 daemon 成为新进程组组长。
func detachedProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// killPID 按 PID 杀进程
func killPID(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

// findIPCListener 非 Windows 平台不依赖 netstat，返回 0（走 PID 文件兜底）
func findIPCListener() int {
	return 0
}
