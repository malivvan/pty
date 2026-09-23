//go:build aix || windows || plan9 || js || wasip1

package pty

import "os"

// open always fails: none of the systems this file is built for has a
// pseudo-terminal interface this package can drive. The rest of the API stays
// available so that portable programs keep compiling.
func open() (*os.File, *os.File, error) {
	return nil, nil, ErrUnsupported
}
