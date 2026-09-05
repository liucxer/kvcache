#!/bin/bash

# 启动多个kvcache实例

# 基础目录
BASE_DIR=$(pwd)

# 检查是否传入了路径参数
if [ $# -eq 0 ]; then
    echo "Usage: $0 <instance_path1> <instance_path2> ..."
    echo "Example: $0 /path/to/instance1 /path/to/instance2"
    exit 1
fi

# 启动实例
for instance_path in "$@"; do
    # 确保路径是绝对路径
    if [[ ! "$instance_path" == /* ]]; then
        instance_path="$BASE_DIR/$instance_path"
    fi
    
    # 创建实例目录
    mkdir -p "$instance_path"
    mkdir -p "$instance_path/data"
    mkdir -p "$instance_path/value_data"
    
    # 生成实例名称（基于路径的最后部分）
    instance_name=$(basename "$instance_path")
    
    # 启动实例
    echo "Starting instance $instance_name..."
    cd "$instance_path"
    nohup "$BASE_DIR/kvcache" > "$instance_path/$instance_name.log" 2>&1 &
    
    # 记录进程ID
    echo $! > "$instance_path/$instance_name.pid"
    
    cd "$BASE_DIR"
done

echo "All instances started."
echo "Check logs in respective instance directories."
