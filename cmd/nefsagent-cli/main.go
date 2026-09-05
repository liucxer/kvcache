// nefsagent-cli: 高性能长连接 RPC 客户端 CLI，替代旧 nefsproxy.exe
//
// 用法：
//
//	nefsagent-cli exec <cmd> [--cwd <dir>] [--timeout <sec>] [--no-daemon]
//	nefsagent-cli upload <local> <remote>
//	nefsagent-cli download <remote> <local>
//	nefsagent-cli ping [--no-daemon]
//	nefsagent-cli daemon start|stop|status
//
// 全局选项：
//
//	--addr       服务端地址（默认 100.71.128.12:9528）
//	--token      鉴权 token（默认 nefsagent）
//	--no-daemon  强制直连不走 daemon
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kvcache/nefsagent"
)

const (
	defaultAddr  = "100.71.128.12:9528"
	defaultToken = "nefsagent"
)

// cliAddr/cliToken 全局，供 daemon.go 使用
var (
	cliAddr  = defaultAddr
	cliToken = defaultToken
)

func main() {
	// 全局 flag
	addr := flag.String("addr", defaultAddr, "服务端地址")
	token := flag.String("token", defaultToken, "鉴权 token")
	flag.Parse()
	cliAddr = *addr
	cliToken = *token

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(2)
	}

	ctx := context.Background()
	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "daemon":
		runDaemonCmd(cmdArgs)
	case "exec":
		runExec(ctx, cmdArgs)
	case "upload":
		// upload 始终直连（走 nefsagent 长连接，daemon 不转发文件传输）
		c := mustClient()
		defer c.Close()
		runUpload(ctx, c, cmdArgs)
	case "download":
		c := mustClient()
		defer c.Close()
		runDownload(ctx, c, cmdArgs)
	case "ping", "health":
		runPing(ctx, cmdArgs)
	case "-h", "--help", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `nefsagent-cli - 高性能远程命令执行工具 (基于 nefsagent 长连接协议)

用法:
  nefsagent-cli [全局选项] <命令> [参数]

全局选项:
  --addr       服务端地址 (默认 %s)
  --token      鉴权 token (默认 %s)
  --no-daemon  强制直连不走 daemon

命令:
  exec <cmd> [--cwd <dir>] [--timeout <sec>] [--no-daemon]
      在远程执行命令，stdout/stderr 直写本地，exit code 透传
      默认走 daemon 模式（自动拉起，复用长连接池）

  upload <local> <remote>
      上传本地文件到远程（直连）

  download <remote> <local>
      下载远程文件到本地（直连）

  ping / health [--no-daemon]
      健康检查

  daemon start
      启动常驻进程 nefsdemo（Web UI + CLI 长连接池）

  daemon stop
      停止常驻进程

  daemon status
      查看常驻进程状态

  daemon install
      注册开机自启（Windows 注册表 Run）

  daemon uninstall
      移除开机自启

示例:
  nefsagent-cli exec "uname -a"
  nefsagent-cli exec "ls -la /root" --timeout 10
  nefsagent-cli exec "cat /var/log/messages | grep error"
  nefsagent-cli upload .\build\kvcache /root/kvcache
  nefsagent-cli download /root/nefsagent.log .\local.log
  nefsagent-cli ping
  nefsagent-cli daemon status
`, defaultAddr, defaultToken)
}

