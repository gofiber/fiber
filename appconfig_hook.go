package fiber

import (
	"github.com/gofiber/fiber/v3/internal/appconfig"
)

// The internal packages behind the extractors read a handful of Config bits on
// every request, and Config() hands back a copy of the whole struct to answer
// each one. This reads them off the app directly, which the packages that need
// them cannot do themselves: config is unexported, and importing them here
// would be a cycle.
func init() {
	appconfig.Lookup = func(a any) (appconfig.Hot, bool) {
		app, ok := a.(*App)
		if !ok || app == nil {
			return appconfig.Hot{}, false
		}
		return appconfig.Hot{
			Immutable:                app.config.Immutable,
			DisableHeaderNormalizing: app.config.DisableHeaderNormalizing,
			UnescapePath:             app.config.UnescapePath,
		}, true
	}
}
