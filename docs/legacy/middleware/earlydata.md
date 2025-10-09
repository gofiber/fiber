---
id: earlydata
---

# EarlyData

The Early Data middleware adds TLS 1.3 "0-RTT" support to [Fiber](https://github.com/gofiber/fiber). When the client and server share a PSK, TLS 1.3 lets the client send data with the first flight and skip the initial round trip.

Enable Fiber's `TrustProxy` option before using this middleware to avoid spoofed client headers.

Enabling early data in a reverse proxy (for example, `ssl_early_data on;` in nginx) makes requests replayable. Review these resources before proceeding:

- [datatracker](https://datatracker.ietf.org/doc/html/rfc8446#section-8)
- [trailofbits](https://blog.trailofbits.com/2019/03/25/what-application-developers-need-to-know-about-tls-early-data-0rtt)

By default, the middleware permits early data only for safe methods (`GET`, `HEAD`, `OPTIONS`, `TRACE`) and rejects other requests before your handler runs. Override this behavior with the `AllowEarlyData` option.

## Signatures

```go
func New(config ...Config) fiber.Handler
func IsEarly(c fiber.Ctx) bool
```

`IsEarly` returns `true` when a request used early data and the middleware allowed it to proceed.

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/earlydata"
)
```

Once your Fiber app is initialized, use the middleware like this:

```go
// Initialize default config
app.Use(earlydata.New())

// Or extend your config for customization
app.Use(earlydata.New(earlydata.Config{
    Error: fiber.ErrTooEarly,
    // ...
}))
```

## Config

| Property       | Type                    | Description | Default                                                |
|:---------------|:------------------------|:-----------|:-------------------------------------------------------|
| Next           | `func(fiber.Ctx) bool` | Skip this middleware when the function returns true. | `nil` |
| IsEarlyData    | `func(fiber.Ctx) bool` | Reports whether the request used early data. | Function checking if "Early-Data" header equals "1" |
| AllowEarlyData | `func(fiber.Ctx) bool` | Decides if an early-data request should be allowed. | Function rejecting on unsafe and allowing safe methods |
| Error          | `error`                 | Returned when an early-data request is rejected. | `fiber.ErrTooEarly` |

## Default Config

```go
var ConfigDefault = Config{
    IsEarlyData: func(c fiber.Ctx) bool {
        return c.Get(DefaultHeaderName) == DefaultHeaderTrueValue
    },
    AllowEarlyData: func(c fiber.Ctx) bool {
        return fiber.IsMethodSafe(c.Method())
    },
    Error: fiber.ErrTooEarly,
}
```

## Constants

```go
const (
    DefaultHeaderName      = "Early-Data"
    DefaultHeaderTrueValue = "1"
)
```
