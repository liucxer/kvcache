#!/bin/bash
# Comprehensive scan of all key prefixes
echo "=== Scan with empty prefix to see all keys on nvme5 (33001) ==="
curl -s "http://127.0.0.1:33001/api/v1/scan?limit=20" 2>/dev/null | python3 -c "
import sys,json
data = json.load(sys.stdin)
print(f'Count: {data[\"count\"]}, Limit: {data[\"limit\"]}, Prefix: \"{data[\"prefix\"]}\"')
# Extract prefixes from keys
prefixes = set()
for k in data.get('results', {}):
    parts = k.split(':')
    if len(parts) >= 1:
        prefixes.add(parts[0])
print(f'Prefixes found: {sorted(prefixes)[:20]}')
# Print first 5 keys
for i, k in enumerate(list(data.get('results', {}).keys())[:5]):
    print(f'  key[{i}]: {k}')
" 2>/dev/null

echo ""
echo "=== Scan with prefix bench3n on all instances ==="
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bench3n&limit=5" 2>/dev/null)
  count=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count', '?'))" 2>/dev/null)
  echo "  :$port -> count=$count"
done

echo ""
echo "=== Scan with prefix bigdata on all instances ==="
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bigdata&limit=5" 2>/dev/null)
  count=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count', '?'))" 2>/dev/null)
  echo "  :$port -> count=$count"
done

echo ""
echo "=== Scan with prefix testbw on all instances ==="
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=testbw&limit=5" 2>/dev/null)
  count=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count', '?'))" 2>/dev/null)
  echo "  :$port -> count=$count"
done

echo ""
echo "=== Check unique prefixes by scanning with empty prefix ==="
python3 -c "
import subprocess, json
# Get all keys from all instances
for port in [33001, 33021, 33031]:
    result = subprocess.run(['curl', '-s', f'http://127.0.0.1:{port}/api/v1/scan?limit=20'], capture_output=True, text=True)
    data = json.loads(result.stdout)
    prefixes = set()
    keys_list = list(data.get('results', {}).keys())
    for k in keys_list:
        parts = k.split(':')
        if len(parts) >= 1:
            prefixes.add(parts[0])
    print(f':{port} -> prefixes={sorted(prefixes)}, total_keys_shown={len(keys_list)}')
" 2>/dev/null

echo ""
echo "=== Delete testbw and bigdata keys (from previous failed test) ==="
# These were written by our previous failed test, we should clean them up
# Actually, let's first check their size
for port in 33001 33021 33031; do
  result=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=testbw&limit=2" 2>/dev/null)
  echo "  :$port testbw -> $(echo $result | head -c 100)"
done