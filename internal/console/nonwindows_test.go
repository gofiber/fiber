//go:build !windows

package console

import (
	"bytes"
	"os"
	"testing"
)

func TestNonColorableStripsANSI(t *testing.T) {
	var buf bytes.Buffer
	writer := NonColorable(&buf)
	if writer == nil {
		t.Fatalf("expected writer")
	}
	if _, err := writer.Write([]byte("start\x1b[31mred\x1b[0mend")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got := buf.String(); got != "startredend" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestIsTerminalDetectsPipes(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	if IsTerminal(r.Fd()) {
		t.Fatal("expected read end of pipe to not be a terminal")
	}
	if IsTerminal(w.Fd()) {
		t.Fatal("expected write end of pipe to not be a terminal")
	}
	if IsCygwinTerminal(r.Fd()) || IsCygwinTerminal(w.Fd()) {
		t.Fatal("cygwin detection should be false on non-Windows")
	}
}

func TestIsTerminalTTY(t *testing.T) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("unable to open /dev/tty: %v", err)
	}
	defer func() { _ = tty.Close() }()

	if !IsTerminal(tty.Fd()) {
		t.Fatal("expected /dev/tty to be detected as terminal")
	}
}
