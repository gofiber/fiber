---
id: extractor
title: 🧲 Extractor Utilities
sidebar_position: 9
---

The `extractor` package provides small helper functions for retrieving values from an incoming request. These helpers are used by several middleware packages but can also be used directly in your own code.

## Signatures

```go
package extractor

// Extractor defines a value extraction function with optional metadata.
type Extractor struct {
    Extract func(fiber.Ctx) (string, error)
    Key     string      // parameter/header name
    Chain   []Extractor // list of extractors when using Chain
}
```

### Built-in Helpers

- `FromAuthHeader(name, scheme string)` – extracts a value from the `Authorization` header, typically used for API keys or tokens.
- `FromCookie(name string)`
- `FromHeader(name string)`
- `FromQuery(name string)`
- `FromForm(name string)`
- `FromParam(name string)`
- `Chain(extractors ...Extractor)` – tries each extractor in order

Each helper returns an `Extractor` that can be reused across middleware or your own handlers.

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/extractor"
)

func main() {
    app := fiber.New()

    tokenFromHeader := extractor.FromHeader("X-Auth")
    tokenFromCookie := extractor.FromCookie("access_token")

    combined := extractor.Chain(tokenFromHeader, tokenFromCookie)

    app.Get("/", func(c fiber.Ctx) error {
        token, err := combined.Extract(c)
        if err != nil {
            return err
        }
        return c.SendString(token)
    })

    app.Listen(":3000")
}
```
