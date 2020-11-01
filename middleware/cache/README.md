# Cache
Cache middleware for [Fiber](https://github.com/gofiber/fiber) designed to intercept responses and cache them. This middleware will cache the `Body`, `Content-Type` and `StatusCode` using the `c.Path()` as unique identifier. Special thanks to [@codemicro](https://github.com/codemicro/fiber-cache) for creating this middleware for Fiber core!

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
app.Use(cache.New(cache.Config{
	Next: func(c *fiber.Ctx) bool {
		return c.Query("refresh") == "true"
	},
	Expiration: 30 * time.Minute,
	CacheControl: true,
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
	// Optional. Default: 1 * time.Minute
	Expiration time.Duration

	// CacheControl enables client side caching if set to true
	//
	// Optional. Default: false
	CacheControl bool

	// Key allows you to generate custom keys, by default c.Path() is used
	//
	// Default: func(c *fiber.Ctx) string {
	//   return c.Path()
	// }
	Key func(*fiber.Ctx) string

	// Store is used to store the state of the middleware
	//
	// Default: an in memory store for this process only
	Store fiber.Storage
}
```

### Default Config
```go
// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:         nil,
	Expiration:   1 * time.Minute,
	CacheControl: false,
	Key: func(c *fiber.Ctx) string {
		return c.Path()
	},
}
```
