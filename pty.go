package pty

import (
	"errors"
	"os"
)

// ErrUnsupported is returned by functions that have no implementation on the
// current platform. Use [errors.Is] to test for it.
var ErrUnsupported = errors.New("pty: unsupported")

// Default size requested by [NewConPTY] and [NewConPTYWithPipes] when they are
// asked for a non-positive width or height.
const (
	DefaultWidth  = 80
	DefaultHeight = 25
)

// Open allocates a pseudo-terminal pair and returns its master end (the ptmx)
// and its slave end (the tty).
//
// Both files belong to the caller, which must close them. Systems without
// pseudo-terminal support return [ErrUnsupported].
func Open() (ptmx, tty *os.File, err error) {
	return open()
}
