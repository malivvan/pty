//go:build dragonfly

package pty

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// ioctlFIODGNAME is the request that fills a fiodgnameArg with the name of the
// device backing a master end; see <sys/filio.h>.
var ioctlFIODGNAME = _IOW('f', 120, unsafe.Sizeof(fiodgnameArg{}))

// openMaster opens the /dev/ptmx cloning device.
func openMaster() (*os.File, error) {
	return os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
}

// ptsname returns the path of the slave device paired with master, as reported
// by the FIODGNAME ioctl: DragonFly BSD names the master /dev/ptmN, which is
// rewritten to the /dev/ptsN of the matching slave.
func ptsname(master *os.File) (string, error) {
	name := make([]byte, _C_SPECNAMELEN)
	arg := fiodgnameArg{
		Name: (*byte)(unsafe.Pointer(&name[0])),
		Len:  _C_SPECNAMELEN,
	}

	if err := ioctl(master, ioctlFIODGNAME, uintptr(unsafe.Pointer(&arg))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return "", err
	}

	for i, c := range name {
		if c == 0 {
			return strings.Replace("/dev/"+string(name[:i]), "ptm", "pts", 1), nil
		}
	}
	return "", errors.New("pty: FIODGNAME returned a name that is not NUL-terminated")
}

// grantpt and unlockpt are the two halves of the single TIOCISPTMASTER ioctl
// that DragonFly BSD offers in place of grantpt(3) and unlockpt(3).
func grantpt(master *os.File) error {
	_, err := isptmaster(master)
	return err
}

func unlockpt(master *os.File) error {
	_, err := isptmaster(master)
	return err
}

// isptmaster reports whether master really is a pseudo-terminal master, using
// the TIOCISPTMASTER ioctl.
func isptmaster(master *os.File) (bool, error) {
	err := ioctl(master, syscall.TIOCISPTMASTER, 0)
	return err == nil, err
}

// openSlave opens the slave device at path, which the TIOCISPTMASTER ioctl
// above has already unlocked.
func openSlave(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // The path comes from the kernel.
}
