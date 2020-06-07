package middleware

import (
	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
)

// Middleware types
type (
	// HelmetConfig defines the config for Hel;met middleware.
	HelmetConfig struct {
		// Next defines a function to skip this middleware.
		Next func(ctx *fiber.Ctx) bool

		ContentSecurityPolicy string            //  x
		CrossDomain           string            //  x
		DNSPrefetchControl    bool              //  ✓
		ExpectCTMaxAge        int               //  x
		ExpectCTEnfore        bool              //  x
		ExpectCTReportURI     string            //  x
		FeaturePolicy         map[string]string //  x
		FrameGuard            string            //  ✓
		HSTSMaxAge            int               //  ✓
		HSTSExcludeSubDomains bool              //  ✓
		IEnoOpen              bool              //  ✓
		NoSniff               bool              //  ✓
		ReferrerPolicy        string            //  x
		XSSFilter             bool              //  ✓
		XSSFilterReportURI    string            //  ✓
	}
)

// Helmet helps secure your apps by setting various HTTP headers
func Helmet() fiber.Handler {
	return func(ctx *fiber.Ctx) {
		// Get id from request
		rid := ctx.Get(fiber.HeaderXRequestID)
		// Create new UUID if empty
		if len(rid) <= 0 {
			rid = utils.UUID()
		}
		// Set new id to response
		ctx.Set(fiber.HeaderXRequestID, rid)
		// Continue stack
		ctx.Next()
	}
}
