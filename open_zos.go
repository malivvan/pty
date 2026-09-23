//go:build zos

package pty

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// System call numbers of the C library entry points this file calls, and the
// constants they take. They come from the z/OS LE callable service table.
const (
	SYS_GRANTPT      = 0x37A
	SYS_UNLOCKPT     = 0x37B
	SYS_POSIX_OPENPT = 0xC66
	SYS_FCNTL        = 0x18C
	SYS___PTSNAME_A  = 0x718

	// Argument of the F_CONTROL_CVT fcntl, which tags a file descriptor with
	// the code page used for its data.
	SETCVTON = 1

	// fcntl commands and open flags.
	O_NONBLOCK    = 0x04
	F_SETFL       = 4
	F_CONTROL_CVT = 13
)

// fCnvrt is the argument of the F_CONTROL_CVT fcntl; it mirrors struct
// f_cnvrt of <fcntl.h>.
type fCnvrt struct {
	Cvtcmd int32
	Pccsid int16
	Fccsid int16
}

// open allocates a pseudo-terminal pair.
//
// z/OS needs the file descriptors to be tagged explicitly: without the
// F_CONTROL_CVT fcntl the data read from the terminal would be garbled when
// the slave device is not tagged with the expected code page.
func open() (ptmx, tty *os.File, err error) {
	ptmxFD, err := posixOpenpt(os.O_RDWR | syscall.O_NOCTTY)
	if err != nil {
		return nil, nil, err
	}
	ptmx = os.NewFile(uintptr(ptmxFD), "/dev/ptmx")

	// Make sure the master end never leaks when a step below fails.
	defer func() {
		if err != nil {
			_ = ptmx.Close() // Best effort.
		}
	}()

	cvt := fCnvrt{Cvtcmd: SETCVTON, Pccsid: 0, Fccsid: 1047}
	if _, err := fcntl(uintptr(ptmxFD), F_CONTROL_CVT, uintptr(unsafe.Pointer(&cvt))); err != nil {
		return nil, nil, err
	}

	slaveName, err := ptsname(ptmxFD)
	if err != nil {
		return nil, nil, err
	}

	if _, err := grantpt(ptmxFD); err != nil {
		return nil, nil, err
	}

	if _, err := unlockpt(ptmxFD); err != nil {
		return nil, nil, err
	}

	slaveFD, err := syscall.Open(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	tty = os.NewFile(uintptr(slaveFD), slaveName)

	if _, err := fcntl(uintptr(slaveFD), F_CONTROL_CVT, uintptr(unsafe.Pointer(&cvt))); err != nil {
		_ = tty.Close() // Best effort.
		return nil, nil, err
	}

	return ptmx, tty, nil
}

// posixOpenpt calls posix_openpt(3).
func posixOpenpt(flags int) (int, error) {
	fd, _, errno := runtime.CallLeFuncWithErr(runtime.GetZosLibVec()+SYS_POSIX_OPENPT<<4, uintptr(flags))
	if errno != 0 {
		return 0, syscall.Errno(errno)
	}
	return int(fd), nil
}

// fcntl calls fcntl(3).
func fcntl(fd uintptr, cmd int, arg uintptr) (int, error) {
	value, _, errno := runtime.CallLeFuncWithErr(runtime.GetZosLibVec()+SYS_FCNTL<<4, fd, uintptr(cmd), arg)
	if errno != 0 {
		return 0, syscall.Errno(errno)
	}
	return int(value), nil
}

// ptsname calls __ptsname_a(3), which returns a pointer to the name of the
// slave device paired with fd.
func ptsname(fd int) (string, error) {
	name, _, errno := runtime.CallLeFuncWithPtrReturn(runtime.GetZosLibVec()+SYS___PTSNAME_A<<4, uintptr(fd))
	if errno != 0 {
		return "", syscall.Errno(errno)
	}
	return cStringPointer(unsafe.Pointer(name)), nil
}

// grantpt calls grantpt(3).
func grantpt(fd int) (int, error) {
	rc, _, errno := runtime.CallLeFuncWithErr(runtime.GetZosLibVec()+SYS_GRANTPT<<4, uintptr(fd))
	if errno != 0 {
		return 0, syscall.Errno(errno)
	}
	return int(rc), nil
}

// unlockpt calls unlockpt(3).
func unlockpt(fd int) (int, error) {
	rc, _, errno := runtime.CallLeFuncWithErr(runtime.GetZosLibVec()+SYS_UNLOCKPT<<4, uintptr(fd))
	if errno != 0 {
		return 0, syscall.Errno(errno)
	}
	return int(rc), nil
}

// cStringPointer converts a NUL-terminated C string into a Go string.
func cStringPointer(cstr unsafe.Pointer) string {
	if cstr == nil {
		return ""
	}

	// The callers get device names out of this, which never come near 1 KiB.
	buf := (*[1024]uint8)(cstr)
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return string(buf[:n])
}
