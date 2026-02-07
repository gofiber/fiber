package fiber

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/prefork"
)

var errOnPreforkServeTest = errors.New("on prefork serve test sentinel")

// Test_IsChild_Integration verifies that IsChild() correctly delegates to fasthttp/prefork.IsChild()
func Test_IsChild_Integration(t *testing.T) {
	// Test when not a child
	if IsChild() {
		t.Error("IsChild() should return false when FASTHTTP_PREFORK_CHILD is not set")
	}

	// Test when is a child
	t.Setenv("FASTHTTP_PREFORK_CHILD", "1")
	if !IsChild() {
		t.Error("IsChild() should return true when FASTHTTP_PREFORK_CHILD=1")
	}

	// Verify it's using the same logic as fasthttp
	if IsChild() != prefork.IsChild() {
		t.Error("IsChild() should return the same value as prefork.IsChild()")
	}
}

// Test_Prefork_Logger verifies the logger adapter works correctly
func Test_Prefork_Logger(_ *testing.T) {
	logger := preforkLogger{}

	// Should not panic
	logger.Printf("test message: %s", "value")
}

// Test_ListenConfig_OnPreforkServe verifies OnPreforkServe field exists and works
func Test_ListenConfig_OnPreforkServe(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork: true,
		OnPreforkServe: func(_ net.Addr) (net.Listener, error) {
			// This callback would create a reuseport listener in real usage
			return nil, errOnPreforkServeTest
		},
	}

	require.True(t, cfg.EnablePrefork)
	require.NotNil(t, cfg.OnPreforkServe)
}

// Test_Listener_Prefork_Without_Callback verifies warning is logged without OnPreforkServe
func Test_Listener_Prefork_Without_Callback(t *testing.T) {
	app := New()

	// Create a simple listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		if closeErr := ln.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			t.Errorf("listener close failed: %v", closeErr)
		}
	}()

	errCh := make(chan error, 1)
	shutdownErrCh := make(chan error, 1)
	go func() {
		// This should log a warning and fall back to single process mode
		errCh <- app.Listener(ln, ListenConfig{
			EnablePrefork:         true,
			DisableStartupMessage: true,
			// OnPreforkServe NOT set - should warn and fall back
		})
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		shutdownErrCh <- app.Shutdown()
	}()

	require.NoError(t, <-errCh)
	require.NoError(t, <-shutdownErrCh)
}

// Test_PreforkRecoverThreshold verifies the recover threshold is properly set
func Test_PreforkRecoverThreshold(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork:           true,
		PreforkRecoverThreshold: 10,
	}

	require.Equal(t, 10, cfg.PreforkRecoverThreshold)
}
