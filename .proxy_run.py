"""Upload proxy_test.py to build machine and run it there."""
import paramiko

HOST = "100.71.128.13"
PORT = 2222
USER = "root"
PASS = "q@@&&4806608"

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect(HOST, port=PORT, username=USER, password=PASS,
                timeout=15, allow_agent=False, look_for_keys=False)

# Upload
sftp = client.open_sftp()
sftp.put("d:\\workspace\\kvcache\\.proxy_test.py", "/root/.proxy_test.py")
sftp.close()
print("uploaded")

# Run
stdin, stdout, stderr = client.exec_command("python3 /root/.proxy_test.py", timeout=60)
out = stdout.read().decode()
err = stderr.read().decode()
print("=== STDOUT ===")
print(out)
if err.strip():
    print("=== STDERR ===")
    print(err)
client.close()
