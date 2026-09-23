//go:build linux

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
	ModeVSWTCH:   func(t *unix.Termios, v uint32) { t.Cc[unix.VSWTC] = uint8(v) },
	ModeVDISCARD: func(t *unix.Termios, v uint32) { t.Cc[unix.VDISCARD] = uint8(v) },
}

// sshModeFlagsExtra holds the flags of this system that are not part of the
// portable set in ssh_termios.go.
var sshModeFlagsExtra = map[uint8]func(*unix.Termios, bool){
	ModeIUCLC:   func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IUCLC, on) },
	ModeIMAXBEL: func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IMAXBEL, on) },
	ModeIUTF8:   func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IUTF8, on) },

	ModeXCASE:   func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.XCASE, on) },
	ModeECHOCTL: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOCTL, on) },
	ModeECHOKE:  func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOKE, on) },
	ModePENDIN:  func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.PENDIN, on) },

	ModeOLCUC: func(t *unix.Termios, on bool) { setFlag(&t.Oflag, unix.OLCUC, on) },
}

// sshBaudCodesExtra holds the baud rates of this system that are not part of
// the portable set in ssh_termios.go.
var sshBaudCodesExtra = map[uint32]uint32{
	230400:  unix.B230400,
	460800:  unix.B460800,
	500000:  unix.B500000,
	576000:  unix.B576000,
	921600:  unix.B921600,
	1000000: unix.B1000000,
	1152000: unix.B1152000,
	1500000: unix.B1500000,
	2000000: unix.B2000000,
	2500000: unix.B2500000,
	3000000: unix.B3000000,
	3500000: unix.B3500000,
	4000000: unix.B4000000,
}

// setTermiosSpeed stores a baud code in one of the two speed fields of a
// termios structure.
func setTermiosSpeed(t *unix.Termios, input bool, speed uint32) {
	if input {
		setSpeed(&t.Ispeed, speed)
		return
	}
	setSpeed(&t.Ospeed, speed)
}
