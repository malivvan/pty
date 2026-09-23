//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import (
	"os"
	"os/exec"
	"syscall"
)

// StartWithSize is like [Start] but resizes the pseudo-terminal to size before
// starting cmd; a nil size keeps the default size of a new pseudo-terminal.
//
// cmd is put in a new session (setsid) and the slave end of the terminal is
// made its controlling terminal (TIOCSCTTY), which is what interactive
// programs and job control expect.
func StartWithSize(cmd *exec.Cmd, size *Winsize) (*os.File, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true

	return StartWithAttr(cmd, size, cmd.SysProcAttr)
}
