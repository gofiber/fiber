---
id: helmet
---

# Helmet

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

| Property                  | Type                    | Description                                 | Default          |
|:--------------------------|:------------------------|:--------------------------------------------|:-----------------|
| Next                      | `func(*fiber.Ctx) bool` | Next defines a function to skip middleware. | `nil`            |
| XSSProtection             | `string`                | XSSProtection                               | "0"              |
| ContentTypeNosniff        | `string`                | ContentTypeNosniff                          | "nosniff"        |
| XFrameOptions             | `string`                | XFrameOptions                               | "SAMEORIGIN"     |
| HSTSMaxAge                | `int`                   | HSTSMaxAge                                  | 0                |
| HSTSExcludeSubdomains     | `bool`                  | HSTSExcludeSubdomains                       | false            |
| ContentSecurityPolicy     | `string`                | ContentSecurityPolicy                       | ""               |
| CSPReportOnly             | `bool`                  | CSPReportOnly                               | false            |
| HSTSPreloadEnabled        | `bool`                  | HSTSPreloadEnabled                          | false            |
| ReferrerPolicy            | `string`                | ReferrerPolicy                              | "ReferrerPolicy" |
| PermissionPolicy          | `string`                | Permissions-Policy                          | ""               |
| CrossOriginEmbedderPolicy | `string`                | Cross-Origin-Embedder-Policy                | "require-corp"   |
| CrossOriginOpenerPolicy   | `string`                | Cross-Origin-Opener-Policy                  | "same-origin"    |
| CrossOriginResourcePolicy | `string`                | Cross-Origin-Resource-Policy                | "same-origin"    |
| OriginAgentCluster        | `string`                | Origin-Agent-Cluster                        | "?1"             |
| XDNSPrefetchControl       | `string`                | X-DNS-Prefetch-Control                      | "off"            |
| XDownloadOptions          | `string`                | X-Download-Options                          | "noopen"         |
| XPermittedCrossDomain     | `string`                | X-Permitted-Cross-Domain-Policies           | "none"           |

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
