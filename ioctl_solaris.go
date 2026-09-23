//go:build solaris

package pty

import (
	"syscall"
	"unsafe"
)

// Requests and commands of the Solaris STREAMS pseudo-terminal driver. The
// values are the ones the C library passes to ioctl(2); see <sys/stropts.h>
// and <sys/ptms.h>.
const (
	I_PUSH = uintptr((int32('S') << 8) | 002)
	I_STR  = uintptr((int32('S') << 8) | 010)
	I_FIND = uintptr((int32('S') << 8) | 013)

	ISPTM   = (int32('P') << 8) | 1
	UNLKPT  = (int32('P') << 8) | 2
	OWNERPT = (int32('P') << 8) | 5
)

// strioctl is the argument of the I_STR request, which passes a STREAMS ioctl
// through to a module; it mirrors struct strioctl of <sys/stropts.h>.
type strioctl struct {
	icCmd     int32
	icTimeout int32
	icLen     int32
	icDP      unsafe.Pointer
}

//go:cgo_import_dynamic libc_ioctl ioctl "libc.so"
//go:linkname procioctl libc_ioctl
var procioctl uintptr

// sysvicall6 is defined in ioctl_solaris_amd64.s.
func sysvicall6(trap, nargs, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

// ioctlInner issues the ioctl request on the raw file descriptor fd.
//
// Solaris has no system call number for ioctl that Go can use, so the ioctl
// function of the C library is called through the dynamic linker instead.
func ioctlInner(fd, request, arg uintptr) error {
	if _, _, errno := sysvicall6(uintptr(unsafe.Pointer(&procioctl)), 3, fd, request, arg, 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}
