// Package pty provides pseudo-terminal support for Unix-like systems and for
// Windows console pseudo-terminals (ConPTY).
//
// # Unix pseudo-terminals
//
// On Unix-like systems a pseudo-terminal is a pair of devices: a master
// (commonly called the ptmx) and a slave (the pty, or tty). The master end is
// driven by the program that owns the terminal, the slave end is used by the
// program that believes it is talking to a real terminal.
//
// This package covers the whole life cycle of such a pair:
//
//   - [Open] allocates a master/slave pair.
//   - [Start], [StartWithSize] and [StartWithAttr] run an [os/exec.Cmd] on a
//     fresh pair, optionally in a new session with the slave as its controlling
//     terminal.
//   - [GetSize], [GetFullSize] and [SetSize] query and change the terminal
//     size, and [InheritSize] keeps one terminal in sync with another
//     (typically from a SIGWINCH handler).
//   - [ErrUnsupported] is returned by every function that the current system
//     cannot implement.
//
// # Windows pseudo-terminals
//
// Windows has no pseudo-terminal device, but its console host provides ConPTY:
// a pseudo-console that a child process can be attached to and that is driven
// through a pair of pipes. Use [NewConPTY] (or [NewConPTYWithPipes] to supply
// the pipes yourself) to create one, [ConPTY.Spawn] to start a process inside
// it and [ConPTY.Read]/[ConPTY.Write] to exchange data with that process.
//
// # Platform support
//
// Unix pseudo-terminals are implemented on Darwin, DragonFly BSD, FreeBSD,
// Linux, NetBSD, OpenBSD, Solaris/illumos and z/OS. ConPTY is implemented on
// Windows 10 1809 and later.
//
// On every other system (AIX, Plan 9 and the WebAssembly targets among them)
// the package still compiles and the same API stays available, but terminal
// operations fail with [ErrUnsupported].
//
// # Blocking operations
//
// The files returned by this package are ordinary [*os.File] values owned by
// the caller. Making Close interrupt a concurrent Read, or making deadlines
// work, requires non-blocking I/O: call syscall.SetNonblock on the file
// descriptor first. See the examples in the repository README.
package pty
