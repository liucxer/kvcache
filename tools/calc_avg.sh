#!/bin/bash
OUT=/root/perf_big_1788512154

echo "=== DISK: active window (block 9-69, exclude 8s warmup + tail) ==="
awk '/^nvme5n1 /{d5=$3} /^nvme6n1 /{d6=$3} /^nvme7n1 /{d7=$3} /^nvme5n1 /{n++; if(n>=9 && n<=69){s5+=d5;s6+=d6;s7+=d7;c++}} END {printf "samples=%d  nvme5_avg=%.0f nvme6_avg=%.0f nvme7_avg=%.0f TOTAL_avg=%.0f MB/s = %.2f GB/s\n", c, s5/c, s6/c, s7/c, (s5+s6+s7)/c, (s5+s6+s7)/c/1024}' $OUT/iostat.log

echo ""
echo "=== DISK: full window avg (block 1-71 as-is) ==="
awk '/^nvme5n1 /{d5=$3} /^nvme6n1 /{d6=$3} /^nvme7n1 /{d7=$3} /^nvme5n1 /{n++; s5+=d5;s6+=d6;s7+=d7;c++} END {printf "samples=%d TOTAL_avg=%.0f MB/s = %.2f GB/s\n", c, (s5+s6+s7)/c, (s5+s6+s7)/c/1024}' $OUT/iostat.log

echo ""
echo "=== NET lo: active read window avg (05:00:39 - 05:01:34) ==="
grep ' lo ' $OUT/sar.log | awk -F'[: ]+' '{tm=$1*3600+$2*60+$3; if($4=="PM") tm+=43200; else if($4=="AM" && $1==12) tm-=43200; if(tm>=61239 && tm<=61294){s+=$8; c++}} END {printf "samples=%d  lo_avg=%.0f MB/s = %.2f GB/s (rx=tx)\n", c, s/c/1024, s/c/1024/1024}'