//go:build windows

package pty

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// The helpers below mirror the ones the standard library uses to prepare a
// CreateProcess call, so that [ConPTY.Spawn] accepts the same [syscall.ProcAttr]
// values as [os/exec] does.

// lookExtensions resolves a program path the way CreateProcess does: relative
// names are looked up on the PATH and the file extension is added, but a
// directory given in attr.Dir is honoured before the process switches to it.
func lookExtensions(path, dir string) (string, error) {
	if filepath.Base(path) == path {
		path = filepath.Join(".", path)
	}

	if dir == "" || filepath.VolumeName(path) != "" || (len(path) > 1 && os.IsPathSeparator(path[0])) {
		return exec.LookPath(path)
	}

	dirAndPath := filepath.Join(dir, path)

	// LookPath can only ever add the file extension to the path it is given.
	resolved, err := exec.LookPath(dirAndPath)
	if err != nil {
		return "", err
	}

	return path + strings.TrimPrefix(resolved, dirAndPath), nil
}

// isSlash reports whether c is a path separator.
func isSlash(c uint8) bool {
	return c == '\\' || c == '/'
}

// normalizeDir returns the absolute form of dir, rejecting the \\server\share
// form, which a working directory cannot have.
func normalizeDir(dir string) (string, error) {
	absDir, err := syscall.FullPath(dir)
	if err != nil {
		return "", err
	}
	if len(absDir) > 2 && isSlash(absDir[0]) && isSlash(absDir[1]) {
		return "", syscall.EINVAL
	}
	return absDir, nil
}

// volToUpper upper-cases a drive letter so that two of them can be compared.
func volToUpper(ch int) int {
	if 'a' <= ch && ch <= 'z' {
		ch += 'A' - 'a'
	}
	return ch
}

// joinExeDirAndFName joins the program path p and the working directory dir into
// an absolute program path, which is what CreateProcess effectively does once it
// has switched to dir.
func joinExeDirAndFName(dir, p string) (string, error) {
	if len(p) == 0 {
		return "", syscall.EINVAL
	}

	if len(p) > 2 && isSlash(p[0]) && isSlash(p[1]) {
		// \\server\share\path is already absolute.
		return p, nil
	}

	if len(p) > 1 && p[1] == ':' {
		// p carries a drive letter.
		if len(p) == 2 {
			return "", syscall.EINVAL
		}
		if isSlash(p[2]) {
			return p, nil
		}

		absDir, err := normalizeDir(dir)
		if err != nil {
			return "", err
		}
		if volToUpper(int(p[0])) == volToUpper(int(absDir[0])) {
			return syscall.FullPath(absDir + "\\" + p[2:])
		}
		return syscall.FullPath(p)
	}

	// p has no drive letter: it is relative to dir.
	absDir, err := normalizeDir(dir)
	if err != nil {
		return "", err
	}
	if isSlash(p[0]) {
		return windows.FullPath(absDir[:2] + p)
	}
	return windows.FullPath(absDir + "\\" + p)
}
