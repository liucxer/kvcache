"""Test nefs-proxy single-call latency from build machine."""
import time
import urllib.request
import json

PROXY = "http://127.0.0.1:9527"
TOKEN = "95279527"

def call(path, method="GET", body=None):
    url = PROXY + path
    data = None
    headers = {"X-Token": TOKEN}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    t0 = time.perf_counter()
    with urllib.request.urlopen(req, timeout=30) as resp:
        payload = resp.read().decode()
        status = resp.status
    dt = (time.perf_counter() - t0) * 1000
    return status, dt, payload

print("=== proxy rules ===")
st, dt, body = call("/proxy")
print(f"status={st} latency={dt:.1f}ms body={body[:500]}")

print("\n=== 5x single /exec calls (hostname) ===")
for i in range(5):
    st, dt, body = call("/exec", method="POST", body={"cmd": "hostname; date +%s%N"})
    print(f"#{i+1}: status={st} latency={dt:.1f}ms body={body.strip()}")

print("\n=== proxy exec ping 128.12 ===")
st, dt, body = call("/exec", method="POST", body={"cmd": "ping -c 3 100.71.128.12"})
print(f"status={st} latency={dt:.1f}ms")
print(body[:800])
