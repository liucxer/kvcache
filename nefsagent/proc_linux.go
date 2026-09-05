//go:build linux

package nefsagent

import (
	"os/exec"
	"syscall"
)

// configureCmd 配置命令进程属性（Linux：Setpgid 便于整组 kill）。
func configureCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup 按进程组 kill 整个命令树。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
