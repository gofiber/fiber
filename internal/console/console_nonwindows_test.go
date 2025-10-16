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
	input := []byte("plain \x1b[31mred\x1b[0m text\x1b[2J")
	if _, err := writer.Write(input); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if got, want := buf.String(), "plain red text"; got != want {
		t.Fatalf("unexpected output: %q != %q", got, want)
	}
}

func TestIsTerminalPipeFalse(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	t.Cleanup(func() {
		r.Close()
		w.Close()
	})
	if IsTerminal(r.Fd()) {
		t.Fatal("pipe must not report as terminal")
	}
}

func TestIsTerminalDevTTY(t *testing.T) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("unable to open /dev/tty: %v", err)
	}
	t.Cleanup(func() {
		tty.Close()
	})
	if !IsTerminal(tty.Fd()) {
		t.Fatal("/dev/tty should report as terminal")
	}
}
