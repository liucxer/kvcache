#!/bin/bash
# 写路径对照验证: gRPC--raw(基线) vs raw-write batch=1 vs raw-write batch=8
# 146 本机 loopback, value=4MB, workers=64, duration=100s
set -e
cd /root

# === SAFETY GATE ===
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi
echo "No fio, safe"

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
echo "kvcache PIDs: $PIDS"

BASE=/root
TS=$(date +%Y%m%d_%H%M%S)
echo "TS=$TS"

run_round() {
  local NAME=$1; shift
  local PREFIX=$1; shift
  local EXTRA_ARGS=("$@")
  local OUT=$BASE/wcmp_${NAME}_${TS}
  mkdir -p $OUT
  echo "===== ROUND $NAME (prefix=$PREFIX, extra=${EXTRA_ARGS[*]}) ====="

  for pid in $PIDS; do
    [ -f /proc/$pid/io ] && grep '^write_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid
  done

  pidstat -d -p $PIDS_CSV 1 > $OUT/pidstat.log 2>&1 &
  PIDSTAT_PID=$!
  iostat -xm 1 125 > $OUT/iostat.log 2>&1 &
  IOSTAT_PID=$!
  sar -n DEV 1 125 > $OUT/sar.log 2>&1 &
  SAR_PID=$!

  # 服务端 pprof（gRPC 端口+1 为 HTTP pprof 端口: 33001/33021/33031），覆盖窗口 ~88s
  for port in 33001 33021 33031; do
    (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=88" -o $OUT/pprof_server_${port}.pb 2>/dev/null) &
  done
  (sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=88" -o $OUT/pprof_client.pb 2>/dev/null || echo "client pprof skipped (old bench?)") &
  CLIENT_PPROF_PID=$!

  sleep 2
  START=$(date +%s)
  set +e
  timeout 115 ./bench --mode write --workers 64 --duration 100s --value-size 4194304 \
    --prefix $PREFIX "${EXTRA_ARGS[@]}" > $OUT/bench.log 2>&1
  RC=$?
  set -e
  echo "bench exit=$RC elapsed=$(( $(date +%s) - START ))s"
  grep -E 'Result|elapsed|errors|throughput|rawWrite|batch' $OUT/bench.log

  sleep 3; kill $PIDSTAT_PID $IOSTAT_PID $SAR_PID 2>/dev/null; wait 2>/dev/null
  [ -n "$CLIENT_PPROF_PID" ] && wait $CLIENT_PPROF_PID 2>/dev/null || true

  echo "=== /proc/PID/io DELTA (write_bytes) ==="
  TOTAL_DELTA=0
  for pid in $PIDS; do
    AFTER=$(grep '^write_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
    BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
    DELTA=$((AFTER - BEFORE)); TOTAL_DELTA=$((TOTAL_DELTA + DELTA))
    echo "pid=$pid: delta=$((DELTA/1024/1024/1024)) GB"
  done
  echo "TOTAL proc write: $((TOTAL_DELTA/1024/1024/1024)) GB"

  echo "=== iostat wMB/s avg & peak (skip 12s warmup) ==="
  awk '/^nvme5n1 /{d5=$9} /^nvme6n1 /{d6=$9} /^nvme7n1 /{d7=$9} /^nvme5n1 /{n++; if(n>=13){s5+=d5;s6+=d6;s7+=d7;c++; if(d5>p5)p5=d5; if(d6>p6)p6=d6; if(d7>p7)p7=d7; if(d5+d6+d7>pt)pt=d5+d6+d7}} END {printf "samples=%d\n", c; printf "AVG nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s5/c, s6/c, s7/c, (s5+s6+s7)/c, (s5+s6+s7)/c/1024; printf "PEAK nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p5, p6, p7, pt, pt/1024}' $OUT/iostat.log

  echo "=== sar lo avg & peak ==="
  grep ' lo ' $OUT/sar.log | awk 'NR>13{s+=$6; if($6>mx)mx=$6; c++} END {printf "lo_avg=%.0f MB/s (%.2f GB/s)  lo_peak=%.0f MB/s (%.2f GB/s)\n", s/c/1024, s/c/1024/1024, mx/1024, mx/1024/1024}'

  echo "Output: $OUT"
  echo ""
}

# 组A: gRPC --raw（基线，direct 直连 3 实例 gRPC 端口，round-robin）
GRPC_ADDRS="127.0.0.1:33000,127.0.0.1:33020,127.0.0.1:33030"
# 组B/C: raw-write 数据面（direct 指定 raw-write 端口，每 worker 固定连接）
RAWWR_ADDRS="127.0.0.1:29301,127.0.0.1:29321,127.0.0.1:29331"

run_round grpc_raw vwgrpc --raw --direct $GRPC_ADDRS
sleep 5
run_round rawwrite_b1 vwraw1 --raw-write --direct $RAWWR_ADDRS --batch 1
sleep 5
run_round rawwrite_b8 vwraw8 --raw-write --direct $RAWWR_ADDRS --batch 8

echo "===== ALL DONE ====="
echo "TS=$TS"