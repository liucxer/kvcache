#!/bin/bash
export PATH=$PATH:/usr/local/go/bin
cd /root/kvcache

echo "=== CHECK rocksdb libs ==="
ls /usr/local/lib/librocksdb* /usr/local/lib/libgflags* 2>/dev/null || echo "MISSING LIBS"

echo ""
echo "=== BUILD kvcache_perf with CGO ==="
export CGO_ENABLED=1
export CGO_CFLAGS="-I/usr/local/include"
export CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd -Wl,-rpath,/usr/local/lib"
go build -o kvcache_perf . 2>&1
echo "build exit=$?"
ls -la kvcache_perf 2>/dev/null && echo "BUILD OK" || echo "BUILD FAIL"

echo ""
echo "=== START kvcache_perf ==="
pkill -x kvcache_perf 2>/dev/null
sleep 2
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
tail -5 /root/kvcache_perf.log

echo ""
echo "=== WRITE DATA (80w x 1000 x 4MB = 320GB) ==="
PREFIX="clean1"
COUNT=1000
START=$(date +%s)
./bench --mode write --raw --direct-addr 127.0.0.1:33000 \
  --workers 80 --duration 400s --value-size 4194304 \
  --count-per-worker $COUNT --prefix $PREFIX > /root/write_clean1.log 2>&1
echo "write exit=$? elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|throughput|data=' /root/write_clean1.log

echo ""
echo "=== DISK after write ==="
df -h /mnt/nvme1n1 | tail -1

echo ""
echo "=== VERIFY ==="
CGO_ENABLED=0 go run ./tools/verify_io/ 2>&1 | head -10

echo ""
echo "===== SETUP DONE ====="