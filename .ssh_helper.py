"""SSH helper: connect to build machine, run commands, also jump to runtime machines."""
import sys
import paramiko

BUILD_HOST = "100.71.128.13"
BUILD_PORT = 2222
BUILD_USER = "root"
BUILD_PASS = "q@@&&4806608"

def connect_build():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(BUILD_HOST, port=BUILD_PORT, username=BUILD_USER,
                   password=BUILD_PASS, timeout=15, allow_agent=False, look_for_keys=False)
    return client

def run(client, cmd, timeout=60):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout, get_pty=False)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    rc = stdout.channel.recv_exit_status()
    return rc, out, err

def main():
    action = sys.argv[1] if len(sys.argv) > 1 else "probe"
    client = connect_build()
    try:
        if action == "probe":
            # 基本信息
            rc, out, err = run(client, "echo === build host ===; hostname; uname -a; "
                                       "echo === home ===; ls -la ~; "
                                       "echo === kvcache ===; ls -la ~/kvcache 2>/dev/null || ls -la /root/kvcache 2>/dev/null || find / -maxdepth 4 -name kvcache -type d 2>/dev/null | head -5")
            print("RC:", rc)
            print("STDOUT:\n", out)
            if err.strip():
                print("STDERR:\n", err)
        elif action == "run":
            # 直接执行后续参数组成的命令
            cmd = sys.argv[2]
            rc, out, err = run(client, cmd)
            print("RC:", rc)
            print("STDOUT:\n", out)
            if err.strip():
                print("STDERR:\n", err)
        elif action == "jump":
            # 跳到运行机执行：jump <runtime_ip> <cmd>
            rt_ip = sys.argv[2]
            cmd = sys.argv[3]
            # 通过编译机 ssh 跳转（编译机已配免密）
            full = f"ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 {rt_ip} \"{cmd}\""
            rc, out, err = run(client, full, timeout=120)
            print("RC:", rc)
            print("STDOUT:\n", out)
            if err.strip():
                print("STDERR:\n", err)
    finally:
        client.close()

if __name__ == "__main__":
    main()
