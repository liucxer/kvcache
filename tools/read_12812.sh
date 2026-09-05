#!/bin/bash
# 128.12 读性能测试（sendfile / --into 路径）
set -e
cd /root

if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/rdread_${TS}
mkdir -p $OUT
echo "TS=$TS out=$OUT"

# 读前 io 基线
for pid in $PIDS; do
  [ -f /proc/$pid/io ] && grep '^read_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
done

echo "=== drop caches ==="
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

# 监控（读 60s，采集 75s）
pidstat -d -p $PIDS_CSV 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!
iostat -xm 1 80 > $OUT/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 80 > $OUT/sar.log 2>&1 &
SAR_PID=$!

# pprof：服务端 33001/33021/33041（HTTP pprof），客户端 16060
for port in 33001 33021 33041; do
  (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=57" -o $OUT/pprof_server_${port}.pb 2>/dev/null) &
done
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=57" -o $OUT/pprof_client.pb 2>/dev/null || echo "client pprof failed") &
CLIENT_PPROF_PID=$!

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
[ -n "$CLIENT_PPROF_PID" ] && wait $CLIENT_PPROF_PID 2>/dev/null || true
sleep 1

echo "=== /proc/PID/io DELTA (read_bytes) ==="
TOTAL_DELTA=0
for pid in $PIDS; do
  AFTER=$(grep '^read_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
  DELTA=$((AFTER - BEFORE)); TOTAL_DELTA=$((TOTAL_DELTA + DELTA))
  echo "pid=$pid: delta=$((DELTA/1024/1024/1024)) GB"
done
echo "TOTAL proc read: $((TOTAL_DELTA/1024/1024/1024)) GB"

echo "=== iostat rMB/s (col4) avg & peak (skip 10s warmup) ==="
awk '/^nvme1n1 /{d1=$4} /^nvme2n1 /{d2=$4} /^nvme3n1 /{d3=$4} /^nvme1n1 /{n++; if(n>=11){s1+=d1;s2+=d2;s3+=d3;c++; if(d1>p1)p1=d1; if(d2>p2)p2=d2; if(d3>p3)p3=d3; if(d1+d2+d3>pt)pt=d1+d2+d3}} END {printf "samples=%d\n", c; printf "AVG nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s1/c, s2/c, s3/c, (s1+s2+s3)/c, (s1+s2+s3)/c/1024; printf "PEAK nvme1=%.0f nvme2=%.0f nvme3=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p1, p2, p3, pt, pt/1024}' $OUT/iostat.log

echo "=== sar lo avg rx ==="
grep ' lo ' $OUT/sar.log | awk 'NR>11{s+=$5; if($5>mx)mx=$5; c++} END {printf "lo_rx_avg=%.0f MB/s (%.2f GB/s)  lo_rx_peak=%.0f MB/s (%.2f GB/s)\n", s/c/1024, s/c/1024/1024, mx/1024, mx/1024/1024}'

echo "Output: $OUT"
echo "===== READ TEST DONE ====="