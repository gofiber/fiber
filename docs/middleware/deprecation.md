---
id: deprecation
---

# Deprecation

Deprecation middleware for [Fiber](https://github.com/gofiber/fiber) that tells clients a route is deprecated, or will stop working, with the `Deprecation` header ([RFC 9745](https://www.rfc-editor.org/rfc/rfc9745)) and the `Sunset` header ([RFC 8594](https://www.rfc-editor.org/rfc/rfc8594)).

Deprecation does not change how the route behaves. The headers are a hint: clients and API gateways can log them, warn developers, or plan a migration.

`Deprecation` is sent as a Structured Field Date (`@` and Unix seconds), and `Sunset` as an HTTP-date. A date in the future announces an upcoming deprecation or sunset. Both carry whole seconds. RFC 9745 forbids a `Sunset` earlier than `Deprecation`, so `New` panics on that config, compared at that resolution. It also panics on a `Sunset` outside the years 0000 to 9999 and on a `Deprecation` beyond the range of a Structured Field Date, which the headers cannot express.

`Link` and `SunsetLink` may be set without their date, to publish a deprecation or sunset policy before anything is deprecated ([RFC 9745 Section 3.1](https://www.rfc-editor.org/rfc/rfc9745#section-3.1), [RFC 8594 Section 6](https://www.rfc-editor.org/rfc/rfc8594#section-6)). Each is a URI reference ([RFC 8288 Section 3](https://www.rfc-editor.org/rfc/rfc8288#section-3)): absolute, or relative to the request URI. It may hold only the characters [RFC 3986](https://www.rfc-editor.org/rfc/rfc3986) allows, so percent-encode anything else, `%` only as the start of a percent-encoding, and no userinfo (`user:password@`), which every client would see. An `http` or `https` URI, and a network-path reference starting with `//`, needs a host. `New` panics otherwise.

## When the headers are written

The headers are set before the rest of the chain runs, like those of `helmet` and `cors`, so a response the handler sends itself with `c.End()`, and the 500 an outer `recover.New()` sends for a panic, carry them. A `103 Early Hints` response carries only the `Link` lines, so it gets the deprecation and sunset links but not the two dates.

Anything later in the chain that replaces or edits the response headers wins. That includes a proxied response (`proxy.Do`, `proxy.Forward` and the other proxy helpers copy the upstream's headers over the response), a handler that calls `c.Response().Reset()`, the `304` and `416` responses of `SendFile` and `static`, the `503` that `timeout.New` sends, and a `c.Set` of `Deprecation`, `Sunset` or `Link`. With `DisableHeaderNormalizing: true`, a `c.Set` that spells the name another way, such as `deprecation`, adds a second line instead of replacing the middleware's. Set the headers in the handler after those calls when such a route must carry them, or add links with `c.Links`, which appends to the `Link` header instead of replacing it.

A link that the `Link` header already holds in the form the middleware writes (`<URL>; rel="deprecation"`) is not added again, for example when `c.RestartRouting()` runs the middleware a second time. The same link written another way, such as `<URL>;rel=deprecation`, is not recognized, and the middleware adds its own form next to it.

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

The dates below are placeholders. Use the real dates of your deprecation, and keep `Sunset` in the future: RFC 8594 reads a past one as "now".

### Deprecate a group of routes

```go
v1 := app.Group("/v1", deprecation.New(deprecation.Config{
    Deprecation: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
    Sunset:      time.Date(2027, time.July, 1, 0, 0, 0, 0, time.UTC),
    Link:        "https://example.com/docs/v2-migration",
}))
```

A response from `/v1` then carries:

```http
Deprecation: @1798761600
Sunset: Thu, 01 Jul 2027 00:00:00 GMT
Link: <https://example.com/docs/v2-migration>; rel="deprecation"
```

### Announce a sunset only

```go
app.Get("/legacy", deprecation.New(deprecation.Config{
    Sunset:     time.Date(2027, time.December, 31, 23, 59, 59, 0, time.UTC),
    SunsetLink: "https://example.com/sunset-policy",
}), handler)
```

### Publish a deprecation policy only

```go
app.Use(deprecation.New(deprecation.Config{
    Link: "/docs/deprecation-policy",
}))
```

### Skip logic

```go
app.Use(deprecation.New(deprecation.Config{
    Deprecation: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
    Next: func(c fiber.Ctx) bool {
        return c.Path() == "/healthz"
    },
}))
```

## Ordering

- Register it before `cache`. A cache hit returns before the middleware behind it runs, and with the default `StoreResponseHeaders: false` the cache replays only a fixed set of headers, so later responses would lose the headers. `StoreResponseHeaders: true` keeps them too, but a hit then replays the stored `Deprecation`, `Sunset` and `Link` over the ones the middleware just set: after you change a date or link, or skip the route with `Next`, entries stored earlier, including ones in an external `Storage` that outlives a deploy, serve the old values until they expire.
- With `idempotency` registered after it, a handler that adds links with `c.Links` rewrites the `Link` line, and a replayed response repeats the deprecation link. Register `idempotency` first.
- When it is registered at two levels, such as `app.Use` and a group, the inner instance replaces `Deprecation` and `Sunset` one by one, and both add their links. Set `Deprecation` and `Sunset` together at the level that overrides, so the pair stays in order. The middleware does not check the pair across levels: an outer `Sunset` of 2026-03-01 with an inner `Deprecation` of 2026-06-01 sends a `Sunset` earlier than `Deprecation`, which RFC 9745 Section 4 forbids. That is a known deviation, kept so an instance needs no knowledge of the others.

## Config

| Property | Type | Description | Default |
| :------- | :--- | :---------- | :------ |
| Next | `func(c fiber.Ctx) bool` | Defines a function to skip this middleware when it returns `true`. | `nil` |
| Deprecation | `time.Time` | When the route was or will be deprecated. Sent in the `Deprecation` header. | zero time (not sent) |
| Sunset | `time.Time` | When the route is expected to stop responding. Sent in the `Sunset` header. Must not be earlier than `Deprecation`. | zero time (not sent) |
| Link | `string` | URI reference that documents the deprecation, added to the `Link` header with `rel="deprecation"`. | `""` |
| SunsetLink | `string` | URI reference that documents the sunset policy, added to the `Link` header with `rel="sunset"`. | `""` |

At least one field other than `Next` is required. The links are appended to a `Link` header that is already set when the middleware runs.

## Default Config

```go
var ConfigDefault = Config{}
```

There is no useful default — you must provide at least `Deprecation`, `Sunset`, `Link` or `SunsetLink`.
