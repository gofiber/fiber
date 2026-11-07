---
id: deprecation
---

# Deprecation

Deprecation middleware for [Fiber](https://github.com/gofiber/fiber) that tells clients a route is deprecated, or will stop working, with the `Deprecation` header ([RFC 9745](https://www.rfc-editor.org/rfc/rfc9745)) and the `Sunset` header ([RFC 8594](https://www.rfc-editor.org/rfc/rfc8594)).

Deprecation does not change how the route behaves. The headers are a hint: clients and API gateways can log them, warn developers, or plan a migration. The headers are set before the rest of the chain runs, so a response whose headers are flushed inside the handler (`SendStreamWriter`, SSE, early hints) carries them too.

`Deprecation` is sent as a Structured Field Date (`@` and Unix seconds), and `Sunset` as an HTTP-date. A date in the future announces an upcoming deprecation or sunset. Both carry whole seconds. RFC 9745 forbids a `Sunset` earlier than `Deprecation`, so `New` panics on that config, compared at that resolution.

`Link` and `SunsetLink` may be set without their date, to publish a deprecation or sunset policy before anything is deprecated ([RFC 9745 Section 3.1](https://www.rfc-editor.org/rfc/rfc9745#section-3.1), [RFC 8594 Section 6](https://www.rfc-editor.org/rfc/rfc8594#section-6)). Each must be an absolute URI without `<`, `>`, whitespace or control characters, or `New` panics.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the package:

```go
import (
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/deprecation"
)
```

### Deprecate a group of routes

```go
v1 := app.Group("/v1", deprecation.New(deprecation.Config{
    Deprecation: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
    Sunset:      time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
    Link:        "https://example.com/docs/v2-migration",
}))
```

A response from `/v1` then carries:

```http
Deprecation: @1767225600
Sunset: Wed, 01 Jul 2026 00:00:00 GMT
Link: <https://example.com/docs/v2-migration>; rel="deprecation"
```

### Announce a sunset only

```go
app.Get("/legacy", deprecation.New(deprecation.Config{
    Sunset:     time.Date(2026, time.December, 31, 23, 59, 59, 0, time.UTC),
    SunsetLink: "https://example.com/sunset-policy",
}), handler)
```

### Publish a deprecation policy only

```go
app.Use(deprecation.New(deprecation.Config{
    Link: "https://example.com/deprecation-policy",
}))
```

### Skip logic

```go
app.Use(deprecation.New(deprecation.Config{
    Deprecation: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
    Next: func(c fiber.Ctx) bool {
        return c.Path() == "/healthz"
    },
}))
```

## Config

| Property | Type | Description | Default |
| :------- | :--- | :---------- | :------ |
| Next | `func(c fiber.Ctx) bool` | Defines a function to skip this middleware when it returns `true`. | `nil` |
| Deprecation | `time.Time` | When the route was or will be deprecated. Sent in the `Deprecation` header. | zero time (not sent) |
| Sunset | `time.Time` | When the route is expected to stop responding. Sent in the `Sunset` header. Must not be earlier than `Deprecation`. | zero time (not sent) |
| Link | `string` | URI that documents the deprecation, added to the `Link` header with `rel="deprecation"`. | `""` |
| SunsetLink | `string` | URI that documents the sunset policy, added to the `Link` header with `rel="sunset"`. | `""` |

At least one field other than `Next` is required. The links are appended to a `Link` header that is already set when the middleware runs. A later handler that calls `c.Set(fiber.HeaderLink, ...)` replaces them, so add links with `c.Links` instead.

## Default Config

```go
var ConfigDefault = Config{}
```

There is no useful default — you must provide at least `Deprecation`, `Sunset`, `Link` or `SunsetLink`.
