// nefsagent-cli daemon 模式：统一管理 nefsdemo.exe 常驻进程。
// nefsdemo 同时提供 Web UI (8080) 和本地 IPC (19528)，
// CLI 通过本地 TCP IPC 复用其到 128.12 的长连接池，
// 消灭每次 CLI 调用的远程握手与进程启动开销。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"kvcache/nefsagent"
)

// isDaemonRunning 检查 daemon (nefsdemo) 是否在运行
func isDaemonRunning() bool {
	conn, err := net.DialTimeout("tcp", nefsagent.IPCAddr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// startDaemon 后台启动 nefsdemo.exe（与 nefsagent-cli.exe 同目录）
func startDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	demoPath := filepath.Join(filepath.Dir(exe), "nefsdemo.exe")
	if _, err := os.Stat(demoPath); err != nil {
		return fmt.Errorf("nefsdemo.exe 不在 nefsagent-cli.exe 同目录: %v", err)
	}
	cmd := exec.Command(demoPath, "--addr", cliAddr, "--token", cliToken)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Windows 下用 DETACHED_PROCESS，Linux 下用 Setpgid
	cmd.SysProcAttr = detachedProcAttr()
	if err := cmd.Start(); err != nil {
		return err
	}
	// 等待 daemon 就绪（最多 2s）
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if isDaemonRunning() {
			return nil
		}
	}
	return fmt.Errorf("daemon 启动超时")
}

// ensureDaemon 确保 daemon 在运行，没运行则拉起
func ensureDaemon() error {
	if isDaemonRunning() {
		return nil
	}
	return startDaemon()
}

// callDaemon 向 daemon 发请求
func callDaemon(req *nefsagent.IPCRequest) (*nefsagent.IPCResponse, error) {
	conn, err := net.DialTimeout("tcp", nefsagent.IPCAddr, 1*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// IPC deadline = 命令超时 + 缓冲；无显式超时给 2h（服务端默认 1h）
	deadline := 2 * time.Hour
	if req.Timeout > 0 {
		deadline = time.Duration(req.Timeout*float64(time.Second)) + 30*time.Second
	}
	conn.SetDeadline(time.Now().Add(deadline))

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// 简单协议：4B 长度 + JSON payload
	header := make([]byte, 4)
	header[0] = byte(len(data) >> 24)
	header[1] = byte(len(data) >> 16)
	header[2] = byte(len(data) >> 8)
	header[3] = byte(len(data))
	conn.Write(header)
	conn.Write(data)

	// 读响应
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	respLen := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	respData := make([]byte, respLen)
	if _, err := io.ReadFull(conn, respData); err != nil {
		return nil, err
	}
	var resp nefsagent.IPCResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// runDaemonStatus daemon 状态
func runDaemonStatus() {
	if isDaemonRunning() {
		pid, _ := nefsagent.ReadPID()
		fmt.Printf("daemon running on %s (pid=%d), web ui http://127.0.0.1:8080\n", nefsagent.IPCAddr, pid)
		os.Exit(0)
	}
	fmt.Println("daemon not running")
	os.Exit(1)
}

// runDaemonStop 停止 daemon
func runDaemonStop() {
	// 优先按 IPC 端口定位真实 PID（避免 PID 文件过时导致杀错/杀不掉）
	if pid := findIPCListener(); pid > 0 {
		if err := killPID(pid); err != nil {
			fmt.Fprintf(os.Stderr, "kill daemon: %v\n", err)
			os.Exit(1)
		}
		nefsagent.RemovePID()
		fmt.Println("daemon stopped")
		return
	}

	// 兜底：按 PID 文件
	pid, err := nefsagent.ReadPID()
	if err != nil {
		fmt.Println("daemon not running (no pid file)")
		os.Exit(0)
	}
	if err := killPID(pid); err != nil {
		fmt.Fprintf(os.Stderr, "kill daemon: %v\n", err)
		os.Exit(1)
	}
	nefsagent.RemovePID()
	fmt.Println("daemon stopped")
}

// runDaemonInstall 注册表 Run 键开机自启 nefsdemo.exe
func runDaemonInstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		os.Exit(1)
	}
	demoPath := filepath.Join(filepath.Dir(exe), "nefsdemo.exe")
	if _, err := os.Stat(demoPath); err != nil {
		fmt.Fprintf(os.Stderr, "nefsdemo.exe 不在 nefsagent-cli.exe 同目录: %v\n", err)
		os.Exit(1)
	}
	// 注册表 Run 键（HKCU，无需管理员），exec.Command 直传参数避免 shell 转义
	value := fmt.Sprintf(`"%s" --addr %s --token %s`, demoPath, cliAddr, cliToken)
	cmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "NefsDemo", "/t", "REG_SZ", "/d", value, "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "install failed: %v\n%s\n", err, out)
		os.Exit(1)
	}
	fmt.Printf("installed autostart: %s (addr=%s)\n", demoPath, cliAddr)
	fmt.Println("下次开机自动启动 nefsdemo (Web UI http://127.0.0.1:8080 + CLI IPC)")
}

// runDaemonUninstall 移除注册表自启
func runDaemonUninstall() {
	cmd := exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "NefsDemo", "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "uninstall failed: %v\n%s\n", err, out)
		os.Exit(1)
	}
	fmt.Println("autostart removed")
}
