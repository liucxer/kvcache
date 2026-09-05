#!/bin/bash
# Analyze iostat + sar for read test period
OUT=/root/perf_big_1788512154

echo "=== sar.log head (time range) ==="
grep -E '^\d\d:\d\d' $OUT/sar.log | head -2
grep -E '^\d\d:\d\d' $OUT/sar.log | tail -2
echo ""

echo "=== sar bond1 during read (rxkB/s txkB/s) ==="
grep -E '^\d\d:\d\d.*bond1 ' $OUT/sar.log | head -80