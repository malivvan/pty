//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pty

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// terminalModes is a named type over map[uint8]uint32, exactly as
// ssh.TerminalModes of golang.org/x/crypto/ssh is, so that the tests cover the
// fact that a client can pass the modes it received as they are.
type terminalModes map[uint8]uint32

func TestApplyTerminalModes(t *testing.T) {
	t.Parallel()

	ptmx, _ := openClose(t)

	modes := terminalModes{
		ModeECHO:        0,
		ModeICRNL:       0,
		ModeVERASE:      0x15,
		ModeVINTR:       0x03,
		ModeTTYOpISpeed: 38400,
		ModeTTYOpOSpeed: 38400,
		// A mode the system does not have is ignored.
		ModeVFLUSH: 0x0f,
	}
	noError(t, ApplyTerminalModes(int(ptmx.Fd()), 132, 43, modes), "Unexpected error from ApplyTerminalModes")

	termios, err := unix.IoctlGetTermios(int(ptmx.Fd()), termiosGetRequest)
	noError(t, err, "Unexpected error reading the terminal settings")

	if termios.Lflag&unix.ECHO != 0 {
		t.Error("The echo flag should have been cleared.")
	}
	if termios.Iflag&unix.ICRNL != 0 {
		t.Error("The input carriage return flag should have been cleared.")
	}
	assert(t, uint8(0x15), termios.Cc[unix.VERASE], "Unexpected erase character")
	assert(t, uint8(0x03), termios.Cc[unix.VINTR], "Unexpected interrupt character")

	rows, cols, err := GetSize(ptmx)
	noError(t, err, "Unexpected error from GetSize")
	assert(t, 43, rows, "Unexpected number of rows")
	assert(t, 132, cols, "Unexpected number of columns")
}

// TestApplyTerminalModesWithoutModes checks that a request without modes still
// resizes the terminal.
func TestApplyTerminalModesWithoutModes(t *testing.T) {
	t.Parallel()

	ptmx, _ := openClose(t)

	noError(t, ApplyTerminalModes(int(ptmx.Fd()), 100, 30, nil), "Unexpected error from ApplyTerminalModes")

	rows, cols, err := GetSize(ptmx)
	noError(t, err, "Unexpected error from GetSize")
	assert(t, 30, rows, "Unexpected number of rows")
	assert(t, 100, cols, "Unexpected number of columns")
}

// TestApplyTerminalModesOnFile checks that applying the modes to a file that is
// not a terminal reports the failure of the system call.
func TestApplyTerminalModesOnFile(t *testing.T) {
	t.Parallel()

	file, err := os.Open(os.DevNull)
	noError(t, err, "Unexpected error opening "+os.DevNull)
	defer func() { _ = file.Close() }() // Best effort.

	if err := ApplyTerminalModes(int(file.Fd()), 80, 24, terminalModes{ModeECHO: 1}); err == nil {
		t.Error("Applying terminal modes to a file that is not a terminal should have failed.")
	}
}

// TestApplyTerminalModesEveryMode checks that every mode of the protocol can be
// applied, whichever of them the system has: the ones it does not have are
// ignored, and the others are stored in the settings of the terminal.
func TestApplyTerminalModesEveryMode(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	opcodes := []uint8{
		ModeVINTR, ModeVQUIT, ModeVERASE, ModeVKILL, ModeVEOF, ModeVEOL, ModeVEOL2,
		ModeVSTART, ModeVSTOP, ModeVSUSP, ModeVDSUSP, ModeVREPRINT, ModeVWERASE,
		ModeVLNEXT, ModeVFLUSH, ModeVSWTCH, ModeVSTATUS, ModeVDISCARD, ModeVMIN, ModeVTIME,
		ModeIGNPAR, ModePARMRK, ModeINPCK, ModeISTRIP, ModeINLCR, ModeIGNCR, ModeICRNL,
		ModeIUCLC, ModeIXON, ModeIXANY, ModeIXOFF, ModeIMAXBEL, ModeIUTF8,
		ModeISIG, ModeICANON, ModeXCASE, ModeECHO, ModeECHOE, ModeECHOK, ModeECHONL,
		ModeNOFLSH, ModeTOSTOP, ModeIEXTEN, ModeECHOCTL, ModeECHOKE, ModePENDIN,
		ModeOPOST, ModeOLCUC, ModeONLCR, ModeOCRNL, ModeONOCR, ModeONLRET,
		ModeCS7, ModeCS8, ModePARENB, ModePARODD,
		ModeTTYOpISpeed, ModeTTYOpOSpeed,
	}

	modes := make(map[uint8]uint32, len(opcodes))
	for _, opcode := range opcodes {
		modes[opcode] = 1
	}
	noError(t, ApplyTerminalModes(int(ptmx.Fd()), 90, 30, modes), "Unexpected error from ApplyTerminalModes")

	// The terminal is still a terminal, and the command end of it was left
	// alone.
	rows, cols, err := GetSize(tty)
	noError(t, err, "Unexpected error from GetSize")
	assert(t, 30, rows, "Unexpected number of rows")
	assert(t, 90, cols, "Unexpected number of columns")

	termios, err := unix.IoctlGetTermios(int(ptmx.Fd()), termiosGetRequest)
	noError(t, err, "Unexpected error reading the terminal settings")
	assert(t, uint8(1), termios.Cc[unix.VINTR], "Unexpected interrupt character after applying every mode")
}
