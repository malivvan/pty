package pty

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// terminalReader reads a terminal in a background goroutine and keeps
// everything it read, so that the tests can wait for output with a deadline
// instead of blocking forever, and so that a reader never waits for a test to
// consume what it read.
type terminalReader struct {
	t    *testing.T
	mu   sync.Mutex
	got  strings.Builder
	done bool
	wake chan struct{}
}

// newTerminalReader starts reading r in the background until it fails or is
// closed.
func newTerminalReader(t *testing.T, r io.Reader) *terminalReader {
	t.Helper()

	tr := &terminalReader{t: t, wake: make(chan struct{}, 1)}
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				tr.mu.Lock()
				tr.got.Write(buf[:n])
				tr.mu.Unlock()
				tr.notify()
			}
			if err != nil {
				tr.mu.Lock()
				tr.done = true
				tr.mu.Unlock()
				tr.notify()
				return
			}
		}
	}()
	return tr
}

// notify wakes whoever is waiting for the reader.
func (tr *terminalReader) notify() {
	select {
	case tr.wake <- struct{}{}:
	default:
	}
}

// read returns what was read so far and whether the reader has stopped.
func (tr *terminalReader) read() (string, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	return tr.got.String(), tr.done
}

// expect waits until everything read so far contains want, and returns it.
func (tr *terminalReader) expect(want string) string {
	tr.t.Helper()

	deadline := time.After(testTimeout)
	for {
		got, done := tr.read()
		if strings.Contains(got, want) {
			return got
		}
		if done {
			tr.t.Fatalf("Reading ended after %q, want %q.", got, want)
		}

		select {
		case <-tr.wake:
		case <-deadline:
			tr.t.Fatalf("Timed out waiting for %q, got %q.", want, got)
		}
	}
}

// drain reads until the reader stops, and returns everything it read.
func (tr *terminalReader) drain() string {
	tr.t.Helper()

	deadline := time.After(testTimeout)
	for {
		got, done := tr.read()
		if done {
			return got
		}

		select {
		case <-tr.wake:
		case <-deadline:
			tr.t.Fatalf("Reading did not end within %s, got %q.", testTimeout, got)
		}
	}
}

// expectNothing fails the test when anything is read within d.
func (tr *terminalReader) expectNothing(d time.Duration) {
	tr.t.Helper()

	tr.mu.Lock()
	before := tr.got.Len()
	tr.mu.Unlock()

	deadline := time.After(d)
	for {
		select {
		case <-tr.wake:
			tr.mu.Lock()
			grew, got := tr.got.Len() > before, tr.got.String()
			tr.mu.Unlock()

			if grew {
				tr.t.Fatalf("Read %q, but nothing was expected.", got)
			}
			// A notification about what was read before; keep waiting.
		case <-deadline:
			return
		}
	}
}

// newVirtualPTY returns a virtual terminal of a known size, closed when the
// test ends.
func newVirtualPTY(t *testing.T, width, height int) *VirtualPTY {
	t.Helper()

	vt := NewVirtualPTY(width, height)
	t.Cleanup(func() { _ = vt.Close() })
	return vt
}

// TestVirtualPTYLines checks that a line typed on the terminal reaches the
// hosted program and is echoed back to the one typing it.
func TestVirtualPTYLines(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)
	program := newTerminalReader(t, vt.Slave())

	const line = "echo hello\n"
	if _, err := vt.Write([]byte(line)); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	program.expect(line)
	master.expect("echo hello\r\n")
}

