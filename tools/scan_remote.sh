#!/bin/bash
export PATH=$PATH:/usr/local/go/bin
cd /root/kvcache
# 修改 scan_keys 的 prefix
sed 's/prefix := "bench3n:"/prefix := "single2:"/' tools/scan_keys/main.go > /tmp/scan_single2.go
echo "=== Scan single2: keys ==="
CGO_ENABLED=0 go run /tmp/scan_single2.go 2>&1 | head -20
echo ""
echo "=== Scan testnologic: keys ==="
sed 's/prefix := "bench3n:"/prefix := "testnologic:"/' tools/scan_keys/main.go > /tmp/scan_test.go
CGO_ENABLED=0 go run /tmp/scan_test.go 2>&1 | head -20