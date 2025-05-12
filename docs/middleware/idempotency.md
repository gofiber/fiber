---
id: idempotency
---

# Idempotency

Idempotency middleware for [Fiber](https://github.com/gofiber/fiber) allows for fault-tolerant APIs where duplicate requests—for example due to networking issues on the client-side — do not erroneously cause the same action to be performed multiple times on the server-side.

Refer to [IETF RFC 7231 §4.2.2](https://tools.ietf.org/html/rfc7231#section-4.2.2) for definitions of safe and idempotent HTTP methods.

## HTTP Method Categories

* **Safe Methods** (do not modify server state): `GET`, `HEAD`, `OPTIONS`, `TRACE`.
* **Idempotent Methods** (multiple identical requests have the same effect as a single one): all safe methods **plus** `PUT` and `DELETE`.

> According to the RFC, safe methods are guaranteed not to change server state, while idempotent methods may change state but make identical requests safe to repeat.

## Signatures

```go
func New(config ...Config) fiber.Handler
func IsFromCache(c fiber.Ctx) bool
func WasPutToCache(c fiber.Ctx) bool
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/idempotency"
)
```

After you initiate your Fiber app, you can configure the middleware:

### Default Config (Skip **Safe** Methods)

By default, the `Next` function skips middleware for safe methods only:

```go
app.Use(idempotency.New())
```

### Skip **Idempotent** Methods Instead

If you prefer to skip middleware on all idempotent methods (including `PUT`, `DELETE`), override `Next`:

```go
app.Use(idempotency.New(idempotency.Config{
    Next: func(c *fiber.Ctx) bool {
        // Skip middleware for idempotent methods (safe + PUT, DELETE)
        return fiber.IsMethodIdempotent(c.Method())
    },
}))
```

### Custom Config

```go
app.Use(idempotency.New(idempotency.Config{
    Lifetime: 42 * time.Minute,
    // ...
}))
```

## Config

| Property            | Type                    | Description                                                                                                                             | Default                                                             |
|:--------------------|:------------------------|:----------------------------------------------------------------------------------------------------------------------------------------|:--------------------------------------------------------------------|
| Next                | `func(*fiber.Ctx) bool` | Function to skip this middleware when returning `true`. Choose between `IsMethodSafe` or `IsMethodIdempotent` based on RFC definitions. | `func(c *fiber.Ctx) bool { return fiber.IsMethodSafe(c.Method()) }` |
| Lifetime            | `time.Duration`         | Maximum lifetime of an idempotency key.                                                                                                 | `30 * time.Minute`                                                  |
| KeyHeader           | `string`                | Header name containing the idempotency key.                                                                                             | `"X-Idempotency-Key"`                                               |
| KeyHeaderValidate   | `func(string) error`    | Function to validate idempotency header syntax (e.g., UUID).                                                                            | UUID length check (`36` characters)                                 |
| KeepResponseHeaders | `[]string`              | List of headers to preserve from original response.                                                                                     | `nil` (keep all headers)                                            |
| Lock                | `Locker`                | Locks an idempotency key to prevent race conditions.                                                                                    | In-memory locker                                                    |
| Storage             | `fiber.Storage`         | Stores response data by idempotency key.                                                                                                | In-memory storage                                                   |

## Default Config Values

```go
var ConfigDefault = Config{
    Next: func(c *fiber.Ctx) bool {
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

    KeepResponseHeaders: nil,

    Lock: nil, // Set in configDefault so we don't allocate data here.

    Storage: nil, // Set in configDefault so we don't allocate data here.
}
```
