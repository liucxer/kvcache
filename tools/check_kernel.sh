#!/bin/bash
echo "=== kernel version ==="
uname -r
uname -m

echo ""
echo "=== os release ==="
cat /etc/os-release 2>/dev/null | head -3

echo ""
echo "=== io_uring support ==="
grep -c 'io_uring' /proc/config.gz 2>/dev/null && zcat /proc/config.gz | grep -E '^CONFIG_IO_URING|^CONFIG_IO_URING_' 2>/dev/null || echo "no /proc/config.gz, checking kallsyms"
grep -i io_uring /proc/kallsyms 2>/dev/null | head -3 || echo "no io_uring in kallsyms"

echo ""
echo "=== /dev/io_uring device ==="
ls -la /dev/io_uring 2>/dev/null || echo "no /dev/io_uring"

echo ""
echo "=== syscall table check (syscall numbers) ==="
# splice 275, io_uring_setup 425, sendfile 40, sendmsg 46
grep -E ' (sys_splice|sys_io_uring_setup|sys_sendfile|sys_sendmsg|sys_sendmmsg|__arm64_sys_splice|__arm64_sys_io_uring_setup|__arm64_sys_sendfile|__arm64_sys_sendmsg) ' /proc/kallsyms 2>/dev/null | head -10 || echo "kallsyms restricted"

echo ""
echo "=== liburing / headers ==="
ls /usr/include/liburing.h 2>/dev/null || echo "no liburing.h"
gcc --version 2>/dev/null | head -1 || echo "no gcc"

echo ""
echo "=== test splice/sendfile/sendmsg availability via getconf ==="
which sendfile splice 2>/dev/null
dd --version 2>/dev/null | head -1