// mustClient 创建直连客户端（upload/download 用）
func mustClient() *nefsagent.Client {
	c, err := nefsagent.NewClient(cliAddr, cliToken, nefsagent.WithPoolSize(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect %s: %v\n", cliAddr, err)
		os.Exit(1)
	}
	return c
}

// runExec 执行命令（优先走 daemon）
func runExec(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	cwd := fs.String("cwd", "", "远程工作目录")
	timeout := fs.Float64("timeout", 0, "超时秒数 (0=默认)")
	noDaemon := fs.Bool("no-daemon", false, "强制直连不走 daemon")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	// flag 包遇到第一个非 flag 参数会停止解析，
	// 手动扫描 rest 中残留的 flags，支持 `exec "cmd" --timeout 3` 写法
	rest := fs.Args()
	var cmdParts []string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--timeout":
			if i+1 < len(rest) {
				if v, err := strconv.ParseFloat(rest[i+1], 64); err == nil {
					*timeout = v
				}
				i++
			}
		case "--cwd":
			if i+1 < len(rest) {
				*cwd = rest[i+1]
				i++
			}
		case "--no-daemon":
			*noDaemon = true
		default:
			cmdParts = append(cmdParts, rest[i])
		}
	}
	cmd := strings.Join(cmdParts, " ")
	if cmd == "" {
		fmt.Fprintln(os.Stderr, "exec: 缺少命令参数")
		os.Exit(2)
	}

	// daemon 模式（默认）
	if !*noDaemon {
		resp, err := execViaDaemon(cmd, *cwd, *timeout)
		if err == nil {
			outputExecResp(resp)
			return
		}
		// daemon 失败回退直连
		fmt.Fprintf(os.Stderr, "[daemon fallback: %v]\n", err)
	}

	// 直连模式
	c := mustClient()
	defer c.Close()
	var to time.Duration
	if *timeout > 0 {
		to = time.Duration(*timeout * float64(time.Second))
	}
	resp, err := c.Exec(ctx, cmd, to, *cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exec error: %v\n", err)
		os.Exit(1)
	}
	outputExecResp(resp)
}

// execViaDaemon 通过 daemon 执行
func execViaDaemon(cmd, cwd string, timeout float64) (*nefsagent.ExecResp, error) {
	if err := ensureDaemon(); err != nil {
		return nil, err
	}
	resp, err := callDaemon(&nefsagent.IPCRequest{
		Op: "exec", Cmd: cmd, Cwd: cwd, Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf(resp.Error)
	}
	return &nefsagent.ExecResp{
		ExitCode: resp.ExitCode, TimedOut: resp.TimedOut,
		Stdout: resp.Stdout, Stderr: resp.Stderr,
	}, nil
}

// outputExecResp 输出 exec 响应（直写 stdout/stderr，透传 exit code）
func outputExecResp(resp *nefsagent.ExecResp) {
	if resp.Stdout != "" {
		io.WriteString(os.Stdout, resp.Stdout)
	}
	if resp.Stderr != "" {
		io.WriteString(os.Stderr, resp.Stderr)
	}
	if resp.TimedOut {
		fmt.Fprintln(os.Stderr, "[nefsagent-cli: timed out]")
		os.Exit(124)
	}
	os.Exit(resp.ExitCode)
}

func runUpload(ctx context.Context, c *nefsagent.Client, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: upload <local> <remote>")
		os.Exit(2)
	}
	local, remote := args[0], args[1]
	size, err := c.UploadFile(ctx, local, remote)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("uploaded %d bytes -> %s\n", size, remote)
}

func runDownload(ctx context.Context, c *nefsagent.Client, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: download <remote> <local>")
		os.Exit(2)
	}
	remote, local := args[0], args[1]
	if dir := filepath.Dir(local); dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}
	if err := c.DownloadFile(ctx, remote, local); err != nil {
		fmt.Fprintf(os.Stderr, "download error: %v\n", err)
		os.Exit(1)
	}
	info, _ := os.Stat(local)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	fmt.Printf("downloaded %d bytes <- %s\n", size, remote)
}

// runPing 健康检查（优先走 daemon）
func runPing(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	noDaemon := fs.Bool("no-daemon", false, "强制直连不走 daemon")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if !*noDaemon {
		if err := ensureDaemon(); err == nil {
			if resp, err := callDaemon(&nefsagent.IPCRequest{Op: "ping"}); err == nil && resp.OK {
				fmt.Println("pong")
				return
			}
		}
	}

	c := mustClient()
	defer c.Close()
	if err := c.Health(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("pong")
}

// runDaemonCmd daemon 子命令
func runDaemonCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: daemon start|stop|status|install|uninstall")
		os.Exit(2)
	}
	switch args[0] {
	case "start":
		if isDaemonRunning() {
			fmt.Println("daemon already running (web ui http://127.0.0.1:8080)")
			return
		}
		if err := startDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "start daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("daemon started (web ui http://127.0.0.1:8080)")
	case "stop":
		runDaemonStop()
	case "status":
		runDaemonStatus()
	case "install":
		runDaemonInstall()
	case "uninstall":
		runDaemonUninstall()
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon subcommand: %s\n", args[0])
		os.Exit(2)
	}
}
