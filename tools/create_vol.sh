#!/bin/bash
# 通过 mgmt HTTP API 创建卷（绕过 CLI 问题）
VOL=$(uuidgen)
echo "VOL=$VOL"
echo "==== CREATE VOL ===="
curl -s -X POST "http://127.0.0.1:8081/admin/createVol" -H "Content-Type: application/json" \
  -d "{\"Name\":\"$VOL\",\"Capacity\":1000,\"Inodes\":100000,\"BlockSize\":4096,\"Target\":\"nefs-target\",\"Compress\":\"none\",\"DirStats\":true}"
echo
echo "==== LIST VOL ===="
curl -s "http://127.0.0.1:8081/admin/listVol"
echo
echo "$VOL" > /root/nefs_vol.txt
