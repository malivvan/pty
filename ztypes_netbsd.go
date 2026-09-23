//go:build netbsd

// Created by cgo -godefs - DO NOT EDIT
// cgo -godefs types_netbsd.go

package pty

type ptmget struct {
	Cfd int32
	Sfd int32
	Cn  [1024]int8
	Sn  [1024]int8
}

var (
	ioctl_TIOCPTSNAME = 0x48087448
	ioctl_TIOCGRANTPT = 0x20007447
)
