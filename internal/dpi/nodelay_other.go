//go:build !windows

package dpi

import "syscall"

// setNoDelayFD enables TCP_NODELAY on a raw file descriptor.
func setNoDelayFD(fd uintptr) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
}
