//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

// These examples are not run by the test suite: they would either attach the
// terminal of the test process to a command or never return. They are here to
// show up in the documentation of the package.

package pty_test

import (
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/malivvan/pty"
)

// ExampleStart runs a command on a pseudo-terminal and echoes what it prints,
// the way a terminal emulator would.
func ExampleStart() {
	cmd := exec.Command("grep", "--color=always", "pty")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = ptmx.Close() }() // Best effort.

	go func() {
		_, _ = ptmx.Write([]byte("pty\n"))
		_, _ = ptmx.Write([]byte{4}) // EOT, which ends the input of grep.
	}()

	_, _ = io.Copy(os.Stdout, ptmx)
}

// ExampleInheritSize keeps the size of a pseudo-terminal in step with the
// terminal the program is using, which is what an interactive shell needs.
func ExampleInheritSize() {
	cmd := exec.Command("bash")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = ptmx.Close() }() // Best effort.

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer func() { signal.Stop(winch); close(winch) }() // Best effort.

	go func() {
		for range winch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("resizing pty: %v", err)
			}
		}
	}()
	winch <- syscall.SIGWINCH // Resize once, to match the current terminal.

	_, _ = io.Copy(ptmx, os.Stdin)
}