// TestVirtualPTYCanonical checks that a line is only handed to the hosted
// program once it is complete, while it is echoed as it is typed.
func TestVirtualPTYCanonical(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)
	program := newTerminalReader(t, vt.Slave())

	if _, err := vt.Write([]byte("half")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	master.expect("half")
	master.expectNothing(50 * time.Millisecond)
	program.expectNothing(50 * time.Millisecond)

	if _, err := vt.Write([]byte(" line\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expect("half line\n")
	master.expect("half line\r\n")
}

// TestVirtualPTYErase checks the erase character.
func TestVirtualPTYErase(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)
	program := newTerminalReader(t, vt.Slave())

	if _, err := vt.Write([]byte("pingg\x7f\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	program.expect("ping\n")
	master.expect("pingg\b \b\r\n")

	// Erasing an empty line does nothing.
	if _, err := vt.Write([]byte("\x7f\x7f\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expect("ping\n\n")
}

// TestVirtualPTYCarriageReturn checks that a carriage return is read as a line
// feed, as it is on a terminal.
func TestVirtualPTYCarriageReturn(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	program := newTerminalReader(t, vt.Slave())

	if _, err := vt.Write([]byte("done\r")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	program.expect("done\n")
}

// TestVirtualPTYEndOfFile checks that the end-of-file character ends a read of
// the hosted program without ending the terminal.
func TestVirtualPTYEndOfFile(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)
	program := newTerminalReader(t, vt.Slave())

	if _, err := vt.Write([]byte("last\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expect("last\n")
	master.expect("last\r\n")

	// A ^D at the start of a line is an end of file. The terminal does not
	// echo it.
	if _, err := vt.Write([]byte{charEOF}); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	assert(t, "last\n", program.drain(), "Unexpected input of the program")
	master.expectNothing(50 * time.Millisecond)

	// The program can read again afterwards, as it can on a terminal.
	program = newTerminalReader(t, vt.Slave())
	if _, err := vt.Write([]byte("more\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expect("more\n")
}

// TestVirtualPTYEndOfFiles checks that every end-of-file character typed at the
// start of a line reports an end of file of its own.
func TestVirtualPTYEndOfFiles(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)

	if _, err := vt.Write([]byte{charEOF, charEOF}); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	buf := make([]byte, 1)
	for i := 1; i <= 2; i++ {
		n, err := readWithTimeout(t, vt.Slave(), buf)
		if n != 0 || !errors.Is(err, io.EOF) {
			t.Errorf("Read %d of the end of files: want (0, %v), got (%d, %v).", i, io.EOF, n, err)
		}
	}
}

// TestVirtualPTYEndOfFileMidLine checks that a ^D in the middle of a line hands
// the line over without a line feed.
func TestVirtualPTYEndOfFileMidLine(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	program := newTerminalReader(t, vt.Slave())

	if _, err := vt.Write([]byte{'h', 'a', 'l', 'f', charEOF}); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	program.expect("half")
	program.expectNothing(50 * time.Millisecond)
}

// TestVirtualPTYInterrupt checks that the interrupt character reaches the
// handler registered for it and drops the line being typed.
func TestVirtualPTYInterrupt(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)
	program := newTerminalReader(t, vt.Slave())

	interrupted := make(chan struct{}, 1)
	vt.OnInterrupt(func() { interrupted <- struct{}{} })

	if _, err := vt.Write([]byte("dropped\x03kept\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	select {
	case <-interrupted:
	case <-time.After(testTimeout):
		t.Fatal("The interrupt handler was not called.")
	}

	program.expect("kept\n")
	master.expect("^C\r\nkept\r\n")
}

// TestVirtualPTYOutput checks that the output of the hosted program is handed
// to the reader of the terminal with line feeds expanded.
func TestVirtualPTYOutput(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)

	if _, err := vt.Slave().Write([]byte("one\ntwo\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	master.expect("one\r\ntwo\r\n")
}

// TestVirtualPTYOutputWithCarriageReturn checks that a program that writes its
// own line endings gets them expanded, as it would on a terminal: every line
// feed becomes a carriage return and a line feed, whatever came before it.
func TestVirtualPTYOutputWithCarriageReturn(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)

	// What a Windows program writes when it prints a line.
	if _, err := vt.Slave().Write([]byte("windows program\r\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}

	master.expect("windows program\r\r\n")
}

// TestVirtualPTYRaw checks that raw mode hands input and output over without
// any processing.
func TestVirtualPTYRaw(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)
	program := newTerminalReader(t, vt.Slave())

	vt.SetRaw(true)

	// Input arrives byte by byte, is not buffered and is not echoed.
	if _, err := vt.Write([]byte("raw")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expect("raw")
	master.expectNothing(50 * time.Millisecond)

	// Output is not translated either.
	if _, err := vt.Slave().Write([]byte("one\ntwo\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	master.expect("one\ntwo\n")

	// Switching back to canonical mode buffers again.
	vt.SetRaw(false)
	if _, err := vt.Write([]byte("cooked")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expectNothing(50 * time.Millisecond)
	master.expect("cooked")

	if _, err := vt.Write([]byte("\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expect("cooked\n")
}

// TestVirtualPTYRawFlushesBufferedInput checks that input the terminal had not
// handed over yet when it stops assembling lines is handed over right away.
func TestVirtualPTYRawFlushesBufferedInput(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	program := newTerminalReader(t, vt.Slave())

	if _, err := vt.Write([]byte("no newline yet")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	program.expectNothing(50 * time.Millisecond)

	vt.SetRaw(true)
	program.expect("no newline yet")
}

// TestVirtualPTYSize checks the size of the terminal.
func TestVirtualPTYSize(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 0, 0)

	width, height, err := vt.Size()
	noError(t, err, "Unexpected error from Size")
	assert(t, DefaultWidth, width, "Unexpected default width")
	assert(t, DefaultHeight, height, "Unexpected default height")

	noError(t, vt.Resize(120, 40), "Unexpected error from Resize")
	width, height, err = vt.Size()
	noError(t, err, "Unexpected error from Size")
	assert(t, 120, width, "Unexpected width after Resize")
	assert(t, 40, height, "Unexpected height after Resize")
}

// TestVirtualPTYProgramCloses checks that the terminal reports the end of its
// output once the hosted program closes its end, what a program that exits
// does.
func TestVirtualPTYProgramCloses(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	master := newTerminalReader(t, vt)

	if _, err := vt.Slave().Write([]byte("bye\n")); err != nil {
		t.Fatalf("Unexpected error from Write: %s.", err)
	}
	noError(t, vt.Slave().Close(), "Unexpected error from Close")

	// What was written before the end is read first.
	assert(t, "bye\r\n", master.drain(), "Unexpected output of the terminal")

	if _, err := vt.Write([]byte("input\n")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("Write after the program closed: want %v, got %v.", os.ErrClosed, err)
	}
}

// TestVirtualPTYClose checks that closing the terminal ends both of its ends
// and can be repeated.
func TestVirtualPTYClose(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)
	program := newTerminalReader(t, vt.Slave())

	noError(t, vt.Close(), "Unexpected error from Close")
	noError(t, vt.Close(), "Unexpected error from the second Close")

	// The hosted program sees the end of its input.
	assert(t, "", program.drain(), "Unexpected input of the program")

	if _, err := vt.Write([]byte("input\n")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("Write after Close: want %v, got %v.", os.ErrClosed, err)
	}
	if _, err := vt.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Errorf("Read after Close: want %v, got %v.", io.EOF, err)
	}
	if _, err := vt.Slave().Write([]byte("output\n")); !errors.Is(err, os.ErrClosed) {
		t.Errorf("Write on the program end after Close: want %v, got %v.", os.ErrClosed, err)
	}
}

// TestVirtualPTYEmptyReads checks that reading into an empty buffer does
// nothing, as [io.Reader] documents it.
func TestVirtualPTYEmptyReads(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)

	if n, err := vt.Read(nil); n != 0 || err != nil {
		t.Errorf("Read of an empty buffer: want (0, nil), got (%d, %v).", n, err)
	}
	if n, err := vt.Slave().Read(nil); n != 0 || err != nil {
		t.Errorf("Read of an empty buffer on the program end: want (0, nil), got (%d, %v).", n, err)
	}
}

// TestVirtualPTYBackpressure checks that neither side of the terminal fills
// memory without bound: a writer waits for the other side to read, as a writer
// on a real terminal does once its buffers are full.
func TestVirtualPTYBackpressure(t *testing.T) {
	t.Parallel()

	const line = "backpressure\n"
	payload := []byte(strings.Repeat(line, 2*virtualQueueLimit/len(line)))

	// The echo is read, so the input of the program is what holds the write
	// back.
	vt := newVirtualPTY(t, 80, 24)
	go func() { _, _ = io.Copy(io.Discard, vt) }()

	written := make(chan error, 1)
	go func() {
		_, err := vt.Write(payload)
		written <- err
	}()

	select {
	case err := <-written:
		t.Fatalf("Write of more than the terminal holds returned early: %v.", err)
	case <-time.After(100 * time.Millisecond):
		// The write is waiting for the program, which is what this test is
		// about.
	}

	// Reading the input of the program is what lets the write finish.
	go func() { _, _ = io.Copy(io.Discard, vt.Slave()) }()
	select {
	case err := <-written:
		noError(t, err, "Unexpected error from Write")
	case <-time.After(testTimeout):
		t.Fatal("Write did not finish once the program read the input.")
	}

	// The same holds the other way around: what the program writes waits for
	// the terminal to read it.
	console := newVirtualPTY(t, 80, 24)
	read := make(chan error, 1)
	go func() {
		_, err := console.Slave().Write(payload)
		read <- err
	}()

	select {
	case err := <-read:
		t.Fatalf("The program wrote more than the terminal holds: %v.", err)
	case <-time.After(100 * time.Millisecond):
	}

	go func() { _, _ = io.Copy(io.Discard, console) }()
	select {
	case err := <-read:
		noError(t, err, "Unexpected error from the program side")
	case <-time.After(testTimeout):
		t.Fatal("Writing from the program did not finish once the terminal read it.")
	}
}

// TestVirtualPTYConcurrent checks that the terminal can be written and read by
// several goroutines at once. Run with -race.
func TestVirtualPTYConcurrent(t *testing.T) {
	t.Parallel()

	vt := newVirtualPTY(t, 80, 24)

	const (
		writers = 4
		lines   = 20
	)

	var (
		wg    sync.WaitGroup
		drain sync.WaitGroup
	)

	// One goroutine drains the input of the hosted program, another one drains
	// the terminal itself, so that neither side blocks.
	drain.Add(2)
	go func() {
		defer drain.Done()
		_, _ = io.Copy(io.Discard, vt.Slave())
	}()
	go func() {
		defer drain.Done()
		_, _ = io.Copy(io.Discard, vt)
	}()

	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < lines; j++ {
				if _, err := vt.Write([]byte("line\n")); err != nil {
					t.Errorf("Unexpected error from Write: %s.", err)
					return
				}
				if _, err := vt.Slave().Write([]byte("output\n")); err != nil {
					t.Errorf("Unexpected error from the program side: %s.", err)
					return
				}
			}
		}()
	}

	wg.Wait()
	noError(t, vt.Close(), "Unexpected error from Close")
	drain.Wait()
}
