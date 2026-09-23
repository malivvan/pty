//go:build windows

package pty

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// startCommand starts the command inside the pseudo-console with
// [ConPTY.Spawn], and keeps an [os.Process] so that the command can be waited
// for the way any other process is.
func (c *ConPTY) startCommand(cmd *Cmd) error {
	pid, handle, err := c.Spawn(cmd.Path, cmd.Args, &syscall.ProcAttr{
		Dir: cmd.Dir,
		Env: cmd.Env,
		Sys: cmd.SysProcAttr,
	})
	if err != nil {
		return err
	}

	// The handle of the process is of no use here: the process is waited for
	// through an os.Process of its own, which holds a handle of its own.
	defer func() { _ = windows.CloseHandle(windows.Handle(handle)) }() // Best effort.

	process, err := os.FindProcess(pid)
	if err != nil {
		// Nothing can wait for the process any more, so it is stopped rather
		// than left running with nobody to reap it.
		if killErr := windows.TerminateProcess(windows.Handle(handle), 1); killErr != nil {
			return errors.Join(err, fmt.Errorf("pty: stopping the process that cannot be waited for: %w", killErr))
		}
		return err
	}

	cmd.Process = process
	cmd.wait = process.Wait
	return nil
}
