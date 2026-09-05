#!/bin/bash
set -e
cd /root

# === SAFETY GATE ===
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi
echo "No fio, safe to start"

# === Config ===
PREFIX="bigread2"
VALUE_SIZE=4194304  # 4MB
WORKERS=64
COUNT_PER_WORKER=5000  # 64*5000*4MB = 1280GB
WRITE_DURATION=300s  # 确保 count-per-worker 全部写完
DURATION=60s

# Get kvcache PIDs
PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
if [ -z "$PIDS" ]; then echo "ABORT: no kvcache"; exit 1; fi
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
echo "kvcache PIDs: $PIDS"
echo "Prefix: $PREFIX, Workers: $WORKERS, Count/worker: $COUNT_PER_WORKER (~768GB)"

OUTDIR=/root/perf_big_$(date +%s)
mkdir -p $OUTDIR
echo "OUTDIR=$OUTDIR"

# === WRITE ===
echo "WRITE ($(date))..."
START=$(date +%s)
timeout 500 ./bench --mode write --workers $WORKERS --count-per-worker $COUNT_PER_WORKER \
  --duration $WRITE_DURATION \
  --value-size $VALUE_SIZE --prefix $PREFIX --node 146 > $OUTDIR/bench_write.log 2>&1
echo "Write exit=$? elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|data=' $OUTDIR/bench_write.log

# Check errors
WRITE_ERR=$(grep -c 'Set failed' $OUTDIR/bench_write.log 2>/dev/null || echo 0)
echo "Write errors: $WRITE_ERR"
if [ "$WRITE_ERR" -gt 0 ]; then echo "ABORT: write errors"; exit 1; fi

# Verify key count via scan
echo "Verifying keys..."
for port in 33001 33021 33031; do
  c=$(curl -s "http://127.0.0.1:${port}/api/v1/scan?prefix=${PREFIX}&limit=1" | python3 -c "import sys,json; print(json.load(sys.stdin).get('count',0))" 2>/dev/null)
  echo "  :$port -> $PREFIX count=$c"
done

# === DROP CACHE ===
echo "Dropping page cache..."
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 2; free -g | head -2

# === IO BEFORE ===
echo "=== /proc/PID/io BEFORE ==="
for pid in $PIDS; do
  [ -f /proc/$pid/io ] && grep read_bytes /proc/$pid/io | awk "{print \"pid=$pid \" \$0}" && grep read_bytes /proc/$pid/io | awk '{print $2}' > $OUTDIR/io_before_$pid
done

# === MONITORS ===
echo "Starting monitors..."
pidstat -d -p $PIDS_CSV 1 > $OUTDIR/pidstat.log 2>&1 &
PIDSTAT_PID=$!
iostat -xm 1 75 > $OUTDIR/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 75 > $OUTDIR/sar.log 2>&1 &
SAR_PID=$!

# === SERVER PPROF (all 3 instances via HTTP API port) ===
echo "Starting server pprof collection..."
SERVER_PPROF_PIDS=""
for port in 33001 33021 33031; do
  echo "  Server HTTP port=$port pprof=127.0.0.1:$port"
  (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=57" -o $OUTDIR/pprof_server_${port}.pb 2>/dev/null) &
  SERVER_PPROF_PIDS="$SERVER_PPROF_PIDS $!"
done

sleep 2

# === READ ===
echo "READ ($PREFIX, --into, workers=$WORKERS, $DURATION)..."
START=$(date +%s)
set +e
# 读取进行中采集客户端(bench :16060) pprof，覆盖 57s（读取 60s）
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=57" -o $OUTDIR/pprof_client.pb 2>/dev/null || echo "client pprof failed") &
CLIENT_PPROF_PID=$!
timeout 70 ./bench --mode read --into --workers $WORKERS --duration $DURATION \
  --value-size $VALUE_SIZE --prefix $PREFIX \
  --read-max-seq $COUNT_PER_WORKER --read-wid-mod $WORKERS --node 146 \
  > $OUTDIR/bench_read.log 2>&1
READ_RC=$?
set -e
echo "Read exit=$READ_RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|latency|data=' $OUTDIR/bench_read.log

# === STOP MONITORS ===
sleep 3; kill $PIDSTAT_PID $IOSTAT_PID $SAR_PID 2>/dev/null; wait 2>/dev/null
for p in $SERVER_PPROF_PIDS; do wait $p 2>/dev/null || true; done
[ -n "$CLIENT_PPROF_PID" ] && wait $CLIENT_PPROF_PID 2>/dev/null || true

# === IO AFTER & DELTA ===
echo "=== /proc/PID/io DELTA ==="
TOTAL_DELTA=0
READ_SECONDS=$(grep -oP 'elapsed=\K[0-9.]+' $OUTDIR/bench_read.log 2>/dev/null || echo 60)
READ_SECONDS=$(echo $READ_SECONDS | sed 's/s//')
for pid in $PIDS; do
  AFTER=$(grep read_bytes /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUTDIR/io_before_$pid 2>/dev/null || echo 0)
  DELTA=$((AFTER - BEFORE)); TOTAL_DELTA=$((TOTAL_DELTA + DELTA))
  echo "pid=$pid: delta=$((DELTA/1024/1024/1024)) GB"
done
echo "TOTAL proc disk read: $((TOTAL_DELTA/1024/1024/1024)) GB in ${READ_SECONDS}s"

# === PIDSTAT SUMMARY ===
echo "=== pidstat peak kB_rd/s ==="
for pid in $PIDS; do
  peak=$(awk -v p=$pid '$3==p && $4>max {max=$4} END {print max+0}' $OUTDIR/pidstat.log 2>/dev/null)
  echo "pid=$pid peak=${peak} kB/s ($((peak/1024)) MB/s)"
done

echo "=== iostat peak ==="
for dev in nvme5n1 nvme6n1 nvme7n1; do
  peak=$(awk -v d=$dev '$1==d && $3>max {max=$3} END {print max+0}' $OUTDIR/iostat.log 2>/dev/null)
  echo "$dev peak=${peak} MB/s"
done

echo "=== sar network ==="
grep -E 'bond1|eth0' $OUTDIR/sar.log | tail -5

# === PPROF ANALYSIS ===
echo ""
echo "=== CLIENT PPROF TOP ==="
go tool pprof -top $OUTDIR/pprof_client.pb 2>/dev/null | head -30 || echo "no client pprof"

echo ""
echo "=== SERVER PPROF TOP (each instance) ==="
for f in $OUTDIR/pprof_server_*.pb; do
  if [ -f "$f" ]; then
    echo "--- $(basename $f) ---"
    go tool pprof -top "$f" 2>/dev/null | head -20 || echo "pprof failed for $f"
    echo
  fi
done

echo ""
echo "===== DONE ====="
echo "Output: $OUTDIR"
