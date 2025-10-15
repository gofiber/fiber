//go:build solaris && !appengine

package console

import "golang.org/x/sys/unix"

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermio(int(fd), unix.TCGETA)
	return err == nil
}

// IsCygwinTerminal always reports false on Solaris.
func IsCygwinTerminal(fd uintptr) bool {
	return false
}
