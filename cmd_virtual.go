package pty

import (
	"io"
	"os"
	"os/exec"
)

// commandInput is the input of a virtual terminal as one command sees it.
//
// It reads the same lines the program end of the terminal reads, but it can be
// closed on its own. That is what makes waiting for a command possible: the
// standard library copies the input of a command that is not an [*os.File]
// through a goroutine of its own, which only returns once the end of the input
// is reached.
type commandInput struct {
	vt     *VirtualPTY
	closed bool // guarded by vt.mu
}

// Read reads a line typed on the terminal.
func (in *commandInput) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	v := in.vt
	v.mu.Lock()
	defer v.mu.Unlock()

	for {
		if len(v.toProgram) > 0 {
			n := serve(p, &v.toProgram)
			// Reading makes room for the terminal to be typed into again.
			v.cond.Broadcast()
			return n, nil
		}
		if v.eofPending > 0 {
			v.eofPending--
			return 0, io.EOF
		}
		if in.closed || v.closed || v.programClosed {
			return 0, io.EOF
		}
		v.cond.Wait()
	}
}

// Close stops the command from reading the terminal. The terminal itself, and
// the other readers of it, are left alone.
func (in *commandInput) Close() error {
	v := in.vt
	v.mu.Lock()
	defer v.mu.Unlock()

	in.closed = true
	v.cond.Broadcast()
	return nil
}

// startCommand starts the command on a virtual terminal: its standard streams
// are the terminal, so what the command writes is read from it and what is
// typed on it is read by the command.
//
// The command does not get a controlling terminal, as the end of a virtual
// terminal is not a device; see [VirtualPTY].
func (v *VirtualPTY) startCommand(c *Cmd) error {
	v.mu.Lock()
	closed := v.closed
	v.mu.Unlock()
	if closed {
		return os.ErrClosed
	}

	input := &commandInput{vt: v}

	cmd := exec.Command(c.Path, c.Args[1:]...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.SysProcAttr = c.SysProcAttr
	cmd.Stdin = input
	cmd.Stdout = v.Slave()
	cmd.Stderr = v.Slave()

	if err := cmd.Start(); err != nil {
		return err
	}

	c.Process = cmd.Process
	c.wait = func() (*os.ProcessState, error) {
		// Stop reading for the command before waiting for it: the goroutine
		// that copies its input only stops at the end of that input, and
		// waiting for the command waits for that goroutine.
		_ = input.Close()

		err := cmd.Wait()
		return cmd.ProcessState, err
	}
	return nil
}
