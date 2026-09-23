//go:build aix

package pty

// ioctlInner always fails on AIX: this package does not implement the AIX
// terminal ioctls, so every terminal operation reports ErrUnsupported.
func ioctlInner(uintptr, uintptr, uintptr) error {
	return ErrUnsupported
}
