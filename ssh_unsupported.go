//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package pty

// applyTerminalModes always fails: the systems this file is built for have
// either no terminal settings to apply or none that this package can drive.
//
// AIX and z/OS have their own flavour of termios, which the Go library does not
// describe well enough to be trusted; a Windows pseudo-console is driven
// through ConPTY, which has no terminal settings at all; and the WebAssembly
// targets have no terminals.
func applyTerminalModes(fd int, width, height int, modes map[uint8]uint32) error {
	return ErrUnsupported
}
