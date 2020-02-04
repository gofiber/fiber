package middleware

import (
	"fmt"

	"github.com/gofiber/fiber"
)

// HelmetOptions https://github.com/helmetjs/helmet#how-it-works
type HelmetOptions struct {
	ContentSecurityPolicy string
	CrossDomain           string
	DNSPrefetchControl    string // default
	ExpectCt              string
	FeaturePolicy         string
	FrameGuard            string // default
	Hpkp                  string
	Hsts                  string // default
	IeNoOpen              string // default
	NoCache               string
	NoSniff               string // default
	ReferrerPolicy        string
	XSSFilter             string // default
}

// Helmet : Helps secure your apps by setting various HTTP headers.
func Helmet(opt ...*HelmetOptions) func(*fiber.Ctx) {
	return func(c *fiber.Ctx) {
		fmt.Println("Helmet is still under development, this middleware does nothing yet.")
		c.Next()
	}
}
