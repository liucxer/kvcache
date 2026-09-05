#!/bin/bash
# 停旧 3 实例 -> 替换为 O_SYNC 二进制 -> setsid 重启 3 实例
set -u
echo "=== STOP old instances ==="
pkill -x kvcache_perf; sleep 2
pkill -9 -x kvcache_perf 2>/dev/null; sleep 1
if pgrep -x kvcache_perf >/dev/null; then echo "STILL_RUNNING"; else echo "STOPPED"; fi

echo "=== REPLACE binary ==="
cp /tmp/kvcache_perf_new /root/kvcache/kvcache_perf
chmod +x /root/kvcache/kvcache_perf
ls -la /root/kvcache/kvcache_perf

echo "=== START 3 instances (setsid, detach) ==="
i=1
for spec in \
  "perf-nvme1 :33000 :29300 :29301 128.12-1 nvme1n1" \
  "perf-nvme2 :33010 :29310 :29311 128.12-2 nvme2n1" \
  "perf-nvme3 :33020 :29320 :29321 128.12-3 nvme3n1"; do
  set -- $spec
  NAME=$1 GRPC=$2 RAW=$3 RW=$4 NODE=$5 DISK=$6
  setsid nohup /root/kvcache/kvcache_perf \
    --name=$NAME \
    --data-dir=/mnt/$DISK/kvcache/data \
    --value-dir=/mnt/$DISK/kvcache/value_data \
    --grpc-addr=$GRPC --raw-addr=$RAW --raw-write-addr=$RW \
    --node=$NODE --tikv-pd=127.0.0.1:2379 \
    >/tmp/kc_$i.log 2>&1 </dev/null &
  echo "launched $NAME (grpc$GRPC raw$RAW rw$RW)"
  i=$((i+1))
done
sleep 4
echo "=== RUNNING count ==="
pgrep -x kvcache_perf | wc -l
echo "=== listeners ==="
ss -tlnp 2>/dev/null | grep -E ':3300[01]|:2930[01]' | awk '{print $4}'
echo "=== per-instance log tail ==="
for l in /tmp/kc_1.log /tmp/kc_2.log /tmp/kc_3.log; do echo "-- $l --"; tail -3 "$l" 2>/dev/null; done