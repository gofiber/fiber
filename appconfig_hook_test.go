package fiber

import (
	"testing"

	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/stretchr/testify/require"
)

// Test_AppConfigHook covers the read the internal hot paths use instead of
// Config(): it must report what the app was configured with, and refuse
// anything that is not an app so those callers fall back to the by-value read.
func Test_AppConfigHook(t *testing.T) {
	t.Parallel()

	t.Run("reports the configured bits", func(t *testing.T) {
		t.Parallel()

		app := New(Config{Immutable: true, DisableHeaderNormalizing: true, UnescapePath: true})
		h, ok := appconfig.Lookup(app)
		require.True(t, ok)
		require.Equal(t, appconfig.Hot{Immutable: true, DisableHeaderNormalizing: true, UnescapePath: true}, h)
	})

	t.Run("reports the defaults", func(t *testing.T) {
		t.Parallel()

		h, ok := appconfig.Lookup(New())
		require.True(t, ok)
		require.Equal(t, appconfig.Hot{}, h)
	})

	t.Run("agrees with Config", func(t *testing.T) {
		t.Parallel()

		app := New(Config{DisableHeaderNormalizing: true})
		h, ok := appconfig.Lookup(app)
		require.True(t, ok)
		cfg := app.Config()
		require.Equal(t, cfg.Immutable, h.Immutable)
		require.Equal(t, cfg.DisableHeaderNormalizing, h.DisableHeaderNormalizing)
		require.Equal(t, cfg.UnescapePath, h.UnescapePath)
	})

	t.Run("refuses anything that is not an app", func(t *testing.T) {
		t.Parallel()

		for _, arg := range []any{nil, "app", 0, (*App)(nil), App{}} {
			h, ok := appconfig.Lookup(arg)
			require.False(t, ok, "%T must not answer", arg)
			require.Equal(t, appconfig.Hot{}, h)
		}
	})
}
