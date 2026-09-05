#!/bin/bash
echo "=== file check ==="
ls -la /mnt/nvme1n1/.fio_test/testfile
df -Th /mnt/nvme1n1 | tail -1

echo "=== drop caches ==="
sync; echo 3 > /proc/sys/vm/drop_caches

echo "=== fio raw output (30s quick test) ==="
fio --name=test --filename=/mnt/nvme1n1/.fio_test/testfile \
  --ioengine=libaio --direct=1 --rw=write --bs=4M --iodepth=64 --numjobs=1 \
  --runtime=30 --ramp_time=5 --time_based --group_reporting 2>&1 | tail -25