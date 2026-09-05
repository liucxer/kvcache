#!/bin/bash
set -e
cd /root

mkdir -p /root/fio_result
TS=$(date +%Y%m%d_%H%M%S)
echo "TS=$TS"

# drop caches 确保读真实磁盘
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

# 单盘 nvme1n1 顺序读
fio --name=seqread_nvme1 \
  --filename=/mnt/nvme1n1/.fio_test/testfile \
  --ioengine=libaio --direct=1 --rw=read --bs=4M \
  --iodepth=64 --numjobs=1 --runtime=60 --ramp_time=10 \
  --time_based --group_reporting \
  --output-format=json --output=/root/fio_result/read_nvme1_${TS}.json

# 解析结果
python3 -c "
import json
d=json.load(open('/root/fio_result/read_nvme1_${TS}.json'))
r=d['jobs'][0]['read']
bw_mb=r['bw']/1024
print('=== nvme1n1 SEQ READ (4M, iodepth=64, numjobs=1, libaio, O_DIRECT) ===')
print('bw: %.0f MB/s (%.2f GB/s)' % (bw_mb, bw_mb/1024))
print('io_bytes: %.1f GB' % (r['io_bytes']/1024/1024/1024))
print('iops: %.0f' % r['iops'])
print('runtime: %d ms' % r['runtime'])
print('slat mean: %.1f us' % (r['slat_ns']['mean']/1000))
print('clat mean: %.1f ms' % (r['clat_ns']['mean']/1000000))
p=r['clat_ns']['percentile']
print('clat p50: %.1f ms' % (p['50.000000']/1000000))
print('clat p99: %.1f ms' % (p['99.000000']/1000000))
print('bw_min/max: %d/%d KB/s' % (r['bw_min'], r['bw_max']))
"

echo "JSON: /root/fio_result/read_nvme1_${TS}.json"