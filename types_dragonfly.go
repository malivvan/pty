//go:build ignore

// This file describes the DragonFly BSD ABI types this package needs in a form
// that cgo can translate. It is never part of a build; run 'make generate' on a
// DragonFly BSD system to regenerate ztypes_dragonfly_amd64.go from it.

package pty

/*
#define _KERNEL
#include <sys/conf.h>
#include <sys/param.h>
#include <sys/filio.h>
*/
import "C"

const (
	_C_SPECNAMELEN = C.SPECNAMELEN /* max length of devicename */
)

type fiodgnameArg C.struct_fiodname_args
