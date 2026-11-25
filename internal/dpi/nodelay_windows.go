//go:build windows

package dpi

import "syscall"

// setNoDelayFD enables TCP_NODELAY on a raw socket handle.
func setNoDelayFD(fd uintptr) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
}
