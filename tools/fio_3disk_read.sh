#!/bin/bash
set -e
cd /root

mkdir -p /root/fio_result
TS=$(date +%Y%m%d_%H%M%S)
echo "TS=$TS"

# drop caches 确保读真实磁盘
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

# 三盘并行启动 fio 读
PIDS=""
for d in 1 2 3; do
  fio --name=seqread_nvme${d} \
    --filename=/mnt/nvme${d}n1/.fio_test/testfile \
    --ioengine=libaio --direct=1 --rw=read --bs=4M \
    --iodepth=64 --numjobs=1 --runtime=60 --ramp_time=10 \
    --time_based --group_reporting \
    --output-format=json --output=/root/fio_result/read_nvme${d}_${TS}.json &
  PIDS="$PIDS $!"
  echo "started nvme${d}n1 pid=$!"
done

echo "waiting for all fio..."
wait $PIDS
echo "all fio done"

# 解析结果
echo ""
echo "=== RESULTS ==="
for d in 1 2 3; do
  python3 -c "
import json
d=json.load(open('/root/fio_result/read_nvme${d}_${TS}.json'))
r=d['jobs'][0]['read']
bw_mb=r['bw']/1024
print('nvme${d}n1: %.0f MB/s (%.2f GB/s)  io_bytes=%.1fGB  iops=%.0f  clat_mean=%.1fms  clat_p99=%.1fms' % (bw_mb, bw_mb/1024, r['io_bytes']/1024/1024/1024, r['iops'], r['clat_ns']['mean']/1000000, r['clat_ns']['percentile']['99.000000']/1000000))
"
done

# 聚合带宽
python3 -c "
import json
total_bw=0
total_io=0
for d in [1,2,3]:
    j=json.load(open('/root/fio_result/read_nvme%d_%s.json' % (d,'${TS}')))
    r=j['jobs'][0]['read']
    total_bw += r['bw']
    total_io += r['io_bytes']
print('AGGREGATE: %.0f MB/s (%.2f GB/s)  total_io=%.1fGB' % (total_bw/1024, total_bw/1024/1024, total_io/1024/1024/1024))
"

echo ""
echo "JSON files:"
ls -la /root/fio_result/read_*_${TS}.json