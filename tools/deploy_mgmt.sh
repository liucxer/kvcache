#!/bin/bash
set -e
# 部署 mgmt 配置 + systemd 服务 到 128.12
cp /etc/mgmt.conf /etc/mgmt.conf.bak.$(date +%s) 2>/dev/null || true

cat > /etc/mgmt.conf <<'EOF'
{
  "role": "master",
  "ip": "100.71.128.12",
  "listen": "8081",
  "port": "8081",
  "id": "1",
  "peers": "1:100.71.128.12:8081",
  "metaPeers": "127.0.0.1:2379",
  "metaTLS": false,
  "retainLogs": "20000",
  "logDir": "/export/Logs/master",
  "logLevel": "info",
  "walDir": "/export/Data/master/raft",
  "storeDir": "/export/Data/master/rocksdbstore",
  "warnLogDir": "/export/home/tomcat/UMP-Monitor/logs/",
  "clusterName": "nefs-test",
  "metaNodeReservedMem": "134217728"
}
EOF

cat > /usr/lib/systemd/system/mgmt.service <<'EOF'
[Unit]
Description=mgmt service
After=network.target

[Service]
Type=simple
User=root
Group=root
Environment="GODEBUG=madvise=1"
Environment="LD_LIBRARY_PATH=/usr/local/lib:/usr/local/rocksdb/lib64/"
ExecStart=/usr/sbin/nefs.mgmt master
ExecStop=/bin/kill -WINCH ${MAINPID}
ExecReload=/bin/kill -s HUP ${MAINPID}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

mkdir -p /export/Logs/master /export/Data/master
systemctl daemon-reload
systemctl enable mgmt.service
systemctl restart mgmt.service
sleep 3
systemctl status mgmt.service --no-pager | head -10
echo "==== LIVENESS ===="
curl -s http://127.0.0.1:8081/mgmt/v1/liveness
echo
echo "==== PORTS ===="
ss -tlnp | grep -E '8081|12379' || true
