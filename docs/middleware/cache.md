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
- `300: Multiple Choices`
- `301: Moved Permanently`
- `404: Not Found`
- `405: Method Not Allowed`
- `410: Gone`
- `414: URI Too Long`
- `501: Not Implemented`

Responses with other status codes result in an `unreachable` cache status. A `206 Partial Content` response is never stored either: the middleware does not understand range requests, so a stored partial body would be replayed to clients asking for the whole representation (RFC 9111 §3.3).

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

The middleware **does not include request body/form values in the default cache key**, except for `QUERY` requests: per [RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html), when `QUERY` is enabled via `Methods` the default key generator incorporates the request body so different bodies on the same URL get distinct keys.

Cache lookup/storage is applied only for `GET` and `HEAD` requests by default. Other HTTP methods bypass the cache middleware. You can change this via the `Methods` config field (for example, adding `fiber.MethodQuery`). If you supply a custom `KeyGenerator` and enable a body-bearing method such as `QUERY`, make sure it incorporates `c.Request().Body()`, otherwise requests with the same URL but different bodies will collide.

If a response sets `Vary`, request lookup/storage is also partitioned by those header values unless `DisableVaryHeaders` is `true`. Responses with `Vary: *` remain uncacheable.

### Cached redirects

`300` and `301` are cacheable statuses, so a redirect can be served from the
cache. Its `Location` is kept with the entry even when `StoreResponseHeaders` is
off, since the status means nothing without it.

`Location` joins a set the entry always carries, each in a field of its own
rather than in the stored header list: `Content-Type`, `Content-Encoding`,
`Cache-Control`, `Expires`, `ETag`, `Date` and `Age`. Those are what the entry
needs to be replayed and revalidated at all. `StoreResponseHeaders` is about
every other response header, none of which is kept without it.

### Header names

