#!/bin/bash
# Try exact TiKV key format via HTTP API
echo "=== Try exact TiKV index keys ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for k in bench-key-0-0 bench-key-0-1 bench-key-0-50 bench-key-0-99; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/kv/${k}" 2>/dev/null)
    echo "  $k -> $code"
  done
done

echo ""
echo "=== Try key format: wid:seq ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for k in 0:0 0:1 1:0 50:0 100:0; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/kv/${k}" 2>/dev/null)
    echo "  $k -> $code"
  done
done

echo ""
echo "=== Try key format: bench-key-{wid}-{seq} with w prefix ==="
for port in 33001 33021 33031; do
  echo "--- Instance HTTP :$port ---"
  for k in bench-key-0-0 bench-key-w0-0 bench-key-w0:0 bench-key-0:0; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/kv/${k}" 2>/dev/null)
    echo "  $k -> $code"
  done
done

echo ""
echo "=== Try reading one complete key via HTTP body ==="
curl -s "http://127.0.0.1:33001/api/v1/kv/bench-key-0-0" 2>/dev/null | head -c 100
echo ""
curl -s "http://127.0.0.1:33001/api/v1/kv/bench-key-0-1" 2>/dev/null | head -c 100