package pty

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestVirtualPTYWithProgram checks that a program that runs in the same process
// and only needs an [io.Reader] and an [io.Writer] can be given a terminal: it
// reads the commands typed on it and answers on it, the way a shell interpreter
// does. No child process and no operating system pseudo-terminal are involved.
func TestVirtualPTYWithProgram(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)

	// The program behind the terminal.
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)

		commands := bufio.NewScanner(vt.Slave())
		for commands.Scan() {
			if _, err := fmt.Fprintf(vt.Slave(), "ran %q\n", commands.Text()); err != nil {
				return
			}
		}
	}()

	if _, err := vt.Write([]byte("echo hello\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	// The terminal echoed the command first, and the program then answered on
	// it.
	got := master.expect("ran \"echo hello\"")
	if !strings.HasPrefix(got, "echo hello\r\n") {
		t.Errorf("The terminal should echo what was typed first, got %q.", got)
	}

	// Closing the terminal ends the program.
	noError(t, vt.Close(), "Unexpected error from Close")
	select {
	case <-stopped:
	case <-time.After(testTimeout):
		t.Fatal("The program did not stop when the terminal was closed.")
	}
}
