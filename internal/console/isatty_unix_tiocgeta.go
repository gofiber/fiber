//go:build (darwin || freebsd || openbsd || netbsd || dragonfly || hurd) && !appengine && !tinygo

package console

import "golang.org/x/sys/unix"

// IsTerminal returns true if the file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TIOCGETA)
	return err == nil
}

// IsCygwinTerminal always reports false on POSIX platforms.
func IsCygwinTerminal(fd uintptr) bool {
	return false
}
