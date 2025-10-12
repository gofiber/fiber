//go:build windows

package console

import "testing"

func TestEnableColorsStdout(t *testing.T) {
	var enabled bool
	restore := EnableColorsStdout(&enabled)
	if restore == nil {
		t.Fatal("expected restore function")
	}
	if !enabled {
		t.Fatal("expected enable flag to be set")
	}
	restore()
}

func TestColorableStdout(t *testing.T) {
	if ColorableStdout() == nil {
		t.Fatal("expected non-nil writer")
	}
}
