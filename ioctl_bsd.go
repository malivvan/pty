//go:build darwin || dragonfly || freebsd

package pty

// Request encoding macros of <sys/ioccom.h>, needed for the requests that no
// syscall package exposes. Only the macros this package uses are repeated
// here; the header lists the complete set.

const (
	_IOC_IN          uintptr = 0x80000000
	_IOC_PARAM_SHIFT         = 13
	_IOC_PARAM_MASK          = (1 << _IOC_PARAM_SHIFT) - 1
)

// _IOC_PARAM_LEN returns the length of the argument encoded in request.
func _IOC_PARAM_LEN(request uintptr) uintptr {
	return (request >> 16) & _IOC_PARAM_MASK
}

// _IOC encodes an ioctl request of the given direction, group, number and
// argument length.
func _IOC(direction uintptr, group byte, number uintptr, paramLen uintptr) uintptr {
	return direction | (paramLen&_IOC_PARAM_MASK)<<16 | uintptr(group)<<8 | number
}

// _IOW encodes an _IOW(group, number, argument) request, which passes data from
// user space to the kernel.
func _IOW(group byte, number uintptr, paramLen uintptr) uintptr {
	return _IOC(_IOC_IN, group, number, paramLen)
}