Response field names are matched case-insensitively, as
[RFC 9110 §5.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-5.1)
requires, so a handler is free to write `cache-control` or `expires` in lower
case under
[`DisableHeaderNormalizing`](../api/fiber.md#config). Those values decide what
they would decide either way, and the fields this middleware writes itself —
`Cache-Control`, `Age`, `Date`, `ETag`, `Expires` — replace whatever spelling
the response already carries rather than being added beside it.

### Responses that are never stored

One entry is served to every client whose request matches its key, so a response
that identifies a single client is not stored at all:

- **A response that sets a cookie.** `Set-Cookie` means the response has been
  personalized for the client that caused the miss, and the body — not just the
  header — is what would be replayed to everyone else. The response still
  reaches that client normally; only the entry is skipped, and `X-Cache` reads
  `unreachable`.
- **A response to a request carrying `Authorization`,** unless the response
  permits shared caching, per
  [RFC 9111 §3.5](https://www.rfc-editor.org/rfc/rfc9111.html#section-3.5).
- **`Cache-Control: no-store`, `private`, `no-cache`, or `Vary: *`.**

A route that genuinely wants both a cookie and a shared entry can say so with
`Cache-Control: public` or an `s-maxage` — directives only a shared cache acts
on, so neither is written by accident. `must-revalidate` and `proxy-revalidate`
do **not** lift the cookie restriction, even though RFC 9111 §3.5 accepts
`must-revalidate` for the `Authorization` case: that allowance holds because a
revalidating cache returns to the origin and the origin re-checks the
credential, and this middleware never revalidates — it serves the stored body
for the whole configured `Expiration`.

Applications that refresh a session cookie on every response will find that most
routes stop being cached — that is the point, since those responses are
per-client. Set the cookie only where it changes, or mark the genuinely public
routes `public`.

:::caution Register the cache outside any middleware that writes cookies

The check above reads the response as it stands when the cache decides to store
it, which is the moment `c.Next()` returns to the cache middleware. A middleware
registered **outside** the cache does its post-`c.Next()` work later, so a cookie
it writes then is not there to be seen — the entry is stored and the next client
reads the first one's body.

Fiber's own [session](./session.md) middleware writes its cookie exactly that
way, so the order matters:

```go
app.Use(cache.New())    // correct: the cookie is written before the cache decides
app.Use(session.New())

app.Use(session.New())  // leaks: the cookie is written after the cache stored
app.Use(cache.New())
```

The same applies to any handler wrapper of the form
`err := c.Next(); c.Cookie(...); return err`. Note also that a response
personalized from a **request** cookie without setting one is cached and shared
by design — use `KeyCookies` or `Vary: Cookie` to key those apart.

:::

### Vary and `Content-Type`

A request `Content-Type` naming a form is folded to lower case before it is used
in the cache key, so the key is the same whether it is built before or after the
handler runs — the form accessors fold that header in place, and without this
the entry would be stored under a key no lookup produces.

A handler on such a route therefore sees the folded media type even if it never
reads a form value: on every request when `KeyHeaders` names `Content-Type`, and
from the second request on when only `Vary` does. The boundary keeps its case,
and a non-form `Content-Type` is left alone.

## Config

| Property             | Type                                           | Description                                                                                                                                                                                                                                                                                                    | Default                                                          |
| :------------------- | :--------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------- |
| Next                 | `func(fiber.Ctx) bool`                         | Next defines a function that is executed before creating the cache entry and can be used to execute the request without cache creation. If an entry already exists, it will be used. If you want to completely bypass the cache functionality in certain cases, you should use the [skip middleware](skip.md). | `nil`                                                            |
| Expiration           | `time.Duration`                                | Expiration is the time that a cached response will live. | `5 * time.Minute`                                                |
| CacheHeader          | `string`                                       | CacheHeader is the header on the response header that indicates the cache status, with the possible return values "hit," "miss," or "unreachable."                                                                                                                                                             | `X-Cache`                                                        |
| DisableCacheControl  | `bool`                                          | DisableCacheControl omits the `Cache-Control` header when set to `true`. | `false`                                                         |
| CacheInvalidator     | `func(fiber.Ctx) bool`                         | CacheInvalidator defines a function that is executed before checking the cache entry. It can be used to invalidate the existing cache manually by returning true. | `nil`                                                            |
| DisableValueRedaction | `bool`                                        | Turns off cache key redaction in logs and error messages when set to `true`. | `false`                                             |
| KeyGenerator         | `func(fiber.Ctx) string`                       | KeyGenerator allows you to generate custom keys. The HTTP method and a key-format version are partitioned internally by the middleware. | structured key from path + canonical query + selected headers/cookies |
| DisableQueryKeys     | `bool`                                         | Disables canonicalized query params in keys. | `false` |
| KeyHeaders           | `[]string`                                     | Header allow-list used for key partitioning. Names are normalized case-insensitively and sorted. Use `[]string{}` to disable header-based partitioning. | `[]string{"accept","accept-encoding","accept-language"}` |
| KeyCookies           | `[]string`                                     | Optional cookie allow-list for key partitioning. Explicit opt-in only; names remain case-sensitive. | `nil` |
| Methods              | `[]string`                                     | HTTP methods eligible for caching. Requests whose method is not in this list bypass the cache. Names are normalized to uppercase. | `[]string{fiber.MethodGet, fiber.MethodHead}` |
| DisableVaryHeaders   | `bool`                                         | Disables response `Vary` dimensions in cache lookup/storage partitioning. | `false` |
| ExpirationGenerator  | `func(fiber.Ctx, *cache.Config) time.Duration` | ExpirationGenerator allows you to generate custom expiration keys based on the request.                                                                                                                                                                                                                        | `nil`                                                            |
| Storage              | `fiber.Storage`                                | Storage is used to store the state of the middleware. Entries are namespaced by a key-format version, so an external store that survives an upgrade starts cold rather than serving entries an older version partitioned by different rules.                                                                                                                                                                                                                                                            | In-memory store                                                  |
| StoreResponseHeaders | `bool`                                         | StoreResponseHeaders allows you to store additional headers generated by next middlewares & handler. Connection-scoped headers and `Set-Cookie` are never stored, since a cache entry is replayed to every client that matches its key.                                                                          | `false`                                                          |
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
