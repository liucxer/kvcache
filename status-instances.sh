#!/bin/bash

# 查看多个kvcache实例的状态

# 基础目录
BASE_DIR=$(pwd)

echo "KVCache Instances Status:"
echo "================================"

# 检查是否传入了路径参数
if [ $# -eq 0 ]; then
    # 查找所有实例PID文件
    for pid_file in $(find "$BASE_DIR" -name "*.pid" | grep -v ".git"); do
        if [ -f "$pid_file" ]; then
            # 读取进程ID
            PID=$(cat "$pid_file")
            
            # 检查进程是否存在
            if ps -p $PID > /dev/null; then
                echo "Instance $(basename "$pid_file" .pid): RUNNING"
                echo "PID: $PID"
                
                # 尝试从日志文件中获取端口信息
                log_file="$(dirname "$pid_file")/$(basename "$pid_file" .pid).log"
                if [ -f "$log_file" ]; then
                    # 提取端口信息
                    ports=$(grep "Selected ports" "$log_file" | tail -n 1)
                    if [ -n "$ports" ]; then
                        echo "$ports"
                    fi
                    
                    # 提取监听地址
                    grpc_addr=$(grep "gRPC server started" "$log_file" | tail -n 1)
                    http_addr=$(grep "HTTP server started" "$log_file" | tail -n 1)
                    if [ -n "$grpc_addr" ]; then
                        echo "$grpc_addr"
                    fi
                    if [ -n "$http_addr" ]; then
                        echo "$http_addr"
                    fi
                fi
            else
                echo "Instance $(basename "$pid_file" .pid): STOPPED (stale PID file)"
                # 删除过期的PID文件
                rm "$pid_file"
            fi
            echo "--------------------------------"
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
                echo "Instance $instance_name: RUNNING"
                echo "PID: $PID"
                
                # 尝试从日志文件中获取端口信息
                log_file="$instance_path/$instance_name.log"
                if [ -f "$log_file" ]; then
                    # 提取端口信息
                    ports=$(grep "Selected ports" "$log_file" | tail -n 1)
                    if [ -n "$ports" ]; then
                        echo "$ports"
                    fi
                    
                    # 提取监听地址
                    grpc_addr=$(grep "gRPC server started" "$log_file" | tail -n 1)
                    http_addr=$(grep "HTTP server started" "$log_file" | tail -n 1)
                    if [ -n "$grpc_addr" ]; then
                        echo "$grpc_addr"
                    fi
                    if [ -n "$http_addr" ]; then
                        echo "$http_addr"
                    fi
                fi
            else
                echo "Instance $instance_name: STOPPED (stale PID file)"
                # 删除过期的PID文件
                rm "$pid_file"
            fi
        else
            echo "Instance $instance_name: STOPPED (no PID file)"
        fi
        echo "--------------------------------"
    done
fi

# 检查是否有运行中的实例但没有PID文件
running_instances=$(ps aux | grep "kvcache" | grep -v grep | grep -v "start-instances.sh" | grep -v "stop-instances.sh" | grep -v "status-instances.sh")
if [ -n "$running_instances" ]; then
    echo "Additional running instances (no PID file):"
    echo "--------------------------------"
    echo "$running_instances"
    echo "--------------------------------"
fi

echo "Status check completed."
