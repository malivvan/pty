//go:build !zos

package pty

import (
	"os"
	"testing"
)

// getNonBlockingFile returns file unchanged on the systems whose file
// descriptors are pollable as they are, so that the tests using it do not have
// to care.
func getNonBlockingFile(t *testing.T, file *os.File, path string) *os.File {
	t.Helper()

	return file
}
