# pty

A Go package for pseudo-terminals, on every platform:

- **Real pseudo-terminals on Unix**, from the `/dev/ptmx` cloning device, with
  helpers to start a command on one and to query and resize terminals.
- **Real pseudo-consoles on Windows**, through ConPTY, the console host Windows
  offers instead of pseudo-terminal devices.
- **Virtual pseudo-terminals everywhere**: an in-memory terminal pair with a
  line discipline that needs neither operating system support nor a child
  process, so that a program expecting to talk to a terminal can run inside the
  process that drives it.
- **Commands on a terminal**, in the spirit of `os/exec`, for the terminals that
  host a process: a Windows pseudo-console and a virtual terminal.
- **SSH terminal modes**: the settings an SSH client sends in its
  pseudo-terminal request, applied to a terminal.

## Install

```sh
go get github.com/malivvan/pty
```

## Platform support

| System                                  | Real pty | ConPTY | Virtual pty | SSH modes |
|-----------------------------------------|----------|--------|-------------|-----------|
| Linux, Android                          | yes      | no     | yes         | yes       |
| macOS, iOS                              | yes      | no     | yes         | yes       |
| FreeBSD, NetBSD, OpenBSD, DragonFly BSD | yes      | no     | yes         | yes       |
| Solaris, illumos                        | yes      | no     | yes         | yes       |
| z/OS                                    | yes      | no     | yes         | no        |
| Windows                                 | no       | yes    | yes         | no        |
| AIX, Plan 9, js/wasm, wasip1/wasm       | no       | no     | yes         | no        |

Everything the package cannot do on a given system fails with
`pty.ErrUnsupported`, so portable programs keep compiling and can degrade
gracefully.

## Running a command on a real pseudo-terminal

`pty.Start` allocates a pair, connects the standard streams of the command to
the slave end, starts it in a new session with that slave as its controlling
terminal, and hands back the master end:

```go
package main

import (
	"io"
	"log"
	"os/exec"

	"github.com/malivvan/pty"
)

func main() {
	cmd := exec.Command("grep", "--color=always", "bar")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = ptmx.Close() }() // Best effort.

	go func() {
		_, _ = ptmx.Write([]byte("foo\n"))
		_, _ = ptmx.Write([]byte("bar\n"))
		_, _ = ptmx.Write([]byte("baz\n"))
		_, _ = ptmx.Write([]byte{4}) // EOT
	}()

	_, _ = io.Copy(os.Stdout, ptmx)
}
```

`pty.StartWithSize` resizes the terminal before the command starts, and
`pty.StartWithAttr` exists for the cases that need the process attributes to be
set up differently. A command created with `exec.CommandContext` is stopped when
its context is done, exactly as it would be without a pseudo-terminal.

### Driving an interactive program

To drive a program as if it were running in the terminal of the user, put that
terminal in raw mode with `golang.org/x/term` and forward its size to the pty
whenever it changes:

```go
ptmx, err := pty.Start(exec.Command("bash"))
if err != nil {
	return err
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
winch <- syscall.SIGWINCH // Initial resize.

go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
_, _ = io.Copy(os.Stdout, ptmx)
```

`pty.InheritSize` reads the size of one terminal and applies it to another;
`pty.GetSize` and `pty.GetFullSize` report it, and `pty.SetSize` changes it.

## Real pseudo-consoles on Windows

Windows has no pseudo-terminal devices, so the package offers ConPTY: a console
host that a process is attached to and that is driven over a pair of pipes.

```go
console, err := pty.NewConPTY(pty.DefaultWidth, pty.DefaultHeight, 0)
if err != nil {
	return err
}
defer func() { _ = console.Close() }() // Best effort.

pid, handle, err := console.Spawn("cmd.exe", []string{"cmd.exe", "/c", "echo hello"}, nil)
if err != nil {
	return err
}
// The process handle belongs to the caller.
defer windows.CloseHandle(windows.Handle(handle))

log.Printf("started process %d", pid)

// A ConPTY is an io.Reader for the output of the command and an io.Writer for
// its input.
_, err = io.Copy(os.Stdout, console)
return err
```

`pty.NewConPTYWithPipes` accepts pipes the caller created with
`pty.CreatePipes`, or in any other way, which is useful when the process is
started somewhere else. `ConPTY.Resize` and `ConPTY.Size` change and report the
size of the pseudo-console.

## Running a command on a terminal

A terminal that can host a process hands out commands, in the spirit of
`os/exec`. This is what makes running a command on a pseudo-terminal portable:
on Windows a process cannot be attached to a console with `os/exec`, and a
virtual terminal has no device to attach it to at all.

```go
// A command inside a Windows pseudo-console.
console, err := pty.NewConPTY(pty.DefaultWidth, pty.DefaultHeight, 0)
if err != nil {
	return err
}
defer func() { _ = console.Close() }() // Best effort.

cmd := console.Command("cmd.exe", "/c", "echo hello")
if err := cmd.Run(); err != nil { // Start and Wait.
	return err
}
log.Printf("the command exited with %v", cmd.ProcessState)
```

The fields of `Cmd` are the ones of `os/exec.Cmd` that every terminal can
honour: `Path`, `Args`, `Env`, `Dir` and `SysProcAttr`, plus `Process` and
`ProcessState` once the command has run. `CommandContext` adds a context: the
process is killed when the context is done, unless `Cmd.Cancel` says otherwise,
and `Wait` reports the cancellation.

## Virtual pseudo-terminals

A virtual terminal is an in-memory pair with a line discipline: input is echoed,
only complete lines reach the program behind it, the erase and end-of-file
characters are processed, and the line feeds the program writes are expanded to
carriage return plus line feed. `VirtualPTY.SetRaw` turns all of that off.

