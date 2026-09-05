#!/bin/bash
set -e
mkdir -p /root/fio_result
TS=$(date +%s)
fio --name=seqwrite_nvme1 --filename=/mnt/nvme1n1/.fio_test/testfile \
  --ioengine=libaio --direct=1 --rw=write --bs=4M --iodepth=64 --numjobs=1 \
  --runtime=60 --ramp_time=10 --time_based --group_reporting \
  --output-format=json --output=/root/fio_result/write_nvme1.json

python3 -c "
import json
d=json.load(open('/root/fio_result/write_nvme1.json'))
j=d['jobs'][0]
w=j['write']
print('=== nvme1n1 SEQ WRITE (4M, iodepth=64, numjobs=1, libaio, O_DIRECT) ===')
print('bw: %.0f MB/s (%.2f GB/s)' % (w['bw']/1024/1024, w['bw']/1024/1024/1024))
print('IOPS: %.0f' % w['iops'])
print('clat p50: %.1f us, p99: %.1f us' % (w['clat_ns']['percentile']['50.0']/1000, w['clat_ns']['percentile']['99.0']/1000))
"
echo "JSON saved: /root/fio_result/write_nvme1.json"