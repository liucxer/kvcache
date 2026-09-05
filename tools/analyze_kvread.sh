#!/bin/bash
OUT=/root/kvread_20260904_232212

echo "=== sar lo avg (skip 8s warmup) ==="
grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$4;s5+=$5;c++} END {printf "samples=%d\nlo_rx_avg=%.0f MB/s (%.2f GB/s)\nlo_tx_avg=%.0f MB/s (%.2f GB/s)\n", c, s4/c/1024, s4/c/1024/1024, s5/c/1024, s5/c/1024/1024}'

echo ""
echo "=== /proc/net/dev lo (raw) ==="
cat $OUT/net_before
cat $OUT/net_after

echo ""
echo "=== iostat nvme avg (col2 = rMB/s, skip 8s warmup) ==="
awk '/^nvme1n1 /{d1=$2} /^nvme2n1 /{d2=$2} /^nvme3n1 /{d3=$2} /^nvme1n1 /{n++; if(n>=9){s1+=d1;s2+=d2;s3+=d3;c++}} END {printf "samples=%d\nAVG nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", c, s1/c, s2/c, s3/c, (s1+s2+s3)/c, (s1+s2+s3)/c/1024}' $OUT/iostat.log

echo ""
echo "=== pprof files ==="
ls -la $OUT/pprof_*.pb 2>/dev/null

echo ""
echo "=== bench result ==="
grep -E 'Result|elapsed|errors|throughput|latency' $OUT/bench.log