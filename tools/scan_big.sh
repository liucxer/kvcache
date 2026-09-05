#!/bin/bash
# Scan with large limit to find all keys
echo "=== Scan bench3n with limit=10000 on nvme5 (33001) ==="
curl -s "http://127.0.0.1:33001/api/v1/scan?prefix=bench3n&limit=10000" 2>/dev/null | python3 -c "
import sys,json
d=json.load(sys.stdin)
keys = list(d.get('results',{}).keys())
print(f'Returned keys: {len(keys)}')
# Extract unique wids and seq ranges
wids = set()
seqs = []
for k in keys:
    parts = k.split(':')
    if len(parts) >= 3:
        w = parts[1]
        s = int(parts[2])
        wids.add(w)
        seqs.append(s)
print(f'Unique wids: {sorted(wids)[:20]}')
if seqs:
    print(f'Seq range: {min(seqs)}-{max(seqs)} ({len(seqs)} unique)')
print(f'Sample keys: {keys[:5]}')
" 2>/dev/null

echo ""
echo "=== Scan nvme7 (33021) ==="
curl -s "http://127.0.0.1:33021/api/v1/scan?prefix=bench3n&limit=10000" 2>/dev/null | python3 -c "
import sys,json
d=json.load(sys.stdin)
keys = list(d.get('results',{}).keys())
print(f'Returned keys: {len(keys)}')
wids=set()
for k in keys:
    parts=k.split(':')
    if len(parts)>=3: wids.add(parts[1])
print(f'Unique wids: {sorted(wids)[:20]}')
print(f'Sample keys: {keys[:5]}')
" 2>/dev/null

echo ""
echo "=== Scan nvme6 (33031) ==="
curl -s "http://127.0.0.1:33031/api/v1/scan?prefix=bench3n&limit=10000" 2>/dev/null | python3 -c "
import sys,json
d=json.load(sys.stdin)
keys = list(d.get('results',{}).keys())
print(f'Returned keys: {len(keys)}')
wids=set()
for k in keys:
    parts=k.split(':')
    if len(parts)>=3: wids.add(parts[1])
print(f'Unique wids: {sorted(wids)[:20]}')
print(f'Sample keys: {keys[:5]}')
" 2>/dev/null

echo ""
echo "=== Scan with empty prefix on nvme5 (limit=10000) ==="
curl -s "http://127.0.0.1:33001/api/v1/scan?limit=10000" 2>/dev/null | python3 -c "
import sys,json
d=json.load(sys.stdin)
keys = list(d.get('results',{}).keys())
print(f'Total returned: {len(keys)}')
prefixes=set()
for k in keys:
    p=k.split(':')[0]
    prefixes.add(p)
print(f'Unique prefixes: {sorted(prefixes)}')
print(f'Sample keys: {keys[:10]}')
" 2>/dev/null