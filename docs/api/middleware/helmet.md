---
id: helmet
title: Helmet
---

Helmet middleware helps secure your apps by setting various HTTP headers.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples
```go
package main

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/helmet"
)

func main() {
  app := fiber.New()

  app.Use(helmet.New())

  app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("Welcome!")
  })

  app.Listen(":3000")
}
```

**Test:**

```curl
curl -I http://localhost:3000
```

## Config

```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip middleware.
	// Optional. Default: nil
	Next func(*fiber.Ctx) bool

	// XSSProtection
	// Optional. Default value "0".
	XSSProtection string

	// ContentTypeNosniff
	// Optional. Default value "nosniff".
	ContentTypeNosniff string

	// XFrameOptions
	// Optional. Default value "SAMEORIGIN".
	// Possible values: "SAMEORIGIN", "DENY", "ALLOW-FROM uri"
	XFrameOptions string

	// HSTSMaxAge
	// Optional. Default value 0.
	HSTSMaxAge int

	// HSTSExcludeSubdomains
	// Optional. Default value false.
	HSTSExcludeSubdomains bool

	// ContentSecurityPolicy
	// Optional. Default value "".
	ContentSecurityPolicy string

	// CSPReportOnly
	// Optional. Default value false.
	CSPReportOnly bool

	// HSTSPreloadEnabled
	// Optional. Default value false.
	HSTSPreloadEnabled bool

	// ReferrerPolicy
	// Optional. Default value "ReferrerPolicy".
	ReferrerPolicy string

	// Permissions-Policy
	// Optional. Default value "".
	PermissionPolicy string

	// Cross-Origin-Embedder-Policy
	// Optional. Default value "require-corp".
	CrossOriginEmbedderPolicy string

	// Cross-Origin-Opener-Policy
	// Optional. Default value "same-origin".
	CrossOriginOpenerPolicy string

	// Cross-Origin-Resource-Policy
	// Optional. Default value "same-origin".
	CrossOriginResourcePolicy string

	// Origin-Agent-Cluster
	// Optional. Default value "?1".
	OriginAgentCluster string

	// X-DNS-Prefetch-Control
	// Optional. Default value "off".
	XDNSPrefetchControl string

	// X-Download-Options
	// Optional. Default value "noopen".
	XDownloadOptions string

	// X-Permitted-Cross-Domain-Policies
	// Optional. Default value "none".
	XPermittedCrossDomain string
}
```

## Default Config

```go
var ConfigDefault = Config{
	XSSProtection:             "0",
	ContentTypeNosniff:        "nosniff",
	XFrameOptions:             "SAMEORIGIN",
	ReferrerPolicy:            "no-referrer",
	CrossOriginEmbedderPolicy: "require-corp",
	CrossOriginOpenerPolicy:   "same-origin",
	CrossOriginResourcePolicy: "same-origin",
	OriginAgentCluster:        "?1",
	XDNSPrefetchControl:       "off",
	XDownloadOptions:          "noopen",
	XPermittedCrossDomain:     "none",
}
```
