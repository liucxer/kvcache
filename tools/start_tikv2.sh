#!/bin/bash
# 128.12 上为 EFS_nefs meta 创建独立 TiKV 集群（PD:2389, TiKV:20170），数据在 nvme1n1
set -e
mkdir -p /mnt/nvme1n1/tikv2/pd/data /mnt/nvme1n1/tikv2/pd/log
mkdir -p /mnt/nvme1n1/tikv2/tikv/data /mnt/nvme1n1/tikv2/tikv/log

docker rm -f ukv_pd2 ukv_tikv2 2>/dev/null || true

echo "==== start ukv_pd2 ===="
docker run -d --name ukv_pd2 --network host \
  -v /mnt/nvme1n1/tikv2:/tikv2 \
  -v /etc/localtime:/etc/localtime \
  registry.paas/nefs/ukv_pd_arm:v7.5.6 \
  --data-dir=/tikv2/pd/data \
  --client-urls=http://127.0.0.1:2389 \
  --advertise-client-urls=http://127.0.0.1:2389 \
  --peer-urls=http://127.0.0.1:2390 \
  --advertise-peer-urls=http://127.0.0.1:2390 \
  --log-file=/tikv2/pd/log/pd.log

echo "==== start ukv_tikv2 ===="
docker run -d --name ukv_tikv2 --network host \
  -v /mnt/nvme1n1/tikv2:/tikv2 \
  -v /etc/localtime:/etc/localtime \
  registry.paas/nefs/ukv_server_arm:v7.5.6 \
  --pd-endpoints=127.0.0.1:2389 \
  --data-dir=/tikv2/tikv/data \
  --addr=127.0.0.1:20170 \
  --advertise-addr=127.0.0.1:20170 \
  --status-addr=127.0.0.1:20180 \
  --log-file=/tikv2/tikv/log/tikv.log

echo "==== wait PD ready ===="
for i in $(seq 1 20); do
  if curl -s -m 2 "http://127.0.0.1:2389/pd/api/v1/version" >/dev/null 2>&1; then
    echo "PD ready after ${i}s"
    break
  fi
  sleep 1
done
echo "==== PD version ===="
curl -s -m 2 "http://127.0.0.1:2389/pd/api/v1/version"
echo
echo "==== store status ===="
curl -s -m 2 "http://127.0.0.1:2389/pd/api/v1/stores" | head -c 400
echo
echo "==== listen ports ===="
ss -tlnp 2>/dev/null | grep -E '2389|2390|20170|20180' | head
echo "==== logs ===="
tail -5 /mnt/nvme1n1/tikv2/pd/log/pd.log 2>/dev/null
echo "---tikv---"
tail -5 /mnt/nvme1n1/tikv2/tikv/log/tikv.log 2>/dev/null
