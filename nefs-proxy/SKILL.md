---
name: nefs-proxy
version: 1.0.0
description: "通过 proxy.py agent（端口 9527）在已部署节点上执行命令、传输文件、管理端口转发规则。TRIGGER: 在 100.71.7.194 / 100.71.128.12 / 10.151.26.146-150,152 节点上执行命令、上传下载文件、配置端口转发、查看 proxy 规则。"
allowed-tools: Bash, Read, Write
keywords: proxy,nefs,exec,命令执行,文件传输,upload,download,端口转发,9527,100.71.7.194,100.71.128.12,10.151.26
---

# NEFS proxy skill

在已部署 `proxy.py server` 的节点上，通过其 HTTP 接口（默认端口 9527，默认 token `95279527`）执行命令、传输文件、管理端口转发。客户端脚本纯标准库，无需安装依赖。

## 已部署节点清单

| 节点 | 主机名 | 访问方式 |
|------|--------|----------|
| 100.71.7.194 | SZYFQ-PM-OS01-BCNFS-GFS41 | Mac 直连 |
| 100.71.128.12 | SZYFQ-PM-OS01-BCNFS-XCKP05（跳板） | Mac 直连 |
| 10.151.26.146/147/148/149/150/152 | fhcsy-...-pm-os01-ebs-10/11/12/13/14/16 | Mac 直连 |

> 所有节点统一从本地 Mac 直接访问 `http://<节点>:9527`。某个节点不可达时，单独排查网络/部署问题即可。

## 用法

```bash
PY=~/.claude/skills/nefs-proxy/proxy_client.py

# 执行命令
python3 $PY --node 194 exec --cmd "ceph -s" --timeout 60
python3 $PY --node 100.71.128.12 exec --cmd "df -h" --cwd /tmp

# 上传 / 下载 / 列目录 / 建目录 / 删除
python3 $PY --node 146 upload   --local ./a.tar.gz --remote /tmp/a.tar.gz
python3 $PY --node 146 download --remote /tmp/a.tar.gz --local ./a.tar.gz
python3 $PY --node 146 ls --path /tmp
python3 $PY --node 146 mkdir --path /tmp/x
python3 $PY --node 146 delete --path /tmp/x

# 健康检查
python3 $PY --node 194 ping

# proxy 规则管理（在目标节点本身上增/查/删）
python3 $PY --node 100.71.128.12 proxy list
python3 $PY --node 100.71.128.12 proxy add --listen-port 19520 --target-ip 127.0.0.1 --target-port 9527 --name test-fwd
python3 $PY --node 100.71.128.12 proxy delete --name test-fwd
```

节点参数 `--node` 支持别名：`194`/`128`/`12`/`jump`/`146`..`152`，也支持完整 IP。可用 `--token` 覆盖默认 token。

## 底层接口（proxy.py agent，同一端口 9527，除 /ping 外需 X-Token）

- 命令执行：`POST /exec`（body `{"cmd","timeout","cwd"}`），返回 `stdout/stderr/exit_code`；`GET /exec?cmd=...` 同
- 文件传输：`PUT/POST /upload?path=...`、`GET /download?path=...`、`GET /ls?path=...`、`POST /mkdir`、`POST /delete`
- proxy 管理：`GET /proxy`、`POST /proxy/add`、`POST /proxy/delete` / `DELETE /proxy?name=...`
- 健康检查：`GET /ping`

## 注意事项

- 节点上进程为 `python3 ./proxy.py server`，文件 `/tmp/proxy.py`，root=/tmp。
- 命令支持 shell 管道/重定向；`timeout` 默认 3600s，超时会整组 kill（返回 `timed_out=true`）。
- 文件传输支持大文件（分块/流式）。
- 涉及敏感信息（token、IP）仅限内网测试环境，勿外传。
