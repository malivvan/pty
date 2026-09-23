//go:build ignore

// This file describes the FreeBSD ABI types this package needs in a form that
// cgo can translate. It is never part of a build; run 'make generate' on a FreeBSD
// system to regenerate ztypes_freebsd_<arch>.go from it.

package pty

/*
#include <sys/param.h>
#include <sys/filio.h>
*/
import "C"

const (
	_C_SPECNAMELEN = C.SPECNAMELEN /* max length of devicename */
)

type fiodgnameArg C.struct_fiodgname_arg
