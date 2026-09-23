//go:build darwin || dragonfly || freebsd || linux || netbsd

package pty

import "os"

// open allocates a pseudo-terminal pair from the /dev/ptmx cloning device.
//
// The system-specific parts live in the open_<system>.go files next to this one,
// as the openMaster, ptsname, grantpt, unlockpt and openSlave hooks; a system
// that does not need one of those steps provides a no-op for it. The sequence
// is the one the systems document (see pts(4) on Linux, pty(4) on FreeBSD):
//
//  1. open the cloning device to obtain the master end,
//  2. ask the kernel for the name of the matching slave device,
//  3. hand ownership of the slave to the caller and unlock it,
//  4. open the slave device.
func open() (ptmx, tty *os.File, err error) {
	ptmx, err = openMaster()
	if err != nil {
		return nil, nil, err
	}
	// Make sure the master end never leaks when a step below fails.
	defer func() {
		if err != nil {
			_ = ptmx.Close() // Best effort.
		}
	}()

	slaveName, err := ptsname(ptmx)
	if err != nil {
		return nil, nil, err
	}

	if err := grantpt(ptmx); err != nil {
		return nil, nil, err
	}

	if err := unlockpt(ptmx); err != nil {
		return nil, nil, err
	}

	tty, err = openSlave(slaveName)
	if err != nil {
		return nil, nil, err
	}
	return ptmx, tty, nil
}
