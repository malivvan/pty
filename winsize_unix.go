//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import (
	"os"
	"syscall"
	"unsafe"
)

// SetSize resizes the terminal backed by tty to size.
func SetSize(tty *os.File, size *Winsize) error {
	return ioctl(tty, syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(size))) //nolint:gosec // The pointer must be handed to the kernel as-is.
}

// GetFullSize returns the complete size description of the terminal backed by
// tty.
func GetFullSize(tty *os.File) (*Winsize, error) {
	var size Winsize
	if err := ioctl(tty, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return nil, err
	}
	return &size, nil
}
