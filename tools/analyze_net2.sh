#!/bin/bash
OUT=/root/perf_big_1788512154

echo "=== lo loopback time series (correct cols: iface=$3 rx=$6 tx=$7) ==="
grep ' lo ' $OUT/sar.log | awk '{printf "%s %s  rx=%.1f MB/s tx=%.1f MB/s\n", $1, $2, $6/1024, $7/1024}'

echo ""
echo "=== per-interface totals during read window ==="
grep -E '^05:00:(3[1-9]|[4-9][0-9]) PM|^05:01:(0[0-9]|[12][0-9]|3[0-2]) PM' $OUT/sar.log | awk '{key=$3; rx[$key]+=$6; tx[$key]+=$7; n[$key]++} END {for(k in rx) printf "%-14s avg_rx=%.1f MB/s  avg_tx=%.1f MB/s  (%d samples)\n", k, rx[k]/n[k]/1024, tx[k]/n[k]/1024, n[k]}' | sort