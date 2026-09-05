#!/bin/bash
# 干净的读测试（服务端 sendfile 路径），带 /sys/block stat 权威计数器
set -e
cd /root
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/rd2_${TS}
mkdir -p $OUT

# 权威计数器 before
for d in nvme1n1 nvme2n1 nvme3n1; do
  [ -f /sys/block/$d/stat ] && awk '{print $1}' /sys/block/$d/stat > $OUT/stat_rd_before_$d
  [ -f /sys/block/$d/stat ] && awk '{print $3}' /sys/block/$d/stat > $OUT/stat_sec_before_$d
done
for pid in $PIDS; do
  [ -f /proc/$pid/io ] && grep '^read_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
done

echo "=== drop caches ==="
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

pidstat -d -p $PIDS_CSV 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!
iostat -xm 1 80 > $OUT/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 80 > $OUT/sar.log 2>&1 &
SAR_PID=$!

for port in 33001 33021 33041; do
  (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=57" -o $OUT/pprof_server_${port}.pb 2>/dev/null) &
done
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=57" -o $OUT/pprof_client.pb 2>/dev/null || echo "client pprof failed") &

sleep 2
echo "=== READ ==="
START=$(date +%s)
set +e
timeout 70 ./bench --mode read --into --workers 64 --duration 60s --value-size 4194304 \
  --prefix rdperf --read-max-seq 4600 --read-wid-mod 64 --node 128.12 \
  --tikv-pd 127.0.0.1:2379 --ca= --cert= --key= > $OUT/bench.log 2>&1
RC=$?
set -e
echo "read exit=$RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|throughput|latency' $OUT/bench.log

sleep 3
kill $PIDSTAT_PID $IOSTAT_PID $SAR_PID 2>/dev/null
sleep 1
free -g | head -2

echo "=== /sys/block stat DELTA (reads completed / sectors read) ==="
for d in nvme1n1 nvme2n1 nvme3n1; do
  RB=$(cat $OUT/stat_rd_before_$d); SB=$(cat $OUT/stat_sec_before_$d)
  RA=$(awk '{print $1}' /sys/block/$d/stat); SA=$(awk '{print $3}' /sys/block/$d/stat)
  echo "$d: reads_delta=$((RA - RB)) sectors_delta=$((SA - SB)) (~$(( (SA - SB) * 512 / 1024 / 1024 / 1024 )) GB)"
done

echo "=== /proc/PID/io DELTA (read_bytes) ==="
for pid in $PIDS; do
  AFTER=$(grep '^read_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
  echo "pid=$pid: delta=$(( (AFTER - BEFORE)/1024/1024/1024 )) GB"
done

echo "=== iostat rMB/s (col2) avg & peak ==="
awk '/^nvme1n1 /{d1=$2} /^nvme2n1 /{d2=$2} /^nvme3n1 /{d3=$2} /^nvme1n1 /{n++; if(n>=8){s1+=d1;s2+=d2;s3+=d3;c++; if(d1>p1)p1=d1; if(d2>p2)p2=d2; if(d3>p3)p3=d3}} END {printf "samples=%d\n", c; printf "AVG nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s1/c, s2/c, s3/c, (s1+s2+s3)/c, (s1+s2+s3)/c/1024; printf "PEAK nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p1, p2, p3, (p1+p2+p3), (p1+p2+p3)/1024}' $OUT/iostat.log

echo "=== sar lo rx/tx avg ==="
grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$4;s5+=$5;c++} END {printf "lo_rx_avg=%.0f MB/s  lo_tx_avg=%.0f MB/s\n", s4/c/1024, s5/c/1024}'
echo "Output: $OUT"
echo "===== DONE ====="