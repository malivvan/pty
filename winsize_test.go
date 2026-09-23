//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import "testing"

func TestGetSize(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	ptmxRows, ptmxCols, err := GetSize(ptmx)
	noError(t, err, "Unexpected error from ptmx GetSize")

	ttyRows, ttyCols, err := GetSize(tty)
	noError(t, err, "Unexpected error from tty GetSize")

	assert(t, ptmxRows, ttyRows, "Rows from GetSize on ptmx and tty should match")
	assert(t, ptmxCols, ttyCols, "Cols from GetSize on ptmx and tty should match")
}

func TestGetFullSize(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	ptmxSize, err := GetFullSize(ptmx)
	noError(t, err, "Unexpected error from ptmx GetFullSize")

	ttySize, err := GetFullSize(tty)
	noError(t, err, "Unexpected error from tty GetFullSize")

	assert(t, ptmxSize.X, ttySize.X, "X from GetFullSize on ptmx and tty should match")
	assert(t, ptmxSize.Y, ttySize.Y, "Y from GetFullSize on ptmx and tty should match")
	assert(t, ptmxSize.Rows, ttySize.Rows, "Rows from GetFullSize on ptmx and tty should match")
	assert(t, ptmxSize.Cols, ttySize.Cols, "Cols from GetFullSize on ptmx and tty should match")
}

func TestSetSize(t *testing.T) {
	t.Parallel()

	ptmx, tty := openClose(t)

	size, err := GetFullSize(ptmx)
	noError(t, err, "Unexpected error from ptmx GetFullSize")

	size.X++
	size.Y++
	size.Rows++
	size.Cols++

	noError(t, SetSize(tty, size), "Unexpected error from SetSize")

	got, err := GetFullSize(tty)
	noError(t, err, "Unexpected error from tty GetFullSize")

	assert(t, size.X, got.X, "Unexpected X after SetSize")
	assert(t, size.Y, got.Y, "Unexpected Y after SetSize")
	assert(t, size.Rows, got.Rows, "Unexpected Rows after SetSize")
	assert(t, size.Cols, got.Cols, "Unexpected Cols after SetSize")
}
