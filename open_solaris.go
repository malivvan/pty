//go:build solaris

package pty

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// open allocates a pseudo-terminal pair.
//
// This is the Solaris flavour of the /dev/ptmx flow: the slave device is
// unlocked and its owner handed to the caller with STREAMS ioctls, and the
// terminal driver modules are pushed onto it before it is handed back.
func open() (ptmx, tty *os.File, err error) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	ptmx = os.NewFile(uintptr(fd), "/dev/ptmx")

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

	slaveFD, err := syscall.Open(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	tty = os.NewFile(uintptr(slaveFD), slaveName)

	// A Solaris terminal only behaves like a terminal once the modules of the
	// terminal driver are pushed onto it; see pts(7).
	for _, mod := range []string{"ptem", "ldterm", "ttcompat"} {
		if err := streamsPush(tty, mod); err != nil {
			_ = tty.Close() // Best effort.
			return nil, nil, err
		}
	}

	return ptmx, tty, nil
}

// ptsname returns the path of the slave device paired with master.
//
// Solaris offers no ioctl that reports that name, so it is derived from the
// minor device number of the master end.
func ptsname(master *os.File) (string, error) {
	dev, err := ptsdev(master)
	if err != nil {
		return "", err
	}

	name := "/dev/pts/" + strconv.FormatInt(int64(dev), 10)
	if err := syscall.Access(name, 0); err != nil {
		return "", err
	}
	return name, nil
}

// ptsdev returns the minor device number of the slave device paired with
// master, as reported by the ISPTM ioctl.
func ptsdev(master *os.File) (uint64, error) {
	arg := strioctl{icCmd: ISPTM}
	if err := ioctl(master, I_STR, uintptr(unsafe.Pointer(&arg))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return 0, err
	}

	conn, err := master.SyscallConn()
	if err != nil {
		return 0, err
	}

	var (
		dev     uint64
		statErr error
	)
	if err := conn.Control(func(fd uintptr) {
		var st syscall.Stat_t
		if err := syscall.Fstat(int(fd), &st); err != nil {
			statErr = err
			return
		}
		dev = minor(st.Rdev)
	}); err != nil {
		return 0, err
	}
	if statErr != nil {
		return 0, statErr
	}
	return dev, nil
}

// minor returns the minor device number encoded in dev.
func minor(dev uint64) uint64 { return dev & 0377 }

// ptOwner is the argument of the OWNERPT request; it mirrors struct pt_own of
// <sys/ptms.h>.
type ptOwner struct {
	uid int32
	gid int32
}

// grantpt hands ownership of the slave device to the calling user through the
// OWNERPT ioctl; it is the equivalent of grantpt(3).
func grantpt(master *os.File) error {
	if _, err := ptsdev(master); err != nil {
		return err
	}

	owner := ptOwner{
		uid: int32(os.Getuid()),
		// TODO: use the gid of the "tty" group (DEFAULT_TTY_GROUP) when it can
		// be looked up, which needs getgrnam(3) and therefore cgo.
		gid: int32(os.Getgid()),
	}
	arg := strioctl{
		icCmd: OWNERPT,
		icLen: int32(unsafe.Sizeof(strioctl{})),
		icDP:  unsafe.Pointer(&owner),
	}
	if err := ioctl(master, I_STR, uintptr(unsafe.Pointer(&arg))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return fmt.Errorf("pty: granting ownership of the slave device: %w", err)
	}
	return nil
}

// unlockpt unlocks the slave device through the UNLKPT ioctl; it is the
// equivalent of unlockpt(3).
func unlockpt(master *os.File) error {
	arg := strioctl{icCmd: UNLKPT}
	return ioctl(master, I_STR, uintptr(unsafe.Pointer(&arg))) //nolint:gosec // The pointer must be handed to the kernel as-is.
}

// streamsPush pushes the STREAMS module mod onto file unless it is already
// pushed.
func streamsPush(file *os.File, mod string) error {
	buf := []byte(mod)

	// I_FIND fails when mod is already pushed, which is exactly the case in
	// which the module must not be pushed a second time. The ioctl is not
	// entirely reliable on every Solaris version, since it can report an error
	// for a module that is in fact present, so only a successful I_FIND is
	// followed by an I_PUSH here.
	if err := ioctl(file, I_FIND, uintptr(unsafe.Pointer(&buf[0]))); err != nil { //nolint:gosec // The pointer must be handed to the kernel as-is.
		return nil
	}
	return ioctl(file, I_PUSH, uintptr(unsafe.Pointer(&buf[0]))) //nolint:gosec // The pointer must be handed to the kernel as-is.
}
