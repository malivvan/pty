//go:build !windows && !js && !wasip1 && !plan9

package pty

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestVirtualPTYCommand checks that a command can be started on a virtual
// terminal: it reads what is typed on the terminal, and its output arrives on
// the terminal.
func TestVirtualPTYCommand(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)

	cmd := vt.Command("cat")
	noError(t, cmd.Start(), "Unexpected error from Start")
	t.Cleanup(func() {
		_ = cmd.Cancel() // Best effort.
		_ = cmd.Wait()   // Best effort.
	})

	if _, err := vt.Write([]byte("through the terminal\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	// The terminal echoes the line back, and the command answers with it.
	master.expect("through the terminal\r\nthrough the terminal\r\n")
}

// TestVirtualPTYCommandRun checks that Run starts a command, waits for it, and
// reports how it went.
func TestVirtualPTYCommandRun(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)

	cmd := vt.Command("true")
	noError(t, cmd.Run(), "Unexpected error from Run")
	if cmd.Process == nil || cmd.ProcessState == nil {
		t.Fatal("Run should have started the command and waited for it.")
	}
	if !cmd.ProcessState.Success() {
		t.Errorf("The command should have succeeded, got %s.", cmd.ProcessState)
	}
}

// TestVirtualPTYCommandWithEnvironment checks that a command started on a
// virtual terminal can be configured like any other command.
func TestVirtualPTYCommandWithEnvironment(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)

	cmd := vt.Command("/bin/sh", "-c", "echo $PTY_TEST_ENV; pwd")
	cmd.Env = []string{"PTY_TEST_ENV=from the environment"}
	cmd.Dir = t.TempDir()
	noError(t, cmd.Run(), "Unexpected error from Run")

	master.expect("from the environment\r\n")
}

// TestCmdContextCancel checks that the cancellation of the context of a command
// stops the process it started.
func TestCmdContextCancel(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := vt.CommandContext(ctx, "cat")
	noError(t, cmd.Start(), "Unexpected error from Start")
	t.Cleanup(func() {
		_ = cmd.Cancel() // Best effort.
		_ = cmd.Wait()   // Best effort.
	})

	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()

	cancel()
	select {
	case err := <-waited:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Wait: want %v, got %v.", context.Canceled, err)
		}
	case <-time.After(testTimeout):
		t.Fatal("The command was not stopped when its context was cancelled.")
	}
}

// TestCmdStartTwice checks that a command can only be started once.
func TestCmdStartTwice(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	cmd := vt.Command("cat")
	noError(t, cmd.Start(), "Unexpected error from Start")

	if err := cmd.Start(); err == nil {
		t.Error("Starting a command twice should have failed.")
	}

	noError(t, cmd.Cancel(), "Unexpected error from Cancel")
	_ = cmd.Wait() // The command was killed, so this reports the signal.
}

// TestCmdWaitTwice checks that a command is only waited for once.
func TestCmdWaitTwice(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	cmd := vt.Command("true")
	noError(t, cmd.Run(), "Unexpected error from Run")

	if err := cmd.Wait(); err == nil {
		t.Error("Waiting for a command twice should have failed.")
	}
}

// TestCmdConcurrentWait checks that only one of several goroutines waiting for
// the same command gets to reap it.
func TestCmdConcurrentWait(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	cmd := vt.Command("true")
	noError(t, cmd.Start(), "Unexpected error from Start")

	waited := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { waited <- cmd.Wait() }()
	}

	failures := 0
	for i := 0; i < 2; i++ {
		if err := <-waited; err != nil {
			failures++
		}
	}
	assert(t, 1, failures, "Exactly one of two concurrent waits should have failed")
}

// TestCmdConcurrentStart checks that only one of several goroutines starting
// the same command gets to start it.
func TestCmdConcurrentStart(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	cmd := vt.Command("true")

	started := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { started <- cmd.Start() }()
	}

	failures := 0
	for i := 0; i < 2; i++ {
		if err := <-started; err != nil {
			failures++
		}
	}
	assert(t, 1, failures, "Exactly one of two concurrent starts should have failed")

	_ = cmd.Wait() // Best effort.
}
