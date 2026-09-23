package pty

// Terminal mode opcodes of RFC 4254 section 8, the modes an SSH client sends in
// its pseudo-terminal request. They are the keys of the modes an SSH client
// asks for, and the values are the settings themselves: the value of a control
// character opcode is the character, the value of a flag opcode is 1 to set the
// flag and 0 to clear it, and the value of a speed opcode is a baud rate.
//
// A client that uses golang.org/x/crypto/ssh passes ssh.TerminalModes, which is
// the same map, to [ApplyTerminalModes].
const (
	ModeVINTR    = 1
	ModeVQUIT    = 2
	ModeVERASE   = 3
	ModeVKILL    = 4
	ModeVEOF     = 5
	ModeVEOL     = 6
	ModeVEOL2    = 7
	ModeVSTART   = 8
	ModeVSTOP    = 9
	ModeVSUSP    = 10
	ModeVDSUSP   = 11
	ModeVREPRINT = 12
	ModeVWERASE  = 13
	ModeVLNEXT   = 14
	ModeVFLUSH   = 15
	ModeVSWTCH   = 16
	ModeVSTATUS  = 17
	ModeVDISCARD = 18
	ModeVMIN     = 19
	ModeVTIME    = 20

	ModeIGNPAR  = 30
	ModePARMRK  = 31
	ModeINPCK   = 32
	ModeISTRIP  = 33
	ModeINLCR   = 34
	ModeIGNCR   = 35
	ModeICRNL   = 36
	ModeIUCLC   = 37
	ModeIXON    = 38
	ModeIXANY   = 39
	ModeIXOFF   = 40
	ModeIMAXBEL = 41
	ModeIUTF8   = 42

	ModeISIG    = 50
	ModeICANON  = 51
	ModeXCASE   = 52
	ModeECHO    = 53
	ModeECHOE   = 54
	ModeECHOK   = 55
	ModeECHONL  = 56
	ModeNOFLSH  = 57
	ModeTOSTOP  = 58
	ModeIEXTEN  = 59
	ModeECHOCTL = 60
	ModeECHOKE  = 61
	ModePENDIN  = 62

	ModeOPOST  = 70
	ModeOLCUC  = 71
	ModeONLCR  = 72
	ModeOCRNL  = 73
	ModeONOCR  = 74
	ModeONLRET = 75

	ModeCS7    = 90
	ModeCS8    = 91
	ModePARENB = 92
	ModePARODD = 93

	ModeTTYOpISpeed = 128
	ModeTTYOpOSpeed = 129
)

// ApplyTerminalModes applies the terminal modes of an SSH pseudo-terminal
// request to the terminal with the given file descriptor, and resizes it to
// width by height character cells unless both are zero.
//
// modes holds the opcodes of RFC 4254 section 8, the [Mode] constants of this
// package; ssh.TerminalModes is the same map and can be passed as it is. A mode
// the system does not have is ignored: a client asks for its terminal settings,
// it does not demand them.
//
// The file descriptor comes from a terminal of this package, either the master
// end returned by [Open] and [Start] or the end returned by [VirtualPTY.Slave],
// and it must be a terminal.
//
// Systems without terminal settings support report [ErrUnsupported].
func ApplyTerminalModes(fd int, width, height int, modes map[uint8]uint32) error {
	return applyTerminalModes(fd, width, height, modes)
}
