#!/bin/bash
OUT=/root/perf_wr_1788514130

echo "=== DISK wMB/s (active window: skip first 12 blocks warmup) ==="
awk '/^nvme5n1 /{d5=$9} /^nvme6n1 /{d6=$9} /^nvme7n1 /{d7=$9} /^nvme5n1 /{n++; if(n>=13){s5+=d5;s6+=d6;s7+=d7;c++; if(d5>p5)p5=d5; if(d6>p6)p6=d6; if(d7>p7)p7=d7; if(d5+d6+d7>pt)pt=d5+d6+d7}} END {printf "samples=%d\n", c; printf "AVG: nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s5/c, s6/c, s7/c, (s5+s6+s7)/c, (s5+s6+s7)/c/1024; printf "PEAK: nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p5, p6, p7, pt, pt/1024}' $OUT/iostat.log

echo ""
echo "=== NET lo (rxkB/s=sample, active: skip first 12s) ==="
grep ' lo ' $OUT/sar.log | awk 'NR>12{split($1,t,":"); s+=$6; mx=($6>mx)?$6:mx; c++} END {printf "samples=%d  lo_avg=%.0f MB/s (%.2f GB/s)  lo_peak=%.0f MB/s (%.2f GB/s)\n", c, s/c/1024, s/c/1024/1024, mx/1024, mx/1024/1024}'