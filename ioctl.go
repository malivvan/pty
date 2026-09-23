//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import "os"

// ioctl issues the device request on the file descriptor of file, with arg as
// the request argument.
//
// Both the request numbers and the meaning of arg are system specific; the
// requests this package uses are defined next to the implementations that
// need them, and ioctlInner does the actual system call.
func ioctl(file *os.File, request, arg uintptr) error {
	return ioctlInner(file.Fd(), request, arg)
}
