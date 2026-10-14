---
id: responsetime
---

# ResponseTime

Response time middleware for [Fiber](https://github.com/gofiber/fiber) that measures the time spent handling a request and exposes it via a response header.

The value is the elapsed time as a Go duration string, as `time.Duration.String` formats it (for example `1.532ms` or `2.1s`), and `time.ParseDuration` reads it back. The clock starts when this middleware runs and stops when the rest of the chain returns, whether it returned an error or not, so register it before the middleware you want included in the measurement.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/responsetime"
)
```

### Default config

```go
app.Use(responsetime.New())
```

### Custom header

```go
app.Use(responsetime.New(responsetime.Config{
    Header: "X-Elapsed",
}))
```

### Skip logic

```go
app.Use(responsetime.New(responsetime.Config{
    Next: func(c fiber.Ctx) bool {
        return c.Path() == "/healthz"
    },
}))
```

## Config

| Property | Type | Description | Default |
| :------- | :--- | :---------- | :------ |
| Next | `func(c fiber.Ctx) bool` | Defines a function to skip this middleware when it returns `true`. | `nil` |
| Header | `string` | Header key used to store the measured response time. If left empty, the default header is used. | `"X-Response-Time"` |

## Default Config

```go
var ConfigDefault = Config{
    Next:   nil,
    Header: fiber.HeaderXResponseTime,
}
```
