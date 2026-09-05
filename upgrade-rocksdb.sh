#!/bin/bash
# 升级RocksDB到8.x版本
set -e

ROCKSDB_VERSION="8.10.0"
BUILD_DIR="/tmp/rocksdb-build"

echo "=== 升级RocksDB到 ${ROCKSDB_VERSION} ==="

# 1. 卸载旧版本
echo "[1/6] 卸载旧版本RocksDB..."
sudo rpm -e rocksdb-devel rocksdb 2>/dev/null || true

# 2. 准备编译环境
echo "[2/6] 安装编译依赖..."
sudo yum install -y gcc gcc-c++ make cmake3 snappy-devel zlib-devel bzip2-devel lz4-devel zstd-devel 2>/dev/null || \
sudo dnf install -y gcc gcc-c++ make cmake snappy-devel zlib-devel bzip2-devel lz4-devel zstd-devel

# 3. 下载源码
echo "[3/6] 下载RocksDB ${ROCKSDB_VERSION}..."
mkdir -p ${BUILD_DIR}
cd ${BUILD_DIR}
wget -q https://github.com/facebook/rocksdb/archive/refs/tags/v${ROCKSDB_VERSION}.tar.gz
tar xzf v${ROCKSDB_VERSION}.tar.gz
cd rocksdb-${ROCKSDB_VERSION}

# 4. 编译
echo "[4/6] 编译RocksDB (这可能需要几分钟)..."
make -j$(nproc) shared_lib

# 5. 安装
echo "[5/6] 安装RocksDB..."
sudo make install-shared
sudo ldconfig

# 6. 验证
echo "[6/6] 验证安装..."
rocksdb_version=$(pkg-config --modversion rocksdb 2>/dev/null || echo "unknown")
echo "RocksDB版本: ${rocksdb_version}"

# 清理
cd /
rm -rf ${BUILD_DIR}

echo "=== RocksDB升级完成 ==="
