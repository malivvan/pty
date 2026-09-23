package pty

import (
	"context"
	"io"
)

// Terminal is the behaviour of a pseudo-terminal that is driven through a
// single object rather than through a pair of files.
//
// It is implemented by [ConPTY], the real Windows pseudo-console, and by
// [VirtualPTY], the in-memory terminal that works everywhere. A pseudo-terminal
// created by [Open] or [Start] is a pair of [*os.File] values instead, driven
// with Read, Write, [SetSize] and [GetSize].
type Terminal interface {
	io.ReadWriteCloser

	// Resize changes the size of the terminal.
	Resize(width, height int) error

	// Size returns the current size of the terminal.
	Size() (width, height int, err error)

	// Command returns a command that runs on the terminal; see [Cmd].
	Command(name string, args ...string) *Cmd

	// CommandContext returns a command that runs on the terminal and is stopped
	// when ctx is done; see [Cmd].
	CommandContext(ctx context.Context, name string, args ...string) *Cmd
}

var (
	_ Terminal = &ConPTY{}
	_ Terminal = &VirtualPTY{}
)
