//go:build !(windows || linux || aix || zos || darwin || freebsd || openbsd || netbsd || dragonfly || hurd || solaris) || appengine || tinygo

package console

// IsTerminal reports whether the file descriptor refers to a terminal.
func IsTerminal(fd uintptr) bool {
	return false
}

// IsCygwinTerminal reports whether the descriptor belongs to a Cygwin terminal.
func IsCygwinTerminal(fd uintptr) bool {
	return false
}
