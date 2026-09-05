#!/bin/bash
# Full big read test: write 750GB + 60s read + drop cache + pprof (client + server)

set -e

# 1. Create output dir
OUTDIR=/root/perf_big_$(date +%s)
mkdir -p $OUTDIR
echo "OUTDIR=$OUTDIR"
echo

# 2. Check fio
echo "=== Check fio ==="
pgrep -x fio >/dev/null && echo "FIO_RUNNING_ABORT" && exit 1
echo "NO_FIO_SAFE"
echo

# 3. Show current memory
echo "=== Memory before ==="
free -h
echo

# 4. Get server PIDs (kvcache_perf instances)
SERVER_PIDS=$(pgrep kvcache_perf)
echo "=== Server PIDs found: $SERVER_PIDS ==="
echo

# 5. Write 750GB (64 workers × 3000 keys × 4MB)
echo "=== WRITE START: $(date) ==="
cd /root && ./kvcache_bench \
  -tikv-pd=10.153.28.202:12379,10.153.28.203:12379,10.153.28.204:12379 \
  -mode=write \
  -workers=64 \
  -count=3000 \
  -valueSize=4194304 \
  -prefix=bigread \
  2>&1 | tee $OUTDIR/bench_write.log
echo
echo "WRITE exit code: $?"
echo "=== WRITE END: $(date) ==="
echo

# 6. Verify key count
echo "=== Verify key count ==="
./probe_keys.sh 2>/dev/null | grep bigread
echo

# 7. Show memory after write
echo "=== Memory after write ==="
free -h
echo

# 8. Drop page cache
echo "=== Drop page cache ==="
echo 3 > /proc/sys/vm/drop_caches
sleep 5
echo "done after 5s sleep"
free -h
echo

# 9. Record /proc/pid/io before (servers)
echo "=== Record io before (servers) ==="
for pid in $SERVER_PIDS; do
  cat /proc/$pid/io > $OUTDIR/io_before_server_$pid
done

# 10. Start system monitors
echo "=== Start system monitors ==="
iostat -x -d 1 nvme5n1 nvme6n1 nvme7n1 > $OUTDIR/iostat.log 2>&1 &
IOSTAT_PID=$!
echo "iostat pid: $IOSTAT_PID"
pidstat -d 1 > $OUTDIR/pidstat.log 2>&1 &
PIDSTAT_PID=$!
echo "pidstat pid: $PIDSTAT_PID"
sar -n DEV 1 > $OUTDIR/sar.log 2>&1 &
SAR_PID=$!
echo "sar pid: $SAR_PID"

sleep 2

# 11. Start server pprof collection
echo "=== Start server pprof collection ==="
for pid in $SERVER_PIDS; do
  PORT=$((6060 + $(echo $pid | cut -c1-2)))
  echo "Collecting server $pid on :$PORT..."
  go tool pprof -seconds=65 http://127.0.0.1:$PORT/debug/pprof/profile > $OUTDIR/pprof_server_$pid.pb 2>/dev/null &
done

sleep 2

# 12. Read 60s with client pprof
echo "=== READ START 60s: $(date) ==="
cd /root && timeout 65s ./kvcache_bench \
  -tikv-pd=10.153.28.202:12379,10.153.28.203:12379,10.153.28.204:12379 \
  -mode=read \
  -workers=64 \
  -duration=60s \
  -valueSize=4194304 \
  -prefix=bigread \
  -into \
  -pprof=:16060 \
  -cpuprof \
  2>&1 | tee $OUTDIR/bench_read.log
echo
echo "READ exit code: $?"
echo "=== READ END: $(date) ==="
echo

# 13. Stop monitors
echo "=== Stop monitors ==="
kill $IOSTAT_PID 2>/dev/null || true
kill $PIDSTAT_PID 2>/dev/null || true
kill $SAR_PID 2>/dev/null || true

# 14. Record /proc/pid/io after (servers)
echo "=== Record io after (servers) ==="
for pid in $SERVER_PIDS 2>/dev/null; do
  cat /proc/$pid/io > $OUTDIR/io_after_server_$pid 2>/dev/null || true
done

# 15. Analyze pprof (client)
echo "=== pprof client top ==="
cd /root && go tool pprof -top $OUTDIR/pprof_cpu.pb 2>/dev/null | head -50 > $OUTDIR/pprof_client_top.txt
cat $OUTDIR/pprof_client_top.txt
echo

# 16. Analyze pprof (servers)
echo "=== pprof server top (each server) ==="
for f in $OUTDIR/pprof_server_*.pb; do
  if [ -f "$f" ]; then
    echo "--- $(basename $f) ---"
    go tool pprof -top "$f" 2>/dev/null | head -20 > "${f%.pb}_top.txt"
    cat "${f%.pb}_top.txt"
    echo
  fi
done

echo
echo "===== DONE ====="
echo "Output dir: $OUTDIR"
