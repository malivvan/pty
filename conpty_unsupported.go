//go:build !windows

package pty

import "syscall"

// ConPTY is the stand-in for the Windows pseudo-console on systems that have no
// ConPTY: it keeps programs that mention the type compiling, and every
// operation fails with [ErrUnsupported]. On Windows it is implemented by
// conpty_windows.go and fully functional.
type ConPTY struct{}

// NewConPTY creates a pseudo-console of the requested size. It always fails
// with [ErrUnsupported] outside Windows.
func NewConPTY(width, height, flags int) (*ConPTY, error) {
	return nil, ErrUnsupported
}

// NewConPTYWithPipes creates a pseudo-console that uses the given pipe handles.
// It always fails with [ErrUnsupported] outside Windows.
func NewConPTYWithPipes(inPipeRead, inPipeWrite, outPipeRead, outPipeWrite uintptr, width, height, flags int) (*ConPTY, error) {
	return nil, ErrUnsupported
}

// CreatePipes creates the pair of pipes a pseudo-console is driven through. It
// always fails with [ErrUnsupported] outside Windows.
func CreatePipes() (inPipeRead, inPipeWrite, outPipeRead, outPipeWrite uintptr, err error) {
	return 0, 0, 0, 0, ErrUnsupported
}

// Close always fails with [ErrUnsupported].
func (*ConPTY) Close() error { return ErrUnsupported }

// Fd always returns 0.
func (*ConPTY) Fd() uintptr { return 0 }

// Read always fails with [ErrUnsupported].
func (*ConPTY) Read([]byte) (int, error) { return 0, ErrUnsupported }

// Write always fails with [ErrUnsupported].
func (*ConPTY) Write([]byte) (int, error) { return 0, ErrUnsupported }

// Resize always fails with [ErrUnsupported].
func (*ConPTY) Resize(width, height int) error { return ErrUnsupported }

// Size always fails with [ErrUnsupported].
func (*ConPTY) Size() (int, int, error) { return 0, 0, ErrUnsupported }

// InPipeReadFd always returns 0.
func (*ConPTY) InPipeReadFd() uintptr { return 0 }

// InPipeWriteFd always returns 0.
func (*ConPTY) InPipeWriteFd() uintptr { return 0 }

// OutPipeReadFd always returns 0.
func (*ConPTY) OutPipeReadFd() uintptr { return 0 }

// OutPipeWriteFd always returns 0.
func (*ConPTY) OutPipeWriteFd() uintptr { return 0 }

// Spawn always fails with [ErrUnsupported].
func (*ConPTY) Spawn(name string, args []string, attr *syscall.ProcAttr) (pid int, handle uintptr, err error) {
	return 0, 0, ErrUnsupported
}
