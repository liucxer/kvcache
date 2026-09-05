#!/bin/bash
# 128.12 kvcache 读调优：80 / 96 workers
set -e
cd /root

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
if [ -z "$PIDS" ]; then echo "ABORT: no kvcache"; exit 1; fi
echo "kvcache PIDs: $PIDS"

TS=$(date +%Y%m%d_%H%M%S)
echo "TS=$TS"

run_round() {
  local W=$1; shift
  local NAME=wr${W}
  local OUT=/root/kvrd_${NAME}_${TS}
  mkdir -p $OUT
  echo "===== ROUND $NAME (workers=$W) ====="

  # 权威计数器 before
  for d in nvme1n1 nvme2n1 nvme3n1; do
    awk '{print $3}' /sys/block/$d/stat > $OUT/stat_sec_before_$d
  done
  for pid in $PIDS; do
    grep '^read_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
  done

  # drop caches
  sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
  free -g | head -2

  # 监控
  iostat -xm 1 75 > $OUT/iostat.log 2>&1 &
  IOSTAT_PID=$!
  sar -n DEV 1 75 > $OUT/sar.log 2>&1 &
  SAR_PID=$!

  sleep 2
  START=$(date +%s)
  set +e
  timeout 70 ./bench --mode read --into --workers $W --duration 60s --value-size 4194304 \
    --prefix rdperf --read-max-seq 4600 --read-wid-mod 64 --node 128.12 \
    --tikv-pd 127.0.0.1:2379 --ca= --cert= --key= > $OUT/bench.log 2>&1
  RC=$?
  set -e
  echo "bench exit=$RC elapsed=$(( $(date +%s) - START ))s"
  grep -E 'Result|elapsed|errors|throughput|latency' $OUT/bench.log

  sleep 3
  kill $IOSTAT_PID $SAR_PID 2>/dev/null
  sleep 1
  free -g | head -2

  # 权威计数器 after
  echo "=== /sys/block stat DELTA ==="
  TOTAL_GB=0
  for d in nvme1n1 nvme2n1 nvme3n1; do
    SB=$(cat $OUT/stat_sec_before_$d)
    SA=$(awk '{print $3}' /sys/block/$d/stat)
    GB=$(( (SA - SB) * 512 / 1024 / 1024 / 1024 ))
    TOTAL_GB=$((TOTAL_GB + GB))
    echo "$d: ~${GB} GB"
  done
  echo "TOTAL disk read: ${TOTAL_GB} GB -> ~$(( TOTAL_GB / 60 )) GB/s"

  echo "=== /proc/PID/io DELTA ==="
  TOTAL_PROC=0
  for pid in $PIDS; do
    AFTER=$(grep '^read_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
    BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
    GB=$(( (AFTER - BEFORE)/1024/1024/1024 )); TOTAL_PROC=$((TOTAL_PROC + GB))
    echo "pid=$pid: $GB"
  done
  echo "TOTAL proc read: ${TOTAL_PROC} GB"

  echo "=== sar lo avg ==="
  grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$4;c++} END {printf "lo_rx_avg=%.0f MB/s (%.2f GB/s)\n", s4/c/1024, s4/c/1024/1024}'
  echo "Output: $OUT"
  echo ""
}

run_round 80
sleep 5
run_round 96

echo "===== ALL DONE ====="