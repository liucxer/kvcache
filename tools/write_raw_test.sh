#!/bin/bash
set -e
cd /root

# === SAFETY GATE ===
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi
echo "No fio, safe"

# === Config ===
PREFIX="wraw"
VALUE_SIZE=4194304  # 4MB
WORKERS=64
WRITE_DURATION=60s

PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
if [ -z "$PIDS" ]; then echo "ABORT: no kvcache"; exit 1; fi
PIDS_CSV=$(echo $PIDS | tr ' ' ',')
echo "kvcache PIDs: $PIDS"

OUTDIR=/root/perf_wr_raw_$(date +%s)
mkdir -p $OUTDIR
echo "OUTDIR=$OUTDIR"

# === IO BEFORE (write_bytes) ===
for pid in $PIDS; do
  [ -f /proc/$pid/io ] && grep '^write_bytes' /proc/$pid/io | awk '{print $2}' > $OUTDIR/io_before_$pid
done

# === MONITORS ===
echo "Starting monitors..."
pidstat -d -p $PIDS_CSV 1 > $OUTDIR/pidstat.log 2>&1 &
PIDSTAT_PID=$!
iostat -xm 1 75 > $OUTDIR/iostat.log 2>&1 &
IOSTAT_PID=$!
sar -n DEV 1 75 > $OUTDIR/sar.log 2>&1 &
SAR_PID=$!

# === SERVER PPROF ===
SERVER_PPROF_PIDS=""
for port in 33001 33021 33031; do
  (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=55" -o $OUTDIR/pprof_server_${port}.pb 2>/dev/null) &
  SERVER_PPROF_PIDS="$SERVER_PPROF_PIDS $!"
done

sleep 2

# === WRITE with --raw (client pprof during write) ===
echo "WRITE --raw ($PREFIX, workers=$WORKERS, $WRITE_DURATION)..."
START=$(date +%s)
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=55" -o $OUTDIR/pprof_client.pb 2>/dev/null || echo "client pprof failed") &
CLIENT_PPROF_PID=$!
set +e
timeout 80 ./bench --mode write --raw --workers $WORKERS --duration $WRITE_DURATION \
  --value-size $VALUE_SIZE --prefix $PREFIX --node 146 > $OUTDIR/bench_write.log 2>&1
WRITE_RC=$?
set -e
echo "Write exit=$WRITE_RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|data=' $OUTDIR/bench_write.log

# === STOP MONITORS ===
sleep 3; kill $PIDSTAT_PID $IOSTAT_PID $SAR_PID 2>/dev/null; wait 2>/dev/null
for p in $SERVER_PPROF_PIDS; do wait $p 2>/dev/null || true; done
[ -n "$CLIENT_PPROF_PID" ] && wait $CLIENT_PPROF_PID 2>/dev/null || true

# === IO AFTER & DELTA (write_bytes) ===
echo "=== /proc/PID/io DELTA (write_bytes) ==="
TOTAL_DELTA=0
WRITE_SECONDS=$(grep -oP 'elapsed=\K[0-9.]+' $OUTDIR/bench_write.log 2>/dev/null || echo 60)
for pid in $PIDS; do
  AFTER=$(grep '^write_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUTDIR/io_before_$pid 2>/dev/null || echo 0)
  DELTA=$((AFTER - BEFORE)); TOTAL_DELTA=$((TOTAL_DELTA + DELTA))
  echo "pid=$pid: delta=$((DELTA/1024/1024/1024)) GB"
done
echo "TOTAL proc write: $((TOTAL_DELTA/1024/1024/1024)) GB in ${WRITE_SECONDS}s"

# === IOSTAT wMB/s (col 9) active window (skip 12s warmup) ===
echo "=== iostat wMB/s avg & peak ==="
awk '/^nvme5n1 /{d5=$9} /^nvme6n1 /{d6=$9} /^nvme7n1 /{d7=$9} /^nvme5n1 /{n++; if(n>=13){s5+=d5;s6+=d6;s7+=d7;c++; if(d5>p5)p5=d5; if(d6>p6)p6=d6; if(d7>p7)p7=d7; if(d5+d6+d7>pt)pt=d5+d6+d7}} END {printf "samples=%d\n", c; printf "AVG: nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", s5/c, s6/c, s7/c, (s5+s6+s7)/c, (s5+s6+s7)/c/1024; printf "PEAK: nvme5=%.0f nvme6=%.0f nvme7=%.0f TOTAL=%.0f MB/s (%.2f GB/s)\n", p5, p6, p7, pt, pt/1024}' $OUTDIR/iostat.log

# === sar lo avg (skip first 12s) ===
echo "=== sar lo avg ==="
grep ' lo ' $OUTDIR/sar.log | awk 'NR>13{s+=$6; mx=($6>mx)?$6:mx; c++} END {printf "lo_avg=%.0f MB/s (%.2f GB/s)  lo_peak=%.0f MB/s (%.2f GB/s) samples=%d\n", s/c/1024, s/c/1024/1024, mx/1024, mx/1024/1024, c}'

echo ""
echo "===== DONE ====="
echo "Output: $OUTDIR"