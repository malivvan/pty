//go:build windows

package pty

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestConPTYSpawn checks the Windows pseudo-console end to end: a command is
// attached to it and what the command prints arrives on the terminal.
func TestConPTYSpawn(t *testing.T) {
	t.Parallel()

	console, err := NewConPTY(80, 24, 0)
	noError(t, err, "Unexpected error from NewConPTY")
	t.Cleanup(func() { _ = console.Close() }) // Best effort.

	width, height, err := console.Size()
	noError(t, err, "Unexpected error from Size")
	assert(t, 80, width, "Unexpected width")
	assert(t, 24, height, "Unexpected height")

	master := newTerminalReader(t, console)

	pid, handle, err := console.Spawn("cmd.exe", []string{"cmd.exe", "/c", "echo hello"}, nil)
	noError(t, err, "Unexpected error from Spawn")
	if pid == 0 {
		t.Error("Spawn returned no process id.")
	}
	if handle == 0 {
		t.Error("Spawn returned no process handle.")
	}
	defer func() { _ = windows.CloseHandle(windows.Handle(handle)) }() // Best effort.

	noError(t, console.Resize(100, 30), "Unexpected error from Resize")
	width, height, err = console.Size()
	noError(t, err, "Unexpected error from Size")
	assert(t, 100, width, "Unexpected width after Resize")
	assert(t, 30, height, "Unexpected height after Resize")

	// The command ran inside the pseudo-console, so its output arrives on the
	// terminal.
	master.expect("hello")

	// Wait for the command to be done, which also proves the pseudo-console
	// made it start at all. A wait that times out is not an error, so the
	// result of the wait is what is checked here.
	event, err := windows.WaitForSingleObject(windows.Handle(handle), 10000)
	if err != nil {
		t.Errorf("Unexpected error waiting for the process: %s.", err)
	}
	if event != windows.WAIT_OBJECT_0 {
		t.Errorf("The process did not finish: wait returned %d.", event)
	}
}

// TestConPTYWithPipes checks that a pseudo-console can be built from pipes the
// caller created.
func TestConPTYWithPipes(t *testing.T) {
	t.Parallel()

	inPipeRead, inPipeWrite, outPipeRead, outPipeWrite, err := CreatePipes()
	noError(t, err, "Unexpected error from CreatePipes")

	console, err := NewConPTYWithPipes(inPipeRead, inPipeWrite, outPipeRead, outPipeWrite, 80, 24, 0)
	noError(t, err, "Unexpected error from NewConPTYWithPipes")
	t.Cleanup(func() { _ = console.Close() }) // Best effort.

	assert(t, inPipeRead, console.InPipeReadFd(), "Unexpected input pipe read end")
	assert(t, inPipeWrite, console.InPipeWriteFd(), "Unexpected input pipe write end")
	assert(t, outPipeRead, console.OutPipeReadFd(), "Unexpected output pipe read end")
	assert(t, outPipeWrite, console.OutPipeWriteFd(), "Unexpected output pipe write end")

	if console.Fd() == 0 {
		t.Error("The pseudo-console has no handle.")
	}
}

// TestConPTYCommand checks that a command can be run inside the pseudo-console
// through the command API, which waits for the process the way any other
// process is waited for.
func TestConPTYCommand(t *testing.T) {
	t.Parallel()

	console, err := NewConPTY(80, 24, 0)
	noError(t, err, "Unexpected error from NewConPTY")
	t.Cleanup(func() { _ = console.Close() }) // Best effort.

	master := newTerminalReader(t, console)

	cmd := console.Command("cmd.exe", "/c", "echo hello from the command")
	noError(t, cmd.Run(), "Unexpected error from Run")
	if cmd.Process == nil || cmd.ProcessState == nil {
		t.Fatal("Run should have started the command and waited for it.")
	}
	if !cmd.ProcessState.Success() {
		t.Errorf("The command should have succeeded, got %s.", cmd.ProcessState)
	}

	master.expect("hello from the command")
}

// TestConPTYCommandStartTwice checks that a command can only be started once.
func TestConPTYCommandStartTwice(t *testing.T) {
	t.Parallel()

	console, err := NewConPTY(80, 24, 0)
	noError(t, err, "Unexpected error from NewConPTY")
	t.Cleanup(func() { _ = console.Close() }) // Best effort.

	// cmd.exe without arguments reads its commands from the console, so it
	// keeps running until it is stopped.
	cmd := console.Command("cmd.exe")
	noError(t, cmd.Start(), "Unexpected error from Start")

	if err := cmd.Start(); err == nil {
		t.Error("Starting a command twice should have failed.")
	}

	noError(t, cmd.Cancel(), "Unexpected error from Cancel")
	_ = cmd.Wait() // The command was killed, so this reports the exit status.
}

// TestConPTYCommandContextCancel checks that the cancellation of the context of
// a command stops the process it started.
func TestConPTYCommandContextCancel(t *testing.T) {
	t.Parallel()

	console, err := NewConPTY(80, 24, 0)
	noError(t, err, "Unexpected error from NewConPTY")
	t.Cleanup(func() { _ = console.Close() }) // Best effort.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := console.CommandContext(ctx, "cmd.exe")
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
