#!/bin/bash
echo "=== Verify bench3n keys ==="
for port in 33001 33021 33031; do
  for k in bench3n:w0:100000 bench3n:w0:100001 bench3n:w0:100002; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/get/${k}" 2>/dev/null)
    if [ "$code" = "200" ]; then
      echo "  :$port GET $k -> $code"
    fi
  done
done

echo "=== Also check bigdata keys ==="
for port in 33001 33021 33031; do
  for k in bigdata:w0:0 bigdata:w0:1 bigdata:w0:100; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/get/${k}" 2>/dev/null)
    if [ "$code" = "200" ]; then
      echo "  :$port GET $k -> $code"
    fi
  done
done