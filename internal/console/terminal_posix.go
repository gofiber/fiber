//go:build !windows

package console

import "golang.org/x/sys/unix"

// IsTerminal reports whether the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// IsCygwinTerminal always returns false on POSIX systems as Cygwin is Windows specific.
func IsCygwinTerminal(fd uintptr) bool {
	_ = fd
	return false
}
