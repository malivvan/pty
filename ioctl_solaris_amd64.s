// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gc

#include "textflag.h"

// System calls on Solaris are implemented in runtime/syscall_solaris.go; the
// wrapper below hands them to this package.

TEXT ·sysvicall6(SB),NOSPLIT,$0-88
	JMP	syscall·sysvicall6(SB)
