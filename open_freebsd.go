//go:build freebsd

package pty

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// ioctlFIODGNAME is the request that fills a fiodgnameArg with the name of the
// device backing a master end; see <sys/filio.h>.
var ioctlFIODGNAME = _IOW('f', 120, unsafe.Sizeof(fiodgnameArg{}))

// openMaster allocates a master end with posix_openpt, the FreeBSD spelling of
// opening /dev/ptmx.
func openMaster() (*os.File, error) {
	fd, err := posixOpenpt(syscall.O_RDWR | syscall.O_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), nil
}

// posixOpenpt calls posix_openpt(3).
func posixOpenpt(flags int) (int, error) {
	fd, _, errno := syscall.Syscall(syscall.SYS_POSIX_OPENPT, uintptr(flags), 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

// ptsname returns the name of the slave device paired with master, relative to
// /dev, as reported by the FIODGNAME ioctl; it is the equivalent of ptsname(3).
func ptsname(master *os.File) (string, error) {
	isMaster, err := isptmaster(master)
	if err != nil {
		return "", err
	}
	if !isMaster {
		return "", syscall.EINVAL
	}

	// FIODGNAME takes the buffer length in its argument and truncates the name
	// instead of failing when it does not fit.
	const nameLen = _C_SPECNAMELEN + 1
	name := make([]byte, nameLen)
	arg := fiodgnameArg{
		Len: nameLen,
		Buf: (*byte)(unsafe.Pointer(&name[0])),
	}
	if err := ioctl(master, ioctlFIODGNAME, uintptr(unsafe.Pointer(&arg))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return "", err
	}

	for i, c := range name {
		if c == 0 {
			return string(name[:i]), nil
		}
	}
	return "", errors.New("pty: FIODGNAME returned a name that is not NUL-terminated")
}

// isptmaster reports whether file is a pseudo-terminal master, using the
// TIOCPTMASTER ioctl.
func isptmaster(file *os.File) (bool, error) {
	err := ioctl(file, syscall.TIOCPTMASTER, 0)
	return err == nil, err
}

// grantpt is a no-op on FreeBSD: posix_openpt hands the slave device to the
// caller with the right ownership.
func grantpt(*os.File) error { return nil }

// unlockpt is a no-op on FreeBSD: the slave device returned by posix_openpt is
// already unlocked.
func unlockpt(*os.File) error { return nil }

// openSlave opens the slave device at path, which is relative to /dev.
func openSlave(path string) (*os.File, error) {
	return os.OpenFile("/dev/"+path, os.O_RDWR, 0) //nolint:gosec // The path comes from the kernel.
}
