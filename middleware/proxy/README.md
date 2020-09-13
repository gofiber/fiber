# Proxy
Proxy middleware for [Fiber](https://github.com/gofiber/fiber) that allows you to proxy requests to multiple hosts.

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)


### Signatures
```go
func New(config Config) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/proxy"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Minimal config
app.Use(proxy.New(proxy.Config{
	Hosts: "gofiber.io:8080, gofiber.io:8081",
}))

// Or extend your config for customization
app.Use(proxy.New(proxy.Config{
	Hosts: "gofiber.io:8080, gofiber.io:8081",
	Before: func(c *fiber.Ctx) error {
		c.Set("X-Real-IP", c.IP())
		return nil
	},
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

	// Comma-separated list of upstream HTTP server host addresses,
	// which are passed to Dial in a round-robin manner.
	//
	// Each address may contain port if default dialer is used.
	// For example,
	//
	//    - foobar.com:80
	//    - foobar.com:443
	//    - foobar.com:8080
	Hosts string

	// Before allows you to alter the request
	Before fiber.Handler

	// After allows you to alter the response
	After fiber.Handler
}
```

### Default Config
```go
var ConfigDefault = Config{
	Next:   nil,
	Hosts:  "",
	Before: nil,
	After:  nil,
}
```
