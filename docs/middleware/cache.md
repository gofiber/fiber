---
id: cache
---

# Cache

Cache middleware for [Fiber](https://github.com/gofiber/fiber) designed to intercept responses and cache them. This middleware will cache the `Body`, `Content-Type` and `StatusCode` using the `c.Path()` as unique identifier. Special thanks to [@codemicro](https://github.com/codemicro/fiber-cache) for creating this middleware for Fiber core!

Request Directives<br />
`Cache-Control: no-cache` will return the up-to-date response but still caches it. You will always get a `miss` cache status.<br />
`Cache-Control: no-store` will refrain from caching. You will always get the up-to-date response.

Cacheable Status Codes<br />

This middleware caches responses with the following status codes according to RFC7231:

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

Additionally, `418: I'm a teapot` is not originally cacheable but is cached by this middleware.
If the status code is other than these, you will always get an `unreachable` cache status.

For more information about cacheable status codes or RFC7231, please refer to the following resources:

- [Cacheable - MDN Web Docs](https://developer.mozilla.org/en-US/docs/Glossary/Cacheable)

- [RFC7231 - Hypertext Transfer Protocol (HTTP/1.1): Semantics and Content](https://datatracker.ietf.org/doc/html/rfc7231)

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/cache"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
app.Use(cache.New())

// Or extend your config for customization
app.Use(cache.New(cache.Config{
    Next: func(c fiber.Ctx) bool {
        return fiber.Query[bool](c, "noCache")
    },
    Expiration: 30 * time.Minute,
    CacheControl: true,
}))
```

Or you can custom key and expire time like this:

```go
app.Use(cache.New(cache.Config{
    ExpirationGenerator: func(c fiber.Ctx, cfg *cache.Config) time.Duration {
        newCacheTime, _ := strconv.Atoi(c.GetRespHeader("Cache-Time", "600"))
        return time.Second * time.Duration(newCacheTime)
    },
    KeyGenerator: func(c fiber.Ctx) string {
        return utils.CopyString(c.Path())
    },
}))

app.Get("/", func(c fiber.Ctx) error {
    c.Response().Header.Add("Cache-Time", "6000")
    return c.SendString("hi")
})
```

You can also invalidate the cache by using the `CacheInvalidator` function as shown below:

```go
app.Use(cache.New(cache.Config{
    CacheInvalidator: func(c fiber.Ctx) bool {
        return fiber.Query[bool](c, "invalidateCache")
    },
}))
```

The `CacheInvalidator` function allows you to define custom conditions for cache invalidation. Return true if conditions such as specific query parameters or headers are met, which require the cache to be invalidated. For example, in this code, the cache is invalidated when the query parameter invalidateCache is set to true.

## Config

| Property             | Type                                           | Description                                                                                                                                                                                                                                                                                                    | Default                                                          |
| :------------------- | :--------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------- |
| Next                 | `func(fiber.Ctx) bool`                         | Next defines a function that is executed before creating the cache entry and can be used to execute the request without cache creation. If an entry already exists, it will be used. If you want to completely bypass the cache functionality in certain cases, you should use the [skip middleware](skip.md). | `nil`                                                            |
| Expiration           | `time.Duration`                                | Expiration is the time that a cached response will live.                                                                                                                                                                                                                                                       | `1 * time.Minute`                                                |
| CacheHeader          | `string`                                       | CacheHeader is the header on the response header that indicates the cache status, with the possible return values "hit," "miss," or "unreachable."                                                                                                                                                             | `X-Cache`                                                        |
| CacheControl         | `bool`                                         | CacheControl enables client-side caching if set to true.                                                                                                                                                                                                                                                       | `false`                                                          |
| CacheInvalidator     | `func(fiber.Ctx) bool`                         | CacheInvalidator defines a function that is executed before checking the cache entry. It can be used to invalidate the existing cache manually by returning true.                                                                                                                                              | `nil`                                                            |
| KeyGenerator         | `func(fiber.Ctx) string`                       | Key allows you to generate custom keys.                                                                                                                                                                                                                                                                        | `func(c fiber.Ctx) string { return utils.CopyString(c.Path()) }` |
| ExpirationGenerator  | `func(fiber.Ctx, *cache.Config) time.Duration` | ExpirationGenerator allows you to generate custom expiration keys based on the request.                                                                                                                                                                                                                        | `nil`                                                            |
| Storage              | `fiber.Storage`                                | Store is used to store the state of the middleware.                                                                                                                                                                                                                                                            | In-memory store                                                  |
| StoreResponseHeaders | `bool`                                         | StoreResponseHeaders allows you to store additional headers generated by next middlewares & handler.                                                                                                                                                                                                           | `false`                                                          |
| MaxBytes             | `uint`                                         | MaxBytes is the maximum number of bytes of response bodies simultaneously stored in cache.                                                                                                                                                                                                                     | `0` (No limit)                                                   |
| Methods              | `[]string`                                     | Methods specifies the HTTP methods to cache.                                                                                                                                                                                                                                                                   | `[]string{fiber.MethodGet, fiber.MethodHead}`                    |

## Default Config

```go
var ConfigDefault = Config{
    Next:         nil,
    Expiration:   1 * time.Minute,
    CacheHeader:  "X-Cache",
    CacheControl: false,
    CacheInvalidator: nil,
    KeyGenerator: func(c fiber.Ctx) string {
        return utils.CopyString(c.Path())
    },
    ExpirationGenerator:  nil,
    StoreResponseHeaders: false,
    Storage:              nil,
    MaxBytes:             0,
    Methods: []string{fiber.MethodGet, fiber.MethodHead},
}
```
