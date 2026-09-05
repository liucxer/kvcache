//go:build !linux

package nefsagent

import "os/exec"

// configureCmd 非 Linux 平台无需特殊配置。
func configureCmd(cmd *exec.Cmd) {}

// killProcessGroup 非 Linux 平台只 kill 主进程。
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
