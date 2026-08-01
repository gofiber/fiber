---
id: redirect
---

# Redirect

Redirect middleware maps old URLs to new ones using simple rules.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/redirect"
)

func main() {
    app := fiber.New()

    app.Use(redirect.New(redirect.Config{
      Rules: map[string]string{
        "/old":   "/new",
        "/old/*": "/new/$1",
      },
      StatusCode: fiber.StatusMovedPermanently,
    }))

    app.Get("/new", func(c fiber.Ctx) error {
      return c.SendString("Hello, World!")
    })
    app.Get("/new/*", func(c fiber.Ctx) error {
      return c.SendString("Wildcard: " + c.Params("*"))
    })

    app.Listen(":3000")
}
```

## Test

```bash
curl http://localhost:3000/old
curl http://localhost:3000/old/hello
```

## Config

| Property   | Type                | Description                               | Default                |
|:-----------|:--------------------|:------------------------------------------|:-----------------------|
| Next       | `func(fiber.Ctx) bool` | Skip when function returns true.          | nil                    |
| Rules      | `map[string]string`   | Map paths to new ones; `$1`, `$2` insert params. | Required               |
| StatusCode | `int`                 | HTTP code for redirects.                  | 302 Temporary Redirect |

## How captures are placed

The values a rule captures come from the request path, so where a `$N` sits in
the target decides what it is allowed to contribute.

**A target with no host of its own** — `"/api/*": "/$1"` — always redirects
within this application. The composed location is read the way a browser reads
it, then held to this origin: a leading run of slashes collapses to one, and a
capture that turned the location into an absolute URL is rooted as a path.
Without that, `/api//evil.com` would send `Location: //evil.com`, which the
browser follows to evil.com.

**A target naming its own host** — `"/ext/*": "https://cdn.example.com/$1"` —
redirects wherever it says. A `$N` after the host may hold anything, including
slashes; it can only add to the path.

**A target with a `$N` inside its host** — `"/cdn/*": "https://$1.cdn.example.com/"` —
means the capture to be a label. The rule is skipped, and the request falls
through to the rest of the stack, whenever the captured value would move the
host instead: one containing `/`, `\`, `?`, `#`, `@` or `:` in that position, or
one that extends the host where the target ends in `$N`
(`"https://cdn.example.com$1"` accepts `/foo.png`, which starts a path, but not
`foo.png` or `@evil.com`).

:::caution

A target that is nothing but a capture — `"/go/*": "https://$1"` — hands the
destination to the request, which is an open redirect: anyone able to shape the
path chooses where the client lands. Fiber logs a warning for such a rule at
startup and otherwise leaves it alone, since there is no way to tell the
intended host from an attacker's. Pin the host in the target instead.

:::

## Default Config

```go
var ConfigDefault = Config{
    StatusCode: fiber.StatusFound,
}
```
