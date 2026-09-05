#!/bin/bash
set -u
cd /root

echo "=== stop old instances ==="
PIDS=$(pgrep -x kvcache_perf | tr '\n' ' ')
echo "old pids: $PIDS"
if [ -n "$PIDS" ]; then
  kill $PIDS
  sleep 3
  for i in $(seq 1 10); do
    pgrep -x kvcache_perf >/dev/null || break
    sleep 1
  done
fi
if pgrep -x kvcache_perf >/dev/null; then
  echo "ERROR: instances still running, abort"
  exit 1
fi
echo "all stopped"

echo "=== backup & replace binary ==="
TS=$(date +%m%d%H%M)
[ -f /root/kvcache_perf ] && mv -f /root/kvcache_perf /root/kvcache_perf.bak.$TS
mv -f /root/kvcache_perf_new /root/kvcache_perf
[ -f /root/bench ] && mv -f /root/bench /root/bench.bak.$TS
mv -f /root/bench_new /root/bench
ls -la /root/kvcache_perf /root/bench

echo "=== start instances ==="
COMMON="--node=146 --ca-path=/nefsdata/meta/tikv-deploy/pd-12379/tls/ca.crt --cert-path=/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.crt --key-path=/nefsdata/meta/tikv-deploy/pd-12379/tls/pd.pem --tikv-pd=10.153.28.202:12379,10.153.28.203:12379,10.153.28.204:12379"

nohup ./kvcache_perf --name=perf-nvme5 --data-dir=/nefsdata/nvme5/data --value-dir=/nefsdata/nvme5/value_data --grpc-addr=:33000 --raw-addr=:29300 --raw-write-addr=:29301 $COMMON > /root/inst_nvme5.log 2>&1 &
nohup ./kvcache_perf --name=perf-nvme7 --data-dir=/nefsdata/nvme7/data --value-dir=/nefsdata/nvme7/value_data --grpc-addr=:33020 --raw-addr=:29320 --raw-write-addr=:29321 $COMMON > /root/inst_nvme7.log 2>&1 &
nohup ./kvcache_perf --name=perf-nvme6 --data-dir=/nefsdata/nvme6/data --value-dir=/nefsdata/nvme6/value_data --grpc-addr=:33030 --raw-addr=:29330 --raw-write-addr=:29331 $COMMON > /root/inst_nvme6.log 2>&1 &

sleep 8
echo "=== run state ==="
pgrep -a kvcache_perf
echo "=== ports ==="
ss -lnt | grep -E ':(29300|29301|29320|29321|29330|29331)\b' | awk '{print $4}'
echo "=== logs tail ==="
for f in /root/inst_nvme5.log /root/inst_nvme7.log /root/inst_nvme6.log; do
  echo "-- $f --"; tail -3 $f
done