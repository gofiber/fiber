# Early Data Middleware

The Early Data middleware for [Fiber](https://github.com/gofiber/fiber) adds support for TLS 1.3's early data ("0-RTT") feature.
Citing [RFC 8446](https://datatracker.ietf.org/doc/html/rfc8446#section-2-3), when a client and server share a PSK, TLS 1.3 allows clients to send data on the first flight ("early data") to speed up the request, effectively reducing the regular 1-RTT request to a 0-RTT request.

Make sure to enable fiber's `EnableTrustedProxyCheck` config option before using this middleware in order to not trust bogus HTTP request headers of the client.

Also be aware that enabling support for early data in your reverse proxy (e.g. nginx, as done with a simple `ssl_early_data on;`) makes requests replayable. Refer to the following documents before continuing:

- https://datatracker.ietf.org/doc/html/rfc8446#section-8
- https://blog.trailofbits.com/2019/03/25/what-application-developers-need-to-know-about-tls-early-data-0rtt/

By default, this middleware allows early data requests on safe HTTP request methods only and rejects the request otherwise, i.e. aborts the request before executing your handler. This behavior can be controlled by the `AllowEarlyData` config option.
Safe HTTP methods — `GET`, `HEAD`, `OPTIONS` and `TRACE` — should not modify a state on the server.

## Table of Contents

- [Early Data Middleware](#early-data-middleware)
	- [Table of Contents](#table-of-contents)
	- [Signatures](#signatures)
	- [Examples](#examples)
		- [Default Config](#default-config)
		- [Custom Config](#custom-config)
		- [Config](#config)
		- [Default Config](#default-config-1)

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

First import the middleware from Fiber,

```go
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/earlydata"
)
```

Then create a Fiber app with `app := fiber.New()`.

### Default Config

```go
app.Use(earlydata.New())
```

### Custom Config

```go
app.Use(earlydata.New(earlydata.Config{
	Error: fiber.ErrTooEarly,
	// ...
}))
```

### Config

```go
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// IsEarlyData returns whether the request is an early-data request.
	//
	// Optional. Default: a function which checks if the "Early-Data" request header equals "1".
	IsEarlyData func(c *fiber.Ctx) bool

	// AllowEarlyData returns whether the early-data request should be allowed or rejected.
	//
	// Optional. Default: a function which rejects the request on unsafe and allows the request on safe HTTP request methods.
	AllowEarlyData func(c *fiber.Ctx) bool

	// Error is returned in case an early-data request is rejected.
	//
	// Optional. Default: fiber.ErrTooEarly.
	Error error
}
```

### Default Config

```go
var ConfigDefault = Config{
	IsEarlyData: func(c *fiber.Ctx) bool {
		return c.Get("Early-Data") == "1"
	},

	AllowEarlyData: func(c *fiber.Ctx) bool {
		return fiber.IsMethodSafe(c.Method())
	},

	Error: fiber.ErrTooEarly,
}
```
