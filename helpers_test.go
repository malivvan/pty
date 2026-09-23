//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// openClose opens a pseudo-terminal pair and arranges for both ends to be
// closed when the test ends.
func openClose(t *testing.T) (ptmx, tty *os.File) {
	t.Helper()

	ptmx, tty, err := Open()
	if err != nil {
		t.Fatalf("Unexpected error from Open: %s.", err)
	}
	t.Cleanup(func() {
		if err := tty.Close(); err != nil {
			t.Errorf("Unexpected error from tty Close: %s.", err)
		}
		if err := ptmx.Close(); err != nil {
			t.Errorf("Unexpected error from ptmx Close: %s.", err)
		}
	})

	return ptmx, tty
}

// noError fails the test when err is not nil.
func noError(t *testing.T, err error, msg string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %s.", msg, err)
	}
}

// assert fails the test when got differs from want.
func assert[T comparable](t *testing.T, want, got T, msg string) {
	t.Helper()

	if want != got {
		t.Errorf("%s: want %v, got %v.", msg, want, got)
	}
}

// assertBytes fails the test when got differs from want. Byte slices are shown
// quoted, because the difference is usually a control character.
func assertBytes(t *testing.T, want, got []byte, msg string) {
	t.Helper()

	if !bytes.Equal(want, got) {
		t.Errorf("%s: want %q, got %q.", msg, want, got)
	}
}

// readN reads exactly n bytes from r and returns them, failing the test when
// that does not work.
func readN(t *testing.T, r io.Reader, n int, msg string) []byte {
	t.Helper()

	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	noError(t, err, msg)
	return buf
}
