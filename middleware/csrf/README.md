# CSRF
CSRF middleware for [Fiber](https://github.com/gofiber/fiber) that provides [Cross-site request forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) protection by passing a csrf token via cookies. This cookie value will be used to compare against the client csrf token in POST requests. When the csrf token is invalid, this middleware will return the `fiber.ErrForbidden` error.

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
	"github.com/gofiber/fiber/v2/middleware/csrf"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Initialize default config
app.Use(csrf.New())

// Or extend your config for customization
app.Use(csrf.New(csrf.Config{
	TokenLookup: "header:X-CSRF-Token",
	ContextKey: "csrf",
	Cookie: &fiber.Cookie{
		Name: "_csrf",
	},
	CookieExpires: 24 * time.Hour,
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

	// TokenLookup is a string in the form of "<source>:<key>" that is used
	// to extract token from the request.
	//
	// Optional. Default value "header:X-CSRF-Token".
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	TokenLookup string

	// Cookie
	//
	// Optional.
	Cookie *fiber.Cookie

	// CookieExpires is the duration before the cookie will expire
	//
	// Optional. Default: 24 * time.Hour
	CookieExpires time.Duration

	// Context key to store generated CSRF token into context.
	//
	// Optional. Default value "csrf".
	ContextKey string
}
```

### Default Config
```go
var ConfigDefault = Config{
	Next:        nil,
	TokenLookup: "header:X-CSRF-Token",
	ContextKey:  "csrf",
	Cookie: &fiber.Cookie{
		Name:     "_csrf",
		Domain:   "",
		Path:     "",
		Secure:   false,
		HTTPOnly: false,
	},
	CookieExpires: 24 * time.Hour,
}
```
