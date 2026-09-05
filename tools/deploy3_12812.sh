#!/bin/bash
BIN=/root/kvcache/kvcache_perf
PD=127.0.0.1:2379
mkdir -p /mnt/nvme1n1/kvcache/data /mnt/nvme1n1/kvcache/value_data /mnt/nvme2n1/kvcache/data /mnt/nvme2n1/kvcache/value_data /mnt/nvme3n1/kvcache/data /mnt/nvme3n1/kvcache/value_data

setsid nohup $BIN --name=perf-nvme1 --data-dir=/mnt/nvme1n1/kvcache/data --value-dir=/mnt/nvme1n1/kvcache/value_data --grpc-addr=:33000 --raw-addr=:29300 --raw-write-addr=:29301 --node=128.12-1 --tikv-pd=$PD > /tmp/kv1.log 2>&1 < /dev/null &
setsid nohup $BIN --name=perf-nvme2 --data-dir=/mnt/nvme2n1/kvcache/data --value-dir=/mnt/nvme2n1/kvcache/value_data --grpc-addr=:33010 --raw-addr=:29310 --raw-write-addr=:29311 --node=128.12-2 --tikv-pd=$PD > /tmp/kv2.log 2>&1 < /dev/null &
setsid nohup $BIN --name=perf-nvme3 --data-dir=/mnt/nvme3n1/kvcache/data --value-dir=/mnt/nvme3n1/kvcache/value_data --grpc-addr=:33020 --raw-addr=:29320 --raw-write-addr=:29321 --node=128.12-3 --tikv-pd=$PD > /tmp/kv3.log 2>&1 < /dev/null &
echo "launched $!"