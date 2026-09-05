#!/bin/bash
cd /root/kvcache
# safety
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT fio"; exit 1; fi
if ! pgrep -x kvcache_perf >/dev/null 2>&1; then echo "ABORT no kvcache"; exit 1; fi
PREFIX="osync2"
VALUE_SIZE=4194304

echo "=== flush old dirty pages from previous w3 (page cache) ==="
sync
echo 3 > /proc/sys/vm/drop_caches
sleep 1
free -g | head -2

TS=$(date +%Y%m%d_%H%M%S)
OUT=/root/osync_${TS}
mkdir -p $OUT
echo "OUT=$OUT start_ns=$(date +%s%N)"

# per-sec authoritative disk WRITE sectors (stat field7 = sectors_written)
(for i in $(seq 1 180); do echo "$(date +%s) s1=$(awk '{print $7}' /sys/block/nvme1n1/stat) s2=$(awk '{print $7}' /sys/block/nvme2n1/stat) s3=$(awk '{print $7}' /sys/block/nvme3n1/stat)"; sleep 1; done) > $OUT/disk.log &
DISK_PID=$!
iostat -dmx 2 > $OUT/iostat.log 2>&1 & IOSTAT_PID=$!
sar -n DEV 1 > $OUT/sar.log 2>&1 & SAR_PID=$!
pidstat -u -d -l 1 > $OUT/pidstat.log 2>&1 & PIDSTAT_PID=$!

# pprof
for port in 33001 33011 33021; do (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=62" -o $OUT/pprof_$port.pb 2>/dev/null) & done
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=62" -o $OUT/pprof_client.pb 2>/dev/null || echo no-client-pprof) &

sleep 2
echo "=== WRITE 3-inst O_SYNC (raw-write sendmmsg, 80w, 60s) ==="
START=$(date +%s)
timeout 70 ./bench --mode write --raw-write --workers 80 --duration 60s \
  --value-size $VALUE_SIZE --prefix $PREFIX --batch 8 \
  --direct 127.0.0.1:29301,127.0.0.1:29311,127.0.0.1:29321 > $OUT/bench.log 2>&1
RC=$?
END=$(date +%s)
echo "bench exit=$RC elapsed=$(( END - START ))s"
grep -E 'Result|elapsed|errors|data=|throughput' $OUT/bench.log | head

sleep 2
echo "end_ns=$(date +%s%N)"
kill $DISK_PID $IOSTAT_PID $SAR_PID $PIDSTAT_PID 2>/dev/null
sleep 1

echo ""
echo "=== /sys real disk write DELTA (first vs last) ==="
F=$(head -1 $OUT/disk.log | grep -oE 's[123]=[0-9]+')
L=$(tail -1 $OUT/disk.log | grep -oE 's[123]=[0-9]+')
FT=$(head -1 $OUT/disk.log | awk '{print $1}')
LT=$(tail -1 $OUT/disk.log | awk '{print $1}')
DT=$(( LT - FT )); [ $DT -lt 1 ] && DT=1
for d in 1 2 3; do
  FB=$(echo "$F" | grep -oE "s$d=[0-9]+" | cut -d= -f2)
  LB=$(echo "$L" | grep -oE "s$d=[0-9]+" | cut -d= -f2)
  DL=$(( (LB - FB) * 512 ))
  echo "nvme${d}n1: delta=$((DL/1024/1024/1024)) GB  -> $((DL/DT/1024/1024)) MB/s"
done

echo ""
echo "=== bench throughput ==="
grep -E 'data=|throughput=|ops=' $OUT/bench.log | head

echo ""
echo "=== sar lo rx/tx avg ==="
grep ' lo ' $OUT/sar.log | awk 'NR>4{r+=$5;t+=$6;c++} END{if(c>0)printf "samples=%d lo_rx=%.0f lo_tx=%.0f MB/s\n", c, r/c/1024, t/c/1024}'

echo ""
echo "=== pidstat avg (kvcache_perf) CPU ==="
grep -i kvcache_perf $OUT/pidstat.log | grep -v '^#' | awk '{u+=$7;c++} END{if(c>0)printf "avg CPU=%.0f%% samples=%d\n", u/c, c; else printf "no samples\n"}'

echo ""
echo "OUT=$OUT"
echo "===== DONE ====="