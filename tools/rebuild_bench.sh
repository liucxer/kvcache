#!/bin/bash
export PATH=$PATH:/usr/local/go/bin
cd /root/kvcache
CGO_ENABLED=0 go build -o bench ./test/bench/... 2>&1
echo "BUILD exit=$?"
ls -la bench 2>/dev/null && echo "BENCH OK" || echo "BENCH FAIL"