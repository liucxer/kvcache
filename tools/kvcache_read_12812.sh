#!/bin/bash
# 128.12 kvcache 读性能测试（sendfile --into 路径）
set -e
cd /root

# === 1. 安全检查 ===
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi
PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
if [ -z "$PIDS" ]; then echo "ABORT: no kvcache_perf"; exit 1; fi
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
echo "kvcache PIDs: $PIDS"

TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/kvread_${TS}
mkdir -p $OUT
echo "OUT=$OUT"

# === 2. 权威计数器 before ===
for d in nvme1n1 nvme2n1 nvme3n1; do
  awk '{print $1}' /sys/block/$d/stat > $OUT/stat_rd_before_$d
  awk '{print $3}' /sys/block/$d/stat > $OUT/stat_sec_before_$d
done
for pid in $PIDS; do
  [ -f /proc/$pid/io ] && grep '^read_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
done
awk '/^  lo:/{print $2, $10}' /proc/net/dev > $OUT/net_before

# === 3. drop caches ===
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
free -g | head -2

# === 4. 启动监控 ===
iostat -xm 1 75 > $OUT/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 75 > $OUT/sar.log 2>&1 &
SAR_PID=$!
pidstat -d -p $PIDS_CSV 1 > $OUT/pidstat.log 2>&1 &
PIDSTAT_PID=$!

# === 5. pprof ===
for port in 33001 33021 33041; do
  (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=57" -o $OUT/pprof_server_${port}.pb 2>/dev/null) &
done
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=57" -o $OUT/pprof_client.pb 2>/dev/null || echo "client pprof failed") &

sleep 2

# === 6. 读测试 ===
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

# === 7. 停止监控 ===
sleep 3
kill $IOSTAT_PID $SAR_PID $PIDSTAT_PID 2>/dev/null
sleep 1
free -g | head -2

# === 8. 权威计数器 after & delta ===
echo ""
echo "=== /sys/block stat DELTA (权威磁盘读) ==="
TOTAL_GB=0
for d in nvme1n1 nvme2n1 nvme3n1; do
  RB=$(cat $OUT/stat_rd_before_$d); SB=$(cat $OUT/stat_sec_before_$d)
  RA=$(awk '{print $1}' /sys/block/$d/stat); SA=$(awk '{print $3}' /sys/block/$d/stat)
  GB=$(( (SA - SB) * 512 / 1024 / 1024 / 1024 ))
  TOTAL_GB=$((TOTAL_GB + GB))
  echo "$d: sectors_delta=$((SA - SB)) (~${GB} GB)"
done
echo "TOTAL disk read: ${TOTAL_GB} GB -> ~$(( TOTAL_GB / 60 )) GB/s"

echo ""
echo "=== /proc/PID/io DELTA (read_bytes) ==="
TOTAL_PROC=0
for pid in $PIDS; do
  AFTER=$(grep '^read_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
  GB=$(( (AFTER - BEFORE)/1024/1024/1024 )); TOTAL_PROC=$((TOTAL_PROC + GB))
  echo "pid=$pid: $GB GB"
done
echo "TOTAL proc read: ${TOTAL_PROC} GB"

echo ""
echo "=== /proc/net/dev lo DELTA ==="
awk '/^  lo:/{print $2, $10}' /proc/net/dev > $OUT/net_after
cat $OUT/net_before $OUT/net_after | python3 -c "
import sys
lines=sys.stdin.read().strip().split('\n')
b=lines[0].split(); a=lines[1].split()
rx=(int(a[0])-int(b[0]))/1024/1024/1024
tx=(int(a[1])-int(b[1]))/1024/1024/1024
print(f'lo rx_delta={rx:.1f}GB tx_delta={tx:.1f}GB  avg={rx/60:.2f} GB/s')
"

echo ""
echo "=== sar lo avg (skip 8s warmup) ==="
grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$4;s5+=$5;c++} END {printf "lo_rx_avg=%.0f MB/s  lo_tx_avg=%.0f MB/s\n", s4/c/1024, s5/c/1024}'

echo ""
echo "Output: $OUT"
echo "===== READ TEST DONE ====="