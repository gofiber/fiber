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

A target that hands the whole host to the request — `"/go/*": "https://$1"`,
`"//$1"`, or `"https:$1"` — is an open redirect: anyone able to shape the path
chooses where the client lands. **Such a rule never fires**, and Fiber logs a
warning naming it at startup, since there is no way to tell the intended host
from an attacker's. Pin the host in the target instead.

A capture hands over the host whenever nothing beside it in the authority is
host text, so these are the same case and are refused too:

| Target | Why it pins no host |
| --- | --- |
| `"https://$1:8080"`, `"https://$1:$2"` | A port is not a host — `evil.com:8080` is still `evil.com`. |
| `"//$1."`, `"//.$1"` | `"///evil.com."` is read host-first by the URL parser; `evil.com.` is `evil.com` with the DNS root spelled out. |
| `"https://$1$2"` | A second capture pins nothing an attacker does not also supply. |
| `"https://example.com@$1"` | Everything before an `@` is a username, so the host is still the capture's. |

Note that `"https://$1@example.com"` is fine — there the author's host follows
the `@` and the capture is only userinfo — as are `"https://cdn.example.com:$1"`
and `"https://tenant-$1.example.com"`.

A refused rule is dropped, so the request continues to the **remaining rules of
this middleware** before reaching the rest of the stack. With
`{"/go/*": "https://$1", "/*": "/home"}`, a request for `/go/evil.com` is
redirected to `/home` by the catch-all.

This check covers the schemes a client navigates by. A scheme-only target in
some other scheme, `"/go/*": "ws:$1"`, still composes `ws://evil.com` from a
captured `//evil.com`, and is neither refused nor warned about — such a
`Location` is not followed as a top-level navigation, but do not write one.

:::

## Default Config

```go
var ConfigDefault = Config{
    StatusCode: fiber.StatusFound,
}
```
