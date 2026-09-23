package pty

import (
	"io"
	"os"
	"sync"
)

// Control characters a terminal in canonical mode reacts to. They are the
// default termios values: ^C interrupts, ^D ends the input, and both ^H and ^?
// erase the last character.
const (
	charInterrupt = 0x03 // ^C
	charEOF       = 0x04 // ^D
	charEraseBack = 0x08 // ^H
	charErase     = 0x7f // ^?
)

// virtualQueueLimit is how much input or output a virtual terminal holds before
// the writer filling that side has to wait for the other side to read, in the
// way a real terminal makes a program wait when its buffers are full. It bounds
// the memory a terminal can take, whatever the programs using it do.
const virtualQueueLimit = 64 << 10

// VirtualPTY is a pseudo-terminal whose two ends live in the same process: it
// needs neither operating system support nor a child process, so it is
// available on every system this package builds on, and it lets a program that
// expects to run on a terminal run inside the process that drives it.
//
// The terminal starts in canonical (cooked) mode and behaves like a real one:
// the input written with Write is echoed back, it only reaches the hosted
// program one complete line at a time, the erase and end-of-file characters are
// processed, and the line feeds the program writes are expanded to carriage
// return plus line feed on the way back. [VirtualPTY.SetRaw] turns all of that
// off.
//
// Like a real terminal, it holds a bounded amount of data: a program that
// writes more than the terminal can hold waits until the terminal is read, and
// so does a terminal that is typed into faster than the program reads.
//
// # Driving a program that runs in the same process
//
// [VirtualPTY.Slave] returns the end of the terminal the hosted program uses.
// Handing it to a program that only needs a reader for its input and writers
// for its output is enough to give that program a terminal, without starting
// another process. A shell interpreter, such as the one in mvdan.cc/sh/v3,
// is the classic example:
//
//	vt := pty.NewVirtualPTY(80, 24)
//	defer vt.Close()
//
//	runner, err := interp.New(interp.StdIO(vt.Slave(), vt.Slave(), vt.Slave()))
//	if err != nil {
//		return err
//	}
//	go runner.Run(ctx, file)
//
//	// Type a command on the terminal and read what the shell answers.
//	vt.Write([]byte("echo hello\n"))
//	io.Copy(os.Stdout, vt)
//
// The same value can be used as the standard streams of a child process with
// [os/exec], but note that such a process does not get a controlling terminal:
// it cannot open /dev/tty, read terminal attributes or receive signals from the
// terminal. Use [Open] or [Start] when that matters.
//
// A VirtualPTY is safe for concurrent use by multiple goroutines.
type VirtualPTY struct {
	mu   sync.Mutex
	cond *sync.Cond

	toProgram []byte // input the hosted program has not read yet
	toMaster  []byte // output the terminal has not read yet
	line      []byte // input typed on the terminal that is not a line yet

	raw     bool
	handler func()

	eofPending    int  // end of files to report to the program, one per ^D
	programClosed bool // the hosted program closed its end of the terminal
	closed        bool // the terminal itself was closed

	width  int
	height int

	slave *virtualSlave
}

// virtualSlave is the end of a [VirtualPTY] that the hosted program reads and
// writes; it is the equivalent of the slave device of a real pseudo-terminal.
type virtualSlave struct {
	vt *VirtualPTY
}

// NewVirtualPTY returns a new in-memory pseudo-terminal of the given size. A
// width or height that is zero or negative is replaced by [DefaultWidth] or
// [DefaultHeight], as [NewConPTY] does.
//
// The terminal must be closed with [VirtualPTY.Close] when it is no longer
// needed.
func NewVirtualPTY(width, height int) *VirtualPTY {
	if width <= 0 {
		width = DefaultWidth
	}
	if height <= 0 {
		height = DefaultHeight
	}

	vt := &VirtualPTY{width: width, height: height}
	vt.cond = sync.NewCond(&vt.mu)
	vt.slave = &virtualSlave{vt: vt}
	return vt
}

// Slave returns the end of the terminal the hosted program uses: what is typed
// with [VirtualPTY.Write] is read from it, line by line, and what is written to
// it is read from the terminal with [VirtualPTY.Read].
//
// Like the slave device of a real terminal it is a single end, so closing it
// ends the input and the output of the program at once. The value belongs to
// the terminal and stays valid until then; it reports [io.EOF] for both reads
// and writes afterwards.
func (v *VirtualPTY) Slave() io.ReadWriteCloser {
	return v.slave
}

