#!/bin/bash
# Probe key prefixes in kvcache instances
echo "=== RocksDB data dir ==="
ls /nefsdata/nvme5/data/ | head -10

echo ""
echo "=== Find key prefixes in SST files ==="
for f in /nefsdata/nvme5/data/*.sst; do
  strings "$f" 2>/dev/null | grep -E '^[a-zA-Z0-9]+:.*:' | head -5
done | sort -u | head -20

echo ""
echo "=== HTTP probe common prefixes on port 33001 (nvme5 HTTP) ==="
for k in bench60:w0:0 testbw:w0:0 bigdata:w0:0 kvcache:w0:0 perf:w0:0 data:w0:0; do
  code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:33001/api/v1/kv/$k" 2>/dev/null)
  echo "$k -> $code"
done

echo ""
echo "=== List RocksDB keys via HTTP? Check if list API exists ==="
curl -s "http://127.0.0.1:33001/api/v1/kv" 2>/dev/null | head -c 500

echo ""
echo "=== Check value_data file count per disk ==="
for d in nvme5 nvme6 nvme7; do
  cnt=$(ls /nefsdata/$d/value_data/ 2>/dev/null | wc -l)
  echo "$d value_data files: $cnt"
done

echo ""
echo "=== Sample value_data filenames (to understand mapping) ==="
ls /nefsdata/nvme5/value_data/ 2>/dev/null | head -5
