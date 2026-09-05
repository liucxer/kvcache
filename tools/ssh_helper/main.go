// SSH helper: connect to build machine, run commands, jump to runtime machines.
// Usage:
//
//	go run ./tools/ssh_helper probe
//	go run ./tools/ssh_helper run "ls -la ~/kvcache"
//	go run ./tools/ssh_helper jump 10.151.26.146 "ps aux | grep kvcache"
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	buildHost = "100.71.128.13"
	buildPort = 2222
	buildUser = "root"
	buildPass = "q@@&&4806608"
)

func connectBuild() (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: buildUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(buildPass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", buildHost, buildPort)
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func run(client *ssh.Client, cmd string, timeout time.Duration) (int, string, error) {
	session, err := client.NewSession()
	if err != nil {
		return -1, "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return -1, "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return -1, "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		return -1, "", fmt.Errorf("start cmd: %w", err)
	}

	outBytes, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)

	done := make(chan error, 1)
	go func() { done <- session.Wait() }()

	select {
	case err := <-done:
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				exitCode = exitErr.ExitStatus()
			} else {
				return -1, string(outBytes), err
			}
		}
		combined := string(outBytes)
		if len(errBytes) > 0 {
			combined += "\n[STDERR]\n" + string(errBytes)
		}
		return exitCode, combined, nil
	case <-time.After(timeout):
		session.Signal(ssh.SIGKILL)
		return -1, string(outBytes) + "\n[TIMEOUT]", fmt.Errorf("command timed out after %v", timeout)
	}
}

func main() {
	action := "probe"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	client, err := connectBuild()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Connect failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	switch action {
	case "probe":
		cmd := `echo === build host ===; hostname; uname -a; echo === go ===; go version 2>/dev/null || echo "no go"; echo === home ===; ls -la ~; echo === kvcache ===; ls -la ~/kvcache 2>/dev/null || find / -maxdepth 4 -name kvcache -type d 2>/dev/null | head -5`
		rc, out, err := run(client, cmd, 30*time.Second)
		fmt.Printf("RC: %d\n%s\n", rc, out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: run <cmd>")
			os.Exit(1)
		}
		cmd := strings.Join(os.Args[2:], " ")
		rc, out, err := run(client, cmd, 120*time.Second)
		fmt.Printf("RC: %d\n%s\n", rc, out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

	case "jump":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: jump <runtime_ip> <cmd>")
			os.Exit(1)
		}
		rtIP := os.Args[2]
		cmd := strings.Join(os.Args[3:], " ")
		// 通过编译机 ssh 跳转（编译机已配免密）
		full := fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o ConnectTimeout=15 %s "%s"`, rtIP, cmd)
		rc, out, err := run(client, full, 180*time.Second)
		fmt.Printf("RC: %d\n%s\n", rc, out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

	case "upload":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: upload <local_file> <remote_path>")
			os.Exit(1)
		}
		localPath := os.Args[2]
		remotePath := os.Args[3]
		rc, err := uploadFile(client, localPath, remotePath)
		fmt.Printf("RC: %d\n", rc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s (probe|run|jump|upload)\n", action)
		os.Exit(1)
	}
}

func uploadFile(client *ssh.Client, localPath, remotePath string) (int, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return -1, fmt.Errorf("open local: %w", err)
	}
	defer f.Close()

	session, err := client.NewSession()
	if err != nil {
		return -1, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return -1, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	// 用 dd 写到远程文件
	cmd := fmt.Sprintf("dd of=%s bs=1M 2>/dev/null; echo __UPLOAD_DONE__", remotePath)
	if err := session.Start(cmd); err != nil {
		return -1, fmt.Errorf("start: %w", err)
	}

	// 异步拷贝
	copyDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(stdin, f)
		stdin.Close()
		copyDone <- err
	}()

	outBytes, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)
	<-copyDone

	if err := session.Wait(); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return exitErr.ExitStatus(), fmt.Errorf("remote: %s", string(errBytes))
		}
		return -1, err
	}
	if !strings.Contains(string(outBytes), "__UPLOAD_DONE__") {
		return -1, fmt.Errorf("upload incomplete: %s", string(outBytes))
	}
	fmt.Printf("uploaded %s -> %s (%d bytes)\n", localPath, remotePath, fileSize(localPath))
	return 0, nil
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}
