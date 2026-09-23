//go:build windows

// This example is not run by the test suite: it would start a real console
// process. It is here to show up in the documentation of the package.

package pty_test

import (
	"io"
	"log"
	"os"

	"golang.org/x/sys/windows"

	"github.com/malivvan/pty"
)

// ExampleConPTY runs a command on a Windows pseudo-console and echoes what it
// prints, the way a terminal emulator would.
func ExampleConPTY() {
	console, err := pty.NewConPTY(pty.DefaultWidth, pty.DefaultHeight, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = console.Close() }() // Best effort.

	pid, handle, err := console.Spawn("cmd.exe", []string{"cmd.exe", "/c", "echo hello"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	// The process handle belongs to the caller.
	defer func() { _ = windows.CloseHandle(windows.Handle(handle)) }() // Best effort.

	log.Printf("started process %d", pid)

	_, _ = io.Copy(os.Stdout, console)
}