// Read reads what the hosted program wrote, with the translation a terminal
// applies to its output: each line feed becomes a carriage return followed by a
// line feed, unless the terminal is in raw mode.
//
// Read blocks until the program writes something, and returns [io.EOF] once the
// program has closed its end of the terminal or the terminal itself is closed.
func (v *VirtualPTY) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	for len(v.toMaster) == 0 && !v.closed && !v.programClosed {
		v.cond.Wait()
	}
	if len(v.toMaster) > 0 {
		n := serve(p, &v.toMaster)
		// Reading makes room for the program to write again.
		v.cond.Broadcast()
		return n, nil
	}
	return 0, io.EOF
}

// Write sends input to the terminal, as a user typing on it would. In canonical
// mode the input is echoed back and only a complete line reaches the hosted
// program; see [VirtualPTY] and [VirtualPTY.SetRaw].
//
// Write waits while the program has not read the input the terminal holds for
// it, and while the echo and the output of the program have not been read. It
// returns [os.ErrClosed] once the terminal is closed, or once the program has
// closed its end of it.
func (v *VirtualPTY) Write(p []byte) (int, error) {
	// The input is handed over piece by piece, so that a write larger than the
	// terminal can hold is held back while the program reads it.
	var handler func()

	for written := 0; written < len(p); {
		v.mu.Lock()

		for !v.closed && !v.programClosed &&
			(len(v.toProgram) >= virtualQueueLimit || len(v.toMaster) >= virtualQueueLimit) {
			v.cond.Wait()
		}
		if v.closed || v.programClosed {
			v.mu.Unlock()
			return written, os.ErrClosed
		}

		// Only as much as fits in the queues the input ends up in.
		chunk := p[written:]
		if room := virtualQueueLimit - len(v.toProgram); len(chunk) > room {
			chunk = chunk[:room]
		}
		if room := virtualQueueLimit - len(v.toMaster); len(chunk) > room {
			chunk = chunk[:room]
		}

		toProgram, echo, interrupted, endOfFiles := v.processInput(chunk)
		v.toProgram = append(v.toProgram, toProgram...)
		v.appendOutput(echo)
		v.eofPending += endOfFiles
		if interrupted {
			handler = v.handler
		}
		v.cond.Broadcast()
		v.mu.Unlock()

		written += len(chunk)
	}

	// The handler runs outside the lock, so that it can use the terminal.
	if handler != nil {
		go handler()
	}
	return len(p), nil
}

// processInput applies the input side of the line discipline to the bytes typed
// on the terminal and reports what the hosted program should read, what the
// terminal echoes back, whether it was interrupted and how many end of files it
// was told about. The caller must hold v.mu.
func (v *VirtualPTY) processInput(p []byte) (toProgram, echo []byte, interrupted bool, endOfFiles int) {
	if v.raw {
		// Raw mode hands every byte to the program as it was typed, without
		// echoing it: the program is in charge, which is what full screen
		// programs expect.
		return p, nil, false, 0
	}

	for _, b := range p {
		switch b {
		case charInterrupt:
			v.line = v.line[:0]
			echo = append(echo, '^', 'C', '\n')
			interrupted = true

		case charErase, charEraseBack:
			if n := len(v.line); n > 0 {
				v.line = v.line[:n-1]
				echo = append(echo, '\b', ' ', '\b')
			}

		case charEOF:
			if len(v.line) == 0 {
				endOfFiles++
				continue
			}
			// ^D after some input hands that input over without a line feed.
			toProgram = append(toProgram, v.line...)
			v.line = v.line[:0]

		default:
			if b == '\r' {
				// Input carriage returns are read as line feeds, as they are
				// on a terminal.
				b = '\n'
			}
			v.line = append(v.line, b)
			echo = append(echo, b)
			if b == '\n' {
				toProgram = append(toProgram, v.line...)
				v.line = v.line[:0]
			}
		}
	}
	return toProgram, echo, interrupted, endOfFiles
}

// appendOutput adds bytes to what the terminal reads, with the translation a
// terminal applies to the output of the program it hosts. The caller must hold
// v.mu.
func (v *VirtualPTY) appendOutput(p []byte) {
	if v.raw {
		v.toMaster = append(v.toMaster, p...)
		return
	}

	for _, b := range p {
		if b == '\n' {
			v.toMaster = append(v.toMaster, '\r')
		}
		v.toMaster = append(v.toMaster, b)
	}
}

