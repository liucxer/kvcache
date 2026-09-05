#!/bin/bash
# Probe kvcache instances to find existing key prefixes
echo "=== Instances ==="
# nvme5: HTTP 33001, gRPC 33000, raw 29300
# nvme6: HTTP 33031, gRPC 33030, raw 29330
# nvme7: HTTP 33021, gRPC 33020, raw 29320

# Try common prefixes via HTTP API - test all 3 instances
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for prefix in bench-key kvcache bigdata testbw bench60 bench perf data default; do
    key="${prefix}:w0:0"
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/kv/${key}" 2>/dev/null)
    echo "  $key -> $code"
  done
done

echo ""
echo "=== Try bench-key prefix with different wids ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for wid in 0 1 10 50 100; do
    key="bench-key-w${wid}:0"
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/kv/${key}" 2>/dev/null)
    echo "  $key -> $code"
  done
done

echo ""
echo "=== Check value_data directory for file naming hints ==="
for d in nvme5 nvme6 nvme7; do
  echo "--- $d ---"
  ls /nefsdata/$d/value_data/ 2>/dev/null | head -3
  echo "  count: $(ls /nefsdata/$d/value_data/ 2>/dev/null | wc -l)"
  du -sh /nefsdata/$d/value_data/ 2>/dev/null
done