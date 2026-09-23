//go:build solaris

package pty

import (
	"golang.org/x/sys/unix"
)

// The ioctls that read and write the terminal settings of a file descriptor.
const (
	termiosGetRequest = unix.TCGETS
	termiosSetRequest = unix.TCSETS
)

// sshModeCharsExtra holds the control characters of this system that are not
// part of the portable set in ssh_termios.go.
var sshModeCharsExtra = map[uint8]func(*unix.Termios, uint32){
	ModeVEOL2:    func(t *unix.Termios, v uint32) { t.Cc[unix.VEOL2] = uint8(v) },
	ModeVREPRINT: func(t *unix.Termios, v uint32) { t.Cc[unix.VREPRINT] = uint8(v) },
	ModeVWERASE:  func(t *unix.Termios, v uint32) { t.Cc[unix.VWERASE] = uint8(v) },
	ModeVLNEXT:   func(t *unix.Termios, v uint32) { t.Cc[unix.VLNEXT] = uint8(v) },
	ModeVDISCARD: func(t *unix.Termios, v uint32) { t.Cc[unix.VDISCARD] = uint8(v) },
}

// sshModeFlagsExtra holds the flags of this system that are not part of the
// portable set in ssh_termios.go.
var sshModeFlagsExtra = map[uint8]func(*unix.Termios, bool){
	ModeXCASE:   func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.XCASE, on) },
	ModeECHOCTL: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOCTL, on) },
	ModeECHOKE:  func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOKE, on) },

	ModeOLCUC: func(t *unix.Termios, on bool) { setFlag(&t.Oflag, unix.OLCUC, on) },
}

// sshBaudCodesExtra holds the baud rates of this system that are not part of
// the portable set in ssh_termios.go.
var sshBaudCodesExtra = map[uint32]uint32{
	230400: unix.B230400,
	460800: unix.B460800,
}

// setTermiosSpeed does nothing: the termios structure of this system has no
// speed fields, and the speed of a pseudo-terminal means nothing anyway.
func setTermiosSpeed(t *unix.Termios, input bool, speed uint32) {}
