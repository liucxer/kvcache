#!/bin/bash
# 128.12 读并发调优验证: 64/128/256 workers，观测磁盘读带宽饱和
set -e
cd /root
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
TS=$(date +%Y%m%d_%H%M%S)
echo "TS=$TS"

run_round() {
  local W=$1; shift
  local NAME=wr$W
  local OUT=/root/rdq_${NAME}_${TS}
  mkdir -p $OUT
  echo "===== ROUND $NAME (workers=$W) ====="

  for d in nvme1n1 nvme2n1 nvme3n1; do
    awk '{print $1}' /sys/block/$d/stat > $OUT/stat_rd_before_$d
    awk '{print $3}' /sys/block/$d/stat > $OUT/stat_sec_before_$d
  done
  for pid in $PIDS; do
    grep '^read_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
  done

  echo "=== drop caches ==="
  sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2
  free -g | head -2

  sar -n DEV 1 80 > $OUT/sar.log 2>&1 &
  SAR_PID=$!
  # pprof 仅最后大并发组采集
  if [ "$W" = "256" ]; then
    for port in 33001 33021 33041; do
      (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=57" -o $OUT/pprof_server_${port}.pb 2>/dev/null) &
    done
    (sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=57" -o $OUT/pprof_client.pb 2>/dev/null) &
  fi

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
  kill $SAR_PID 2>/dev/null
  sleep 1
  free -g | head -2

  echo "=== /sys/block stat DELTA ==="
  TOTAL_GB=0
  for d in nvme1n1 nvme2n1 nvme3n1; do
    RB=$(cat $OUT/stat_rd_before_$d); SB=$(cat $OUT/stat_sec_before_$d)
    RA=$(awk '{print $1}' /sys/block/$d/stat); SA=$(awk '{print $3}' /sys/block/$d/stat)
    GB=$(( (SA - SB) * 512 / 1024 / 1024 / 1024 ))
    TOTAL_GB=$((TOTAL_GB + GB))
    echo "$d: sectors_delta=$((SA - SB)) (~${GB} GB)"
  done
  echo "TOTAL disk read: ${TOTAL_GB} GB -> ~$(( TOTAL_GB / 60 )) GB/s"

  echo "=== /proc/PID/io DELTA (read_bytes, GB) ==="
  TOTAL_PROC=0
  for pid in $PIDS; do
    AFTER=$(grep '^read_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
    BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
    GB=$(( (AFTER - BEFORE)/1024/1024/1024 )); TOTAL_PROC=$((TOTAL_PROC + GB))
    echo "pid=$pid: $GB"
  done
  echo "TOTAL proc read: ${TOTAL_PROC} GB"

  echo "=== sar lo rx/tx avg ==="
  grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$4;s5+=$5;c++} END {printf "lo_rx_avg=%.0f MB/s  lo_tx_avg=%.0f MB/s\n", s4/c/1024, s5/c/1024}'
  echo "Output: $OUT"
  echo ""
}

run_round 64
sleep 5
run_round 128
sleep 5
run_round 256

echo "===== ALL DONE ====="
echo "TS=$TS"