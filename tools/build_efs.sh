#!/bin/bash
# 在编译机 100.71.128.13 构建 EFS_nefs nefs.tools + nefs.client（arm64，s3 变体，含 kvcache）
set -x
cd ~/nefs/EFS_nefs || exit 1
export GOFLAGS=-mod=mod
# s3 变体无需 ceph，纯 Go
make nefs.tools.s3 nefs.client.s3 > /root/build_efs.log 2>&1
RC=$?
echo "BUILD_RC=$RC"
ls -la nefs.tools nefs.client
