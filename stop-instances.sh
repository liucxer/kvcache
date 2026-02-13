#!/bin/bash

# 停止多个kvcache实例

# 基础目录
BASE_DIR=$(pwd)

# 检查是否传入了路径参数
if [ $# -eq 0 ]; then
    echo "Usage: $0 <instance_path1> <instance_path2> ..."
    echo "Example: $0 /path/to/instance1 /path/to/instance2"
    echo "Or: $0 all (to stop all instances)"
    exit 1
fi

# 处理所有实例的情况
if [ "$1" == "all" ]; then
    # 查找所有实例PID文件
    for pid_file in $(find "$BASE_DIR" -name "*.pid" | grep -v ".git"); do
        if [ -f "$pid_file" ]; then
            # 读取进程ID
            PID=$(cat "$pid_file")
            
            # 检查进程是否存在
            if ps -p $PID > /dev/null; then
                echo "Stopping instance with PID $PID..."
                # 发送终止信号
                kill $PID
                
                # 等待进程退出
                wait $PID 2>/dev/null
            fi
            
            # 删除PID文件
            rm "$pid_file"
        fi
    done
else
    # 处理指定路径的实例
    for instance_path in "$@"; do
        # 确保路径是绝对路径
        if [[ ! "$instance_path" == /* ]]; then
            instance_path="$BASE_DIR/$instance_path"
        fi
        
        # 生成实例名称（基于路径的最后部分）
        instance_name=$(basename "$instance_path")
        pid_file="$instance_path/$instance_name.pid"
        
        if [ -f "$pid_file" ]; then
            # 读取进程ID
            PID=$(cat "$pid_file")
            
            # 检查进程是否存在
            if ps -p $PID > /dev/null; then
                echo "Stopping instance $instance_name..."
                # 发送终止信号
                kill $PID
                
                # 等待进程退出
                wait $PID 2>/dev/null
            fi
            
            # 删除PID文件
            rm "$pid_file"
        else
            echo "No PID file found for instance $instance_name"
        fi
    done
fi

echo "All specified instances stopped."
