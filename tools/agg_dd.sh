#!/bin/bash
echo "=== per-stream rates ==="
cat /tmp/dd_*_*.log | grep -o '[0-9.]* MB/s'
echo "=== aggregate ==="
TOTAL=$(cat /tmp/dd_*_*.log | grep -o '[0-9.]* MB/s' | awk '{s+=$1} END {print s}')
N=$(cat /tmp/dd_*_*.log | grep -c 'MB/s')
echo "total=${TOTAL} MB/s count=${N} avg_per_stream=$((TOTAL/N)) MB/s"
rm -f /mnt/nvme1n1/kvcache/value_data/.bwtest_* /mnt/nvme2n1/kvcache/value_data/.bwtest_* /mnt/nvme3n1/kvcache/value_data/.bwtest_*
echo cleaned