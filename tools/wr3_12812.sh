#!/bin/bash
cd /root/kvcache
# safety
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT: fio running"; exit 1; fi
if ! pgrep -x kvcache_perf >/dev/null 2>&1; then echo "ABORT: no kvcache"; exit 1; fi

TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/wr3_${TS}
mkdir -p $OUT
PIDS=$(pgrep -x kvcache_perf | sort | tr '\n' ' ')
echo "instance PIDs: $PIDS"
echo "OUT=$OUT"

# io before per instance
for pid in $PIDS; do [ -f /proc/$pid/io ] && grep '^write_bytes' /proc/$pid/io | awk '{print $2}' > $OUT/io_before_$pid || echo 0 > $OUT/io_before_$pid; done

echo "$(date +%s%N)" > $OUT/start_ns

# disk sampling loop (authoritative /sys/block sectors-written field3)
(for i in $(seq 1 180); do echo "$(date +%s) s1=$(awk '{print $3}' /sys/block/nvme1n1/stat) s2=$(awk '{print $3}' /sys/block/nvme2n1/stat) s3=$(awk '{print $3}' /sys/block/nvme3n1/stat)"; sleep 1; done) > $OUT/disk.log &
DISK_PID=$!
iostat -dmx 2 > $OUT/iostat.log 2>&1 & IOSTAT_PID=$!
sar -n DEV 1 > $OUT/sar.log 2>&1 & SAR_PID=$!
pidstat -u -d 1 > $OUT/pidstat.log 2>&1 & PIDSTAT_PID=$!

# server pprof on each instance
for port in 33001 33011 33021; do (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=55" -o $OUT/pprof_$port.pb 2>/dev/null) & done
# client pprof (bench:16060)
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=55" -o $OUT/pprof_client.pb 2>/dev/null || echo "no client pprof") &
sleep 2

echo "=== WRITE 3-inst (raw-write sendmmsg, round-robin 29301/29311/29321) ==="
START=$(date +%s)
timeout 70 ./bench --mode write --raw-write --workers 80 --duration 60s --value-size 4194304 --prefix wr3 --batch 8 --direct 127.0.0.1:29301,127.0.0.1:29311,127.0.0.1:29321 > $OUT/bench.log 2>&1
RC=$?
echo "bench exit=$RC elapsed=$(( $(date +%s) - START ))s"
grep -E 'Result|elapsed|errors|data=|op/s|GB' $OUT/bench.log | head -20

sleep 2
echo "$(date +%s%N)" > $OUT/end_ns
kill $DISK_PID $IOSTAT_PID $SAR_PID $PIDSTAT_PID 2>/dev/null
sleep 1

echo ""
echo "=== /proc/PID/io write DELTA (per instance) ==="
WRITE_S=$(python -c "print(($(cat $OUT/end_ns)-$(cat $OUT/start_ns))/1e9)" 2>/dev/null)
for pid in $PIDS; do
  AFTER=$(grep '^write_bytes' /proc/$pid/io 2>/dev/null | awk '{print $2}') || AFTER=0
  BEFORE=$(cat $OUT/io_before_$pid 2>/dev/null || echo 0)
  DELTA=$((AFTER - BEFORE))
  echo "pid=$pid write_delta=$((DELTA/1024/1024/1024)) GB  -> $(( DELTA*8/1024/1024/1024/ (${WRITE_S%.*}||1) )) Gbps*nn"
done

echo ""
echo "=== /sys/block sectors delta (real disk write) ==="
for d in 1 2 3; do
  SB=$(head -1 $OUT/disk.log)
  SE=$(tail -1 $OUT/disk.log)
  B=$(echo $SE | grep -oE "s$d=[0-9]+" | cut -d= -f2)
  A=$(echo $SB | grep -oE "s$d=[0-9]+" | cut -d= -f2)
  # per-second max attained is not trivial from one-shot; report total
  echo "nvme${d}n1 first_sectors=$A last_sectors=$B"
done

echo ""
echo "=== disk.log first/last ==="
head -1 $OUT/disk.log
tail -1 $OUT/disk.log

echo ""
echo "=== sar lo (rx/tx, samples after warmup) ==="
grep ' lo ' $OUT/sar.log | awk 'NR>8{s4+=$5;s5+=$6;c++} END {printf "samples=%d lo_rx_avg=%.0f MB/s (%.2f GB/s) lo_tx_avg=%.0f MB/s (%.2f GB/s)\n", c, s4/c/1024, s4/c/1024/1024, s5/c/1024, s5/c/1024/1024}'

echo ""
echo "=== pidstat avg CPU/IO for kvcache_perf ==="
grep -E 'kvcache_perf' $OUT/pidstat.log | grep -v '^#' | awk '{u+=$9; d+=$11; c++} END {if(c>0) printf "%d samples, avg %%CPU=%.0f, %%IO=%.0f\n", c, u/c, d/c}'

echo ""
echo "OUT=$OUT"
echo "===== DONE ====="