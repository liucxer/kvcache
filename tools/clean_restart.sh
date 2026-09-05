#!/bin/bash
export PATH=$PATH:/usr/local/go/bin

echo "=== STOP kvcache_perf ==="
pkill -x kvcache_perf
sleep 3
pgrep -x kvcache_perf && echo "STILL RUNNING" || echo "STOPPED"

echo ""
echo "=== CLEAN old data ==="
rm -rf /mnt/nvme1n1/kvcache/value_data/*
rm -rf /mnt/nvme1n1/kvcache/data/*
echo "cleaned"

echo ""
echo "=== DISK after clean ==="
df -h /mnt/nvme1n1

echo ""
echo "=== START kvcache_perf ==="
cd /root/kvcache
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
pgrep -x kvcache_perf && echo "STARTED PID=$(pgrep -x kvcache_perf)" || echo "FAILED TO START"
tail -5 /root/kvcache_perf.log