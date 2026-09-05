#!/bin/bash
set -e
cd /root

# === SAFETY GATE ===
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi

# === Config ===
PREFIX="perftest"
VALUE_SIZE=4194304  # 4MB
WORKERS=64
COUNT_PER_WORKER=1200  # 64*1200*4MB = 307GB
DURATION=30s

# Get kvcache PIDs
PIDS=$(pgrep -f kvcache_perf | tr '\n' ' ')
if [ -z "$PIDS" ]; then echo "ABORT: no kvcache"; exit 1; fi
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
echo "kvcache PIDs: $PIDS"
echo "Prefix: $PREFIX, Workers: $WORKERS, Count/worker: $COUNT_PER_WORKER (~300GB)"

OUTDIR=/root/perf_$(date +%s)
mkdir -p $OUTDIR

# === WRITE ===
echo "WRITE ($(date))..."
START=$(date +%s)
timeout 300 ./bench --mode write --workers $WORKERS --count-per-worker $COUNT_PER_WORKER \
  --value-size $VALUE_SIZE --prefix $PREFIX --node 146 > $OUTDIR/bench_write.log 2>&1
echo "Write exit=$? elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors' $OUTDIR/bench_write.log

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
iostat -xm 1 30 > $OUTDIR/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 30 > $OUTDIR/sar.log 2>&1 &
SAR_PID=$!
# pprof on first kvcache instance
PPROF_PID=""
FIRST_PID=$(echo $PIDS | awk '{print $1}')
if [ -n "$FIRST_PID" ]; then
  (sleep 5; curl -s "http://$(cat /proc/$FIRST_PID/cmdline 2>/dev/null | tr '\0' '\n' | grep -oP 'pprof-addr=\K[^ ]+' 2>/dev/null || echo '127.0.0.1:16060')/debug/pprof/profile?seconds=25" -o $OUTDIR/pprof_cpu.pb 2>/dev/null) &
  PPROF_PID=$!
fi

# === READ ===
echo "READ ($PREFIX, --into, workers=$WORKERS, $DURATION)..."
START=$(date +%s)
set +e
timeout 60 ./bench --mode read --into --workers $WORKERS --duration $DURATION \
  --value-size $VALUE_SIZE --prefix $PREFIX \
  --read-max-seq $COUNT_PER_WORKER --read-wid-mod $WORKERS --node 146 \
  > $OUTDIR/bench_read.log 2>&1
READ_RC=$?
set -e
echo "Read exit=$READ_RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|latency' $OUTDIR/bench_read.log

# === STOP MONITORS ===
sleep 3; kill $PIDSTAT_PID $IOSTAT_PID $SAR_PID 2>/dev/null; wait 2>/dev/null; [ -n "$PPROF_PID" ] && wait $PPROF_PID 2>/dev/null || true

# === IO AFTER & DELTA ===
echo "=== /proc/PID/io DELTA ==="
TOTAL_DELTA=0
READ_SECONDS=$(grep -oP 'elapsed=\K[0-9.]+' $OUTDIR/bench_read.log 2>/dev/null || echo 30)
READ_SECONDS=$(echo $READ_SECONDS | sed 's/s//')
for pid in $PIDS; do
  AFTER=$(grep read_bytes /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUTDIR/io_before_$pid 2>/dev/null || echo 0)
  DELTA=$((AFTER - BEFORE)); TOTAL_DELTA=$((TOTAL_DELTA + DELTA))
  echo "pid=$pid: delta=$((DELTA/1024/1024/1024)) GB"
done
echo "TOTAL proc disk read: $((TOTAL_DELTA/1024/1024/1024)) GB in ${READ_SECONDS}s"
[ "$READ_SECONDS" -gt 0 ] && echo "Proc/io BW: $((TOTAL_DELTA/1024/1024/READ_SECONDS)) MB/s ($(echo "scale=2; $TOTAL_DELTA/1024/1024/1024/$READ_SECONDS" | bc) GB/s)"

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

echo "=== pprof ==="
ls -la $OUTDIR/pprof_cpu.pb 2>/dev/null || echo "no pprof"
echo "Output: $OUTDIR"