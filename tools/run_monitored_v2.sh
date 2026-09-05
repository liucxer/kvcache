#!/bin/bash
# Comprehensive monitored read test for 146 (shared node safe)
# Safety gate: aborts if fio is running (others doing perf test)
# Process-level monitoring: pidstat -d + /proc/PID/io + pprof
set -e
cd /root

# === SAFETY GATE: check fio ===
if pgrep -x fio >/dev/null 2>&1; then
  echo "ABORT: fio process detected, someone else is running perf test. Cannot run."
  pgrep -a fio
  exit 1
fi
EXTRA=$(ps aux | grep '[f]io ' | grep -v vfio | grep -v grep | head -3)
if [ -n "$EXTRA" ]; then
  echo "ABORT: fio-like process detected: $EXTRA"
  exit 1
fi
echo "SAFETY GATE PASSED: no fio process running"

# === Config ===
PREFIX="${1:-bigdata_v2}"
VALUE_SIZE=4194304
READ_MAX_SEQ=1200
READ_WID_MOD=64
WORKERS=64
DURATION=30s

# Dynamic kvcache PIDs (may change after restart)
PIDS_LIST=$(pgrep -f kvcache_perf | tr '\n' ' ')
if [ -z "$PIDS_LIST" ]; then
  echo "ABORT: no kvcache_perf process found"
  exit 1
fi
PIDS_CSV=$(echo $PIDS_LIST | tr ' ' ',')
echo "kvcache PIDs: $PIDS_LIST"

OUTDIR=/root/perf_test_$(date +%s)
mkdir -p $OUTDIR
echo "Output dir: $OUTDIR"

# === 1. Write complete dataset ===
echo ""
echo "=========================================="
echo "WRITE PHASE: $WORKERS workers x $READ_MAX_SEQ keys x 4MB = ~300GB"
echo "=========================================="
WRITE_START=$(date +%s)
set +e
timeout 600 ./bench --mode write --workers $WORKERS --count-per-worker $READ_MAX_SEQ \
  --value-size $VALUE_SIZE --prefix $PREFIX --node 146 > $OUTDIR/bench_write.log 2>&1
WRITE_RC=$?
set -e
WRITE_END=$(date +%s)
echo "Write exit code: $WRITE_RC, elapsed: $((WRITE_END - WRITE_START))s"
grep -E 'Result|elapsed|data=|note:' $OUTDIR/bench_write.log || true

# Verify 0 write errors
WRITE_ERRORS=$(grep -c 'Set failed' $OUTDIR/bench_write.log 2>/dev/null || echo 0)
if [ "$WRITE_ERRORS" -gt 0 ]; then
  echo "ABORT: Write had $WRITE_ERRORS errors, data incomplete"
  grep 'Set failed' $OUTDIR/bench_write.log | head -5
  exit 1
fi
echo "Write verified: 0 errors"

# === 2. Drop page cache ===
echo ""
echo "=== Dropping page cache ==="
sync
echo 3 > /proc/sys/vm/drop_caches
sleep 2
free -g | head -2

# === 3. Record /proc/PID/io before read ===
echo ""
echo "=== Recording /proc/PID/io (read_bytes) BEFORE read ==="
for pid in $PIDS_LIST; do
  if [ -f /proc/$pid/io ]; then
    BEFORE=$(grep read_bytes /proc/$pid/io | awk '{print $2}')
    echo "pid=$pid read_bytes_before=$BEFORE"
    echo "$BEFORE" > $OUTDIR/io_before_$pid
  fi
done

# === 4. Start background monitors ===
echo ""
echo "Starting background monitors..."
# Process-level disk I/O (isolated from other users)
pidstat -d -p $PIDS_CSV 1 > $OUTDIR/pidstat.log 2>&1 &
PIDSTAT_PID=$!

# pprof on bench client :16060
sleep 1
(
  sleep 3
  curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=25" -o $OUTDIR/pprof_cpu.pb 2>/dev/null || true
) &
PPROF_PID=$!

# System-level (labeled as polluted by shared load)
iostat -xm 1 > $OUTDIR/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 > $OUTDIR/sar.log 2>&1 &
SAR_PID=$!

