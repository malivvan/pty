//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import "testing"

func TestReadWriteText(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	// Write to the slave end, read from the master end.
	{
		text := []byte("ping")

		n, err := tty.Write(text)
		noError(t, err, "Unexpected error from tty Write")
		assert(t, len(text), n, "Unexpected number of bytes written to tty")

		assertBytes(t, text, readN(t, ptmx, len(text), "Unexpected error from ptmx Read"), "Unexpected result from ptmx Read")
	}

	// Write to the master end, read from the slave end. The text has to end in
	// a newline, otherwise the terminal driver keeps it in its line buffer and
	// the read below blocks.
	{
		text := []byte("pong\n")

		n, err := ptmx.Write(text)
		noError(t, err, "Unexpected error from ptmx Write")
		assert(t, len(text), n, "Unexpected number of bytes written to ptmx")

		// The slave end receives the text as it was written.
		assertBytes(t, text, readN(t, tty, len(text), "Unexpected error from tty Read"), "Unexpected result from tty Read")

		// The master end receives the echo of the text, with the line feed
		// expanded to a carriage return plus line feed by the terminal driver.
		echo := []byte("pong\r\n")
		assertBytes(t, echo, readN(t, ptmx, len(echo), "Unexpected error from ptmx Read"), "Unexpected echo from ptmx Read")
	}
}

func TestReadWriteControls(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	// Write the start of a line to the master end.
	n, err := ptmx.WriteString("pind") // Intentional typo.
	noError(t, err, "Unexpected error from ptmx WriteString")
	assert(t, 4, n, "Unexpected number of bytes written to ptmx")

	// Erase the last character.
	n, err = ptmx.WriteString("\b")
	noError(t, err, "Unexpected error from ptmx WriteString of the backspace")
	assert(t, 1, n, "Unexpected number of bytes written to ptmx")

	// Write the correct character and a line feed.
	n, err = ptmx.WriteString("g\n")
	noError(t, err, "Unexpected error from ptmx WriteString of the correction")
	assert(t, 2, n, "Unexpected number of bytes written to ptmx")

	// The slave end sees the control characters as the master sent them.
	assertBytes(t, []byte("pind\bg\n"), readN(t, tty, 7, "Unexpected error from tty Read"), "Unexpected result from tty Read")

	// The master end sees the echo of the line as the terminal driver renders
	// it: the backspace shows up as a caret notation control character.
	assertBytes(t, []byte("pind^Hg\r\n"), readN(t, ptmx, 9, "Unexpected error from ptmx Read"), "Unexpected echo from ptmx Read")
}
