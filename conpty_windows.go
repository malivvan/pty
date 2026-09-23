//go:build windows

package pty

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ConPTY is a Windows pseudo-console.
//
// A pseudo-console is a console host without a window: a process attached to it
// sees the usual console features work (cursor movement, colours, VT escape
// sequences) while the data it writes can be read from and written to through a
// pair of pipes. A ConPTY is an [io.Reader] for the terminal output and an
// [io.Writer] for the terminal input, which is what Read and Write implement.
// See
// https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session
//
// Read, Write and Close may be called concurrently. Resize and Size are safe to
// call from another goroutine as well.
type ConPTY struct {
	pseudoConsole *windows.Handle

	// The slave ends of the pipes, handed to the pseudo-console on creation
	// and owned by it from then on.
	inPipeRead   windows.Handle
	outPipeWrite windows.Handle

	// The master ends of the pipes, owned by this ConPTY and used by Read and
	// Write.
	inPipeWrite windows.Handle
	outPipeRead windows.Handle

	attrList *windows.ProcThreadAttributeListContainer

	sizeMu sync.Mutex
	size   windows.Coord

	closeOnce sync.Once
	closeErr  error
}

var (
	_ io.Writer = &ConPTY{}
	_ io.Reader = &ConPTY{}
)

// CreatePipes creates the two connected pipes a ConPTY is driven through.
//
// Data written to the write end of the input pipe shows up on the standard
// input of the processes hosted by the pseudo-console; data those processes
// write to their standard output shows up on the read end of the output pipe.
// The handles can be inherited by child processes, which is why they can be
// passed to [NewConPTYWithPipes] as-is.
func CreatePipes() (inPipeRead, inPipeWrite, outPipeRead, outPipeWrite uintptr, err error) {
	pSec := &windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}

	var inPipeReadHandle, inPipeWriteHandle windows.Handle
	if err := windows.CreatePipe(&inPipeReadHandle, &inPipeWriteHandle, pSec, 0); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("pty: creating the input pipe of the pseudo-console: %w", err)
	}

	var outPipeReadHandle, outPipeWriteHandle windows.Handle
	if err := windows.CreatePipe(&outPipeReadHandle, &outPipeWriteHandle, pSec, 0); err != nil {
		// Do not leak the input pipe when the output pipe cannot be created.
		_ = windows.CloseHandle(inPipeReadHandle)
		_ = windows.CloseHandle(inPipeWriteHandle)
		return 0, 0, 0, 0, fmt.Errorf("pty: creating the output pipe of the pseudo-console: %w", err)
	}

	return uintptr(inPipeReadHandle), uintptr(inPipeWriteHandle),
		uintptr(outPipeReadHandle), uintptr(outPipeWriteHandle), nil
}

// NewConPTY creates a pseudo-console of the requested size.
//
// flags is passed to CreatePseudoConsole unchanged; see the Windows
// documentation for the PSEUDOCONSOLE_* values. A width or height that is zero
// or negative is replaced by [DefaultWidth] or [DefaultHeight].
func NewConPTY(width, height, flags int) (*ConPTY, error) {
	inPipeRead, inPipeWrite, outPipeRead, outPipeWrite, err := CreatePipes()
	if err != nil {
		return nil, err
	}

	c, err := NewConPTYWithPipes(inPipeRead, inPipeWrite, outPipeRead, outPipeWrite, width, height, flags)
	if err != nil {
		// NewConPTYWithPipes leaves the handles to its caller when it fails,
		// so the pipes created above are closed here.
		_ = closePipes(inPipeRead, inPipeWrite, outPipeRead, outPipeWrite)
		return nil, err
	}
	return c, nil
}

// NewConPTYWithPipes creates a pseudo-console driven through the given pipe
// handles, which must be the four values returned by [CreatePipes] or four
// equivalent inheritable pipe handles. Passing existing pipes is useful when
// the pseudo-console is created for a process that is going to be started
// elsewhere.
//
// On success the pseudo-console takes ownership of the handles: it closes them
// from [ConPTY.Close]. On failure the caller keeps ownership and has to close
// them itself.
//
// The pseudo-console duplicates the slave ends of the pipes (inPipeRead and
// outPipeWrite) for every process it hosts, so the caller may close its own
// copies right after this function returns.
func NewConPTYWithPipes(inPipeRead, inPipeWrite, outPipeRead, outPipeWrite uintptr, width, height, flags int) (*ConPTY, error) {
	if width <= 0 {
		width = DefaultWidth
	}
	if height <= 0 {
		height = DefaultHeight
	}

	c := &ConPTY{
		pseudoConsole: new(windows.Handle),
		size: windows.Coord{
			X: int16(width),
			Y: int16(height),
		},
		inPipeRead:   windows.Handle(inPipeRead),
		inPipeWrite:  windows.Handle(inPipeWrite),
		outPipeRead:  windows.Handle(outPipeRead),
		outPipeWrite: windows.Handle(outPipeWrite),
	}

	if err := windows.CreatePseudoConsole(c.size, c.inPipeRead, c.outPipeWrite, uint32(flags), c.pseudoConsole); err != nil {
		return nil, fmt.Errorf("pty: creating the pseudo-console: %w", err)
	}

	// The attribute list carries the pseudo-console handle into the startup
	// information of the processes started inside it, so a single entry is
	// enough for the whole lifetime of the ConPTY.
	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(*c.pseudoConsole)
		return nil, fmt.Errorf("pty: allocating the process attribute list: %w", err)
	}

	if err := attrList.Update(
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		handleValuePointer(*c.pseudoConsole),
		unsafe.Sizeof(*c.pseudoConsole),
	); err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(*c.pseudoConsole)
		return nil, fmt.Errorf("pty: attaching the pseudo-console to the process attributes: %w", err)
	}
	c.attrList = attrList

	return c, nil
}

