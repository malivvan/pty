//go:build netbsd

package pty

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// openMaster opens the /dev/ptmx cloning device.
func openMaster() (*os.File, error) {
	return os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
}

// ptsname returns the name of the slave device paired with master, as reported
// by the TIOCPTSNAME ioctl, which is the NetBSD equivalent of ptsname(3):
//
//	struct ptmget pm;
//	ioctl(fd, TIOCPTSNAME, &pm) == -1 ? NULL : pm.sn;
func ptsname(master *os.File) (string, error) {
	var arg ptmget
	if err := ioctl(master, uintptr(ioctl_TIOCPTSNAME), uintptr(unsafe.Pointer(&arg))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return "", err
	}

	// Sn is a NUL-terminated C char array, which Go represents as []int8.
	name := make([]byte, len(arg.Sn))
	for i, c := range arg.Sn {
		if c == 0 {
			return string(name[:i]), nil
		}
		name[i] = byte(c)
	}
	return "", errors.New("pty: TIOCPTSNAME returned a name that is not NUL-terminated")
}

// grantpt hands ownership of the slave device to the calling user through the
// TIOCGRANTPT ioctl; it is the equivalent of grantpt(3).
func grantpt(master *os.File) error {
	return ioctl(master, uintptr(ioctl_TIOCGRANTPT), 0)
}

// unlockpt is a no-op on NetBSD: TIOCGRANTPT grants and unlocks the slave
// device in a single step.
func unlockpt(*os.File) error { return nil }

// openSlave opens the slave device at path without making it the controlling
// terminal of the current process.
func openSlave(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|syscall.O_NOCTTY, 0) //nolint:gosec // The path comes from the kernel.
}
