#!/bin/bash
export PATH=$PATH:/usr/local/go/bin
mkdir -p /root/kvcache
cd /root/kvcache
tar -xzf /tmp/kvcache_src.tar.gz
echo "=== Files ==="
ls -la
echo "=== GO BUILD bench ==="
CGO_ENABLED=0 go build -o /root/kvcache/bench ./test/bench/... 2>&1
echo "bench build exit=$?"
ls -la /root/kvcache/bench 2>/dev/null || echo "bench not found"
echo "=== GO BUILD kvcache ==="
CGO_ENABLED=0 go build -o /root/kvcache/kvcache_perf . 2>&1
echo "kvcache build exit=$?"
ls -la /root/kvcache/kvcache_perf 2>/dev/null || echo "kvcache not found"