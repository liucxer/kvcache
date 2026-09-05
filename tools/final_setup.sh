#!/bin/bash
cd /root/kvcache

echo "=== STOP old ==="
pkill -x kvcache_perf 2>/dev/null
sleep 2

echo "=== CLEAN ==="
find /mnt/nvme1n1/kvcache/value_data/ -type f -delete
find /mnt/nvme1n1/kvcache/data/ -type f -delete
df -h /mnt/nvme1n1 | tail -1

echo ""
echo "=== START ==="
nohup ./kvcache_perf \
  --name=perf-nvme1 \
  --data-dir=/mnt/nvme1n1/kvcache/data \
  --value-dir=/mnt/nvme1n1/kvcache/value_data \
  --grpc-addr=:33000 \
  --raw-addr=:29300 \
  --raw-write-addr=:29301 \
  --node=128.12 \
  --tikv-pd=127.0.0.1:2379 \
  --ca-path= --cert-path= --key-path= \
  --readahead-kb 4096 > /root/kvcache_perf.log 2>&1 &
sleep 3
PID=$(pgrep -x kvcache_perf)
echo "PID=$PID"
tail -3 /root/kvcache_perf.log

echo ""
echo "=== WRITE (80w x 1000 x 4MB = 320GB) ==="
PREFIX="clean1"
COUNT=1000
START=$(date +%s)
./bench --mode write --raw --direct-addr 127.0.0.1:33000 \
  --workers 80 --duration 400s --value-size 4194304 \
  --count-per-worker $COUNT --prefix $PREFIX > /root/write_clean1.log 2>&1
echo "write exit=$? elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|throughput|data=' /root/write_clean1.log

echo ""
echo "=== DISK ==="
df -h /mnt/nvme1n1 | tail -1

echo ""
echo "=== VERIFY ==="
CGO_ENABLED=0 go run ./tools/verify_io/ 2>&1 | head -8

echo ""
echo "===== READY ====="