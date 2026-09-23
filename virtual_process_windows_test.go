//go:build windows

package pty

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestVirtualPTYCommand checks that a command can be started on a virtual
// terminal, and that its output arrives on the terminal.
func TestVirtualPTYCommand(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)

	cmd := vt.Command("cmd.exe", "/c", "echo through the terminal")
	noError(t, cmd.Start(), "Unexpected error from Start")

	// cmd.exe writes a carriage return and a line feed of its own, and the
	// terminal turns the line feed into a carriage return and a line feed, the
	// way a terminal with output processing does. The second carriage return
	// is what a program that writes its own line endings gets.
	master.expect("through the terminal\r\r\n")
	noError(t, cmd.Wait(), "Unexpected error from Wait")
	if cmd.ProcessState == nil {
		t.Error("Wait should have filled in the state of the process.")
	}
}

// TestCmdContextCancel checks that the cancellation of the context of a command
// stops the process it started.
func TestCmdContextCancel(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cmd.exe without arguments reads its commands from the terminal, so it
	// keeps running until it is stopped.
	cmd := vt.CommandContext(ctx, "cmd.exe")
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
	cmd := vt.Command("cmd.exe")
	noError(t, cmd.Start(), "Unexpected error from Start")

	if err := cmd.Start(); err == nil {
		t.Error("Starting a command twice should have failed.")
	}

	noError(t, cmd.Cancel(), "Unexpected error from Cancel")
	_ = cmd.Wait() // The command was killed, so this reports the exit status.
}

// TestCmdWaitTwice checks that a command is only waited for once.
func TestCmdWaitTwice(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	cmd := vt.Command("cmd.exe", "/c", "echo hello")
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
	cmd := vt.Command("cmd.exe", "/c", "echo hello")
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
	cmd := vt.Command("cmd.exe", "/c", "echo hello")

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
