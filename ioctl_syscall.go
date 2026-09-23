//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || zos

package pty

import "syscall"

// ioctlInner issues the device request on the raw file descriptor fd with the
// generic SYS_IOCTL system call.
//
// Solaris has its own mechanism (see ioctl_solaris.go), and AIX is not
// implemented at all (see ioctl_aix.go).
func ioctlInner(fd, request, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, arg); errno != 0 {
		return errno
	}
	return nil
}
