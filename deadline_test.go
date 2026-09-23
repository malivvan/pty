//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || zos

package pty

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	errMarker   byte = 0xEE
	testTimeout      = time.Second
)

// fdLock serialises the tests below: they make the file descriptor of the
// master end blocking again through (*os.File).Fd(), which races with the
// non-blocking setup of the other test, so the two must never run at the same
// time.
//
//nolint:gochecknoglobals // Expected global lock to avoid a race on (*os.File).Fd().
var fdLock sync.Mutex

// TestReadDeadline checks that a deadline interrupts an outstanding Read on the
// master end.
//
// https://github.com/malivvan/pty/issues/162
//
//nolint:paralleltest // See fdLock.
func TestReadDeadline(t *testing.T) {
	ptmx, done := prepare(t)

	if err := syscall.SetNonblock(int(ptmx.Fd()), true); err != nil {
		t.Fatalf("Error: set non block: %s", err)
	}

	if err := ptmx.SetDeadline(time.Now().Add(testTimeout / 10)); err != nil {
		if errors.Is(err, os.ErrNoDeadline) {
			t.Skipf("Deadline is not supported on %s/%s.", runtime.GOOS, runtime.GOARCH)
		}
		t.Fatalf("Error: set deadline: %s.", err)
	}

	buf := make([]byte, 1)
	n, err := ptmx.Read(buf)
	done()
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Unexpected read error: %s.", err)
	}
	if n != 0 && buf[0] != errMarker {
		t.Errorf("Received unexpected data from ptmx (%d bytes): 0x%X; err=%v.", n, buf, err)
	}
}

// TestReadClose checks that closing the master end interrupts an outstanding
// Read on it.
//
// https://github.com/malivvan/pty/issues/114
// https://github.com/malivvan/pty/issues/88
//
//nolint:paralleltest // See fdLock.
func TestReadClose(t *testing.T) {
	ptmx, done := prepare(t)

	if err := syscall.SetNonblock(int(ptmx.Fd()), true); err != nil {
		t.Fatalf("Error: set non block: %s", err)
	}

	go func() {
		time.Sleep(testTimeout / 10)
		if err := ptmx.Close(); err != nil {
			t.Errorf("Failed to close ptmx: %s.", err)
		}
	}()

	buf := make([]byte, 1)
	n, err := ptmx.Read(buf)
	done()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Unexpected read error: %s.", err)
	}
	if n != 0 && buf[0] != errMarker {
		t.Errorf("Received unexpected data from ptmx (%d bytes): 0x%X; err=%v.", n, buf, err)
	}
}

// prepare opens a pseudo-terminal pair for the tests above and sets up
// watchdogs for the graceful and the ungraceful way in which they can fail: the
// returned done function reports that the read under test did return.
func prepare(t *testing.T) (ptmx *os.File, done func()) {
	t.Helper()

	if runtime.GOOS == "darwin" {
		t.Log("This package uses blocking i/o on darwin intentionally:")
		t.Log("> https://github.com/malivvan/pty/issues/52")
		t.Log("> https://github.com/malivvan/pty/pull/53")
		t.Log("> https://github.com/golang/go/issues/22099")
		t.SkipNow()
	}

	// (*os.File).Fd() is documented as racy, so these two tests never run in
	// parallel with each other.
	fdLock.Lock()
	t.Cleanup(fdLock.Unlock)

	opened, tty, err := Open()
	if err != nil {
		t.Fatalf("Error: open: %s.\n", err)
	}
	t.Cleanup(func() { _ = opened.Close() }) // Best effort.
	t.Cleanup(func() { _ = tty.Close() })    // Best effort.

	ptmx = getNonBlockingFile(t, opened, "/dev/ptmx")

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	go func() {
		select {
		case <-ctx.Done():
			// The read under test returned, so it did not block forever.
		case <-time.After(testTimeout):
			// Unblock the read so that the test can report the failure.
			if _, err := tty.Write([]byte{errMarker}); err != nil {
				t.Errorf("Failed to write to the slave end: %s.", err)
			}
			t.Error("ptmx.Read() was not unblocked.")
			done() // Cancel the panic below.
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			// The test either failed or succeeded; it did not hang.
		case <-time.After(testTimeout * 10 / 9): // The timeout above plus 11%.
			panic("ptmx.Read() was not unblocked; avoiding a test that hangs forever.") //nolint:forbidigo // Last resort.
		}
	}()

	return ptmx, done
}
