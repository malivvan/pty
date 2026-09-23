//go:build windows || plan9 || js || wasip1

package pty

import (
	"os"
	"os/exec"
)

// StartWithSize always fails with ErrUnsupported: the systems this file is
// built for have no pseudo-terminal to start a command on. On Windows, use
// [NewConPTY] and [ConPTY.Spawn] instead.
func StartWithSize(*exec.Cmd, *Winsize) (*os.File, error) {
	return nil, ErrUnsupported
}
