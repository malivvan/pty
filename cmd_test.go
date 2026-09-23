package pty

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
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

// scriptedTerminal is a terminal whose commands never really start, so that the
// handling of a command can be tested without a program to run.
type scriptedTerminal struct {
	*VirtualPTY

	// wait is what waiting for a command of this terminal reports. A process
	// that was stopped reports no failure of its own on Windows, where being
	// terminated gives an exit code rather than a signal.
	wait func() (*os.ProcessState, error)
}

func (t scriptedTerminal) startCommand(c *Cmd) error {
	c.wait = t.wait
	return nil
}

func (t scriptedTerminal) Command(name string, args ...string) *Cmd {
	return newCmd(t, name, args...)
}

func (t scriptedTerminal) CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	cmd := newCmd(t, name, args...)
	cmd.ctx = ctx
	return cmd
}

// TestCmdContextCancelWithoutProcessError checks that a command that was
// stopped reports the cancellation even when the process it stopped reports no
// failure of its own, which is what a terminated process does on Windows. This
// is what makes errors.Is(err, context.Canceled) hold on every system.
func TestCmdContextCancelWithoutProcessError(t *testing.T) {
	t.Parallel()

	vt := NewVirtualPTY(80, 24)
	term := scriptedTerminal{
		VirtualPTY: vt,
		wait:       func() (*os.ProcessState, error) { return nil, nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := term.CommandContext(ctx, "nothing")
	stopped := make(chan struct{})
	cmd.Cancel = func() error {
		close(stopped)
		return nil
	}
	noError(t, cmd.Start(), "Unexpected error from Start")

	cancel()
	select {
	case <-stopped:
	case <-time.After(testTimeout):
		t.Fatal("The command was not stopped when its context was cancelled.")
	}

	if err := cmd.Wait(); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait: want %v, got %v.", context.Canceled, err)
	}
}

// TestCmdContextCancelAfterTheProcessFinished checks that a command that had
// already finished is not blamed on its context.
func TestCmdContextCancelAfterTheProcessFinished(t *testing.T) {
	t.Parallel()

	vt := NewVirtualPTY(80, 24)
	term := scriptedTerminal{
		VirtualPTY: vt,
		wait:       func() (*os.ProcessState, error) { return nil, nil },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := term.CommandContext(ctx, "nothing")
	// Stopping a process that is already gone reports that it is gone.
	delivered := make(chan struct{})
	cmd.Cancel = func() error {
		close(delivered)
		return os.ErrProcessDone
	}
	noError(t, cmd.Start(), "Unexpected error from Start")

	cancel()
	select {
	case <-delivered:
	case <-time.After(testTimeout):
		t.Fatal("The command was not stopped when its context was cancelled.")
	}

	if err := cmd.Wait(); err != nil {
		t.Errorf("Wait: want no error, got %v.", err)
	}
}
