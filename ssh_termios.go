//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pty

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// applyTerminalModes applies the modes of an SSH pseudo-terminal request to the
// terminal with the given file descriptor.
//
// The modes are applied on top of the current settings, in no particular order:
// each of them stands for itself.
func applyTerminalModes(fd int, width, height int, modes map[uint8]uint32) error {
	termios, err := unix.IoctlGetTermios(fd, termiosGetRequest)
	if err != nil {
		return fmt.Errorf("pty: reading the terminal settings: %w", err)
	}

	for opcode, value := range modes {
		switch {
		case opcode == ModeTTYOpISpeed:
			if speed, ok := sshBaudCodes[value]; ok {
				setTermiosSpeed(termios, true, speed)
			}
		case opcode == ModeTTYOpOSpeed:
			if speed, ok := sshBaudCodes[value]; ok {
				setTermiosSpeed(termios, false, speed)
			}
		case sshModeChars[opcode] != nil:
			sshModeChars[opcode](termios, value)
		case sshModeFlags[opcode] != nil:
			sshModeFlags[opcode](termios, value > 0)
		}
	}

	if err := unix.IoctlSetTermios(fd, termiosSetRequest, termios); err != nil {
		return fmt.Errorf("pty: applying the terminal settings: %w", err)
	}

	if width != 0 || height != 0 {
		size := &unix.Winsize{Row: uint16(height), Col: uint16(width)}
		if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, size); err != nil {
			return fmt.Errorf("pty: resizing the terminal: %w", err)
		}
	}
	return nil
}

// sshModeChars maps the mode opcodes that set a control character to the
// function that stores their value in a termios structure. The entries that
// only some systems have live in the sshModeCharsExtra maps of the files next
// to this one, which are merged into this one at start-up.
var sshModeChars = map[uint8]func(*unix.Termios, uint32){
	ModeVINTR:  func(t *unix.Termios, v uint32) { t.Cc[unix.VINTR] = uint8(v) },
	ModeVQUIT:  func(t *unix.Termios, v uint32) { t.Cc[unix.VQUIT] = uint8(v) },
	ModeVERASE: func(t *unix.Termios, v uint32) { t.Cc[unix.VERASE] = uint8(v) },
	ModeVKILL:  func(t *unix.Termios, v uint32) { t.Cc[unix.VKILL] = uint8(v) },
	ModeVEOF:   func(t *unix.Termios, v uint32) { t.Cc[unix.VEOF] = uint8(v) },
	ModeVEOL:   func(t *unix.Termios, v uint32) { t.Cc[unix.VEOL] = uint8(v) },
	ModeVSTART: func(t *unix.Termios, v uint32) { t.Cc[unix.VSTART] = uint8(v) },
	ModeVSTOP:  func(t *unix.Termios, v uint32) { t.Cc[unix.VSTOP] = uint8(v) },
	ModeVSUSP:  func(t *unix.Termios, v uint32) { t.Cc[unix.VSUSP] = uint8(v) },
	ModeVMIN:   func(t *unix.Termios, v uint32) { t.Cc[unix.VMIN] = uint8(v) },
	ModeVTIME:  func(t *unix.Termios, v uint32) { t.Cc[unix.VTIME] = uint8(v) },
}

