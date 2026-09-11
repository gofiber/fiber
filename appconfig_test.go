package fiber

import (
	"testing"

	"github.com/gofiber/fiber/v3/internal/appconfig"
	"github.com/stretchr/testify/require"
)

// Test_AppConfigReader covers the read the internal hot paths use instead of
// Config(): it must report what the app was configured with, answer nothing
// for anything that is not an app, and stay the reader this package installed.
func Test_AppConfigReader(t *testing.T) {
	t.Parallel()

	t.Run("reports the configured bits", func(t *testing.T) {
		t.Parallel()

		app := New(Config{Immutable: true, DisableHeaderNormalizing: true, UnescapePath: true})
		require.Equal(t, appconfig.Hot{Immutable: true, DisableHeaderNormalizing: true, UnescapePath: true}, appconfig.Of(app))
	})

	t.Run("reports the defaults", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, appconfig.Hot{}, appconfig.Of(New()))
	})

	t.Run("agrees with Config", func(t *testing.T) {
		t.Parallel()

		app := New(Config{DisableHeaderNormalizing: true})
		h := appconfig.Of(app)
		cfg := app.Config()
		require.Equal(t, cfg.Immutable, h.Immutable)
		require.Equal(t, cfg.DisableHeaderNormalizing, h.DisableHeaderNormalizing)
		require.Equal(t, cfg.UnescapePath, h.UnescapePath)
	})

	t.Run("answers nothing for what is not an app", func(t *testing.T) {
		t.Parallel()

		for _, arg := range []any{nil, "app", 0, App{}} {
			require.Equal(t, appconfig.Hot{}, appconfig.Of(arg), "%T must not answer", arg)
		}
	})

	t.Run("a nil app faults, as reading its Config would", func(t *testing.T) {
		t.Parallel()

		require.Panics(t, func() { appconfig.Of((*App)(nil)) })
	})

	t.Run("the reader cannot be replaced", func(t *testing.T) {
		t.Parallel()

		// This package installed the reader in its init, so a later caller —
		// here, or anywhere else in the module — must not be able to change
		// what Immutable means for the rest of the process.
		appconfig.SetReader(func(any) appconfig.Hot {
			return appconfig.Hot{Immutable: true, DisableHeaderNormalizing: true, UnescapePath: true}
		})
		appconfig.SetReader(nil)

		require.Equal(t, appconfig.Hot{}, appconfig.Of(New()))
	})
}
