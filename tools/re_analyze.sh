#!/bin/bash
OUT=/root/perf_wr_1788514130
echo "=== top nvme5n1 wMB/s samples ==="
awk '/^nvme5n1 /{print $9}' $OUT/iostat.log | sort -rn | head -8

echo "=== nvme5n1 wMB/s distribution ==="
awk '/^nvme5n1 /{if($9<100)z++; else nz++} END{print "zero_or_tiny:"z"  non_zero:"nz}' $OUT/iostat.log

echo "=== avg-cpu block positions ==="
grep -n 'avg-cpu' $OUT/iostat.log

echo "=== total lines ==="
wc -l $OUT/iostat.log

echo "=== first 3 blocks of Device headers with data ==="
awk '/^Device/{b++} /^nvme5n1 /{print "block"b" wMB/s="$9}' $OUT/iostat.log | head -12

echo "=== sar lo samples (first 10 non-0) ==="
grep ' lo ' $OUT/sar.log | awk '{print $6}' | sort -rn | head -5
grep ' lo ' $OUT/sar.log | awk '{if($6>100000)c++} END{print "lo samples>100MB/s:"c}'