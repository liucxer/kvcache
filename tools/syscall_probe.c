#define _GNU_SOURCE
#include <stdio.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include <sys/syscall.h>
#include <sys/uio.h>
#include <fcntl.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <netinet/in.h>

// io_uring_setup = 425 on aarch64 (same as generic)
#ifndef SYS_io_uring_setup
#define SYS_io_uring_setup 425
#endif

#define TEST(name, fn) do { \
    errno = 0; \
    fn; \
    int e = errno; \
    if (e == ENOSYS) printf("%-14s NOT SUPPORTED (ENOSYS)\n", name); \
    else if (e == EPERM) printf("%-14s supported (got EPERM)\n", name); \
    else printf("%-14s supported (errno=%d: %s)\n", name, e, e ? strerror(e) : "OK"); \
} while(0)

int main() {
    int pfd[2];

    // io_uring_setup: returns errno ENOSYS if kernel lacks io_uring
            TEST("io_uring_setup", ({
                char p[160] = {0};              // io_uring_params space, zeroed
                long r = syscall(SYS_io_uring_setup, 16, (void *)p);
                if (r >= 0) close((int)r);
            }));

    // splice: pipe to null / var
    TEST("splice", ({
        if (pipe(pfd) == 0) {
            char buf[16] = "hello";
            write(pfd[1], buf, 5);
            splice(pfd[0], NULL, pfd[1], NULL, 5, 0);  // pipe->pipe
            close(pfd[0]); close(pfd[1]);
        }
    }));

    // sendfile: file -> socket (create loopback socketpair)
    TEST("sendfile", ({
        int sv[2]; int f = open("/etc/hostname", O_RDONLY);
        if (socketpair(AF_UNIX, SOCK_STREAM, 0, sv) == 0 && f >= 0) {
            char out[64];
            sendfile(sv[1], f, NULL, 16);   // sendfile to unix socket (allowed)
            recv(sv[0], out, sizeof(out), 0);
            close(sv[0]); close(sv[1]); close(f);
        } else { if(f>=0) close(f); errno = ENOSYS; }
    }));

    // sendmsg: unix socketpair
    TEST("sendmsg", ({
        int sv[2];
        if (socketpair(AF_UNIX, SOCK_DGRAM, 0, sv) == 0) {
            struct iovec iov = { (void*)"x", 1 };
            struct msghdr m = {0};
            m.msg_iov = &iov; m.msg_iovlen = 1;
            sendmsg(sv[1], &m, 0);
            close(sv[0]); close(sv[1]);
        } else { errno = ENOSYS; }
    }));

    // sendmmsg
    TEST("sendmmsg", ({
        int sv[2];
        if (socketpair(AF_UNIX, SOCK_DGRAM, 0, sv) == 0) {
            struct iovec iov = { (void*)"x", 1 };
            struct mmsghdr m = {0};
            m.msg_hdr.msg_iov = &iov; m.msg_hdr.msg_iovlen = 1;
            sendmmsg(sv[1], &m, 1, 0);
            close(sv[0]); close(sv[1]);
        } else { errno = ENOSYS; }
    }));

    // recvmmsg
    TEST("recvmmsg", ({
        int sv[2];
        if (socketpair(AF_UNIX, SOCK_DGRAM, 0, sv) == 0) {
            write(sv[1], "x", 1);
            struct iovec iov = { (void*)"x", 1 };
            struct mmsghdr m = {0};
            m.msg_hdr.msg_iov = &iov; m.msg_hdr.msg_iovlen = 1;
            recvmmsg(sv[0], &m, 1, 0, NULL);
            close(sv[0]); close(sv[1]);
        } else { errno = ENOSYS; }
    }));

    // process_vm / O_DIRECT check
    TEST("O_DIRECT mock", ({
        int f = open("/dev/null", O_WRONLY | O_DIRECT);
        if (f >= 0) close(f); else { /* leave errno */ }
    }));

    return 0;
}