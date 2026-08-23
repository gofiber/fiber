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
      RuleList: []redirect.Rule{
        {From: "/old", To: "/new"},
        {From: "/old/*", To: "/new/$1"},
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
| RuleList   | `[]Rule`              | Rules tried in order, first match wins; `$1`, `$2` insert params. | Required |
| Rules      | `map[string]string`   | **Deprecated.** Use `RuleList`. A map has no order, so precedence is decided by a heuristic. | nil |
| StatusCode | `int`                 | HTTP code for redirects.                  | 302 Temporary Redirect |

A `Rule` is a pattern and a target:

```go
type Rule struct {
    From string // the path pattern, where "*" matches a run of any length
    To   string // the target, where "$1", "$2" stand for what the asterisks captured
}
```

Setting both `RuleList` and `Rules` panics: the two disagree about what decides
precedence, so there is no sensible way to honour them together.

## Rule order

Rules are tried in the order you write them and the first one whose `From`
matches answers, exactly as [routes](../guide/routing.md) are matched. Put the
specific rules before the catch-alls:

```go
RuleList: []redirect.Rule{
    {From: "/api/users", To: "/v2/users"},  // checked first
    {From: "/api/*", To: "/v2/$1"},         // takes everything else
}
```

Written the other way round, `/api/*` answers `/api/users` too and the second
rule never fires. Fiber logs a warning at startup naming any rule an earlier one
shadows outright, so a dead rule tells you rather than going quiet.

The deprecated `Rules` map has no order of its own, so Fiber sorts it: most path
text pinned before the first `*` wins, then most path text overall, then fewest
asterisks, then the key itself. That covers rules written with path text and
`*`. Rules relying on regular-expression syntax beyond `*` may order differently
Use `RuleList`, where the order is yours.

## How captures are placed

The values a rule captures come from the request path, so a `$N` may stand in
the path, the query or the fragment of a target, but never in its host.

**A target with no host of its own**, `"/api/*": "/$1"`, always redirects within
this application. The composed location is read the way a browser reads it, then
held to this origin: a leading run of slashes collapses to one, and a capture
that turned the location into an absolute URL is rooted as a path. Without that,
`/api//evil.com` would send `Location: //evil.com`, which the browser follows to
evil.com.

**A target naming its own host**, `"/ext/*": "https://cdn.example.com/$1"`,
redirects wherever it says. A `$N` after the host may hold anything, including
slashes; it can only add to the path.

:::caution

**A `$N` inside the host is refused.** Fiber logs a warning and the rule never
fires, so `"https://$1.cdn.example.com/"`, `"https://cdn.example.com:$1"` and
`"https://$1"` all do nothing. Whether a value is safe inside a host depends on
percent-decoding, IDNA mapping, numeric labels read as IPv4 addresses, IPv6
brackets and userinfo, and each of those was a way to move the host somewhere
the author did not write. Write the host in full and capture only what follows
it.

If you need per-tenant destinations, pick the host in a handler and use
`c.Redirect()`, where the value is yours to validate.

:::

## Default Config

```go
var ConfigDefault = Config{
    StatusCode: fiber.StatusFound,
}
```
