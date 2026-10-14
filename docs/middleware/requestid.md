---
id: requestid
---

# RequestID

The RequestID middleware generates or propagates a request identifier, adding it to the response headers and request context.

## Signatures

```go
func New(config ...Config) fiber.Handler
func FromContext(ctx any) string
```

`FromContext` accepts a `fiber.CustomCtx`, `fiber.Ctx`, a `*fasthttp.RequestCtx`, or a `context.Context`. It returns an empty string when no request ID is stored, for example when `Next` skipped the middleware. When the client sent a valid request ID, the string is that header value, which points into the request's buffer unless [`Immutable`](../api/fiber.md#immutable) is set: copy it with `strings.Clone` before a goroutine keeps it.

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
will regenerate the value using up to three attempts from the configured generator (or SecureToken when no generator is set). When a
custom generator fails to produce a valid ID, the middleware falls back to SecureToken to keep headers RFC-compliant
across transports.

The middleware also registers the `${requestid}` and `${request-id}` tags, so the [logger](./logger.md) middleware and [`log.WithContext`](../api/log.md#context-tags) can print the ID. Call `requestid.New()` before `logger.New()`: a logger only sees the tags registered when it is built, and one built earlier prints an empty string for `${requestid}`. `log.WithContext` has no such order requirement.

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
| Header    | `string`             | Header key used to read an incoming request ID and to store the ID on the response. | "X-Request-ID" |
| Generator | `func() string`      | Function that generates the identifier.  | utils.SecureToken |

## Default Config

The default config uses a cryptographically secure token generator for better security and privacy.

```go
var ConfigDefault = Config{
    Next:       nil,
    Header:     fiber.HeaderXRequestID,
    Generator:  utils.SecureToken,
}
```
