---
id: cache
---

# Cache

Cache middleware for [Fiber](https://github.com/gofiber/fiber) that intercepts responses and stores the body, `Content-Type`, and status code under a deterministic key derived from request dimensions. Special thanks to [@codemicro](https://github.com/codemicro/fiber-cache) for contributing this middleware to Fiber core.

By default, cached responses expire after five minutes and the middleware stores up to 1 MB of response bodies.

## Request directives

- `Cache-Control: no-cache` returns the latest response while still caching it, so the status is always `miss`.
- `Cache-Control: no-store` skips caching and always forwards a fresh response.

If the response includes a `Cache-Control: max-age` directive, its value sets the cache entry's expiration.

## Cacheable status codes

The middleware caches these RFC 7231 status codes:

- `200: OK`
- `203: Non-Authoritative Information`
- `204: No Content`
- `206: Partial Content`
- `300: Multiple Choices`
- `301: Moved Permanently`
- `404: Not Found`
- `405: Method Not Allowed`
- `410: Gone`
- `414: URI Too Long`
- `501: Not Implemented`

Responses with other status codes result in an `unreachable` cache status.

For more about cacheable status codes and RFC 7231, see:

- [Cacheable - MDN Web Docs](https://developer.mozilla.org/en-US/docs/Glossary/Cacheable)

- [RFC7231 - Hypertext Transfer Protocol (HTTP/1.1): Semantics and Content](https://datatracker.ietf.org/doc/html/rfc7231)

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/cache"
    "github.com/gofiber/utils/v2"
)
```

Once your Fiber app is initialized, register the middleware:

```go
// Initialize default config
app.Use(cache.New())

// Or extend the config for customization
app.Use(cache.New(cache.Config{
    Next: func(c fiber.Ctx) bool {
        return fiber.Query[bool](c, "noCache")
    },
    Expiration: 30 * time.Minute,
    DisableCacheControl: true,
}))
```

Customize expiration and cache key behavior:

```go
app.Use(cache.New(cache.Config{
    ExpirationGenerator: func(c fiber.Ctx, cfg *cache.Config) time.Duration {
        newCacheTime, _ := strconv.Atoi(c.GetRespHeader("Cache-Time", "600"))
        return time.Second * time.Duration(newCacheTime)
    },
    // Optional: fully custom key
    KeyGenerator: func(c fiber.Ctx) string {
        return utils.CopyString(c.Path()) + "|tenant=" + c.Get("X-Tenant-ID")
    },
}))

app.Get("/", func(c fiber.Ctx) error {
    c.Response().Header.Add("Cache-Time", "6000")
    return c.SendString("hi")
})
```

Use `CacheInvalidator` to invalidate entries programmatically:

```go
app.Use(cache.New(cache.Config{
    CacheInvalidator: func(c fiber.Ctx) bool {
        return fiber.Query[bool](c, "invalidateCache")
    },
}))
```

`CacheInvalidator` defines custom invalidation rules. Return `true` to bypass the cache. In the example above, setting the `invalidateCache` query parameter to `true` invalidates the entry.

Cache keys are masked in logs and error messages by default. Set `DisableValueRedaction` to `true` if you explicitly need the raw key for debugging.

### Default cache key behavior (safe by default)

By default, cache keys include:

- request method (partitioned internally by the middleware),
- request path,
- canonicalized query string (enabled unless `DisableQueryKeys` is `true`),
- representation-driving request headers (`accept`, `accept-encoding`, `accept-language`).

This prevents common collisions from path-only keys (for example, `/?id=1` vs `/?id=2`) while keeping fragmentation bounded.

The middleware **does not include request body/form values in the default cache key**.

Cache lookup/storage is applied only for `GET` and `HEAD` requests by default. Other HTTP methods bypass the cache middleware. You can change this via the `Methods` config field.

If a response sets `Vary`, request lookup/storage is also partitioned by those header values unless `DisableVaryHeaders` is `true`. Responses with `Vary: *` remain uncacheable.

## Config

| Property             | Type                                           | Description                                                                                                                                                                                                                                                                                                    | Default                                                          |
| :------------------- | :--------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------- |
| Next                 | `func(fiber.Ctx) bool`                         | Next defines a function that is executed before creating the cache entry and can be used to execute the request without cache creation. If an entry already exists, it will be used. If you want to completely bypass the cache functionality in certain cases, you should use the [skip middleware](skip.md). | `nil`                                                            |
| Expiration           | `time.Duration`                                | Expiration is the time that a cached response will live. | `5 * time.Minute`                                                |
| CacheHeader          | `string`                                       | CacheHeader is the header on the response header that indicates the cache status, with the possible return values "hit," "miss," or "unreachable."                                                                                                                                                             | `X-Cache`                                                        |
| DisableCacheControl  | `bool`                                          | DisableCacheControl omits the `Cache-Control` header when set to `true`. | `false`                                                         |
| CacheInvalidator     | `func(fiber.Ctx) bool`                         | CacheInvalidator defines a function that is executed before checking the cache entry. It can be used to invalidate the existing cache manually by returning true. | `nil`                                                            |
| DisableValueRedaction | `bool`                                        | Turns off cache key redaction in logs and error messages when set to `true`. | `false`                                             |
| KeyGenerator         | `func(fiber.Ctx) string`                       | KeyGenerator allows you to generate custom keys. The HTTP method is partitioned internally by the middleware. | structured key from path + canonical query + selected headers/cookies |
| DisableQueryKeys     | `bool`                                         | Disables canonicalized query params in keys. | `false` |
| KeyHeaders           | `[]string`                                     | Header allow-list used for key partitioning. Names are normalized case-insensitively and sorted. Use `[]string{}` to disable header-based partitioning. | `[]string{"accept","accept-encoding","accept-language"}` |
| KeyCookies           | `[]string`                                     | Optional cookie allow-list for key partitioning. Explicit opt-in only; names remain case-sensitive. | `nil` |
| Methods              | `[]string`                                     | HTTP methods eligible for caching. Requests whose method is not in this list bypass the cache. Names are normalized to uppercase. | `[]string{fiber.MethodGet, fiber.MethodHead}` |
| DisableVaryHeaders   | `bool`                                         | Disables response `Vary` dimensions in cache lookup/storage partitioning. | `false` |
| ExpirationGenerator  | `func(fiber.Ctx, *cache.Config) time.Duration` | ExpirationGenerator allows you to generate custom expiration keys based on the request.                                                                                                                                                                                                                        | `nil`                                                            |
| Storage              | `fiber.Storage`                                | Storage is used to store the state of the middleware.                                                                                                                                                                                                                                                            | In-memory store                                                  |
| StoreResponseHeaders | `bool`                                         | StoreResponseHeaders allows you to store additional headers generated by next middlewares & handler.                                                                                                                                                                                                           | `false`                                                          |
| MaxBytes             | `uint`                                         | MaxBytes is the maximum number of bytes of response bodies simultaneously stored in cache. | `1 * 1024 * 1024` (~1 MB)                                                  |

## Default Config

```go
var ConfigDefault = Config{
    Next:         nil,
    Expiration:   5 * time.Minute,
    CacheHeader:  "X-Cache",
    DisableCacheControl: false,
    CacheInvalidator: nil,
    DisableValueRedaction: false,
    KeyGenerator: nil, // uses structured default key generator
    DisableQueryKeys: false,
    KeyHeaders: []string{
        fiber.HeaderAccept,
        fiber.HeaderAcceptEncoding,
        fiber.HeaderAcceptLanguage,
    },
    KeyCookies: nil,
    Methods: []string{fiber.MethodGet, fiber.MethodHead},
    DisableVaryHeaders: false,
    ExpirationGenerator:  nil,
    StoreResponseHeaders: false,
    Storage:              nil,
    MaxBytes:             1 * 1024 * 1024,
}
```
