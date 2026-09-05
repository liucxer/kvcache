"""Upload nefsproxy package to build machine, compile CLI, run smoke tests."""
import paramiko
import io

HOST = "100.71.128.13"
PORT = 2222
USER = "root"
PASS = "q@@&&4806608"

REMOTE_KVCACHE = "/root/nefs/kvcache"

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect(HOST, port=PORT, username=USER, password=PASS,
                timeout=15, allow_agent=False, look_for_keys=False)

sftp = client.open_sftp()

# Upload new files
uploads = [
    (r"d:\workspace\kvcache\nefsproxy\client.go", f"{REMOTE_KVCACHE}/nefsproxy/client.go"),
    (r"d:\workspace\kvcache\cmd\nefsproxy\main.go", f"{REMOTE_KVCACHE}/cmd/nefsproxy/main.go"),
]
# ensure dirs
for remote in uploads:
    parts = remote[1].rsplit("/", 1)[0]
    client.exec_command(f"mkdir -p {parts}")[1].channel.recv_exit_status()

for local, remote in uploads:
    sftp.put(local, remote)
    print(f"uploaded {local} -> {remote}")
sftp.close()

def run(cmd, timeout=120):
    print(f"\n$ {cmd}")
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout)
    rc = stdout.channel.recv_exit_status()
    out = stdout.read().decode(errors="replace")
    err = stderr.read().decode(errors="replace")
    print(out)
    if err.strip():
        print("STDERR:", err)
    print(f"[rc={rc}]")
    return rc

# Build CLI (pure stdlib, CGO_ENABLED=0)
run(f"cd {REMOTE_KVCACHE} && CGO_ENABLED=0 go build -o /root/nefsproxy ./cmd/nefsproxy")

# Smoke tests against real proxy.py on 127.0.0.1:9527
NP = "/root/nefsproxy --addr 127.0.0.1:9527"
run(f"{NP} ping")
run(f'{NP} exec "hostname; date +%s%N; echo hello-nefsproxy"')

# File transfer roundtrip
run(f"echo 'nefsproxy smoke test file' > /tmp/nefs_local.txt")
run(f"{NP} upload /tmp/nefs_local.txt /tmp/nefs_remote.txt")
run(f"{NP} download /tmp/nefs_remote.txt /tmp/nefs_downloaded.txt")
run(f"diff /tmp/nefs_local.txt /tmp/nefs_downloaded.txt && echo FILE_ROUNDTRIP_OK")
run(f"{NP} ls /tmp")

# Proxy rule CRUD
run(f"{NP} proxy-add --name test-rule --listen-port 19999 --target-ip 127.0.0.1 --target-port 22")
run(f"{NP} proxy-list")
run(f"{NP} proxy-del test-rule")
run(f"{NP} proxy-list")

client.close()
print("\n=== ALL SMOKE TESTS DONE ===")
