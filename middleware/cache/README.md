# Cache
Cache middleware for [Fiber](https://github.com/gofiber/fiber) designed to intercept responses and cache them. This middleware will cache the `Body`, `Content-Type` and `StatusCode` using the `c.Path()` as unique identifier. Special thanks to [@codemicro](github.com/codemicro/fiber-cache).

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)


### Signatures
```go
func New(config ...Config) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Initialize default config
app.Use(cache.New())

// Or extend your config for customization
app.Use(cache.New(cacge.Config{
	Expiration: 30 * time.Minute,
	Hydrate: func(c *fiber.Ctx) error {
		// Dynamic changes
		c.Set("X-Query", c.Query("id"))
		return nil
	}
}))
```

### Config
```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Expiration is the time that an cached response will live
	//
	// Optional. Default: 5 * time.Minute
	Expiration time.Duration

	// Hydrate is run before the response is returned to the client.
	// Because this middleware is backend-agnostic, it makes no assumptions
	// about what you want to do with cached response other than caching the statuscode,
	// content-type and response body. Hydrate allows you to alter the cached response.
	//
	// Optional. Default: nil
	Hydrate fiber.Handler
}
```

### Default Config
```go
// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:       nil,
	Expiration: 5 * time.Minute,
	Hydrate:    nil,
}
```