# === 5. Read test ===
echo ""
echo "=========================================="
echo "READ TEST: workers=$WORKERS duration=$DURATION prefix=$PREFIX"
echo "=========================================="
READ_START=$(date +%s)
set +e
timeout 60 ./bench --mode read --into --workers $WORKERS --duration $DURATION \
  --value-size $VALUE_SIZE --prefix $PREFIX \
  --read-max-seq $READ_MAX_SEQ --read-wid-mod $READ_WID_MOD --node 146 \
  > $OUTDIR/bench_read.log 2>&1
READ_RC=$?
set -e
READ_END=$(date +%s)
READ_SECONDS=$((READ_END - READ_START))
echo "Read test exit code: $READ_RC, elapsed: ${READ_SECONDS}s"

# === 6. Stop monitors ===
sleep 3
kill $PIDSTAT_PID 2>/dev/null || true
kill $IOSTAT_PID 2>/dev/null || true
kill $SAR_PID 2>/dev/null || true
wait $PIDSTAT_PID 2>/dev/null || true
wait $IOSTAT_PID 2>/dev/null || true
wait $SAR_PID 2>/dev/null || true
[ -n "$PPROF_PID" ] && wait $PPROF_PID 2>/dev/null || true

# === 7. Record /proc/PID/io after read and calculate delta ===
echo ""
echo "=== /proc/PID/io deltas (process-level disk read bytes) ==="
TOTAL_DELTA=0
for pid in $PIDS_LIST; do
  if [ -f /proc/$pid/io ]; then
    AFTER=$(grep read_bytes /proc/$pid/io | awk '{print $2}')
    BEFORE=$(cat $OUTDIR/io_before_$pid 2>/dev/null || echo 0)
    DELTA=$((AFTER - BEFORE))
    TOTAL_DELTA=$((TOTAL_DELTA + DELTA))
    echo "pid=$pid: delta=$DELTA bytes ($(echo "scale=2; $DELTA/1024/1024/1024" | bc 2>/dev/null || echo '?') GB)"
  fi
done
echo "TOTAL process-level disk read: $TOTAL_DELTA bytes"
if [ "$READ_SECONDS" -gt 0 ]; then
  echo "Proc/io bandwidth: $(echo "scale=2; $TOTAL_DELTA/1024/1024/$READ_SECONDS" | bc 2>/dev/null || echo '?') MB/s"
fi

# === 8. Summary ===
echo ""
echo "=========================================="
echo "BENCH RESULT"
echo "=========================================="
grep -E 'Result|elapsed|data=|latency|note:' $OUTDIR/bench_read.log
READ_ERRORS=$(grep -c 'Get failed' $OUTDIR/bench_read.log 2>/dev/null || echo 0)
echo "Read errors: $READ_ERRORS"
[ "$READ_ERRORS" -gt 0 ] && grep 'Get failed' $OUTDIR/bench_read.log | sort | uniq -c | head -3

echo ""
echo "=========================================="
echo "PROCESS-LEVEL DISK I/O (pidstat, isolated)"
echo "=========================================="
echo "Per-process peak kB_rd/s:"
for pid in $PIDS_LIST; do
  PEAK=$(awk -v p=$pid '$3==p {if($4>max) max=$4} END {print max+0}' $OUTDIR/pidstat.log 2>/dev/null)
  echo "  pid=$pid: peak=${PEAK} kB_rd/s ($(echo "scale=1; $PEAK/1024" | bc 2>/dev/null || echo '?') MB/s)"
done

echo ""
echo "=========================================="
echo "SYSTEM-LEVEL (polluted by shared load: essd+nefs)"
echo "=========================================="
echo "Peak rMB/s for kvcache disks:"
for dev in nvme5n1 nvme6n1 nvme7n1; do
  PEAK=$(awk -v d=$dev '$1==d && $3>max {max=$3} END {print max+0}' $OUTDIR/iostat.log 2>/dev/null)
  echo "  $dev: peak=${PEAK} MB/s"
done

echo ""
echo "=========================================="
echo "pprof (client :16060)"
echo "=========================================="
ls -la $OUTDIR/pprof_cpu.pb 2>/dev/null || echo "pprof not collected"

echo ""
echo "Output dir: $OUTDIR"
echo "=== DONE ==="