// serve copies as much of a queue as fits into p and drops what was copied. The
// caller must hold v.mu.
func serve(p []byte, queue *[]byte) int {
	n := copy(p, *queue)
	*queue = (*queue)[n:]
	if len(*queue) == 0 {
		// Keep the capacity for the next batch instead of dropping the array.
		*queue = (*queue)[:0]
	}
	return n
}

// SetRaw switches the terminal between raw mode and canonical (cooked) mode,
// the mode it starts in. In raw mode input is handed to the hosted program byte
// by byte, without echo, without line buffering and without any translation of
// the input or the output, which is what a full screen program expects.
//
// What a terminal has typed but not handed over yet when it stops assembling
// lines is handed to the program right away.
func (v *VirtualPTY) SetRaw(raw bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if raw && !v.raw {
		v.toProgram = append(v.toProgram, v.line...)
	}
	v.line = nil
	v.raw = raw
	v.cond.Broadcast()
}

// OnInterrupt registers a handler that is called, in its own goroutine, when
// the interrupt character (^C) is typed on the terminal. A nil handler removes
// the current one.
//
// A real terminal delivers a signal to the programs running on it instead;
// since a virtual terminal hosts no processes, the program driving it is told
// about the interruption here. Cancelling the context of a shell interpreter,
// for instance, is what makes it stop the command it is running. Interrupt
// characters typed in the same call to [VirtualPTY.Write] are reported as a
// single interruption.
func (v *VirtualPTY) OnInterrupt(handler func()) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.handler = handler
}

// Resize changes the size of the terminal. The size is stored as given, so that
// a program that asks for a size of zero, which some do to mean "unknown", gets
// exactly that.
func (v *VirtualPTY) Resize(width, height int) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.width, v.height = width, height
	return nil
}

// Size returns the size the terminal was created with or last resized to.
//
// The error result is always nil; the method returns one so that its signature
// matches the other terminals of this package.
func (v *VirtualPTY) Size() (width, height int, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.width, v.height, nil
}

// Close ends the terminal: the hosted program still reads the input the
// terminal had set aside for it and then sees the end of its input, and the
// terminal still reads the output the program had written; after that, reads of
// the terminal report [io.EOF] and writes fail with [os.ErrClosed]. Closing a
// terminal more than once is safe.
func (v *VirtualPTY) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.closed = true
	v.handler = nil
	v.cond.Broadcast()
	return nil
}

// Read reads the input typed on the terminal, one complete line at a time.
//
// It blocks until a line is ready and reports [io.EOF] when the terminal is
// closed. A ^D typed at the start of a line reports an end of file once, after
// which the program can read again, as it can on a real terminal.
func (s *virtualSlave) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	v := s.vt
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
		if v.closed || v.programClosed {
			return 0, io.EOF
		}
		v.cond.Wait()
	}
}

// Write writes the output of the hosted program, which the terminal reads with
// [VirtualPTY.Read]. It waits while the terminal has not read what the program
// already wrote, the way a program writing to a real terminal waits once its
// buffers are full.
func (s *virtualSlave) Write(p []byte) (int, error) {
	v := s.vt

	for written := 0; written < len(p); {
		v.mu.Lock()

		for len(v.toMaster) >= virtualQueueLimit && !v.closed && !v.programClosed {
			v.cond.Wait()
		}
		if v.closed || v.programClosed {
			v.mu.Unlock()
			return written, os.ErrClosed
		}

		chunk := p[written:]
		if room := virtualQueueLimit - len(v.toMaster); len(chunk) > room {
			chunk = chunk[:room]
		}

		v.appendOutput(chunk)
		v.cond.Broadcast()
		v.mu.Unlock()

		written += len(chunk)
	}
	return len(p), nil
}

// Close closes the end of the terminal the hosted program uses, as a program
// that exits closes its standard streams: the terminal reads what is left and
// then reports [io.EOF], and writing to the program fails.
func (s *virtualSlave) Close() error {
	v := s.vt
	v.mu.Lock()
	defer v.mu.Unlock()

	v.programClosed = true
	v.cond.Broadcast()
	return nil
}
