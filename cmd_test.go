package pty

import (
	"errors"
	"os"
	"testing"
)

// TestCmdWaitBeforeStart checks that waiting for a command that was never
// started is an error rather than a hang.
func TestCmdWaitBeforeStart(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	if err := vt.Command("true").Wait(); err == nil {
		t.Error("Wait of a command that was not started should have failed.")
	}
}

// TestCmdNilContext checks that a command created with a nil context reports
// that mistake instead of starting a process that nothing can stop.
func TestCmdNilContext(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)

	cmd := vt.CommandContext(nil, "true")
	if err := cmd.Start(); !errors.Is(err, ErrNilContext) {
		t.Errorf("Start: want %v, got %v.", ErrNilContext, err)
	}
	if cmd.Process != nil {
		t.Error("No process should have been started.")
	}

	cmd = vt.CommandContext(nil, "true")
	if err := cmd.Run(); !errors.Is(err, ErrNilContext) {
		t.Errorf("Run: want %v, got %v.", ErrNilContext, err)
	}
}

// TestCmdOnClosedTerminal checks that a command cannot be started on a terminal
// that is closed.
func TestCmdOnClosedTerminal(t *testing.T) {
	t.Parallel()

	vt := NewVirtualPTY(80, 24)
	noError(t, vt.Close(), "Unexpected error from Close")

	if err := vt.Command("true").Start(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("Start: want %v, got %v.", os.ErrClosed, err)
	}
}