Like a real terminal it holds a bounded amount of input and output, and the
writer waits when the other side has not read what it holds, so neither side of
it can grow memory without bound.

Because it is not a device, a virtual terminal hosts commands in the process
itself, through their standard streams:

```go
vt := pty.NewVirtualPTY(80, 24)
defer vt.Close()

cmd := vt.Command("bash")
if err := cmd.Start(); err != nil {
	return err
}

// Type a command on the terminal and read what the shell answers.
if _, err := vt.Write([]byte("echo hello\n")); err != nil {
	return err
}
_, err = io.Copy(os.Stdout, vt)
return err
```

### Driving a program that runs in the same process

`VirtualPTY.Slave` returns the end of the terminal the hosted program uses.
Handing it to a program that only needs a reader for its input and writers for
its output is enough to give that program a terminal, without starting another
process. A shell interpreter, such as the one in mvdan.cc/sh/v3, is the classic
example:

```go
vt := pty.NewVirtualPTY(80, 24)
defer vt.Close()

runner, err := interp.New(interp.StdIO(vt.Slave(), vt.Slave(), vt.Slave()))
if err != nil {
	return err
}
go func() { _, _ = runner.Run(ctx, script) }()

// Type a command on the terminal and read what the shell answers.
if _, err := vt.Write([]byte("echo hello\n")); err != nil {
	return err
}
_, err = io.Copy(os.Stdout, vt)
return err
```

A real terminal delivers signals to the programs running on it; a virtual one
has no processes to signal, so the interrupt character is reported to the
program driving it instead:

```go
ctx, cancel := context.WithCancel(context.Background())
vt.OnInterrupt(cancel) // ^C cancels whatever the hosted program is doing.
```

## SSH pseudo-terminals

An SSH server receives the settings of the terminal of its client in the
`pty-req` request of a session, and has to apply them to the pseudo-terminal the
session runs on. `ApplyTerminalModes` does that, and resizes the terminal at the
same time:

```go
var req struct {
	Term   string
	Cols   uint32
	Rows   uint32
	Width  uint32
	Height uint32
	Modes  ssh.TerminalModes
}
if err := ssh.Unmarshal(payload, &req); err != nil {
	return err
}

ptmx, err := pty.Start(exec.Command("/bin/sh"))
if err != nil {
	return err
}
defer func() { _ = ptmx.Close() }() // Best effort.

// ssh.TerminalModes is a map[uint8]uint32, which is exactly what
// ApplyTerminalModes takes, so the modes of the request are passed as they are.
if err := pty.ApplyTerminalModes(int(ptmx.Fd()), int(req.Cols), int(req.Rows), req.Modes); err != nil {
	return err
}
```

The keys of the modes are the opcodes of
[RFC 4254 section 8](https://www.rfc-editor.org/rfc/rfc4254#section-8), which
this package also exports as the `Mode*` constants, so an SSH server that does
not use an SSH library can build them itself. A mode the system does not have is
ignored: a client asks for its terminal settings, it does not demand them.

The modes are applied on the systems where the package can drive terminal
settings, which are the ones where it can allocate a pseudo-terminal as well:
Linux, macOS, the BSDs and Solaris/illumos. Elsewhere the function reports
`ErrUnsupported`.

## The terminal interface

`ConPTY` and `VirtualPTY` are both terminals driven through a single value, and
both implement `pty.Terminal`:

```go
type Terminal interface {
	io.ReadWriteCloser
	Resize(width, height int) error
	Size() (width, height int, err error)
	Command(name string, args ...string) *Cmd
	CommandContext(ctx context.Context, name string, args ...string) *Cmd
}
```

A terminal obtained from `Open`, `Start` or `StartWithSize` is the classic pair
of `*os.File` values instead, driven with `Read`, `Write`, `GetSize` and
`SetSize`.

## Non-blocking I/O

The files returned by this package are ordinary `*os.File` values owned by the
caller. Making `Close` interrupt a concurrent `Read`, or making deadlines work,
requires non-blocking I/O first:

```go
if err := syscall.SetNonblock(int(ptmx.Fd()), true); err != nil {
	return err
}
```

Reading a pseudo-terminal whose other end is gone reports end of file on some
systems and an input/output error on others, so portable code should treat both
as the end of the terminal.

## Development

```sh
make            # build, vet and test on the host system
make test-race  # run the tests with the race detector
make test-wasm  # run the tests on js/wasm and wasip1/wasm
make cross      # compile for every supported GOOS/GOARCH pair
make cross-vet  # type-check the package and its tests for all of them
make check      # everything the CI workflow runs on the host system
make help       # list all targets
```

The tests are laid out by what they cover: `open_test.go`, `winsize_test.go`,
`io_test.go` and `start_test.go` cover the real terminals of the host system,
`conpty_windows_test.go` covers the pseudo-console on Windows, which the CI runs
there, `ssh_test.go` covers the SSH terminal modes,
`conpty_unsupported_test.go` and `unsupported_test.go` cover the systems without
support, and `virtual_test.go`, `virtual_program_test.go` and
`virtual_process_test.go` cover the virtual terminal and the commands it hosts,
which are tested on every system.

The `ztypes_*.go` files, which describe the C types and constants of the BSD
pseudo-terminal interfaces, are generated by `make generate` from the
`types_*.go` files next to them. They are the only generated files in the
repository, and they are generated on the system they describe, since the
generator reads the C headers of the machine it runs on:

```sh
GOOS=freebsd GOARCH=arm64 make generate
```

## License

MIT, see [LICENSE](LICENSE).
