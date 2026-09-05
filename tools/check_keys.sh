#!/bin/bash
echo "=== WRITE RESULT ==="
grep -E 'Result|elapsed|ops|data' /root/perf_big_1788510855/bench_write.log

echo ""
echo "=== ACTUAL KEY COUNT PER INSTANCE ==="
for port in 33001 33021 33031; do
  echo "--- HTTP port $port ---"
  curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bigread" -o /tmp/scan_${port}.json
  python3 -c "import json; d=json.load(open('/tmp/scan_${port}.json')); print('count:', d.get('count',0), 'keys returned:', len(d.get('keys',[])))"
done

echo ""
echo "=== TIkv INDEX COUNT (via scan_index tool) ==="
# Use scan_index to count bigread entries in TiKV
timeout 30 /root/scan_index --prefix=/kvcache/index/bigread --limit=5 2>/dev/null | head -20 || echo "scan_index not available"

echo ""
echo "=== READ RESULT ==="
grep -E 'elapsed|data=|errors|latency' /root/perf_big_1788510855/bench_read.log

echo ""
echo "=== READ ERROR SAMPLES ==="
grep -i 'error\|fail\|not found' /root/perf_big_1788510855/bench_read.log | head -5
echo "Total error lines:"
grep -ic 'error\|fail\|not found' /root/perf_big_1788510855/bench_read.log

echo ""
echo "=== KEY DISTRIBUTION CHECK ==="
# Check which workers have keys
for wid in 0 1 2 3 63; do
  # Try direct read via gRPC port
  key="bigread:w${wid}:0"
  echo "  Checking key ${key} on each instance:"
  for port in 33001 33021 33031; do
    result=$(curl -s "http://127.0.0.1:${port}/api/v1/get?key=${key}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('error','OK')[:50])" 2>/dev/null)
    echo "    port ${port}: ${result}"
  done
done
