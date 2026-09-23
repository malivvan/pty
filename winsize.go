package pty

import "os"

// Winsize describes the size of a terminal, in character cells and in pixels.
//
// Rows and Cols are what terminal applications care about; X and Y are the
// pixel size and stay zero on terminals that do not report it.
type Winsize struct {
	Rows uint16 // Number of rows, in cells (ws_row).
	Cols uint16 // Number of columns, in cells (ws_col).
	X    uint16 // Width in pixels (ws_xpixel).
	Y    uint16 // Height in pixels (ws_ypixel).
}

// InheritSize reads the terminal size of from and applies it to to.
//
// It is commonly used in a SIGWINCH handler to keep a pseudo-terminal in step
// with the terminal the user is actually looking at:
//
//	winch := make(chan os.Signal, 1)
//	signal.Notify(winch, syscall.SIGWINCH)
//	for range winch {
//		if err := InheritSize(os.Stdin, ptmx); err != nil {
//			log.Printf("resizing pty: %v", err)
//		}
//	}
func InheritSize(from, to *os.File) error {
	size, err := GetFullSize(from)
	if err != nil {
		return err
	}
	return SetSize(to, size)
}

// GetSize returns the number of rows and the number of columns of the terminal
// backed by tty.
func GetSize(tty *os.File) (rows, cols int, err error) {
	size, err := GetFullSize(tty)
	if err != nil {
		return 0, 0, err
	}
	return int(size.Rows), int(size.Cols), nil
}
