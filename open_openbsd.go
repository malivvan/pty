//go:build openbsd

package pty

import (
	"os"
	"syscall"
	"unsafe"
)

// open allocates a pseudo-terminal pair.
//
// OpenBSD does not follow the /dev/ptmx scheme of the other BSDs: /dev/ptm
// hands out the master and slave file descriptors of a freshly allocated pair
// in a single PTMGET ioctl, so there is no name to look up and no grant/unlock
// step. See ptm(4).
func open() (ptmx, tty *os.File, err error) {
	ptm, err := os.OpenFile("/dev/ptm", os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	// The /dev/ptm handle is only needed for the ioctl below: the descriptors it
	// returns are independent of it.
	defer func() { _ = ptm.Close() }() // Best effort.

	var arg ptmget
	if err := ioctl(ptm, uintptr(ioctl_PTMGET), uintptr(unsafe.Pointer(&arg))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return nil, nil, err
	}

	ptmx = os.NewFile(uintptr(arg.Cfd), cString(arg.Cn[:]))
	tty = os.NewFile(uintptr(arg.Sfd), cString(arg.Sn[:]))
	return ptmx, tty, nil
}

// cString converts a NUL-terminated C char array into a Go string. Bytes are
// taken as they are, so non-ASCII device names survive the int8 representation
// the kernel uses.
func cString(buf []int8) string {
	out := make([]byte, 0, len(buf))
	for _, c := range buf {
		if c == 0 {
			break
		}
		out = append(out, byte(c))
	}
	return string(out)
}
