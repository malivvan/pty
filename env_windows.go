//go:build windows

package pty

import (
	"os"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The helpers below build the environment block a spawned process inherits;
// they mirror the ones the standard library uses for [os/exec].

// maxEnvBlockWords bounds the view taken of an environment block below.
// CreateEnvironmentBlock ends the block with an empty entry, which is where the
// loop stops, well before this limit.
const maxEnvBlockWords = 1 << 20

// execEnvDefault returns the environment a new process started with the given
// attributes should inherit: either the environment of the current process, or
// the one belonging to the user the process is started as.
func execEnvDefault(sys *syscall.SysProcAttr) ([]string, error) {
	if sys == nil || sys.Token == 0 {
		return syscall.Environ(), nil
	}

	var block *uint16
	if err := windows.CreateEnvironmentBlock(&block, windows.Token(sys.Token), false); err != nil {
		return nil, err
	}
	defer windows.DestroyEnvironmentBlock(block) //nolint:errcheck // Nothing can be done about a failure here.

	// The block holds a "name=value" string per entry, each ended by a NUL, and
	// is closed by an empty entry.
	words := unsafe.Slice(block, maxEnvBlockWords)

	var env []string
	for i := 0; ; {
		start := i
		for words[i] != 0 {
			i++
		}
		if i == start {
			return env, nil
		}

		env = append(env, string(utf16.Decode(words[start:i])))
		i++ // Skip the NUL that ends the entry.
	}
}

// createEnvBlock converts a list of environment strings into the UTF-16 block
// CreateProcess expects: the entries, each ended by a NUL, followed by an extra
// NUL.
func createEnvBlock(env []string) *uint16 {
	if len(env) == 0 {
		return &utf16.Encode([]rune("\x00\x00"))[0]
	}

	length := 1
	for _, entry := range env {
		length += len(entry) + 1
	}

	block := make([]byte, length)
	offset := 0
	for _, entry := range env {
		offset += copy(block[offset:], entry)
		block[offset] = 0
		offset++
	}
	block[offset] = 0

	return &utf16.Encode([]rune(string(block)))[0]
}

// dedupEnvCase drops duplicated environment variables, keeping the last
// occurrence of each name. Variable names are compared case-insensitively when
// caseInsensitive is true, which is how Windows treats them.
func dedupEnvCase(caseInsensitive bool, env []string) []string {
	out := make([]string, 0, len(env))
	index := make(map[string]int, len(env)) // name => position in out

	for _, entry := range env {
		equals := strings.Index(entry, "=")
		if equals < 0 {
			out = append(out, entry)
			continue
		}

		name := entry[:equals]
		if caseInsensitive {
			name = strings.ToLower(name)
		}

		if previous, duplicate := index[name]; duplicate {
			out[previous] = entry
			continue
		}
		index[name] = len(out)
		out = append(out, entry)
	}
	return out
}

// addCriticalEnv adds the environment variables a Windows process needs but
// that are often missing from the environment handed to CreateProcess.
// Currently that is SYSTEMROOT only, without which many programs refuse to
// start.
func addCriticalEnv(env []string) []string {
	for _, entry := range env {
		equals := strings.Index(entry, "=")
		if equals < 0 {
			continue
		}
		if strings.EqualFold(entry[:equals], "SYSTEMROOT") {
			return env
		}
	}
	return append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
}
