# Limiter Middleware

Limiter middleware for [Fiber](https://github.com/gofiber/fiber) that is used to limit repeat requests to public APIs and/or endpoints such as password reset. It is also useful for API clients, web crawling, or other tasks that need to be throttled.

_NOTE: This middleware uses our [Storage](https://github.com/gofiber/storage) package to support various databases through a single interface. The default configuration for this middleware saves data to memory, see the examples below for other databases._

**NOTE: this module does not share state with other processes/servers by default.**

## Table of Contents

- [Limiter Middleware](#limiter-middleware)
	- [Table of Contents](#table-of-contents)
	- [Signatures](#signatures)
	- [Examples](#examples)
		- [Default Config](#default-config)
		- [Custom Config](#custom-config)
		- [Custom Storage/Database](#custom-storagedatabase)
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
  "github.com/gofiber/fiber/v2/middleware/limiter"
)
```

Then create a Fiber app with `app := fiber.New()`.

### Default Config

```go
// Default middleware config
app.Use(limiter.New())
```

### Custom Config

```go
// Or extend your config for customization
app.Use(limiter.New(limiter.Config{
	Next: func(c *fiber.Ctx) bool {
		return c.IP() == "127.0.0.1"
	},
	Max:          20,
	Expiration:     30 * time.Second,
	KeyGenerator: func(c *fiber.Ctx) string{
  		return "key"
	}
	LimitReached: func(c *fiber.Ctx) error {
		return c.SendFile("./toofast.html")
	},
	Storage: myCustomStore{}
}))
```

### Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3
app.Use(limiter.New(limiter.Config{
	Storage: storage,
}))
```

## Config

```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Max number of recent connections during `Duration` seconds before sending a 429 response
	//
	// Default: 5
	Max int

	// KeyGenerator allows you to generate custom keys, by default c.IP() is used
	//
	// Default: func(c *fiber.Ctx) string {
	//   return c.IP()
	// }
	KeyGenerator func(*fiber.Ctx) string

	// Expiration is the time on how long to keep records of requests in memory
	//
	// Default: 1 * time.Minute
	Expiration time.Duration

	// LimitReached is called when a request hits the limit
	//
	// Default: func(c *fiber.Ctx) error {
	//   return c.SendStatus(fiber.StatusTooManyRequests)
	// }
	LimitReached fiber.Handler

	// Store is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Storage fiber.Storage
}
```

A custom store can be used if it implements the `Storage` interface - more details and an example can be found in `store.go`.

### Default Config

```go
var ConfigDefault = Config{
	Max:        5,
	Expiration: 1 * time.Minute,
	KeyGenerator: func(c *fiber.Ctx) string {
		return c.IP()
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTooManyRequests)
	},
}
```
