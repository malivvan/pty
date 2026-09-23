package pty

import "syscall"

// conPTYDevice is the contract every ConPTY implementation in this package
// honours: the real one in conpty_windows.go and the stub that takes its place
// on all other systems. The assertion below keeps the two in step, so that
// [NewConPTY] behaves the same way wherever it is called.
type conPTYDevice interface {
	Close() error
	Fd() uintptr
	InPipeReadFd() uintptr
	InPipeWriteFd() uintptr
	OutPipeReadFd() uintptr
	OutPipeWriteFd() uintptr
	Read(p []byte) (int, error)
	Resize(width, height int) error
	Size() (width, height int, err error)
	Spawn(name string, args []string, attr *syscall.ProcAttr) (pid int, handle uintptr, err error)
	Write(p []byte) (int, error)
}

var _ conPTYDevice = &ConPTY{}
