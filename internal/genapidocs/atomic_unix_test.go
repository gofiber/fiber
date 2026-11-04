//go:build linux || darwin

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWriteAtomicWriteFailure uses a child-only resource limit to reproduce a failed disk write.
func TestWriteAtomicWriteFailure(t *testing.T) {
	t.Parallel()
	if path := os.Getenv("GENAPIDOCS_ATOMIC_FAILURE"); path != "" {
		signal.Ignore(syscall.SIGXFSZ)
		var limit syscall.Rlimit
		require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &limit))
		require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 0, Max: limit.Max}))
		// Restore before the child test harness writes its coverage files.
		defer func() { require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit)) }()
		require.ErrorContains(t, writeAtomic(path, []byte("new")), "prepare temp file:")
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "docs.md")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	executable, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(executable, "-test.run=^TestWriteAtomicWriteFailure$")
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			cmd.Args = append(cmd.Args, arg)
		}
	}
	cmd.Env = append(os.Environ(), "GENAPIDOCS_ATOMIC_FAILURE="+path)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "original", string(original))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
