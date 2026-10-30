---
id: idempotency
---

# Idempotency

The Idempotency middleware helps build fault-tolerant APIs. Duplicate requests—such as retries after network issues—won't trigger the same action twice on the server.

Refer to [IETF RFC 7231 §4.2.2](https://tools.ietf.org/html/rfc7231#section-4.2.2) for definitions of safe and idempotent HTTP methods.

## HTTP Method Categories

* **Safe Methods** (do not modify server state): `GET`, `HEAD`, `OPTIONS`, `TRACE`, `QUERY`
* **Idempotent Methods** (identical requests have the same effect as a single one): all safe methods **plus** `PUT` and `DELETE`

> According to the RFC, safe methods never change server state, while idempotent methods may change state but remain safe to repeat.

## Signatures

```go
func New(config ...Config) fiber.Handler
func IsFromCache(c fiber.Ctx) bool
func WasPutToCache(c fiber.Ctx) bool
func NewMemoryLock() *MemoryLock

type Locker interface {
    Lock(key string) error
    Unlock(key string) error
}
```

`IsFromCache` reports whether the middleware replayed a stored response for the current request, and `WasPutToCache` whether it stored the response of the current request. Both are set as the middleware returns, so read them from a middleware registered before it, after `c.Next()`.

`NewMemoryLock` returns the in-memory `Locker` the middleware uses by default. It only locks within the current process; implement `Locker` yourself to coordinate several instances. The middleware calls `Lock` before it runs the handler and `Unlock` once the request is done, so `Lock` has to block until the caller holds the key.

## Behavior

* A request passes through untouched when `Next` returns `true` or when it has no `KeyHeader` value. The header is optional: nothing rejects a request that omits it.
* The first request with a key runs the handler. If the handler returns without an error, its status, body and headers are stored under the key for `Lifetime`, whatever the status code. A handler that returns an error is not stored, so the next request with that key runs the handler again.
* A streamed response that declares no length (`SendStreamWriter`, or `SendStream` without a size) is sent as it is produced and not stored, since storing it would mean reading it to its end first. The next request with that key runs the handler again. A stream of known length, such as `SendFile` serves, is read and stored like any other response when it is no longer than 1 MiB and delivers that length without an error; otherwise it is sent as read and not stored.
* Cookies are not stored: `Set-Cookie` and the obsolete `Set-Cookie2` are left out of the stored response unless `KeepResponseHeaders` names them. The client that caused the miss still receives them, and a replay does not, so a retry after a lost response does not set the cookie. A stored entry is replayed through the same filter, so one that holds a cookie, written by an earlier version or under a configuration that named it, is replayed without it.
* Later requests with the same key receive the stored response and the handler does not run. A request that arrives while the first is still running waits for `Lock` and then receives the stored response.
* The key alone identifies a request, unless `KeyScope` is set. The method, path and body are not compared, so a retry with the same key but a different payload, or on another route, gets the original response, and so does anyone else who sends the key.
* With `KeyScope`, a response is stored under the scope and the key together. A request replays only what was stored in its own scope, and a request in another scope that sends the same key runs the handler and stores a response of its own.
* A key that fails `KeyHeaderValidate` is returned as an error to the error handler, which answers `500` unless the validator returns a `*fiber.Error`.
* A request the server refused while reading it, such as one declaring a body over `BodyLimit`, runs no handler, so nothing is stored under its key and the retry with that key runs the handler.

:::caution Treat a key as a secret
Whoever sends a stored key receives the response stored for it, whatever the user, route, method or body of their request: its status, its body and every header the config keeps. The default validator checks only the length, so `00000000-0000-0000-0000-000000000000` passes. Have clients draw keys from a cryptographically secure random source, such as a version 4 UUID, and never derive one from a counter, a timestamp or an identifier like a user ID that another client can guess. When responses differ per user, set `KeyScope` to something that identifies the caller, so that a key someone else sent or guessed returns nothing stored for you. If you name a cookie in `KeepResponseHeaders`, scope the keys too: an unscoped replay would hand that cookie to whoever sends the key. Only the cookies are left out by default. A session or token carried in another response header, such as the session ID the session middleware writes to the header it reads it from, is stored and replayed like any header: list the headers to replay in `KeepResponseHeaders` so that it is left out, or scope the keys.
:::

## Examples

Import the middleware package:

```go
import (
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/basicauth"
    "github.com/gofiber/fiber/v3/middleware/idempotency"
)
```

Once your Fiber app is initialized, configure the middleware:

### Default Config (Skip **Safe** Methods)

By default, the `Next` function skips middleware for safe methods only:

```go
app.Use(idempotency.New())
```

### Skip **Idempotent** Methods Instead

Skip all idempotent methods (including `PUT` and `DELETE`) by overriding `Next`:

```go
app.Use(idempotency.New(idempotency.Config{
    Next: func(c fiber.Ctx) bool {
        // Skip middleware for idempotent methods (safe + PUT, DELETE)
        return fiber.IsMethodIdempotent(c.Method())
    },
}))
```

### Scope Keys by User

`KeyScope` returns the scope a key belongs to. Register the middleware that authenticates the request before this one, so that the user is known by the time the scope is asked for. A response is then stored, replayed and locked per user, and a key another user sent returns nothing stored for you:

```go
app.Use(basicauth.New(basicauth.Config{
    Users: users,
}))

app.Use(idempotency.New(idempotency.Config{
    KeyScope: func(c fiber.Ctx) string {
        return basicauth.UsernameFromContext(c)
    },
}))
```

The scope and the key are hashed together into the key the storage sees, so a user ID never appears in it and a long scope fits a storage that limits the length of its keys. Requests for which the function returns the same string share a scope, the empty string included, so return something for every request that has a key. Scoped and unscoped entries share one keyspace, so give an instance with `KeyScope` a storage of its own, or the same `KeyScope` to every instance that shares the storage.

### Keep a Cookie

A cookie is stored and replayed only when `KeepResponseHeaders` names it. Name the headers you want to replay, and scope the keys:

```go
app.Use(idempotency.New(idempotency.Config{
    KeepResponseHeaders: []string{fiber.HeaderContentType, fiber.HeaderSetCookie},
    KeyScope: func(c fiber.Ctx) string {
        return basicauth.UsernameFromContext(c)
    },
}))
```

A route that issues the cookie, such as a login, has no authenticated user yet, so a scope taken from the session or the credentials is the same empty string for every caller there, and the kept cookie would replay to whoever sends the key. Scope such a route by an identity the request itself carries, such as the account name in the login form, or skip the middleware on it with `Next`.

### Custom Config

```go
app.Use(idempotency.New(idempotency.Config{
    Lifetime: 42 * time.Minute,
    // ...
}))
```

## Config

Idempotency keys are hidden in logs and error messages by default. Set `DisableValueRedaction` to `true` only when you need to expose them for debugging.

| Property            | Type                   | Description                                                                                                                             | Default                                                            |
|:--------------------|:-----------------------|:----------------------------------------------------------------------------------------------------------------------------------------|:-------------------------------------------------------------------|
| Next                | `func(fiber.Ctx) bool` | Function to skip this middleware when it returns `true`; use `IsMethodSafe` or `IsMethodIdempotent`. | `func(c fiber.Ctx) bool { return fiber.IsMethodSafe(c.Method()) }` |
| Lifetime            | `time.Duration`        | Maximum lifetime of an idempotency key.                                                                                                 | `30 * time.Minute`                                                 |
| KeyHeader           | `string`               | Header name containing the idempotency key.                                                                                             | `"X-Idempotency-Key"`                                              |
| KeyHeaderValidate   | `func(string) error`   | Function to validate idempotency header syntax (e.g., UUID). The error it returns is passed to the error handler as is; the default one wraps `ErrInvalidIdempotencyKey`. | UUID length check (`36` characters)                                |
| KeyScope            | `func(fiber.Ctx) string` | Returns the scope a key belongs to, such as the ID of the authenticated user. A response is stored and replayed per scope and key, so a key another user sent or guessed returns nothing stored for you. Requests for which it returns the same string share a scope, the empty one included. | `nil` (the key alone identifies a request)                         |
| KeepResponseHeaders | `[]string`             | List of headers to preserve from original response, matched case-insensitively. An empty list keeps all headers, like `nil`, but for `Set-Cookie` and `Set-Cookie2`, which are kept only when named. | `nil` (keep all headers but the cookies)                           |
| DisableValueRedaction | `bool`                | Disables idempotency key redaction in logs and error messages. | `false`                                              |
| Lock                | `Locker`               | Locks an idempotency key to prevent race conditions. The default only locks within this process. | `NewMemoryLock()` (in-memory locker)                               |
| Storage             | `fiber.Storage`        | Stores response data by idempotency key. An entry is stored under `idempotency:` followed by the key, or by its digest with `KeyScope`, so the storage can be shared with other middleware. Responses stored by an earlier version are not replayed after the upgrade, so a retry with such a key runs the handler again, and the old entries expire on their own. The default store is private to this process. | In-memory storage                                                  |

## Default Config Values

```go
var ConfigDefault = Config{
    Next: func(c fiber.Ctx) bool {
        // Skip middleware for safe methods per RFC 7231 §4.2.2
        return fiber.IsMethodSafe(c.Method())
    },

    Lifetime: 30 * time.Minute,

    KeyHeader: "X-Idempotency-Key",
    KeyHeaderValidate: func(k string) error {
        if l, wl := len(k), 36; l != wl { // UUID length is 36 chars
            return fmt.Errorf("%w: invalid length: %d != %d", ErrInvalidIdempotencyKey, l, wl)
        }

        return nil
    },

    KeyScope: nil,

    KeepResponseHeaders: nil,

    Lock: nil, // Set in configDefault so we don't allocate data here.

    Storage: nil, // Set in configDefault so we don't allocate data here.
    DisableValueRedaction: false,
}
```
