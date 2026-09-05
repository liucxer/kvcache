#!/bin/bash
# 通过 mgmt HTTP API 创建 storage target（绕过 CLI "no valid master" 问题）
POOL=$(cat /root/nefs_pool.txt)
echo "POOL=$POOL"
echo "==== SET TARGET ===="
curl -s "http://127.0.0.1:8081/storage/setTarget?name=nefs-target&poolID=1&poolName=$POOL&desc=NEFS+kvcache+target&createTime=$(date +%s)"
echo
echo "==== LIST TARGET ===="
curl -s "http://127.0.0.1:8081/storage/listTarget"
echo
