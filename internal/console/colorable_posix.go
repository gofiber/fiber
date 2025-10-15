//go:build !windows

package console

import (
	"io"
	"os"
)

// ColorableStdout returns stdout on non-Windows platforms because ANSI escape
// sequences are already supported natively.
func ColorableStdout() io.Writer {
	return os.Stdout
}
