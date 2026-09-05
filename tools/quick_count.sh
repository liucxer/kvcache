#!/bin/bash
# Quick scan of key counts
echo "=== bench3n key counts ==="
for port in 33001 33021 33031; do
  c=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bench3n&limit=1" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null)
  echo "Instance :$port -> bench3n count=$c"
done

echo "=== bigdata key counts ==="
for port in 33001 33021 33031; do
  c=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bigdata&limit=1" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null)
  echo "Instance :$port -> bigdata count=$c"
done

echo "=== testbw key counts ==="
for port in 33001 33021 33031; do
  c=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=testbw&limit=1" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null)
  echo "Instance :$port -> testbw count=$c"
done

echo "=== Sample bench3n keys ==="
for port in 33001 33021 33031; do
  r=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=bench3n&limit=3" | python3 -c "
import sys,json
d=json.load(sys.stdin)
keys=list(d.get('results',{}).keys())
print(f'{port}: keys={keys[:3]}')
" 2>/dev/null)
  echo "$r"
done