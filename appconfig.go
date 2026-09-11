package fiber

import (
	"github.com/gofiber/fiber/v3/internal/appconfig"
)

// The internal packages behind the extractors read a handful of Config bits on
// every request, and Config() hands back a copy of the whole struct to answer
// each one. This reads them off the app directly, which the packages that need
// them cannot do themselves: config is unexported, and importing them here
// would be a cycle.
//
// Installed from init rather than from New, because a request can reach those
// packages through any app and they must never see the uninstalled reader.
func init() {
	appconfig.SetReader(func(a any) appconfig.Hot {
		app, ok := a.(*App)
		if !ok {
			return appconfig.Hot{}
		}
		// A nil app faults here, as reading Config() off one did for the
		// callers this replaced.
		return appconfig.Hot{
			Immutable:                app.config.Immutable,
			DisableHeaderNormalizing: app.config.DisableHeaderNormalizing,
			UnescapePath:             app.config.UnescapePath,
		}
	})
}
