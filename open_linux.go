//go:build linux

package pty

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// openMaster opens the /dev/ptmx cloning device.
func openMaster() (*os.File, error) {
	return os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
}

// ptsname returns the path of the slave device paired with master, derived from
// the index reported by the TIOCGPTN ioctl.
func ptsname(master *os.File) (string, error) {
	// TIOCGPTN writes the slave index as a C unsigned int.
	var index uint32
	if err := ioctl(master, syscall.TIOCGPTN, uintptr(unsafe.Pointer(&index))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return "", err
	}
	return "/dev/pts/" + strconv.Itoa(int(index)), nil
}

// grantpt is a no-op on Linux: the kernel gives the slave device to the caller
// of openMaster, and TIOCSPTLCK in unlockpt releases it.
func grantpt(*os.File) error { return nil }

// unlockpt unlocks the slave device so that it can be opened; it is the
// equivalent of unlockpt(3).
func unlockpt(master *os.File) error {
	// TIOCSPTLCK expects a pointer to a C int holding zero.
	var locked int32
	return ioctl(master, syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&locked))) //nolint:gosec // The pointer must be handed to the kernel as-is.
}

// openSlave opens the slave device at path without making it the controlling
// terminal of the current process.
func openSlave(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|syscall.O_NOCTTY, 0) //nolint:gosec // The path comes from the kernel.
}
