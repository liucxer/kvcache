#!/bin/bash
OUT=/root/perf_big_1788512154

echo "=== all interfaces during read window 05:00:31-05:01:31 ==="
for t in 05:00:31 PM 05:00:40 PM 05:00:50 PM 05:01:00 PM 05:01:10 PM 05:01:20 PM 05:01:30 PM; do
  echo "--- $t ---"
  grep "$t" $OUT/sar.log | awk '{printf "  %-14s rx=%.1f MB/s tx=%.1f MB/s pkts=%s/%s\n", $8, $5/1024, $6/1024, $3, $4}'
done

echo ""
echo "=== lo (loopback) time series with absolute ts ==="
grep -E '^\d\d:\d\d:\d\d (AM|PM) +lo ' $OUT/sar.log | awk '{printf "%s rx=%.2f MB/s tx=%.2f MB/s\n", $1" "$2, $5/1024, $6/1024}' | head -80