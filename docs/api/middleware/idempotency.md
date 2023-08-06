---
id: idempotency
---

# Idempotency

Idempotency middleware for [Fiber](https://github.com/gofiber/fiber) allows for fault-tolerant APIs where duplicate requests — for example due to networking issues on the client-side — do not erroneously cause the same action performed multiple times on the server-side.

Refer to https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-idempotency-key-header-02 for a better understanding.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/idempotency"
)
```

After you initiate your Fiber app, you can use the following possibilities:

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

| Property            | Type                    | Description                                                                              | Default                        |
|:--------------------|:------------------------|:-----------------------------------------------------------------------------------------|:-------------------------------|
| Next                | `func(*fiber.Ctx) bool` | Next defines a function to skip this middleware when returned true.                      | A function for safe methods    |
| Lifetime            | `time.Duration`         | Lifetime is the maximum lifetime of an idempotency key.                                  | 30 * time.Minute               |
| KeyHeader           | `string`                | KeyHeader is the name of the header that contains the idempotency key.                   | "X-Idempotency-Key"            |
| KeyHeaderValidate   | `func(string) error`    | KeyHeaderValidate defines a function to validate the syntax of the idempotency header.   | A function for UUID validation |
| KeepResponseHeaders | `[]string`              | KeepResponseHeaders is a list of headers that should be kept from the original response. | nil (keep all headers)         |
| Lock                | `Locker`                | Lock locks an idempotency key.                                                           | An in-memory locker            |
| Storage             | `fiber.Storage`         | Storage stores response data by idempotency key.                                         | An in-memory storage           |

## Default Config

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
