---
id: requestid
---

# RequestID

The RequestID middleware generates or propagates a request identifier, adding it to the response headers and request context.

## Signatures

```go
func New(config ...Config) fiber.Handler
func FromContext(c fiber.Ctx) string
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/requestid"
)
```

Once your Fiber app is initialized, add the middleware like this:

```go
// Initialize default config
app.Use(requestid.New())

// Or extend your config for customization
app.Use(requestid.New(requestid.Config{
    Header:    "X-Custom-Header",
    Generator: func() string {
        return "static-id"
    },
}))
```

If the request already includes the configured header, that value is reused instead of generating a new one. The middleware
rejects IDs containing characters outside the visible ASCII range (for example, control characters or obs-text bytes) and
will regenerate the value using the configured generator or a UUID to keep headers RFC-compliant across transports.

Retrieve the request ID

```go
func handler(c fiber.Ctx) error {
    id := requestid.FromContext(c)
    log.Printf("Request ID: %s", id)
    return c.SendString("Hello, World!")
}
```

## Config

| Property  | Type                 | Description                              | Default        |
|:----------|:---------------------|:-----------------------------------------|:---------------|
| Next      | `func(fiber.Ctx) bool` | Skip when the function returns `true`.    | `nil`          |
| Header    | `string`             | Header key used to store the request ID. | "X-Request-ID" |
| Generator | `func() string`      | Function that generates the identifier.  | utils.UUID     |

## Default Config

The default config uses a fast UUID generator which will expose the number of
requests made to the server. To conceal this value for better privacy, use the
`utils.UUIDv4` generator.

```go
var ConfigDefault = Config{
    Next:       nil,
    Header:     fiber.HeaderXRequestID,
    Generator:  utils.UUID,
}
```
