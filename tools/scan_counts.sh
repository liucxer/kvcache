#!/bin/bash
# Scan with higher limit to get real key counts
echo "=== Scan bench3n with limit=10000 ==="
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bench3n&limit=10000" 2>/dev/null)
  count=$(echo "$result" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('count','?'))" 2>/dev/null)
  echo "  :$port -> count=$count"
done

echo ""
echo "=== Scan bigdata with limit=10000 ==="
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bigdata&limit=10000" 2>/dev/null)
  count=$(echo "$result" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('count','?'))" 2>/dev/null)
  echo "  :$port -> count=$count"
done

echo ""
echo "=== Scan testbw with limit=10000 ==="
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=testbw&limit=10000" 2>/dev/null)
  count=$(echo "$result" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('count','?'))" 2>/dev/null)
  if [ "$count" != "0" ] && [ "$count" != "?" ]; then
    echo "  :$port -> count=$count"
  else
    echo "  :$port -> count=$count or no data"
  fi
done

echo ""
echo "=== Try reading a bench3n key via HTTP to verify ==="
for port in 33001 33021 33031; do
  for k in bench3n:w0:0 bench3n:w0:100000 bench3n:w0:100005; do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}/api/v1/get/${k}" 2>/dev/null)
    echo "  :$port /api/v1/get/$k -> $code"
  done
done

echo ""
echo "=== Check if bench3n data is the 4T data: verify key count per instance ==="
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bench3n&limit=10000" 2>/dev/null)
  echo "$result" | python3 -c "
import sys,json
d=json.load(sys.stdin)
keys = list(d.get('results',{}).keys())
print(f':{port} -> count={d.get(\"count\")}, keys_returned={len(keys)}')
# Extract unique wid values
wids = set()
for k in keys[:100]:
    parts = k.split(':')
    if len(parts) >= 2:
        w = parts[1]
        wids.add(w)
print(f'  unique wids (first 100 keys): {sorted(wids)[:20]}')
" 2>/dev/null
done