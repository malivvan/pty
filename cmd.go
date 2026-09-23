package pty

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
)

var (
	// ErrInvalidCommand is returned by [Cmd.Start] when the command does not
	// come from a terminal that can start processes.
	ErrInvalidCommand = errors.New("pty: invalid command")

	// ErrNilContext is returned by [Cmd.Start] when the command was created
	// with [Terminal.CommandContext] and a nil context, rather than starting a
	// process that nothing can stop.
	ErrNilContext = errors.New("pty: nil context")
)

// Cmd is a command that runs on a pseudo-terminal.
//
// Its fields are the ones of [os/exec.Cmd] that every terminal can honour. A
// command always comes from the terminal it runs on, which is what decides how
// a process is attached to it:
//
//	shell := vt.Command("bash")         // a terminal in this process
//	shell := console.Command("cmd.exe") // the Windows console host
//
// The command is started with [Cmd.Start], waited for with [Cmd.Wait], and
// [Cmd.Run] does both. A command is started and waited for once: the fields
// below, [Cmd.Cancel] included, have to be set before [Cmd.Start] is called,
// and only one goroutine may do it.
type Cmd struct {
	term Terminal

	// Path is the path of the program to run.
	Path string

	// Args holds the command line arguments, [Cmd.Path] being Args[0].
	Args []string

	// Env is the environment of the process. A nil Env means the environment
	// of the current process.
	Env []string

	// Dir is the working directory of the process. An empty Dir means the
	// current directory.
	Dir string

	// SysProcAttr holds the operating system specific settings of the process.
	SysProcAttr *syscall.SysProcAttr

	// Process is the process, once it has been started.
	Process *os.Process

	// ProcessState describes the process once it has exited.
	ProcessState *os.ProcessState

	// Cancel stops the process when the context of the command is done. It
	// defaults to killing the process with [os.Process.Kill], and has to be
	// set before the command is started.
	Cancel func() error

	// mu guards the state below, so that a command can only be started once
	// and waited for once, even when several goroutines race for it.
	mu      sync.Mutex
	started bool
	waited  bool

	ctx        context.Context
	ctxErr     error
	stopCancel func() bool
	wait       func() (*os.ProcessState, error)
}

// newCmd returns a command that runs on term.
func newCmd(term Terminal, name string, args ...string) *Cmd {
	return &Cmd{
		term: term,
		Path: name,
		Args: append([]string{name}, args...),
	}
}

// start starts the command on the terminal it came from.
//
// It reports the mistakes of the caller, and otherwise hands over to the
// terminal, which knows how a process is attached to it.
func (c *Cmd) start() error {
	starter, ok := c.term.(commandStarter)
	if !ok {
		return ErrInvalidCommand
	}
	return starter.startCommand(c)
}

// commandStarter is implemented by the terminals that can host a process.
type commandStarter interface {
	startCommand(c *Cmd) error
}

var (
	_ commandStarter = &ConPTY{}
	_ commandStarter = &VirtualPTY{}
)

// Start starts the command on the terminal it came from.
//
// A command that was created with [Terminal.CommandContext] is stopped with
// [Cmd.Cancel] when its context is done, in the same way [os/exec] stops a
// command created with [os/exec.CommandContext].
func (c *Cmd) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case c.ctxErr != nil:
		return c.ctxErr
	case c.started:
		return errors.New("pty: the command has already been started")
	}

	if err := c.start(); err != nil {
		return err
	}
	c.started = true

	if c.Cancel == nil {
		process := c.Process
		c.Cancel = func() error { return process.Kill() }
	}
	if c.ctx != nil {
		// The handler runs in its own goroutine, so the function it calls is
		// the one that was in place when the command started.
		cancel := c.Cancel
		c.stopCancel = context.AfterFunc(c.ctx, func() { _ = cancel() })
	}
	return nil
}

// Wait waits for the command to exit and releases what the terminal held for
// it. It returns an error when the command has not been started, or has already
// been waited for.
func (c *Cmd) Wait() error {
	c.mu.Lock()
	switch {
	case !c.started:
		c.mu.Unlock()
		return errors.New("pty: the command has not been started")
	case c.waited:
		c.mu.Unlock()
		return errors.New("pty: the command has already been waited for")
	}
	// The state is claimed before the wait itself, so that a second caller is
	// turned away rather than waiting for the same process twice.
	c.waited = true
	stopCancel := c.stopCancel
	c.stopCancel = nil
	c.mu.Unlock()

	// Waiting makes the cancellation of the context pointless.
	if stopCancel != nil {
		stopCancel()
	}

	state, err := c.wait()
	c.ProcessState = state
	if err != nil && c.ctx != nil && c.ctx.Err() != nil {
		// A command that was cancelled reports the cancellation, as
		// os/exec.Cmd does.
		return c.ctx.Err()
	}
	return err
}

// Run starts the command and waits for it to exit.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Command returns a command that runs on the terminal; see [Cmd].
func (c *ConPTY) Command(name string, args ...string) *Cmd {
	return newCmd(c, name, args...)
}

// CommandContext returns a command that runs on the terminal and is stopped
// when ctx is done. A nil context is a mistake, which [Cmd.Start] reports as
// [ErrNilContext].
func (c *ConPTY) CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	cmd := newCmd(c, name, args...)
	if ctx == nil {
		cmd.ctxErr = ErrNilContext
		return cmd
	}
	cmd.ctx = ctx
	return cmd
}

// Command returns a command that runs on the terminal; see [Cmd].
//
// The input of the terminal is shared with the other readers of its program
// end, so what is typed reaches the command in turn, as it does when several
// processes read the same terminal. Waiting for the command gives the input
// back: see [Cmd.Wait].
func (v *VirtualPTY) Command(name string, args ...string) *Cmd {
	return newCmd(v, name, args...)
}

// CommandContext returns a command that runs on the terminal and is stopped
// when ctx is done. A nil context is a mistake, which [Cmd.Start] reports as
// [ErrNilContext].
func (v *VirtualPTY) CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	cmd := newCmd(v, name, args...)
	if ctx == nil {
		cmd.ctxErr = ErrNilContext
		return cmd
	}
	cmd.ctx = ctx
	return cmd
}
