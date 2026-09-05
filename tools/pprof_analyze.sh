#!/bin/bash
echo "=== CLIENT pprof top ==="
go tool pprof -top -nodecount=25 /tmp/pprof_client.pb 2>/dev/null

echo ""
echo "=== SERVER pprof top ==="
go tool pprof -top -nodecount=25 /tmp/pprof_server.pb 2>/dev/null