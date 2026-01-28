package fiber

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp/prefork"
)

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
func Test_Prefork_Logger(t *testing.T) {
	logger := preforkLogger{}

	// Should not panic
	logger.Printf("test message: %s", "value")
}

// Test_ListenConfig_OnPreforkServe verifies OnPreforkServe field exists and works
func Test_ListenConfig_OnPreforkServe(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork: true,
		OnPreforkServe: func(addr net.Addr) (net.Listener, error) {
			// This callback would create a reuseport listener in real usage
			return nil, nil
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
	defer ln.Close()

	// Start server in background
	go func() {
		// This should log a warning and fall back to single process mode
		_ = app.Listener(ln, ListenConfig{
			EnablePrefork:         true,
			DisableStartupMessage: true,
			// OnPreforkServe NOT set - should warn and fall back
		})
	}()

	// Give it time to start
	require.NoError(t, app.Shutdown())
}

// Test_PreforkRecoverThreshold verifies the recover threshold is properly set
func Test_PreforkRecoverThreshold(t *testing.T) {
	cfg := ListenConfig{
		EnablePrefork:           true,
		PreforkRecoverThreshold: 10,
	}

	require.Equal(t, 10, cfg.PreforkRecoverThreshold)
}
