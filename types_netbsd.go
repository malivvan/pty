//go:build ignore

// This file describes the NetBSD ABI types this package needs in a form that
// cgo can translate. It is never part of a build; run 'make generate' on a NetBSD
// system to regenerate ztypes_netbsd.go from it.

package pty

/*
#include <sys/time.h>
#include <stdlib.h>
#include <sys/tty.h>
*/
import "C"

type ptmget C.struct_ptmget

var (
	ioctl_TIOCPTSNAME = C.TIOCPTSNAME
	ioctl_TIOCGRANTPT = C.TIOCGRANTPT
)
