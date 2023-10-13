---
id: requestid
---

# RequestID

RequestID middleware for [Fiber](https://github.com/gofiber/fiber) that adds an identifier to the response.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/requestid"
)
```

After you initiate your Fiber app, you can use the following possibilities:

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

## Config

| Property   | Type                    | Description                                                                                       | Default        |
|:-----------|:------------------------|:--------------------------------------------------------------------------------------------------|:---------------|
| Next       | `func(*fiber.Ctx) bool` | Next defines a function to skip this middleware when returned true.                               | `nil`          |
| Header     | `string`                | Header is the header key where to get/set the unique request ID.                                  | "X-Request-ID" |
| Generator  | `func() string`         | Generator defines a function to generate the unique identifier.                                   | utils.UUID     |
| ContextKey | `interface{}`           | ContextKey defines the key used when storing the request ID in the locals for a specific request. | "requestid"    |

## Default Config
The default config uses a fast UUID generator which will expose the number of
requests made to the server. To conceal this value for better privacy, use the
`utils.UUIDv4` generator.

```go
var ConfigDefault = Config{
    Next:       nil,
    Header:     fiber.HeaderXRequestID,
	Generator:  utils.UUID,
	ContextKey: "requestid",
}
```
