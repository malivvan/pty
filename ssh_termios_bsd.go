//go:build dragonfly || freebsd || netbsd || openbsd

package pty

import (
	"golang.org/x/sys/unix"
)

// The ioctls that read and write the terminal settings of a file descriptor.
const (
	termiosGetRequest = unix.TIOCGETA
	termiosSetRequest = unix.TIOCSETA
)

// sshModeCharsExtra holds the control characters of these systems that are not
// part of the portable set in ssh_termios.go.
var sshModeCharsExtra = map[uint8]func(*unix.Termios, uint32){
	ModeVEOL2:    func(t *unix.Termios, v uint32) { t.Cc[unix.VEOL2] = uint8(v) },
	ModeVDSUSP:   func(t *unix.Termios, v uint32) { t.Cc[unix.VDSUSP] = uint8(v) },
	ModeVREPRINT: func(t *unix.Termios, v uint32) { t.Cc[unix.VREPRINT] = uint8(v) },
	ModeVWERASE:  func(t *unix.Termios, v uint32) { t.Cc[unix.VWERASE] = uint8(v) },
	ModeVLNEXT:   func(t *unix.Termios, v uint32) { t.Cc[unix.VLNEXT] = uint8(v) },
	ModeVSTATUS:  func(t *unix.Termios, v uint32) { t.Cc[unix.VSTATUS] = uint8(v) },
	ModeVDISCARD: func(t *unix.Termios, v uint32) { t.Cc[unix.VDISCARD] = uint8(v) },
}

// sshModeFlagsExtra holds the flags of these systems that are not part of the
// portable set in ssh_termios.go.
var sshModeFlagsExtra = map[uint8]func(*unix.Termios, bool){
	ModeECHOCTL: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOCTL, on) },
	ModeECHOKE:  func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOKE, on) },
	ModePENDIN:  func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.PENDIN, on) },
}

// sshBaudCodesExtra holds the baud rates of these systems that are not part of
// the portable set in ssh_termios.go.
var sshBaudCodesExtra = map[uint32]uint32{
	57600:  unix.B57600,
	115200: unix.B115200,
	230400: unix.B230400,
}

// setTermiosSpeed stores a baud code in one of the two speed fields of a
// termios structure, whose type differs between these systems.
func setTermiosSpeed(t *unix.Termios, input bool, speed uint32) {
	if input {
		setSpeed(&t.Ispeed, speed)
		return
	}
	setSpeed(&t.Ospeed, speed)
}
