//go:build windows

package console

import (
	"os"
	"testing"
	"unsafe"
)

func TestColorableStdoutEnablesVirtualTerminal(t *testing.T) {
	handle := os.Stdout.Fd()
	var mode uint32
	r, _, err := procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		t.Skip("stdout is not a console")
	}

	if mode&cENABLE_VIRTUAL_TERMINAL_PROCESSING == 0 {
		if _, _, setErr := procSetConsoleMode.Call(handle, uintptr(mode|cENABLE_VIRTUAL_TERMINAL_PROCESSING)); setErr != nil {
			t.Skipf("unable to enable virtual terminal processing: %v", setErr)
		}
		t.Cleanup(func() {
			procSetConsoleMode.Call(handle, uintptr(mode))
		})
	}

	if got := ColorableStdout(); got != os.Stdout {
		t.Fatalf("expected stdout writer, got %T", got)
	}
}
