//go:build darwin

package pty

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// openMaster opens the /dev/ptmx cloning device.
func openMaster() (*os.File, error) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), nil
}

// ptsname returns the path of the slave device paired with master, as reported
// by the TIOCPTYGNAME ioctl; it is the equivalent of ptsname(3).
func ptsname(master *os.File) (string, error) {
	// TIOCPTYGNAME writes a NUL-terminated path into a buffer whose size is
	// encoded in the request itself.
	name := make([]byte, _IOC_PARAM_LEN(syscall.TIOCPTYGNAME))

	if err := ioctl(master, syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return "", err
	}

	for i, c := range name {
		if c == 0 {
			return string(name[:i]), nil
		}
	}
	return "", errors.New("pty: TIOCPTYGNAME returned a path that is not NUL-terminated")
}

// grantpt hands ownership of the slave device to the calling user; it is the
// equivalent of grantpt(3).
func grantpt(master *os.File) error {
	return ioctl(master, syscall.TIOCPTYGRANT, 0)
}

// unlockpt unlocks the slave device so that it can be opened; it is the
// equivalent of unlockpt(3).
func unlockpt(master *os.File) error {
	return ioctl(master, syscall.TIOCPTYUNLK, 0)
}

// openSlave opens the slave device at path without making it the controlling
// terminal of the current process.
func openSlave(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|syscall.O_NOCTTY, 0) //nolint:gosec // The path comes from the kernel.
}
