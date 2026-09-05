#!/bin/bash
cd /root/kvcache
# safety
if pgrep -x fio >/dev/null 2>&1; then echo "ABORT fio"; exit 1; fi
VALUE=4194304; W=80; COUNT=1000
TS=$(date +%Y%m%d_%H%M%S); OUT=/root/rd3_$TS; mkdir -p $OUT
echo "OUT=$OUT"

echo "=== WRITE 3 instances (parallel, independent prefix, O_SYNC) ==="
for spec in "rd3-i1 29301" "rd3-i2 29311" "rd3-i3 29321"; do
  set -- $spec; P=$1; RW=$2
  timeout 300 ./bench --mode write --raw-write --workers $W --count-per-worker $COUNT \
    --value-size $VALUE --prefix $P --batch 8 --direct 127.0.0.1:$RW > $OUT/write_$P.log 2>&1 &
done
wait
for l in $OUT/write_*.log; do echo "-- $l --"; grep -E 'Result|errors|data=' "$l" | head -2; done

echo "=== DROP CACHES (release memory) ==="
sync; echo 3 > /proc/sys/vm/drop_caches; sleep 5; free -g | head -2

echo "=== READ 3 instances (parallel, exact traversal, --into) ==="
(for i in $(seq 1 300); do echo "$(date +%s) r1=$(awk '{print $3}' /sys/block/nvme1n1/stat) r2=$(awk '{print $3}' /sys/block/nvme2n1/stat) r3=$(awk '{print $3}' /sys/block/nvme3n1/stat)"; sleep 1; done) > $OUT/disk.log &
DISK_PID=$!
iostat -dmx 2 > $OUT/iostat.log 2>&1 & IOSTAT_PID=$!
sar -n DEV 1 > $OUT/sar.log 2>&1 & SAR_PID=$!
pidstat -u -d -l 1 > $OUT/pidstat.log 2>&1 & PIDSTAT_PID=$!
mpstat -P ALL 1 > $OUT/mpstat.log 2>&1 & MPSTAT_PID=$!

for port in 33001 33011 33021; do (sleep 3; curl -s "http://127.0.0.1:$port/debug/pprof/profile?seconds=62" -o $OUT/pprof_$port.pb 2>/dev/null) & done
(sleep 3; curl -s "http://127.0.0.1:16060/debug/pprof/profile?seconds=62" -o $OUT/pprof_client.pb 2>/dev/null) &

sleep 2
START=$(date +%s)
for spec in "rd3-i1 33000 29300" "rd3-i2 33010 29310" "rd3-i3 33020 29320"; do
  set -- $spec; P=$1; GA=$2; RA=$3
  timeout 300 ./bench --mode read --into --workers $W --duration 120s --value-size $VALUE \
    --prefix $P --read-max-seq $COUNT --read-wid-mod $W \
    --direct-addr 127.0.0.1:$GA --direct-raw-addr 127.0.0.1:$RA > $OUT/read_$P.log 2>&1 &
done
wait
echo "read done elapsed=$(( $(date +%s) - START ))s"
sleep 2; kill $DISK_PID $IOSTAT_PID $SAR_PID $PIDSTAT_PID $MPSTAT_PID 2>/dev/null; sleep 1

echo "=== READ logs ==="
for l in $OUT/read_*.log; do echo "-- $l --"; grep -E 'Result|errors|data=|elapsed' "$l" | head -4; done

echo "=== /sys READ delta (field3=sectors_read) ==="
F=$(head -1 $OUT/disk.log); L=$(tail -1 $OUT/disk.log)
FT=$(echo "$F"|awk '{print $1}'); LT=$(echo "$L"|awk '{print $1}'); DT=$((LT-FT)); [ $DT -lt 1 ]&&DT=1
for d in 1 2 3; do
  FB=$(echo "$F"|grep -oE "r$d=[0-9]+"|cut -d= -f2); LB=$(echo "$L"|grep -oE "r$d=[0-9]+"|cut -d= -f2)
  DL=$(( (LB-FB)*512 ))
  echo "nvme${d}n1: read_delta=$(( DL/1024/1024/1024 )) GB -> $(( DL/DT/1024/1024 )) MB/s"
done
echo "=== pidstat avg CPU (kvcache_perf + bench) ==="
grep -iE 'kvcache_perf|./bench' $OUT/pidstat.log | grep -v '^#' | awk '{C+=$8;c++} END{if(c>0)printf "avg CPU=%.0f%% samples=%d\n",C/c,c;else print "no samples"}'
echo "OUT=$OUT"
echo DONE