// 本地 daemon IPC 协议：JSON over TCP（4B 长度前缀 + JSON payload）。
// 供 nefsagent-cli（客户端侧）与 nefsdemo（服务端侧）共享。
package nefsagent

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// IPCAddr 本地 daemon 监听地址
	IPCAddr = "127.0.0.1:19528"
	// IPCPIDFile PID 文件名（放 %TEMP% 下，跨用户隔离）
	IPCPIDFile = "nefsagent-daemon.pid"
)

// IPCRequest CLI → daemon 的 IPC 请求
type IPCRequest struct {
	Op      string  `json:"op"` // "exec" | "ping"
	Cmd     string  `json:"cmd,omitempty"`
	Cwd     string  `json:"cwd,omitempty"`
	Timeout float64 `json:"timeout,omitempty"`
}

// IPCResponse daemon → CLI 的 IPC 响应
type IPCResponse struct {
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
	TimedOut  bool   `json:"timed_out,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
}

// IPCPIDPath 返回 PID 文件路径
func IPCPIDPath() string {
	return filepath.Join(os.TempDir(), IPCPIDFile)
}

// WritePID 写 PID 文件
func WritePID() error {
	return os.WriteFile(IPCPIDPath(), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
}

// ReadPID 读 PID 文件
func ReadPID() (int, error) {
	data, err := os.ReadFile(IPCPIDPath())
	if err != nil {
		return 0, err
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)
	return pid, nil
}

// RemovePID 删除 PID 文件
func RemovePID() {
	os.Remove(IPCPIDPath())
}
