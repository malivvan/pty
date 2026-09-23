//go:build windows || plan9 || js || wasip1

package pty

import "os"

// SetSize always fails with ErrUnsupported: the systems this file is built for
// have no terminal size request. It exists so that portable code keeps
// compiling.
func SetSize(*os.File, *Winsize) error {
	return ErrUnsupported
}

// GetFullSize always fails with ErrUnsupported: the systems this file is built
// for have no terminal size request. It exists so that portable code keeps
// compiling.
func GetFullSize(*os.File) (*Winsize, error) {
	return nil, ErrUnsupported
}