// handleValuePointer returns the pointer the attribute lists of a new process
// expect for a pseudo-console handle.
//
// PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE takes the handle itself in the pointer
// slot of UpdateProcThreadAttribute, not the address of a variable holding it,
// so the handle value is reinterpreted as a pointer here. See the lpValue
// parameter of
// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute
func handleValuePointer(handle windows.Handle) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&handle))
}

// Fd returns the handle of the pseudo-console itself, which is not a file
// descriptor. The handles that carry the terminal data are the ones returned by
// [ConPTY.InPipeWriteFd] and [ConPTY.OutPipeReadFd].
func (c *ConPTY) Fd() uintptr {
	return uintptr(*c.pseudoConsole)
}

// Close shuts the pseudo-console down and releases the pipes it was created
// with, including the master ends that Read and Write use. Closing a ConPTY
// more than once is safe and returns the result of the first call.
func (c *ConPTY) Close() error {
	c.closeOnce.Do(func() {
		// The slave ends belong to the pseudo-console and are closed by it;
		// closing them here first is what makes the hosted processes see
		// end-of-file and the console exit.
		c.closeErr = errors.Join(
			windows.CloseHandle(c.inPipeRead),
			windows.CloseHandle(c.outPipeWrite),
		)

		if c.attrList != nil {
			c.attrList.Delete()
			c.attrList = nil
		}
		windows.ClosePseudoConsole(*c.pseudoConsole)

		// The master ends belong to this ConPTY, not to the console.
		c.closeErr = errors.Join(
			c.closeErr,
			windows.CloseHandle(c.inPipeWrite),
			windows.CloseHandle(c.outPipeRead),
		)
	})
	return c.closeErr
}

// InPipeReadFd returns the handle of the slave end of the input pipe, from
// which the pseudo-console reads the terminal input.
func (c *ConPTY) InPipeReadFd() uintptr {
	return uintptr(c.inPipeRead)
}

// InPipeWriteFd returns the handle of the master end of the input pipe, to
// which terminal input is written.
func (c *ConPTY) InPipeWriteFd() uintptr {
	return uintptr(c.inPipeWrite)
}

// OutPipeReadFd returns the handle of the master end of the output pipe, from
// which the terminal output is read.
func (c *ConPTY) OutPipeReadFd() uintptr {
	return uintptr(c.outPipeRead)
}

// OutPipeWriteFd returns the handle of the slave end of the output pipe, to
// which the pseudo-console writes the terminal output.
func (c *ConPTY) OutPipeWriteFd() uintptr {
	return uintptr(c.outPipeWrite)
}

// Read reads terminal output from the master end of the output pipe; it
// implements [io.Reader]. Keep in mind that a read of a pipe only returns once
// data is available.
func (c *ConPTY) Read(buf []byte) (n int, err error) {
	var read uint32
	err = windows.ReadFile(c.outPipeRead, buf, &read, nil)
	return int(read), err
}

// Write writes terminal input to the master end of the input pipe; it
// implements [io.Writer].
func (c *ConPTY) Write(data []byte) (n int, err error) {
	var written uint32
	err = windows.WriteFile(c.inPipeWrite, data, &written, nil)
	return int(written), err
}

// Resize changes the size of the pseudo-console window. The programs running
// inside it are notified through the usual console size event; whether they act
// on it is up to them.
func (c *ConPTY) Resize(width, height int) error {
	size := windows.Coord{X: int16(width), Y: int16(height)}
	if err := windows.ResizePseudoConsole(*c.pseudoConsole, size); err != nil {
		return fmt.Errorf("pty: resizing the pseudo-console: %w", err)
	}

	c.sizeMu.Lock()
	c.size = size
	c.sizeMu.Unlock()

	return nil
}

// Size returns the size the pseudo-console was created with or last resized to.
//
// The error result is always nil; the method returns one so that its signature
// matches the terminal size helpers of this package.
func (c *ConPTY) Size() (width, height int, err error) {
	c.sizeMu.Lock()
	defer c.sizeMu.Unlock()

	return int(c.size.X), int(c.size.Y), nil
}

