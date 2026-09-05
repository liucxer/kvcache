#!/bin/bash
# Probe using correct HTTP API path: /api/v1/get/:key
echo "=== Correct HTTP path: /api/v1/get/:key ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for prefix in bench-key kvcache bigdata testbw bench60 bench perf data default; do
    key="${prefix}:w0:0"
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/get/${key}" 2>/dev/null)
    echo "  $key -> $code"
  done
done

echo ""
echo "=== Try exact TiKV index keys on correct path ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for k in bench-key-0-0 bench-key-0-1 bench-key-0-50 bench-key-0-99; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/get/${k}" 2>/dev/null)
    echo "  $k -> $code"
  done
done

echo ""
echo "=== Scan API to find existing keys ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port scan(prefix='') ---"
  curl -s "http://127.0.0.1:${port}/api/v1/scan?limit=5" 2>/dev/null | head -c 300
  echo ""
done

echo ""
echo "=== Scan with empty prefix to see all keys ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port scan(prefix='') ---"
  curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=&limit=5" 2>/dev/null | head -c 300
  echo ""
done

echo ""
echo "=== Scan with common prefixes ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for p in bench bench-key bigdata kvcache; do
    result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=${p}&limit=3" 2>/dev/null)
    echo "  prefix=$p -> $result"
  done
done