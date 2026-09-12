---
id: redirect
---

# Redirect

Redirect middleware maps old URLs to new ones using simple rules. A `RuleList`
is tried in the order you write it and the first match wins; the deprecated
`Rules` map has no order of its own and is ranked by a heuristic.

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
| RuleList   | `[]Rule`              | Rules tried in order, first match wins; `$1`, `$2` insert wildcard captures. | nil |
| Rules      | `map[string]string`   | **Deprecated.** Use `RuleList`. A map has no order, so precedence is decided by a heuristic. | nil |
| StatusCode | `int`                 | HTTP code for redirects.                  | 302 Temporary Redirect |

A `Rule` is a pattern and a target:

```go
type Rule struct {
    From string // the path pattern, where "*" matches a run of any length
    To   string // the target, where "$1", "$2" stand for what the asterisks captured
}
```

`From` reaches the compiled pattern as a regular expression, so the other regexp
metacharacters keep their meaning: `/a.b` also matches `/aXb`, and `/faq?` also
matches `/fa`. Only `*` is a documented placeholder; write a rule out of path
text and `*` and it behaves as it reads.

Setting both `RuleList` and `Rules` panics: the two disagree about what decides
precedence, so there is no sensible way to honour them together.

## Rule order

Rules are tried in the order you write them and the first one whose `From`
matches answers, exactly as [routes](../guide/routing.md) are matched. Nothing
reorders a `RuleList`. Put the specific rules before the catch-alls:

```go
RuleList: []redirect.Rule{
    {From: "/api/users", To: "/v2/users"},  // checked first
    {From: "/api/*", To: "/v2/$1"},         // takes everything else
}
```

Written the other way round, `/api/*` answers `/api/users` too and the second
rule never fires. For lists of 100 rules or fewer, Fiber logs a warning at startup naming any rule
an earlier one shadows outright, so a dead rule tells you rather than going
quiet.

The deprecated `Rules` map has no order of its own, so Fiber sorts it: most path
text pinned before the first `*` or other regular-expression character wins,
then most path text overall, then fewest asterisks, then the key itself. That
covers rules written with path text and `*`. Rules relying on
regular-expression syntax beyond `*` may order differently. Use `RuleList`,
where the order is yours.

## How captures are placed

The values a rule captures come from the request path, so a `$N` may stand in
the path, the query or the fragment of a target, but never in its authority.

**A target with no scheme and no authority**, `{From: "/api/*", To: "/$1"}`,
always redirects within this application. The composed location is read the way
a browser reads it, then held to this origin: a leading run of slashes collapses
to one, and a capture that turned the location into an absolute URL is rooted as
a path. Without that, `/api//evil.com` would send `Location: //evil.com`, which
the browser follows to evil.com.

**A target naming its own authority**, `{From: "/ext/*", To: "https://cdn.example.com/$1"}`,
redirects wherever it says. A `$N` after the authority may hold anything,
including slashes; it can only add to the path.

**A target naming a scheme but no authority**, `{From: "/m/*", To: "myapp:$1"}`,
also leaves this application, so the rooting above does not apply to it. A
capture there may not open an authority of its own: a value that writes the
`//` starting one is refused for that request.

:::caution

**A `$N` inside the authority is refused.** Fiber logs a warning and the rule never
fires, so `"https://$1.cdn.example.com/"`, `"https://cdn.example.com:$1"` and
`"https://$1"` all do nothing. That covers the host, the port and any userinfo.
Whether a value is safe in there depends on percent-decoding, IDNA mapping,
numeric labels read as IPv4 addresses, IPv6 brackets and userinfo, and each of
those was a way to move the host somewhere the author did not write. Write the
authority in full and capture only what follows it.

If you need per-tenant destinations, pick the host in a handler and use
`c.Redirect()`, where the value is yours to validate.

:::

## What the location carries

The request's query string is appended to every `Location`, so `/old?a=1`
redirects to `/new?a=1`, and a query the target already carries is merged rather
than replaced. A trailing slash on the request path is trimmed before matching,
so `/old/` is answered by the rule for `/old`.

## Default Config

```go
var ConfigDefault = Config{
    StatusCode: fiber.StatusFound,
}
```
