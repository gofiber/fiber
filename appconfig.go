package fiber

import (
	"github.com/gofiber/fiber/v3/internal/appconfig"
)

// The internal packages behind the extractors read a few Config bits per
// request, and Config() copies the whole struct to answer each one. They
// cannot read them directly: config is unexported, and importing those
// packages here would be a cycle.
//
// From init rather than New, so no request can reach them before it is set.
func init() {
	appconfig.SetReader(func(a any) appconfig.Hot {
		app, ok := a.(*App)
		if !ok {
			return appconfig.Hot{}
		}
		// A nil app faults here, as reading Config() off one did before.
		return appconfig.Hot{
			Immutable:                app.config.Immutable,
			DisableHeaderNormalizing: app.config.DisableHeaderNormalizing,
			UnescapePath:             app.config.UnescapePath,
		}
	})
}
