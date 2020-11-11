# CSRF
CSRF middleware for [Fiber](https://github.com/gofiber/fiber) that provides [Cross-site request forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) protection by passing a csrf token via cookies. This cookie value will be used to compare against the client csrf token in POST requests. When the csrf token is invalid, this middleware will delete the `_csrf` cookie and return the `fiber.ErrForbidden` error.
CSRF Tokens are generated on GET requests.

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
	Expiration: 24 * time.Hour,
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

	// KeyLookup is a string in the form of "<source>:<key>" that is used
	// to extract token from the request.
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	// - "cookie:<name>"
	//
	// Optional. Default: "header:X-CSRF-Token"
	KeyLookup string

	// Cookie settings to pass the CSRF token to the client on GET
	// requests.
	//
	// Optional.
	Cookie *fiber.Cookie

	// Expiration is the duration before csrf token will expire
	//
	// Optional. Default: 1 * time.Hour
	Expiration time.Duration

	// Store is used to store the state of the middleware
	//
	// Optional. Default: memory.New()
	Storage fiber.Storage

	// Context key to store generated CSRF token into context.
	// If left empty, token will not be stored in context.
	//
	// Optional. Default: ""
	ContextKey string

	// Optional. ID generator function.
	//
	// Optional. Default: utils.UUID
	KeyGenerator func() string
}
```

### Default Config
```go
var ConfigDefault = Config{
	KeyLookup: "header:X-Csrf-Token",
	Cookie: &fiber.Cookie{
		Name:     "_csrf",
		SameSite: "Strict",
	},
	Expiration:   1 * time.Hour,
	KeyGenerator: utils.UUID,
}
```
