//go:build zos

package pty

import (
	"os"
	"testing"
)

// getNonBlockingFile returns a file that reads without blocking. z/OS does not
// hand out a pollable file descriptor for a pseudo-terminal, so the descriptor
// has to be switched to non-blocking I/O first.
func getNonBlockingFile(t *testing.T, file *os.File, path string) *os.File {
	t.Helper()

	if _, err := fcntl(uintptr(file.Fd()), F_SETFL, O_NONBLOCK); err != nil {
		t.Fatalf("Error: zos-nonblock: %s.\n", err)
	}

	nonBlocking := os.NewFile(file.Fd(), path)
	t.Cleanup(func() { _ = nonBlocking.Close() }) // Best effort.
	return nonBlocking
}
