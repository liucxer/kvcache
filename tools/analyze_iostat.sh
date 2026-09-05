#!/bin/bash
OUT=/root/perf_big_1788512154

echo "=== iostat nvme disks: per-sample rMB/s (blocks = 1s each) ==="
awk '/^nvme5n1 /{d5=$3} /^nvme6n1 /{d6=$3} /^nvme7n1 /{d7=$3} /^nvme5n1 /{n++; if(n>3) printf "%3d  nvme5=%6.0f  nvme6=%6.0f  nvme7=%6.0f  TOTAL=%6.0f MB/s\n", n-3, d5, d6, d7, d5+d6+d7}' $OUT/iostat.log

echo ""
echo "=== iostat averages & peaks (excluding warmup 3s) ==="
awk '/^nvme5n1 /{d5=$3} /^nvme6n1 /{d6=$3} /^nvme7n1 /{d7=$3} /^nvme5n1 /{n++; if(n>3){s5+=d5;s6+=d6;s7+=d7;sum+=d5+d6+d7;c++; if(d5>p5)p5=d5; if(d6>p6)p6=d6; if(d7>p7)p7=d7; if(d5+d6+d7>ptot)ptot=d5+d6+d7}} END {printf "samples=%d\n", c; printf "AVG: nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s5/c, s6/c, s7/c, sum/c, sum/c/1024; printf "PEAK: nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p5, p6, p7, ptot, ptot/1024}' $OUT/iostat.log