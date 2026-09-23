package pty

import (
	"os"
	"os/exec"
	"syscall"
)

// Start starts cmd with its standard input, output and error connected to a
// freshly allocated pseudo-terminal, and returns the master end of that
// terminal.
//
// It is equivalent to StartWithSize(cmd, nil): the command is started in a new
// session with the slave end of the pseudo-terminal as its controlling
// terminal, so it behaves like a program started from an interactive shell.
//
// The returned file belongs to the caller, which must close it. Systems
// without pseudo-terminal support return [ErrUnsupported].
func Start(cmd *exec.Cmd) (*os.File, error) {
	return StartWithSize(cmd, nil)
}

// StartWithAttr starts cmd on a freshly allocated pseudo-terminal, resizing the
// terminal to size first when size is not nil, and returns the master end of
// that terminal.
//
// attrs is assigned to cmd.SysProcAttr and replaces whatever the caller had put
// there, which is how the platform specific StartWithSize arranges for cmd to
// become the session leader that owns the slave end.
//
// Most callers want [Start] or [StartWithSize] instead; this variant exists for
// the rare cases that need a different process setup, such as a command that
// must not receive a controlling terminal.
func StartWithAttr(cmd *exec.Cmd, size *Winsize, attrs *syscall.SysProcAttr) (*os.File, error) {
	ptmx, tty, err := Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tty.Close() }() // Best effort.

	if size != nil {
		if err := SetSize(ptmx, size); err != nil {
			_ = ptmx.Close() // Best effort.
			return nil, err
		}
	}

	// Only the standard streams the caller left unset are redirected, so that
	// an explicitly configured stream (a pipe, a file) is left alone.
	if cmd.Stdin == nil {
		cmd.Stdin = tty
	}
	if cmd.Stdout == nil {
		cmd.Stdout = tty
	}
	if cmd.Stderr == nil {
		cmd.Stderr = tty
	}

	cmd.SysProcAttr = attrs

	if err := cmd.Start(); err != nil {
		_ = ptmx.Close() // Best effort.
		return nil, err
	}
	return ptmx, nil
}
