#!/bin/bash
# 用正确列(iostat rMB/s=col2)解析读测试日志
D=$1
echo "=== iostat rMB/s (col2) ==="
gawk '/^nvme1n1 /{d1=$2} /^nvme2n1 /{d2=$2} /^nvme3n1 /{d3=$2} /^nvme1n1 /{n++; if(n>=13){s1+=d1;s2+=d2;s3+=d3;c++; if(d1>p1)p1=d1; if(d2>p2)p2=d2; if(d3>p3)p3=d3; if(d1+d2+d3>pt)pt=d1+d2+d3}} END {printf "samples=%d\n", c; printf "AVG nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s1/c, s2/c, s3/c, (s1+s2+s3)/c, (s1+s2+s3)/c/1024; printf "PEAK nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p1, p2, p3, pt, pt/1024}' $D/iostat.log

echo "=== sar lo tx/rx ==="
grep ' lo ' $D/sar.log | gawk 'NR>13{s+=$5; if($5>mx)mx=$5; c++} END {printf "lo_tx_avg=%.0f MB/s (%.2f GB/s) peak=%.0f MB/s (%.2f GB/s)\n", s/c/1024, s/c/1024/1024, mx/1024, mx/1024/1024}'
grep ' lo ' $D/sar.log | gawk 'NR>13{s+=$4; if($4>mx)mx=$4; c++} END {printf "lo_rx_avg=%.0f MB/s (%.2f GB/s) peak=%.0f MB/s (%.2f GB/s)\n", s/c/1024, s/c/1024/1024, mx/1024, mx/1024/1024}'
echo "=== per-device sample check (max rMB/s lines) ==="
grep 'nvme1n1' $D/iostat.log | sort -t' ' -k2,2nr | head -3
echo done