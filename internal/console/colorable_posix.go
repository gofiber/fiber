//go:build !windows

package console

import (
	"io"
	"os"
)

// ColorableStdout returns the standard output writer without any modifications on POSIX systems.
func ColorableStdout() io.Writer {
	return os.Stdout
}

// NewColorableStdout is kept for compatibility with the go-colorable API.
func NewColorableStdout() io.Writer {
	return ColorableStdout()
}
