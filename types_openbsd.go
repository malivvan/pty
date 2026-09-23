//go:build ignore

// This file describes the OpenBSD ABI types this package needs in a form that
// cgo can translate. It is never part of a build; run 'make generate' on an OpenBSD
// system to regenerate ztypes_openbsd.go from it.

package pty

/*
#include <sys/time.h>
#include <stdlib.h>
#include <sys/tty.h>
*/
import "C"

type ptmget C.struct_ptmget

var ioctl_PTMGET = C.PTMGET
