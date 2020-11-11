# Session
Session middleware for [Fiber](https://github.com/gofiber/fiber) that recovers from panics anywhere in the stack chain and handles the control to the centralized [ErrorHandler](https://docs.gofiber.io/error-handling).

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
  "github.com/gofiber/fiber/v2/middleware/session"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Default middleware config
store := session.New()

// This panic will be catch by the middleware
app.Get("/", func(c *fiber.Ctx) error {
	sess := store.Get(c)
	defer sess.Save()

	// Get value
	name := sess.Get("name")
	fmt.Println(val)

	// Set key/value
	sess.Set("name", "john")

	// Delete key
	sess.Delete("name")


	return fmt.Fprintf(ctx, "Welcome %v", name)
})
```

### Config
```go
// Config defines the config for middleware.
type Config struct {
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "param:<name>"
	// - "form:<name>"
	// - "cookie:<name>"
	//
	// Optional. Default value "cookie:_csrf".
	// TODO: When to override Cookie.Value?
	KeyLookup string

	// Optional. Session ID generator function.
	//
	// Default: utils.UUID
	KeyGenerator func() string

	// Optional. Cookie to set values on
	//
	// NOTE: Value, MaxAge and Expires will be overriden by the session ID and expiration
	// TODO: Should this be a pointer, if yes why?
	Cookie fiber.Cookie

	// Allowed session duration
	//
	// Optional. Default: 24 * time.Hour
	Expiration time.Duration

	// Storage interface
	//
	// Optional. Default: memory.New()
	Storage fiber.Storage
}
```

### Default Config
```go
var ConfigDefault = Config{
	Cookie: fiber.Cookie{
		Value: "session_id",
	},
	Expiration:   24 * time.Hour,
	KeyGenerator: utils.UUID,
}
```