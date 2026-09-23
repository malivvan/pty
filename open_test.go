//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import (
	"os"
	"testing"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	openClose(t)
}

func TestName(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	// The exact names differ from system to system, but they must not be empty.
	if ptmx.Name() == "" {
		t.Error("Pty name was empty.")
	}
	if tty.Name() == "" {
		t.Error("Tty name was empty.")
	}
}

// TestOpenByName checks that the name of the slave end is valid and can be
// opened by name, instead of by passing the file descriptor around.
func TestOpenByName(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	ttyFile, err := os.OpenFile(tty.Name(), os.O_RDWR, 0o600)
	noError(t, err, "Failed to open tty file")
	defer func() { _ = ttyFile.Close() }() // Best effort.

	// Write to the slave end opened by name and read it back from the master.
	text := []byte("ping")

	n, err := ttyFile.Write(text)
	noError(t, err, "Unexpected error from tty Write")
	assert(t, len(text), n, "Unexpected number of bytes written")

	assertBytes(t, text, readN(t, ptmx, len(text), "Unexpected error from ptmx Read"), "Unexpected result from ptmx Read")
}
