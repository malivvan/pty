//go:build !windows

package pty

import (
	"errors"
	"testing"
)

// TestConPTYUnsupported checks that the stand-in for the Windows pseudo-console
// reports that it is not supported everywhere it is used.
func TestConPTYUnsupported(t *testing.T) {
	t.Parallel()

	if _, err := NewConPTY(80, 24, 0); !errors.Is(err, ErrUnsupported) {
		t.Errorf("NewConPTY: want %v, got %v.", ErrUnsupported, err)
	}
	if _, err := NewConPTYWithPipes(0, 0, 0, 0, 80, 24, 0); !errors.Is(err, ErrUnsupported) {
		t.Errorf("NewConPTYWithPipes: want %v, got %v.", ErrUnsupported, err)
	}
	if _, _, _, _, err := CreatePipes(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("CreatePipes: want %v, got %v.", ErrUnsupported, err)
	}

	console := &ConPTY{}
	if err := console.Close(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Close: want %v, got %v.", ErrUnsupported, err)
	}
	if err := console.Resize(80, 24); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Resize: want %v, got %v.", ErrUnsupported, err)
	}
	if _, _, err := console.Size(); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Size: want %v, got %v.", ErrUnsupported, err)
	}
	if _, err := console.Read(make([]byte, 1)); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Read: want %v, got %v.", ErrUnsupported, err)
	}
	if _, err := console.Write([]byte("input")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Write: want %v, got %v.", ErrUnsupported, err)
	}
	if _, _, err := console.Spawn("cmd.exe", nil, nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Spawn: want %v, got %v.", ErrUnsupported, err)
	}

	if console.Fd() != 0 {
		t.Errorf("Fd: want 0, got %d.", console.Fd())
	}
	if console.InPipeReadFd() != 0 || console.InPipeWriteFd() != 0 {
		t.Error("The input pipe of a pseudo-console has no handle here.")
	}
	if console.OutPipeReadFd() != 0 || console.OutPipeWriteFd() != 0 {
		t.Error("The output pipe of a pseudo-console has no handle here.")
	}
}
