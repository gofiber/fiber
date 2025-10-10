---
id: helmet
---

# Helmet

Helmet secures your app by adding common security headers.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Once your Fiber app is initialized, add the middleware:

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/helmet"
)

func main() {
    app := fiber.New()

    app.Use(helmet.New())

    app.Get("/", func(c fiber.Ctx) error {
      return c.SendString("Welcome!")
    })

    app.Listen(":3000")
}
```

## Test

```bash
curl -I http://localhost:3000
```

## Config

| Property                  | Type                    | Description                                 | Default          |
|:--------------------------|:------------------------|:--------------------------------------------|:-----------------|
| Next                      | `func(fiber.Ctx) bool` | Skips the middleware when the function returns `true`. | `nil`            |
| XSSProtection             | `string`                | Value for the `X-XSS-Protection` header.               | "0"              |
| ContentTypeNosniff        | `string`                | Value for the `X-Content-Type-Options` header.         | "nosniff"        |
| XFrameOptions             | `string`                | Value for the `X-Frame-Options` header.                | "SAMEORIGIN"     |
| HSTSMaxAge                | `int`                   | `max-age` value for `Strict-Transport-Security`.       | 0                |
| HSTSExcludeSubdomains     | `bool`                  | Disables HSTS on subdomains when `true`.               | false            |
| ContentSecurityPolicy     | `string`                | Value for the `Content-Security-Policy` header.        | ""               |
| CSPReportOnly             | `bool`                  | Enables report-only mode for CSP.                      | false            |
| HSTSPreloadEnabled        | `bool`                  | Adds the `preload` directive to HSTS.                  | false            |
| ReferrerPolicy            | `string`                | Value for the `Referrer-Policy` header.                | "no-referrer" |
| PermissionPolicy          | `string`                | Value for the `Permissions-Policy` header.             | ""               |
| CrossOriginEmbedderPolicy | `string`                | Value for the `Cross-Origin-Embedder-Policy` header.   | "require-corp"   |
| CrossOriginOpenerPolicy   | `string`                | Value for the `Cross-Origin-Opener-Policy` header.     | "same-origin"    |
| CrossOriginResourcePolicy | `string`                | Value for the `Cross-Origin-Resource-Policy` header.   | "same-origin"    |
| OriginAgentCluster        | `string`                | Value for the `Origin-Agent-Cluster` header.           | "?1"             |
| XDNSPrefetchControl       | `string`                | Value for the `X-DNS-Prefetch-Control` header.         | "off"            |
| XDownloadOptions          | `string`                | Value for the `X-Download-Options` header.             | "noopen"         |
| XPermittedCrossDomain     | `string`                | Value for the `X-Permitted-Cross-Domain-Policies` header. | "none"        |

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
