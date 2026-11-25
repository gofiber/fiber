---
id: cache
---

# Cache

Cache middleware for [Fiber](https://github.com/gofiber/fiber) that intercepts responses and stores the body, `Content-Type`, and status code under a deterministic key derived from request dimensions. Special thanks to [@codemicro](https://github.com/codemicro/fiber-cache) for contributing this middleware to Fiber core.

By default, cached responses expire after five minutes and the middleware stores up to 1 MB of responses, counting each entry's body, key and stored headers. A `Config` that leaves `MaxBytes` unset gets the same limit, and only `DisableMaxBytes` removes it. A streamed response (`SendStream`, `SendStreamWriter`, `SendFile`, `static`) is judged by the length it declares instead of being read to measure it: one that declares more than `MaxBytes`, or no length at all, is passed on as a stream, is not stored and reports `unreachable`. So is a stream that ends early, fails or runs past the length it declares. Middleware that encodes a stream as it is sent leaves it without a declared length, so `compress` registered between the cache and a file handler keeps those files out of the cache: register `compress.New()` before `cache.New()`, or serve the files with `static.Config{Compress: true}`, to keep caching them.

A hit is sent a copy of the stored body rather than the stored bytes, so middleware in front of the cache can edit the response body, in place or not, without changing what later hits are sent. The copy is made on every hit, which costs time and memory in proportion to the body, about a microsecond for 30 KiB, held until the response has been written.

## Request directives

- `Cache-Control: no-cache` (or `Pragma: no-cache`) returns the latest response while still caching it, so the status is `miss` unless the new response cannot be stored.
- `Cache-Control: no-store` skips caching and always forwards a fresh response, unless the request also sends `only-if-cached`: it is then served the stored response, which `no-store` does not forbid, or answered `504`.
- `Cache-Control: max-age=<seconds>` serves the cached response only if it is no older than that, and `min-fresh=<seconds>` only if it stays fresh for at least that much longer. Otherwise the middleware revalidates: the handler runs and its response replaces the entry. `max-age=0` always revalidates.
- `Cache-Control: max-stale` accepts an expired entry, and `max-stale=<seconds>` one that expired no more than that long ago. The hit carries `Warning: 110 - "Response is stale"`. An entry stored from a response with `must-revalidate` or `proxy-revalidate` is never served stale. Nor is an entry that holds one client's response, as described under [Per-user entries](#per-user-entries-and-a-cache-in-front-of-the-app), or any entry looked up with an `Authorization` header: how long such a response may be served is for the app to say, not for the client asking. Set `AllowStaleCredentialResponses` to let `max-stale` reach those too; an invalidated entry is never served stale. The storage keeps an entry for the longer of its own lifetime and `Expiration`, counted from when it is stored, and `max-stale` can only reach an entry while the storage still holds it after it has stopped being fresh. That leaves a window when the lifetime is shorter than `Expiration`, and also when an upstream `Age` or an earlier `Date` has used up part of a lifetime that is `Expiration` or longer, because the entry stops being fresh that much before the storage drops it. A lifetime taken from `Expires` is not shortened by `Age` or `Date`, so it leaves a window only when it is shorter than `Expiration`.
- `Cache-Control: only-if-cached` answers `504 Gateway Timeout`, with the `unreachable` status, instead of calling the handler when there is no entry to serve. That includes a request that also sends `no-cache` (or `Pragma: no-cache`) or `max-age=0`: a stored response cannot be served to it without validating it, which means calling the handler. The stored entry is kept for the requests that can use it.

## Entry lifetime

The response decides how long its entry lives: `Cache-Control: s-maxage`, then `max-age`, then `Expires` set the expiration, and `Expiration` applies when the response says nothing. `ExpirationGenerator` overrides all of them. Time the response already spent in an upstream cache, reported by `Age` or implied by an earlier `Date` on the upstream response, counts against that lifetime, except a lifetime taken from `Expires`: that date is absolute, so it is not shortened. A response with no lifetime left is not stored and reports `unreachable`, for example one with `max-age=0`, an `Expires` in the past, or an `Age` as large as its lifetime. A response `max-age` or `s-maxage` that appears more than once uses its smallest valid value, and an invalid occurrence such as `max-age=abc` is ignored. RFC 9111 §4.2.1 lets a cache use the first occurrence or treat the response as stale; the smallest value is a deliberate deviation on the safe side, since it is never longer than the first. This applies to response directives only: a repeated request directive keeps its last valid value. A `max-age` or `s-maxage` above 2147483647 seconds, or too large to parse, counts as 2147483647, so a lifetime taken from either is at most about 68 years. An `Age` counts as sent, and one too large to parse uses up every lifetime. Such an `s-maxage` is still an `s-maxage`: a response that carries it is stored for a request with `Authorization`, as with any other `s-maxage`. The `max-age`, `max-stale` and `min-fresh` a request sends are compared as sent, and one too large to parse is above every lifetime.

A hit carries `X-Cache: hit` and an `Age` header with the entry's age in seconds. Its `Date` is the server's current date, not the stored one: fasthttp writes `Date` itself and ignores the value the middleware sets (`DisableDefaultDate` leaves the header out), so the stored `Date` only decides the entry's age. A hit also carries a `Warning` header when it is served stale (`110`) or when an entry older than 24 hours has no `max-age`, `s-maxage` or `Expires` of its own (`113 - "Heuristic expiration"`).

## Cacheable status codes

The middleware caches these status codes:

- `200: OK`
- `203: Non-Authoritative Information`
- `204: No Content`
- `300: Multiple Choices`
- `301: Moved Permanently`
- `308: Permanent Redirect`
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
    "strconv"
    "time"

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

A custom key is stored with `\`, `|` and `:` escaped as `\\`, `\p` and `\c`, so client text it copies, such as the `X-Tenant-ID` above, cannot spell the key of another entry's `Vary` variant.

Use `CacheInvalidator` to invalidate entries programmatically:

```go
app.Use(cache.New(cache.Config{
    CacheInvalidator: func(c fiber.Ctx) bool {
        return fiber.Query[bool](c, "invalidateCache")
    },
}))
```

`CacheInvalidator` defines custom invalidation rules. Return `true` to bypass the cache. In the example above, setting the `invalidateCache` query parameter to `true` invalidates the entry. It is called for a request that finds an entry and has not already asked for a fresh response (`max-age=0`, an unmet `min-fresh`), without any lock of the middleware held, so requests call it concurrently and it must be safe for that; a slow call delays only its own request.

Cache keys are masked in logs and error messages by default. Set `DisableValueRedaction` to `true` if you explicitly need the raw key for debugging.

### Default cache key behavior (safe by default)

By default, cache keys include:

- request method (partitioned internally by the middleware),
- request scheme and host, as `c.Scheme()` and `c.Host()` report them (so `X-Forwarded-Proto` and `X-Forwarded-Host` apply behind a trusted proxy),
- request path,
- query string, its parameters in the bytes they were sent in, sorted by name, those of one name in the order sent (enabled unless `DisableQueryKeys` is `true`),
- the rest of the request target as it was sent, when it holds more than that path and query: a path the router normalized, an absolute-form target, a `?` no query follows, or a fragment,
- representation-driving request headers (`accept`, `accept-encoding`, `accept-language`),
- a digest of the `Authorization` header, when the request carries one. The middleware adds it to every key, a custom `KeyGenerator`'s included, so a response stored for one credential is never served to another.

This prevents common collisions from path-only keys (for example, `/?id=1` vs `/?id=2`, or `/dash` on two hosts routed with `app.Domain()`) while keeping fragmentation bounded. Two requests that differ only in their authority never share an entry, so a page routed by host or a redirect built from `c.BaseURL()` is served only to the host it was generated for. If you supply a custom `KeyGenerator` and your responses depend on the host, include `c.Host()` (and `c.Scheme()`) in the key yourself.

Two requests share an entry when their queries differ only in the order of parameters with different names, such as `/?a=1&b=2` and `/?b=2&a=1`. The parameters of one name stay in the order sent, since `c.Query()` reads the first of them. Two spellings of a parameter, such as `q=%41` and `q=A`, `q=x+y` and `q=x%20y`, or `q` and `q=`, are separate entries: a handler can read the target as it was sent (`c.OriginalURL()`), so a response built from one spelling is not served for another. A query of more than 128 parameters, or longer than 4096 bytes, is keyed in the order it was sent. A handler that tells the order of the parameters apart as well needs a `KeyGenerator` of its own. One that binds the query with `c.Bind().Query()` does when two names are one field to the binder, such as `tags[]` and `tags`, or `user[name]` and `user.name`: their values are bound in the order sent, while the key sorts the two names apart.

The same holds for the rest of the target. `/a/../page`, `/%70age`, `http://example.com/page`, `/page?` and `/page#top` are all routed as `/page`, and each keeps an entry of its own, since a response that echoes one of them is not the page of a client that asked for `/page`. So does a path that middleware registered before the cache rewrote with `c.Path()`: the entry belongs to the target that was sent, and `/old` rewritten to `/new` no longer shares the entry of `/new`. A query such middleware rewrote with `c.Request().URI().SetQueryString()` is keyed as it left it and as it was sent, so two queries rewritten to one do not share an entry either, not even two that differ only in the order of their parameters: the query that was sent is then keyed in the order sent. Middleware that drops parameters so that the requests share an entry, such as tracking parameters, has to rewrite the request line itself with `c.Request().SetRequestURI()`: that line is then the target a handler reads, and the one that is keyed. Arguments it changed through `QueryArgs()` are not in the key, which holds the query that was sent: if they come from anything but the request target, such as a header, name that header in `KeyHeaders` or use a `KeyGenerator` of your own.

The default cache key holds the body of a request that carries one, with its `Content-Type` and `Content-Encoding`, so different bodies on the same URL, and the same bytes declared as another format or coding, get distinct keys. That is every `QUERY` request, as [RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html) asks for once `QUERY` is enabled via `Methods`, and any other request with a body, such as a `POST` once `Methods` names it. So is a request that names a `Content-Encoding` without having a body, since `c.Body()` decodes by it whether or not there is one: one it cannot undo, such as `compress`, makes it write `501 Not Implemented` into the response, a status that is stored, and `gzip` over nothing reads as `EOF`. Left out of the key, one such request replaced the response every other request for the URL was served. A request with neither adds nothing to its key. A `multipart/form-data` request whose form fasthttp parses while reading it, which it does for a body of declared length without a `Content-Encoding` unless `DisablePreParseMultipartForm` or `StreamRequestBody` is set, passes the cache by and reports `unreachable`: fasthttp keeps no copy of that body, and `c.Body()` writes one from the parsed form, its fields in another order each time.

Cache lookup/storage is applied only for `GET` and `HEAD` requests by default. Other HTTP methods bypass the cache middleware, though one that is not safe ends the stored responses of its path when it succeeds (see [Writes end stored responses](#writes-end-stored-responses)). You can change this via the `Methods` config field (for example, adding `fiber.MethodQuery`). If you supply a custom `KeyGenerator` and enable a body-bearing method such as `QUERY`, make sure it incorporates `c.Request().Body()` and those two headers, otherwise requests with the same URL but different bodies will collide. For a multipart form fasthttp parsed while reading it, that body is written anew on each call, so set `DisablePreParseMultipartForm` or have `Next` skip such requests. With [`StreamRequestBody`](../api/fiber.md#streamrequestbody), a request whose body is chunked or declares more than `BodyLimit` bypasses the cache (`X-Cache: unreachable`) and its body is left unread for the handler, so a key generator never reads more than `BodyLimit`; one that sends `only-if-cached` is answered `504` instead. Any other body the default key holds is read for the key, so its handler finds it in `c.Body()`, and `c.Request().BodyStream()` is `nil`; a stream that declares an empty body is left to the handler.

If a response sets `Vary`, request lookup/storage is also partitioned by those header values unless `DisableVaryHeaders` is `true`. Each response is stored under the headers its own `Vary` names, while a lookup uses the list of the response stored last for the URL: when the responses of one URL name different lists, a variant stored under another list is a miss until a response names that list again, so send the same `Vary` on every response of a URL to keep all of them reachable. Both use the header values the request carried when it reached the cache, and a response is stored only if the headers it varies on reach the handler as they came. Middleware registered after the cache that adds, rewrites or removes one of them, as `encryptcookie` does with the cookies it decrypts or drops, or as one deriving `X-User-Id` from the session cookie does, builds the page from a value the key does not hold: stored under the client's value, the page would be served to every client sending the same one, a placeholder included. Such a response is not stored and is sent with `X-Cache: unreachable`, and so is one that varies on a cookie such middleware adds beside the ones the client sent. Register the cache behind that middleware, so the lookup reads what the handler does, and have a response derived from another input vary on the input the client sends (`Vary: Cookie`) rather than on the header derived from it. With `MaxBytes` set, the URL's record of that list, its Vary manifest, is deleted by the next stored response once no entry stored under it is left, so URLs evicted to stay within the limit leave none behind. Responses with `Vary: *` remain uncacheable, and so does a response whose `Vary` names more than 32 fields or is longer than 2048 bytes. That holds when `DisableVaryHeaders` is `true` as well.

With an external `Storage`, an entry is decoded under size limits, and a response past one of them is not stored and reports `unreachable`: 256 bytes for `Content-Type` and `ETag`, 128 for `Content-Encoding` and `Expires`, 2048 for `Cache-Control`, and for the stored header list (`Location` and the [headers that restrict the body](#headers-that-restrict-the-body), or every header with `StoreResponseHeaders`) 1024 fields with 512 bytes per name and 16384 per value. The default in-memory store keeps the entry as it is and has no such limits.

### Cached redirects

`300`, `301` and `308` are cacheable statuses, so a redirect can be served from
the cache. Its `Location` is kept with the entry even when `StoreResponseHeaders` is
off, since the status means nothing without it.

`Location` joins a set the entry always carries, each in a field of its own
rather than in the stored header list: `Content-Type`, `Content-Encoding`,
`Cache-Control`, `Expires`, `ETag` and `Vary`, which a hit sends back, and
`Date` and `Age`, which only decide the `Age` a hit reports. Those are what the
entry needs to be replayed and revalidated at all. `StoreResponseHeaders` is
about every other response header, none of which is kept without it but the
ones below.

### Headers that restrict the body

A response also restricts what a client does with its body, and a hit that came
without those fields would hand the same body over unrestricted: a download sent
with `Content-Disposition: attachment` would be rendered in the page's origin,
and a page would lose its `Content-Security-Policy` or its framing rule for as
long as it was cached. They are therefore kept with every entry and sent on
every hit, each line of them, whether or not `StoreResponseHeaders` is set:
`Content-Disposition`, `Content-Security-Policy` and
`Content-Security-Policy-Report-Only`, `Cross-Origin-Embedder-Policy`,
`Cross-Origin-Opener-Policy` and their `-Report-Only` forms,
`Cross-Origin-Resource-Policy`, `Origin-Agent-Cluster`, `Permissions-Policy`,
`Referrer-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`,
`X-DNS-Prefetch-Control`, `X-Download-Options`, `X-Frame-Options`,
`X-Permitted-Cross-Domain-Policies`, `X-Robots-Tag` and `X-XSS-Protection`.
That covers what [helmet](./helmet.md) sets when it is registered behind the
cache. A hit sends the ones the response was stored with, in place of any that a
middleware in front of the cache set for the request.

A field that grants rather than restricts is not among them, the CORS fields
for one: a hit without it is refused what the first response allowed. Register
[cors](./cors.md) in front of the cache, or set `StoreResponseHeaders`, to send
them on hits.

With `StoreResponseHeaders` on, a header that middleware inside the cache wrote
for the client of the request is still left out of the entry, as `Set-Cookie`
is: it would be sent to every client that hits the entry. The session
middleware does this for the header its ID travels in, when the session ID is
read from a header (`extractors.FromHeader`). The response still reaches the
client that caused the miss with it.

### Header names

Response field names are matched case-insensitively, as
[RFC 9110 §5.1](https://www.rfc-editor.org/rfc/rfc9110.html#section-5.1)
requires, so a handler is free to write `cache-control` or `expires` in lower
case under
[`DisableHeaderNormalizing`](../api/fiber.md#config). Those values decide what
they would decide either way, and the fields this middleware writes itself —
`Cache-Control`, `Age`, `Date`, `ETag`, `Expires`, `Vary` — replace whatever
spelling the response already carries rather than being added beside it. For
`Date`, fasthttp writes one header line with the server's current date.

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

A request the server refused while reading it, such as one declaring a body over `BodyLimit`, is neither served from the cache nor stored, and `X-Cache` reads `unreachable`. Fiber runs the `Use` chain for such a request without its route (see [Errors Before a Handler Runs](../guide/error-handling.md#errors-before-a-handler-runs)), so what the chain produces is not the URL's response.

When revalidation returns `private`, `no-cache`, `no-store`, or `Vary: *`, the middleware removes the previous cached entry and its Vary manifest. A concurrently replaced entry keeps its manifest. Cleanup errors are logged and the origin response is still returned; if storage fails during cleanup, the old entry can still be served until it expires, and it keeps counting toward `MaxBytes` until a later eviction retries the delete. A plain miss answered with `no-store` preserves the shared Vary manifest, so existing sibling variants remain reachable.

This cleanup applies to those four directives on a response with a [cacheable status code](#cacheable-status-codes). Any other status, such as `503` or `429`, leaves the stored entry and its Vary manifest in place whether or not it carries one of them, so an error page does not evict a healthy entry.

`401`, `403` and `407` are the exception. They are the origin refusing the request rather than failing, so a revalidation they answer ends the entry it found, when the refusal is a verdict on a credential the entry belongs to; otherwise the next request with the same key would be served the stored response without the origin being asked again. Ending it is this middleware's choice: [RFC 9111 §4.3.3](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.3.3) only has the refusal, like any full response, answer the request that revalidated in place of the stored response, and [§4.4](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.4) requires an invalidation only after an unsafe method such as `POST`. A handler that refuses by returning `fiber.ErrUnauthorized`, another `*fiber.Error` with one of these codes, or an error wrapping one counts the same as a handler that writes the status. An error that only your `ErrorHandler` maps to a status is not recognized, and neither is any other way of ending a session, such as a redirect to a login page. A request that finds an entry expired, or one `CacheInvalidator` invalidated, deletes it and goes to the origin, and counts the same. A refusal that found no entry changes nothing.

An entry belongs to a credential when the key holds it, that is an `Authorization` header, a cookie named by `KeyCookies`, a header named by `KeyCredentialHeaders`, or one of `Cookie`, `Proxy-Authorization`, `X-API-Key`, `X-Auth-Token` and `X-Access-Token` when `KeyHeaders` names it, or when the response's `Vary` names a header that carries it and the request sends that header. Any other refusal leaves the entry, whatever directives it carries, since they are about the refusal: a client that the origin turns away behind the cache for reasons of its own, such as an IP or geo rule, or one that sends a credential the key does not hold, is no verdict on the entry the others share, and a reload cannot end it. If a middleware does the turning away, register it before this one, and its refusal never reaches the cache. A custom `KeyGenerator`'s key is opaque to the middleware, so there the refusal has to come from a request that sends `Authorization`, which is in every key, or a header named in `KeyCredentialHeaders`, which is taken to be in the key: name every credential header your key holds, `Cookie` included when it holds a cookie. When the key holds the credential, its Vary manifest goes with the entry, so its other variants become unreachable too, and so does what a concurrent request stored under the key meanwhile, which may predate the refusal. A manifest the sessions share, as with `Vary: Cookie`, stays whatever directives the refusal carries: it is the other sessions' as well, and the record described next keeps the refused credential's variants out.

The credential's other entries of the URL, the ones the key tells apart by a header such as `Accept` (the default `KeyHeaders`) or that hang on a manifest the sessions share, as with `Vary: Cookie`, are reached through a record of the refusal: they are revalidated rather than served, for at least `Expiration`, as long as the refused entry had left, and until the longest-lived credential entry this instance has stored in the URL's group stops being kept, so none of them outlives the record. Whatever other sessions do to the URL meanwhile does not bring them back. `GET` and `HEAD` answer for the same resource and credential, so they share the record: a refusal answering one revalidates the other's entries too. The record goes as soon as the origin answers that credential again with a `2xx` or a `304`, stored or not, so a refusal that no longer stands costs one more trip to the origin and no more. A redirect does not end it: an app sends an ended session to its login page that way, and no other `3xx` can be told from one. It is written only for a credential that has an entry, and it is bookkeeping like the Vary manifest, which `MaxBytes` does not count. The in-memory store keeps no more records than one per 256 bytes of `MaxBytes` (never fewer than 1024, and 16384 with `DisableMaxBytes`) and drops the oldest first; an external `Storage` expires them on its own. A record dropped while it is still live is remembered by a hash of its key, at a fraction of the room, and that credential's hits are still revalidated until it would have lapsed; four times as many are remembered that way as the store keeps records, the oldest making room for the next. Refusing more credentials than fit therefore costs no other session a hit, and serves a refused credential's variants again only once that many more have been refused since, until they expire. A hit looks for a record only when its key or the entry's `Vary` can hold a credential, and only while one may exist for its URL: the in-memory store, which only this middleware writes to, is not read before the first record of the URL's group (one of 64, by path) and after the last has expired, and the instances sharing an external `Storage` announce a record with a marker for all URLs and one for its group, each read at most four times a second, so a refusal reaches the others within a quarter of a second and only the hits of its group look for it. A storage that fails a read counts as a refusal, and the entry is revalidated; one that cannot take the write leaves the record with this instance, which remembers it as it does a dropped one, while the others, which learn of a record from the `Storage`, do not hear of it. With a custom `KeyGenerator`, `Authorization` and the headers named in `KeyCredentialHeaders` are taken to be in its key, so the record follows them whatever the variants vary on; a credential that only your key holds is reached by the entry it revalidated alone, and `CacheInvalidator` ends the rest.

Nothing else in `KeyHeaders` is taken for a credential, since nothing in a header's name tells a credential from a tenant or a language. An entry keyed on such a header is shared by every client that sends the same value, so a refusal leaves it, and its Vary manifest. If such a header does carry a credential, name it in `KeyCredentialHeaders` and a refusal ends its entries, its hits are sent as `private`, it is not served stale, and its value is hashed in the key. With a custom `KeyGenerator` the header takes no part in the key, which is yours, but the middleware cannot tell which entries hold the credential, so a hit for a request that sends the header is sent as `private` and is not served stale, while one for a request that does not is treated as any other:

```go
app.Use(cache.New(cache.Config{
    KeyCredentialHeaders: []string{"X-Session"},
}))
```

The separate storage rules for `Set-Cookie` and requests carrying `Authorization` still apply.

A route that genuinely wants both a cookie and a shared entry can say so with
`Cache-Control: public` or an `s-maxage` — directives only a shared cache acts
on, so neither is written by accident. `must-revalidate` and `proxy-revalidate`
do **not** lift the cookie restriction, even though RFC 9111 §3.5 accepts
`must-revalidate` for the `Authorization` case: that allowance holds because a
revalidating cache returns to the origin and the origin re-checks the
credential, and this middleware does not check back with the origin while an
entry is fresh — it serves the stored body until the entry expires.

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
`err := c.Next(); c.Cookie(...); return err`. [EncryptCookie](./encryptcookie.md)
only rewrites the cookies a response already sets, so the cache sees them in the
clear either way round; it also decrypts the request's cookies, and a response
that varies on `Cookie` is stored only when the cache is registered **behind**
it (see `Vary` above). Note also that a response
personalized from a **request** cookie without setting one is cached and shared
by design — use `KeyCookies` or `Vary: Cookie` to key those apart. A hit on an
entry keyed that way is sent with `Cache-Control: private`, as described below.

:::

### Writes end stored responses

A request that is not safe, which is any method but `GET`, `HEAD`, `OPTIONS`, `TRACE` and `QUERY`, ends the responses stored for its path once the app answers it with a `2xx` or `3xx` status, as [RFC 9111 §4.4](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.4) requires of a cache. A `GET` was served what the path held before a `PUT`, a `POST` or a `DELETE` until its entry expired. Every entry of the host and the path goes, whatever its query string, whether it was stored for a `GET` or a `HEAD`, whichever variant of the response's `Vary` or credential it belongs to, and with a custom `KeyGenerator` too, as the path is read from the request and not from the key. That also means an entry is ended for the requests of the host and path the write was sent to: a custom key that leaves out the host, or gives several paths one entry, shares that entry with requests for another host or path, and those are served it until it expires or a request for the written host and path finds it. End such entries with `CacheInvalidator`. The next request goes to the handler, and so does one that sends `Cache-Control: max-stale`, while `only-if-cached` is answered `504`. A write that `timeout.New` gives up on ends them too, whatever it is answered, since its handler may still complete the write; a response stored while that handler goes on running is not ended by it. A request that began before the write succeeded may have read what it changed, so the entry it stores while the write goes through is ended as well, and one stored after is not.

- **A write ends the path it was sent to.** Others the app changes with it, such as the collection a `POST` adds to or the resource named in the `Location` it answers with, are yours to end with `CacheInvalidator`.
- **A method named in `Methods` ends nothing.** It is treated as the read it was enabled for, as `QUERY` is, and its responses are stored.
- **`Next` skips the invalidation too**: a request it returns `true` for ends nothing.
- **An ended entry makes room first.** It is not deleted at once: it stays stored until a request for it finds it ended or, under `MaxBytes`, until room is needed, when it goes before every entry a request can still be served, an expired one kept for `max-stale` included. With `DisableMaxBytes` it stays until a request finds it or it expires.
- **The instance that handled the write remembers it, for as long as it runs.** Another instance sharing the `Storage` goes on serving what it finds until the entry expires, and a restart forgets the writes. The writes are kept in a table of fixed size, indexed by a keyed hash of the host and path, so paths that share a slot may end each other's entries early, and a client cannot make it grow or choose a path that shares the slot of another. The table has 16384 slots, so under steady writes that is no rare event: after writes succeed at `n` distinct paths while an entry lives, it ends early with a chance of about `1 - e^(-n/16384)` although nothing wrote to its path, about 6% after 1000 such paths and half after 11000. Writes repeated at one path take up one slot. The path is taken without regard to case or trailing slashes, as the router takes `/Item` and `/item/` for `/item` unless `CaseSensitive` or `StrictRouting` is set, and the host without its port, so a write sent to `example.com:443` ends the entries of `example.com`, those of its other ports included. A write and an entry are ordered by a clock that reads the wall time once, when the middleware is created, and adds the time elapsed since by the monotonic clock, so a step of the wall clock afterwards, as NTP may make, changes the order of none. An entry another instance stored in a shared `Storage` is judged by that instance's clock, which runs apart from this one by as much as their wall clocks did when each started.

### A Storage that fails

A `Storage` that returns an error does not fail the request. The handler answers it, the failure is logged as a warning, with the key as `[redacted]` unless `DisableValueRedaction` is set, and `X-Cache` reads `unreachable` whenever the response was neither served from the cache nor stored:

- **A lookup that fails**, for the entry or for the Vary manifest of its URL, sends the request to the handler, and the response is not stored.
- **An entry that cannot be decoded**, because this version of the middleware did not write it or the backend returned only part of it, is a miss, and the response stored for the request replaces it. Until a response is stored for its key, each request finds it again, as a miss, and logs it again.
- **A body that cannot be read**, or one the backend dropped while it kept the entry, makes the entry a miss as well.
- **A response that cannot be stored** is sent as the handler produced it: a write fails, the entries that have to make room for it under `MaxBytes` cannot be deleted, or the Vary manifest of its URL cannot be written or, for a response that no longer varies, deleted. What a failed write left behind is deleted again, and an entry that could not be evicted stays counted, so a later eviction retries it.
- **An entry that cannot be deleted** when a request finds it expired, or otherwise not to be served, stays where it is. The request is answered by the handler, whose response replaces the entry if it is stored, and each later request judges the entry again.

`Cache-Control: only-if-cached` forbids calling the handler, so such a request is answered `504 Gateway Timeout` in each of these cases, as it is when nothing is stored. A request whose context has ended, as that of one `timeout.New` gave up on, is not handed to the handler when a lookup, or the read of the body of the entry it found, fails: the `Storage`'s error goes back to what ended it. The record of a refused credential (see [Responses that are never stored](#responses-that-are-never-stored)) that cannot be read still counts as a refusal, which has the entry revalidated rather than served.

While the `Storage` fails, every request reaches the handler, so the app carries the load the cache was taking from it.

### Per-user entries and a cache in front of the app

A hit is sent with the stored response's `Vary`, whether or not
`StoreResponseHeaders` is set, so a cache in front of the app that honors `Vary`
keys the hit the way this middleware did.

When the entry carries no `Cache-Control` of its own and `DisableCacheControl`
is `false`, a hit is sent with `Cache-Control: public, max-age=<remaining>`. An
entry keyed by a cookie or credential holds one user's response, so such a hit
is sent with `Cache-Control: private, max-age=<remaining>` instead: a shared
cache in front of the app does not store it, while the user's own browser still
may. So is a hit for a request that carries a body, `QUERY` aside, as described
below. The entry counts as keyed by a credential when the response's `Vary` lists
`Authorization`, `Cookie`, `Proxy-Authorization`, `X-API-Key`, `X-Auth-Token`,
`X-Access-Token` or a header named in `KeyCredentialHeaders`, when `KeyCookies` or
`KeyCredentialHeaders` is set, or when `KeyHeaders` names one of those headers.
`DisableVaryHeaders` does not change this: a response whose `Vary` lists `Cookie`
is one user's whether or not this middleware partitions on it.

A hit that is given this `Cache-Control` also names, in its `Vary`, the request
headers the key tells entries apart by, after the names the stored `Vary` has:
the headers of `KeyHeaders`, which are `Accept`, `Accept-Encoding` and
`Accept-Language` by default, `Cookie` for `KeyCookies`, and those of
`KeyCredentialHeaders`. Without them, a cache in front of the app that was told
it may store the hit would keep one copy for the URL and send it to every client,
whichever language, format or tenant it asked for. A hit for a request whose body
the key holds, which is one that carries a body, a `QUERY`, or one that names a
`Content-Encoding`, names `Content-Encoding` and `Content-Type` as well. No `Vary`
can name the body itself, so the hit of a request that carries one is sent
`private`, except for `QUERY`, which a cache that serves it keys by its content
([RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html)). A custom
`KeyGenerator`'s key is opaque, so it names the headers of `KeyCredentialHeaders`
alone, and as that key may hold the body, the hit of a request that carries one,
by its `Content-Length` or chunked framing, is sent `private` all the same,
`QUERY` aside. A hit that keeps a `Cache-Control` of the handler's own keeps the `Vary`
the handler chose, and with `DisableCacheControl` nothing is added.

A handler that sends its own `Cache-Control` keeps it on hits. On a per-user
route that says `max-age` without `private`, add `private` yourself if a cache
in front of the app ignores `Vary`. A custom `KeyGenerator` that keys on a
cookie or credential is not visible to this middleware either: send
`Cache-Control: private` from those handlers, set `DisableCacheControl`, or
name the header in `KeyCredentialHeaders`, which marks the hits of the requests
that send it.

:::caution A cached entry outlives the session that produced it

A hit is answered without asking the app, so a session the app has ended keeps
reading what was stored under it until the entry expires or a request for it
reaches the app and is refused. A client can force that with
`Cache-Control: no-cache` or `max-age=0`; when the app answers `401`, `403` or
`407`, the entry is removed, as described above. Once its lifetime ends, an entry
keyed by a credential is not served at all, not even to a request that sends
`Cache-Control: max-stale`, unless you set `AllowStaleCredentialResponses`.

To stop serving a session the moment the app ends it, authenticate in a
middleware registered ahead of the cache, or let `CacheInvalidator` say when a
session has ended. An invalidated entry is not served, whatever
`Cache-Control: max-stale` the client sends:

```go
app.Use(cache.New(cache.Config{
    KeyCookies: []string{"session"},
    CacheInvalidator: func(c fiber.Ctx) bool {
        // Runs for requests that find an entry, so keep the lookup cheap.
        // Requests call it concurrently, so the set it reads must allow that.
        return sessionEnded(c.Cookies("session")) // your own lookup of ended sessions
    },
}))
```

Keep `Expiration` short on per-user routes.

:::

### Vary and `Content-Type`

A request `Content-Type` naming a form is folded to lower case before it is used
in the cache key, so the key is the same whether it is built before or after the
handler runs — the form accessors fold that header in place, and without this
the entry would be stored under a key no lookup produces.

A handler on such a route therefore sees the folded media type even if it never
reads a form value: on every request when `KeyHeaders` names `Content-Type`, and
from the second request on when only `Vary` does. The boundary keeps its case,
and a non-form `Content-Type` is left alone. A `multipart/form-data` request is
keyed at all only where fasthttp keeps its body as sent, as described under the
default key above.

## Config

| Property             | Type                                           | Description                                                                                                                                                                                                                                                                                                    | Default                                                          |
| :------------------- | :--------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------------------------------------------------------- |
| Next                 | `func(fiber.Ctx) bool`                         | Next defines a function to skip this middleware when it returns `true`. It is asked twice: before an entry is looked up, where `true` passes the request on without the cache, so it is not served a stored response, and again before the response is stored, where `true` leaves it unstored. Only the second time has the handler run: the first time the response is the empty one a request starts with, and `c.Route()` is the route the cache is registered on, so a `Next` that decides by the response or the route has to return `false` for that, or no request reaches the cache. It is asked of a request that is not safe as well, which ends the stored responses of its path once it succeeds unless `Next` returns `true` for it. | `nil`                                                            |
| Expiration           | `time.Duration`                                | Expiration is the time that a cached response will live when neither the response nor `ExpirationGenerator` sets a lifetime. A value shorter than one second in magnitude, zero included, selects the default, and `-1 * time.Second` or lower turns the middleware into a pass-through. | `5 * time.Minute`                                                |
| CacheHeader          | `string`                                       | CacheHeader is the header on the response header that indicates the cache status, with the possible return values "hit," "miss," or "unreachable."                                                                                                                                                             | `X-Cache`                                                        |
| DisableCacheControl  | `bool`                                          | DisableCacheControl omits the `Cache-Control` header when set to `true`. When `false`, a hit whose entry has no `Cache-Control` of its own is sent with `public, max-age=<remaining>`, or `private, max-age=<remaining>` when the entry is keyed by a cookie or credential, or by the body of a request other than `QUERY`, which with a custom `KeyGenerator` is taken to be any request other than `QUERY` that carries a body. | `false`                                                         |
| AllowStaleCredentialResponses | `bool` | Lets `Cache-Control: max-stale` reach an expired entry that holds one client's response: one keyed or varied by a cookie or credential header, or looked up with `Authorization`. By default such an entry is never served stale, since the storage keeps it past its lifetime and the client could read what a credential the app has since ended once got. An invalidated entry is never served stale, whatever this says. | `false` |
| CacheInvalidator     | `func(fiber.Ctx) bool`                         | CacheInvalidator defines a function that is executed before checking the cache entry. It can be used to invalidate the existing cache manually by returning true. | `nil`                                                            |
| DisableValueRedaction | `bool`                                        | Turns off cache key redaction in logs and error messages when set to `true`. | `false`                                             |
| KeyGenerator         | `func(fiber.Ctx) string`                       | KeyGenerator allows you to generate custom keys. The HTTP method, a key-format version and, when the request carries `Authorization`, a digest of that header are partitioned internally by the middleware. | structured key from scheme + host + path + sorted query + selected headers/cookies |
| DisableQueryKeys     | `bool`                                         | Leaves the query string out of keys. | `false` |
| KeyHeaders           | `[]string`                                     | Header allow-list used for key partitioning. Names are normalized case-insensitively and sorted. Use `[]string{}` to disable header-based partitioning. | `[]string{"accept","accept-encoding","accept-language"}` |
| KeyCookies           | `[]string`                                     | Optional cookie allow-list for key partitioning. Explicit opt-in only; names remain case-sensitive. Hits on entries keyed this way are sent with `Cache-Control: private`. The values in the key are hashed, so an external `Storage` does not show them; a cookie that is missing and one that is empty share a key. | `nil` |
| KeyCredentialHeaders | `[]string`                                     | Request headers that carry a client's credential, such as a token in a header of your own. They partition entries like `KeyHeaders`; hits on entries keyed this way are sent with `Cache-Control: private` and are not served stale unless `AllowStaleCredentialResponses` is set, and a revalidation the origin refuses with `401`, `403` or `407` ends the entry and the credential's other variants of the URL. The values of the credential headers in the key are hashed, so an external `Storage` does not show them. Names are normalized case-insensitively and sorted. With a custom `KeyGenerator` they take no part in the key, but a hit for a request that sends one is still sent as `private` and not served stale, since the middleware cannot tell which entries hold it; name every credential header its key holds, as a refusal ends its entries only for a request that sends `Authorization` or one of them. | `nil` |
| Methods              | `[]string`                                     | HTTP methods eligible for caching. Requests whose method is not in this list bypass the cache and are answered with `unreachable` in the cache header; one that is not safe ends the stored responses of its path once it succeeds, and a method named here ends nothing. Names are normalized to uppercase. `nil` selects the default, and an empty slice disables caching for every method. | `[]string{fiber.MethodGet, fiber.MethodHead}` |
| DisableVaryHeaders   | `bool`                                         | Disables response `Vary` dimensions in cache lookup/storage partitioning. | `false` |
| ExpirationGenerator  | `func(fiber.Ctx, *cache.Config) time.Duration` | ExpirationGenerator allows you to generate a custom expiration per request. It overrides `Expiration` and any lifetime the response sets with `Cache-Control` or `Expires`, Lifetimes are counted in whole seconds: a duration of zero or less leaves the response uncached, and one under a second never produces a hit.                                                                                                                                                                                                                        | `nil`                                                            |
| Storage              | `fiber.Storage`                                | Storage is used to store the state of the middleware. Entries are namespaced by a key-format version, so an external store that survives an upgrade starts cold rather than serving entries an older version partitioned by different rules. Every key starts with that version, so the storage can be shared with other middleware. A storage that returns an error does not fail the request: see [A Storage that fails](#a-storage-that-fails).                                                                                                                                                                                                                                                            | In-memory store                                                  |
| StoreResponseHeaders | `bool`                                         | StoreResponseHeaders allows you to store additional headers generated by next middlewares & handler. The [headers that restrict the body](#headers-that-restrict-the-body) are stored without it. Connection-scoped headers, `Proxy-Connection` and the ones the response names in `Connection` among them, the `Proxy-Authenticate`, `Proxy-Authentication-Info` and `Proxy-Authorization` of a proxy, `Set-Cookie` and the header a [session](./session.md) writes its ID into are never stored, since a cache entry is replayed to every client that matches its key. `Vary` is kept with every entry and replayed on hits regardless of this setting. | `false`                                                          |
| MaxBytes             | `uint`                                         | MaxBytes is the maximum number of bytes simultaneously stored in cache, counting each entry's body, its key and the headers it stores. When the limit is reached, the entries closest to expiring are evicted to make room, the one stored first going first among those that expire together, and a response larger than the limit is never stored. An entry's lifetime counts as no more than `Expiration` from when it was stored, so a response with a longer lifetime, such as an asset sent with a year-long `max-age`, does not outlast the entries stored after it: a client asking for that route under query strings of its own making cannot keep the other entries out of the cache for as long as they live, and only evicts, in the order they were stored, what it has stored itself and what came before it. Raise `Expiration` to rank long lifetimes higher. An entry that has expired, and is kept only for a request that sends `max-stale`, goes before any that is fresh, and an entry a write ended (see [Writes end stored responses](#writes-end-stored-responses)) goes before both, as no request can be served it. The number of entries is limited along with it, to `MaxBytes / 256` but no fewer than 1024, since an entry also takes memory for bookkeeping that is not counted, which outweighs a short or empty body such as that of a `204`. A streamed response is measured by its declared length, not read, and one that declares none, such as a file `compress` encodes, is not stored even with `DisableMaxBytes`. Responses that other requests are still storing count toward the limit, and one that does not fit beside them is served without being stored and reports `unreachable`. `0` means the default. | `1 * 1024 * 1024` (~1 MB) |
| DisableMaxBytes      | `bool`                                         | Stores responses without the `MaxBytes` limit, however large and however many. Every distinct key a client asks for is then held until it expires, and a stream that declares its length is read into memory to be stored whatever that length is, so set it only where something else bounds what is cached. | `false` |

## Default Config

```go
var ConfigDefault = Config{
    Next:         nil,
    Expiration:   5 * time.Minute,
    CacheHeader:  "X-Cache",
    DisableCacheControl: false,
    AllowStaleCredentialResponses: false,
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
    KeyCredentialHeaders: nil,
    Methods: []string{fiber.MethodGet, fiber.MethodHead},
    DisableVaryHeaders: false,
    ExpirationGenerator:  nil,
    StoreResponseHeaders: false,
    Storage:              nil,
    MaxBytes:             1 * 1024 * 1024,
    DisableMaxBytes:      false,
}
```