// sshModeFlags maps the mode opcodes that set or clear a flag to the function
// that does so. The entries that only some systems have live in the
// sshModeFlagsExtra maps of the files next to this one.
var sshModeFlags = map[uint8]func(*unix.Termios, bool){
	ModeIGNPAR: func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IGNPAR, on) },
	ModePARMRK: func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.PARMRK, on) },
	ModeINPCK:  func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.INPCK, on) },
	ModeISTRIP: func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.ISTRIP, on) },
	ModeINLCR:  func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.INLCR, on) },
	ModeIGNCR:  func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IGNCR, on) },
	ModeICRNL:  func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.ICRNL, on) },
	ModeIXON:   func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IXON, on) },
	ModeIXANY:  func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IXANY, on) },
	ModeIXOFF:  func(t *unix.Termios, on bool) { setFlag(&t.Iflag, unix.IXOFF, on) },

	ModeISIG:   func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ISIG, on) },
	ModeICANON: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ICANON, on) },
	ModeECHO:   func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHO, on) },
	ModeECHOE:  func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOE, on) },
	ModeECHOK:  func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHOK, on) },
	ModeECHONL: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.ECHONL, on) },
	ModeNOFLSH: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.NOFLSH, on) },
	ModeTOSTOP: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.TOSTOP, on) },
	ModeIEXTEN: func(t *unix.Termios, on bool) { setFlag(&t.Lflag, unix.IEXTEN, on) },

	ModeOPOST:  func(t *unix.Termios, on bool) { setFlag(&t.Oflag, unix.OPOST, on) },
	ModeONLCR:  func(t *unix.Termios, on bool) { setFlag(&t.Oflag, unix.ONLCR, on) },
	ModeOCRNL:  func(t *unix.Termios, on bool) { setFlag(&t.Oflag, unix.OCRNL, on) },
	ModeONOCR:  func(t *unix.Termios, on bool) { setFlag(&t.Oflag, unix.ONOCR, on) },
	ModeONLRET: func(t *unix.Termios, on bool) { setFlag(&t.Oflag, unix.ONLRET, on) },

	ModePARENB: func(t *unix.Termios, on bool) { setFlag(&t.Cflag, unix.PARENB, on) },
	ModePARODD: func(t *unix.Termios, on bool) { setFlag(&t.Cflag, unix.PARODD, on) },

	// The character size is not a flag that can be set on its own: the bits of
	// the size are exclusive, so the request is applied by selecting the size.
	ModeCS7: func(t *unix.Termios, on bool) { setCharSize(&t.Cflag, unix.CS7, on) },
	ModeCS8: func(t *unix.Termios, on bool) { setCharSize(&t.Cflag, unix.CS8, on) },
}

// sshBaudCodes maps the baud rates an SSH client can ask for to the baud codes
// a termios structure holds.
//
// Only the rates every system has are here: the ones from 50 to 38400 are the
// ones the standards require, the higher ones are spelled differently on every
// system and live in the sshBaudCodesExtra maps of the files next to this one.
// A rate that is not in here is ignored, as it means nothing to a
// pseudo-terminal anyway.
var sshBaudCodes = map[uint32]uint32{
	0:     unix.B0,
	50:    unix.B50,
	75:    unix.B75,
	110:   unix.B110,
	134:   unix.B134,
	150:   unix.B150,
	200:   unix.B200,
	300:   unix.B300,
	600:   unix.B600,
	1200:  unix.B1200,
	1800:  unix.B1800,
	2400:  unix.B2400,
	4800:  unix.B4800,
	9600:  unix.B9600,
	19200: unix.B19200,
	38400: unix.B38400,
}

// setFlag sets or clears flag in field.
func setFlag[T ~uint32 | ~uint64](field *T, flag T, on bool) {
	if on {
		*field |= flag
		return
	}
	*field &^= flag
}

// setCharSize selects size as the character size of field when on.
func setCharSize[T ~uint32 | ~uint64](field *T, size T, on bool) {
	if !on {
		return
	}
	*field = (*field &^ unix.CSIZE) | size
}

// setSpeed stores a baud code in one of the two speed fields of a termios
// structure, whose type depends on the system.
func setSpeed[T ~int32 | ~int64 | ~uint32 | ~uint64](field *T, speed uint32) {
	*field = T(speed)
}

func init() {
	for opcode, set := range sshModeCharsExtra {
		sshModeChars[opcode] = set
	}
	for opcode, set := range sshModeFlagsExtra {
		sshModeFlags[opcode] = set
	}
	for rate, code := range sshBaudCodesExtra {
		sshBaudCodes[rate] = code
	}
}
