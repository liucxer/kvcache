#!/bin/bash
echo "=== NVMe 队列参数（全部） ==="
for d in nvme1n1 nvme2n1 nvme3n1; do
  echo "--- $d ---"
  for f in max_hw_sectors_kb max_sectors_kb max_segments max_segment_size read_ahead_kb nr_requests scheduler chunk_sectors; do
    echo -n "  $f = "
    cat /sys/block/$d/queue/$f 2>/dev/null || echo "N/A"
  done
done

echo ""
echo "=== NVMe 控制器能力（nvme-cli） ==="
which nvme 2>/dev/null && nvme id-ctrl /dev/nvme1 2>/dev/null | grep -E 'max_hw_sectors|max_segments|max_segment_size|sgls' | head -10 || echo "nvme-cli not found"

echo ""
echo "=== 当前 fio 读测试中的实际 IO 大小（从之前 JSON 推算） ==="
echo "iodepth=64, bs=4M, 若 max_sectors_kb=128 => 每次 4M 读拆成 32 个 128KB IO"
echo "用 iostat 的 r/s 与 rMB/s 比值可反推平均 IO 大小"