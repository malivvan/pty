//go:build aix || windows || plan9 || js || wasip1

package pty

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// TestUnsupportedPlatform checks that every entry point of the package reports
// ErrUnsupported on the systems without pseudo-terminal support, so that a
// program using it can degrade instead of failing to build.
func TestUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	if _, _, err := Open(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Open: want %v, got %v.", ErrUnsupported, err)
	}

	if _, err := Start(exec.Command("sh")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Start: want %v, got %v.", ErrUnsupported, err)
	}
	if _, err := StartWithSize(exec.Command("sh"), nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("StartWithSize: want %v, got %v.", ErrUnsupported, err)
	}
	if _, err := StartWithAttr(exec.Command("sh"), nil, nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("StartWithAttr: want %v, got %v.", ErrUnsupported, err)
	}

	if err := SetSize(os.Stdin, &Winsize{Rows: 24, Cols: 80}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("SetSize: want %v, got %v.", ErrUnsupported, err)
	}
	if _, err := GetFullSize(os.Stdin); !errors.Is(err, ErrUnsupported) {
		t.Errorf("GetFullSize: want %v, got %v.", ErrUnsupported, err)
	}
	if _, _, err := GetSize(os.Stdin); !errors.Is(err, ErrUnsupported) {
		t.Errorf("GetSize: want %v, got %v.", ErrUnsupported, err)
	}
	if err := InheritSize(os.Stdin, os.Stdout); !errors.Is(err, ErrUnsupported) {
		t.Errorf("InheritSize: want %v, got %v.", ErrUnsupported, err)
	}

	// The terminal settings of an SSH pseudo-terminal request cannot be applied
	// here either.
	if err := ApplyTerminalModes(int(os.Stdin.Fd()), 80, 24, nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("ApplyTerminalModes: want %v, got %v.", ErrUnsupported, err)
	}
}
