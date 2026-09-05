#!/bin/bash
# 文件系统顺序读带宽：每盘 8GB，12 流并行 dd（page cache 路径）
set -e
cd /root

for d in 1 2 3; do
  fallocate -l 8G /mnt/nvme${d}n1/kvcache/value_data/.bwtest_${d}
done
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2

for d in 1 2 3; do
  for s in 1 2 3 4; do
    dd if=/mnt/nvme${d}n1/kvcache/value_data/.bwtest_${d} of=/dev/null bs=4M count=2000 2>/tmp/dd_${d}_${s}.log &
  done
done
wait

cat /tmp/dd_*_*.log | grep -o '[0-9.]* [MG]B/s' | sed 's/ GB\/s/*1024/; s/ MB\/s/*1/'

echo "=== aggregate (MB/s) ==="
cat /tmp/dd_*_*.log | grep -o '[0-9.]* [MG]B/s' | awk '{v=$1; if ($2=="GB/s") v*=1024; s+=v; c++} END {printf "total=%.0f MB/s (%.2f GB/s) streams=%d\n", s, s/1024, c}'

rm -f /mnt/nvme1n1/kvcache/value_data/.bwtest_* /mnt/nvme2n1/kvcache/value_data/.bwtest_* /mnt/nvme3n1/kvcache/value_data/.bwtest_*
echo cleaned