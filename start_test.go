//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

// commandOnPTY runs a command on a freshly allocated pseudo-terminal and
// returns what it wrote, together with the exit error of the command.
func commandOnPTY(t *testing.T, cmd *exec.Cmd, size *Winsize) (string, error) {
	t.Helper()

	ptmx, err := StartWithSize(cmd, size)
	noError(t, err, "Unexpected error from StartWithSize")
	t.Cleanup(func() { _ = ptmx.Close() }) // Best effort.

	out, err := io.ReadAll(ptmx)
	// Reading a pseudo-terminal whose other end is gone reports end of file on
	// some systems and an input/output error on others.
	if err != nil && !errors.Is(err, syscall.EIO) {
		t.Fatalf("Unexpected error reading from the pty: %s.", err)
	}
	return string(out), cmd.Wait()
}

func TestStart(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("/bin/sh", "-c", "echo hello")
	ptmx, err := Start(cmd)
	noError(t, err, "Unexpected error from Start")
	t.Cleanup(func() { _ = ptmx.Close() }) // Best effort.

	out, err := io.ReadAll(ptmx)
	if err != nil && !errors.Is(err, syscall.EIO) {
		t.Fatalf("Unexpected error reading from the pty: %s.", err)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("Unexpected output of the command: %q.", out)
	}
	noError(t, cmd.Wait(), "Unexpected error from Wait")
}

func TestStartWithSize(t *testing.T) {
	t.Parallel()

	size := &Winsize{Rows: 40, Cols: 100}
	out, err := commandOnPTY(t, exec.Command("/bin/sh", "-c", "echo sized"), size)
	noError(t, err, "Unexpected error from the command")

	if !strings.Contains(out, "sized") {
		t.Errorf("Unexpected output of the command: %q.", out)
	}
}

func TestStartWithAttr(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("/bin/sh", "-c", "echo attrs")
	ptmx, err := StartWithAttr(cmd, nil, &syscall.SysProcAttr{Setsid: true, Setctty: true})
	noError(t, err, "Unexpected error from StartWithAttr")
	t.Cleanup(func() { _ = ptmx.Close() }) // Best effort.

	out, err := io.ReadAll(ptmx)
	if err != nil && !errors.Is(err, syscall.EIO) {
		t.Fatalf("Unexpected error reading from the pty: %s.", err)
	}
	if !strings.Contains(string(out), "attrs") {
		t.Errorf("Unexpected output of the command: %q.", out)
	}
	noError(t, cmd.Wait(), "Unexpected error from Wait")
}

// TestStartWithSizeAppliesTheSize checks that the terminal is resized before
// the command starts.
func TestStartWithSizeAppliesTheSize(t *testing.T) {
	t.Parallel()

	size := &Winsize{Rows: 40, Cols: 100}
	cmd := exec.Command("/bin/sh", "-c", "sleep 1")
	ptmx, err := StartWithSize(cmd, size)
	noError(t, err, "Unexpected error from StartWithSize")
	t.Cleanup(func() { _ = ptmx.Close() }) // Best effort.
	t.Cleanup(func() { _ = cmd.Wait() })   // Best effort.

	got, err := GetFullSize(ptmx)
	noError(t, err, "Unexpected error from GetFullSize")
	assert(t, size.Rows, got.Rows, "Unexpected number of rows")
	assert(t, size.Cols, got.Cols, "Unexpected number of columns")
}

func TestStartFailure(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("/nonexistent/binary/of/the/pty/tests")
	if _, err := Start(cmd); err == nil {
		t.Error("Starting a command that does not exist should have failed.")
	}
}

// TestInheritSize checks that the size of one terminal is applied to another.
func TestInheritSize(t *testing.T) {
	t.Parallel()

	from, _ := openClose(t)
	to, _ := openClose(t)

	size := &Winsize{Rows: 42, Cols: 132}
	noError(t, SetSize(from, size), "Unexpected error from SetSize")
	noError(t, InheritSize(from, to), "Unexpected error from InheritSize")

	got, err := GetFullSize(to)
	noError(t, err, "Unexpected error from GetFullSize")
	assert(t, size.Rows, got.Rows, "Unexpected number of rows")
	assert(t, size.Cols, got.Cols, "Unexpected number of columns")
}

// TestSizeOnNonTerminal checks that the size requests report the failure of the
// system call on a file that is not a terminal.
func TestSizeOnNonTerminal(t *testing.T) {
	t.Parallel()

	file, err := os.Open(os.DevNull)
	noError(t, err, "Unexpected error opening "+os.DevNull)
	defer func() { _ = file.Close() }() // Best effort.

	if err := SetSize(file, &Winsize{Rows: 24, Cols: 80}); err == nil {
		t.Error("SetSize on a file that is not a terminal should have failed.")
	}
	if _, err := GetFullSize(file); err == nil {
		t.Error("GetFullSize on a file that is not a terminal should have failed.")
	}
	if _, _, err := GetSize(file); err == nil {
		t.Error("GetSize on a file that is not a terminal should have failed.")
	}
	if err := InheritSize(file, file); err == nil {
		t.Error("InheritSize of a file that is not a terminal should have failed.")
	}
}
