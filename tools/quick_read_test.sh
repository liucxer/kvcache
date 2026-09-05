#!/bin/bash
set -e
cd /root

# Safety gate
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi

echo "=== Quick direct read test with bench3n data ==="
# Based on observation: keys are bench3n:w0:100000+ on 3 instances
# Use wide range: 0-1000000, workers=16 for quick test
timeout 30 ./bench --mode read --direct 127.0.0.1:33000,127.0.0.1:33020,127.0.0.1:33030 \
  --workers 16 --duration 10s --value-size 4194304 \
  --prefix bench3n --read-max-seq 1000000 --read-wid-mod 1 \
  --raw 2>&1 | tee /tmp/quick_read_test.log

echo ""
echo "=== Result ==="
grep -E 'Result|elapsed|errors|latency|note' /tmp/quick_read_test.log 2>/dev/null
ERR=$(grep -c 'Get failed' /tmp/quick_read_test.log 2>/dev/null || echo 0)
echo "Total errors: $ERR"
echo ""
echo "=== If errors > 0, check first errors ==="
grep 'Get failed' /tmp/quick_read_test.log 2>/dev/null | head -3