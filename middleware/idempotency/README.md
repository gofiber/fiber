# Idempotency Middleware

Idempotency middleware for [Fiber](https://github.com/gofiber/fiber) allows for fault-tolerant APIs where duplicate requests — for example due to networking issues on the client-side — do not erroneously cause the same action performed multiple times on the server-side.

Refer to https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-02 for a better understanding.

## Table of Contents

- [Idempotency Middleware](#idempotency-middleware)
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
	"github.com/gofiber/fiber/v2/middleware/idempotency"
)
```

Then create a Fiber app with `app := fiber.New()`.

### Default Config

```go
app.Use(idempotency.New())
```

### Custom Config

```go
app.Use(idempotency.New(idempotency.Config{
	Lifetime: 42 * time.Minute,
	// ...
}))
```

### Config

```go
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: a function which skips the middleware on safe HTTP request method.
	Next func(c *fiber.Ctx) bool

	// Lifetime is the maximum lifetime of an idempotency key.
	//
	// Optional. Default: 30 * time.Minute
	Lifetime time.Duration

	// KeyHeader is the name of the header that contains the idempotency key.
	//
	// Optional. Default: X-Idempotency-Key
	KeyHeader string
	// KeyHeaderValidate defines a function to validate the syntax of the idempotency header.
	//
	// Optional. Default: a function which ensures the header is 36 characters long (the size of an UUID).
	KeyHeaderValidate func(string) error

	// KeepResponseHeaders is a list of headers that should be kept from the original response.
	//
	// Optional. Default: nil (to keep all headers)
	KeepResponseHeaders []string

	// Lock locks an idempotency key.
	//
	// Optional. Default: an in-memory locker for this process only.
	Lock Locker

	// Storage stores response data by idempotency key.
	//
	// Optional. Default: an in-memory storage for this process only.
	Storage fiber.Storage
}
```

### Default Config

```go
var ConfigDefault = Config{
	Next: func(c *fiber.Ctx) bool {
		// Skip middleware if the request was done using a safe HTTP method
		return fiber.IsMethodSafe(c.Method())
	},

	Lifetime: 30 * time.Minute,

	KeyHeader: "X-Idempotency-Key",
	KeyHeaderValidate: func(k string) error {
		if l, wl := len(k), 36; l != wl { // UUID length is 36 chars
			return fmt.Errorf("%w: invalid length: %d != %d", ErrInvalidIdempotencyKey, l, wl)
		}

		return nil
	},

	KeepResponseHeaders: nil,

	Lock: nil, // Set in configDefault so we don't allocate data here.

	Storage: nil, // Set in configDefault so we don't allocate data here.
}
```
