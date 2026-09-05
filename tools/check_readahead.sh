#!/bin/bash
echo "=== block read_ahead_kb ==="
for d in nvme1n1 nvme2n1 nvme3n1; do
  echo -n "$d: "
  cat /sys/block/$d/queue/read_ahead_kb 2>/dev/null || echo "N/A"
done
echo ""
echo "=== block scheduler ==="
for d in nvme1n1 nvme2n1 nvme3n1; do
  echo -n "$d: "
  cat /sys/block/$d/queue/scheduler 2>/dev/null || echo "N/A"
done
echo ""
echo "=== block nr_requests ==="
for d in nvme1n1 nvme2n1 nvme3n1; do
  echo -n "$d: "
  cat /sys/block/$d/queue/nr_requests 2>/dev/null || echo "N/A"
done
echo ""
echo "=== block max_sectors_kb ==="
for d in nvme1n1 nvme2n1 nvme3n1; do
  echo -n "$d: "
  cat /sys/block/$d/queue/max_sectors_kb 2>/dev/null || echo "N/A"
done
echo ""
echo "=== vm page-cluster ==="
cat /proc/sys/vm/page-cluster
echo ""
echo "=== vm dirty_ratio / dirty_background_ratio ==="
cat /proc/sys/vm/dirty_ratio
cat /proc/sys/vm/dirty_background_ratio
echo ""
echo "=== kvcache 实例 read_ahead 设置 (命令行) ==="
pgrep -af kvcache_perf | grep -o 'readahead-kb [0-9]*'