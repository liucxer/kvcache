#!/bin/bash
set -e
cd /root

mkdir -p /root/fio_result
TS=$(date +%Y%m%d_%H%M%S)
echo "TS=$TS"

# 1. 预分配三盘测试文件（每盘 16GB）
for d in 1 2 3; do
  mkdir -p /mnt/nvme${d}n1/.fio_test
  if [ ! -f /mnt/nvme${d}n1/.fio_test/testfile ] || [ $(stat -c%s /mnt/nvme${d}n1/.fio_test/testfile) -lt 17179869184 ]; then
    echo "fallocate nvme${d}n1 ..."
    fallocate -l 16G /mnt/nvme${d}n1/.fio_test/testfile
  else
    echo "nvme${d}n1 testfile exists ($(stat -c%s /mnt/nvme${d}n1/.fio_test/testfile) bytes)"
  fi
done

# 2. drop caches
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2

# 3. 三盘并行启动 fio
PIDS=""
for d in 1 2 3; do
  fio --name=seqwrite_nvme${d} \
    --filename=/mnt/nvme${d}n1/.fio_test/testfile \
    --ioengine=libaio --direct=1 --rw=write --bs=4M \
    --iodepth=64 --numjobs=1 --runtime=60 --ramp_time=10 \
    --time_based --group_reporting \
    --output-format=json --output=/root/fio_result/write_nvme${d}_${TS}.json &
  PIDS="$PIDS $!"
  echo "started nvme${d}n1 pid=$!"
done

echo "waiting for all fio..."
wait $PIDS
echo "all fio done"

# 4. 解析结果
echo ""
echo "=== RESULTS ==="
TOTAL_BW=0
for d in 1 2 3; do
  python3 -c "
import json
d=json.load(open('/root/fio_result/write_nvme${d}_${TS}.json'))
w=d['jobs'][0]['write']
bw_mb=w['bw']/1024
print('nvme${d}n1: %.0f MB/s (%.2f GB/s)  io_bytes=%.1fGB  iops=%.0f  clat_mean=%.1fms  clat_p99=%.1fms' % (bw_mb, bw_mb/1024, w['io_bytes']/1024/1024/1024, w['iops'], w['clat_ns']['mean']/1000000, w['clat_ns']['percentile']['99.000000']/1000000))
"
done

# 聚合带宽
python3 -c "
import json
total_bw=0
total_io=0
for d in [1,2,3]:
    j=json.load(open('/root/fio_result/write_nvme%d_%s.json' % (d,'${TS}')))
    w=j['jobs'][0]['write']
    total_bw += w['bw']
    total_io += w['io_bytes']
print('AGGREGATE: %.0f MB/s (%.2f GB/s)  total_io=%.1fGB' % (total_bw/1024, total_bw/1024/1024, total_io/1024/1024/1024))
"

echo ""
echo "JSON files:"
ls -la /root/fio_result/write_*_${TS}.json