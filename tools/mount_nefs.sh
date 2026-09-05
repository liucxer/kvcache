#!/bin/bash
# 挂载 kvcache 卷
export PATH=/usr/bin:/usr/sbin:/usr/local/bin:$PATH
VOL=$(cat /root/nefs_vol.txt)
echo "VOL=$VOL"
mkdir -p /mnt/nefs
pkill -f 'nefs.client mount' 2>/dev/null
sleep 1
nohup nefs.client mount "$VOL" /mnt/nefs --master 127.0.0.1:8081 --enable-xattr > /root/nefs_mount.log 2>&1 &
echo "mount started, pid=$!"
sleep 8
echo "==== MOUNT ===="
mount | grep nefs
echo "==== LOG ===="
tail -30 /root/nefs_mount.log
