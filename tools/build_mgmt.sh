#!/bin/bash
# 在编译机 100.71.128.13 上重建 mgmt（arm64，RocksDB 8.11.5 CGO）
set -x
cd ~/nefs/mgmt || exit 1
export CGO_ENABLED=1
export CGO_CFLAGS="-I/usr/local/include"
export CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd -Wl,-rpath,/usr/local/lib"
# make bin 会执行 go mod tidy（经 goproxy.cn 拉取依赖）
make bin > /root/build_mgmt.log 2>&1
RC=$?
echo "BUILD_RC=$RC"
ls -la ~/nefs/mgmt/build/bin/nefs.mgmt