// Spawn starts a process attached to the pseudo-console and returns its process
// id together with its handle. The caller owns that handle and has to close it
// with windows.CloseHandle once the process is done.
//
// name is the program to run and args its arguments, in the form [os/exec]
// expects: the command line is built from both, unless attr.Sys.CmdLine
// overrides it. attr may be nil, which means: the current directory, the
// environment of the current process, and no special creation flags.
func (c *ConPTY) Spawn(name string, args []string, attr *syscall.ProcAttr) (pid int, handle uintptr, err error) {
	if attr == nil {
		attr = &syscall.ProcAttr{}
	}

	// A closed pseudo-console cannot host any more processes, and its process
	// attribute list is gone with it.
	if c.attrList == nil {
		return 0, 0, errors.New("pty: the pseudo-console is closed")
	}

	appName, err := lookExtensions(name, attr.Dir)
	if err != nil {
		return 0, 0, err
	}
	if len(attr.Dir) != 0 {
		// CreateProcess looks up appName relative to the current directory and
		// only switches to attr.Dir after the process has started. Building an
		// absolute path here makes up for that difference.
		appName, err = joinExeDirAndFName(attr.Dir, appName)
		if err != nil {
			return 0, 0, err
		}
	}
	appNamePtr, err := windows.UTF16PtrFromString(appName)
	if err != nil {
		return 0, 0, err
	}

	cmdLine := windows.ComposeCommandLine(args)
	if attr.Sys != nil && attr.Sys.CmdLine != "" {
		cmdLine = attr.Sys.CmdLine
	}
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return 0, 0, err
	}

	var dirPtr *uint16
	if len(attr.Dir) != 0 {
		dirPtr, err = windows.UTF16PtrFromString(attr.Dir)
		if err != nil {
			return 0, 0, err
		}
	}

	// attr.Env is only read, never modified, so the caller stays in control of
	// the value it passed in.
	env := attr.Env
	if env == nil {
		env, err = execEnvDefault(attr.Sys)
		if err != nil {
			return 0, 0, err
		}
	}
	envBlock := createEnvBlock(addCriticalEnv(dedupEnvCase(true, env)))

	procSec := defaultSecurityAttributes()
	threadSec := defaultSecurityAttributes()
	if attr.Sys != nil {
		if attr.Sys.ProcessAttributes != nil {
			procSec = &windows.SecurityAttributes{
				Length:        attr.Sys.ProcessAttributes.Length,
				InheritHandle: attr.Sys.ProcessAttributes.InheritHandle,
			}
		}
		if attr.Sys.ThreadAttributes != nil {
			threadSec = &windows.SecurityAttributes{
				Length:        attr.Sys.ThreadAttributes.Length,
				InheritHandle: attr.Sys.ThreadAttributes.InheritHandle,
			}
		}
	}

	startupInfo := &windows.StartupInfoEx{}
	startupInfo.Flags = windows.STARTF_USESTDHANDLES
	startupInfo.ProcThreadAttributeList = c.attrList.List() //nolint:govet // Read by CreateProcess through the syscall.
	startupInfo.Cb = uint32(unsafe.Sizeof(*startupInfo))

	// EXTENDED_STARTUPINFO_PRESENT is required: it is what makes CreateProcess
	// look at the attribute list holding the pseudo-console handle.
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT | windows.EXTENDED_STARTUPINFO_PRESENT)
	if attr.Sys != nil && attr.Sys.CreationFlags != 0 {
		flags |= attr.Sys.CreationFlags
	}

	procInfo := &windows.ProcessInformation{}
	if attr.Sys != nil && attr.Sys.Token != 0 {
		err = windows.CreateProcessAsUser(
			windows.Token(attr.Sys.Token),
			appNamePtr,
			cmdLinePtr,
			procSec,
			threadSec,
			false,
			flags,
			envBlock,
			dirPtr,
			&startupInfo.StartupInfo,
			procInfo,
		)
	} else {
		err = windows.CreateProcess(
			appNamePtr,
			cmdLinePtr,
			procSec,
			threadSec,
			false,
			flags,
			envBlock,
			dirPtr,
			&startupInfo.StartupInfo,
			procInfo,
		)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("pty: starting the process: %w", err)
	}

	// The thread handle is of no use to the caller.
	defer windows.CloseHandle(procInfo.Thread) //nolint:errcheck // Closing a handle cannot be recovered from.

	return int(procInfo.ProcessId), uintptr(procInfo.Process), nil
}

// defaultSecurityAttributes returns the inheritable attributes both the process
// and the thread of a spawned process are created with by default.
func defaultSecurityAttributes() *windows.SecurityAttributes {
	return &windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
}

// closePipes closes the given pipe handles and reports the first failure.
func closePipes(handles ...uintptr) error {
	errs := make([]error, 0, len(handles))
	for _, h := range handles {
		errs = append(errs, windows.CloseHandle(windows.Handle(h)))
	}
	return errors.Join(errs...)
}
