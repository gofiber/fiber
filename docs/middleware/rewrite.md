---
id: rewrite
---

# Rewrite

The Rewrite middleware remaps the request path using custom rules, helping with backward compatibility and cleaner URLs. Rules are tried in the order you write them and the first match wins.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Config

| Property | Type                  | Description                                           | Default    |
|:---------|:----------------------|:------------------------------------------------------|:-----------|
| Next     | `func(fiber.Ctx) bool` | Skip when function returns `true`.                    | `nil`      |
| RuleList | `[]Rule`          | Rules tried in order, first match wins; `$1`, `$2` insert wildcard captures. | nil |
| Rules    | `map[string]string`   | **Deprecated.** Use `RuleList`. A map has no order, so precedence is decided by a heuristic. | nil |

Set one of `RuleList` or `Rules`. With neither, the middleware rewrites nothing.

A rule's `From` is path text: `*` matches a run of any bytes but a newline, and
every other byte stands for itself, so `/preis-1.000-euro` rewrites that path
and not `/preis-1X000-euro`. There is no escape for a literal `*`.

Setting both `RuleList` and `Rules` panics.

## Rule order

Rules are tried in the order you write them and the first one whose `From`
matches answers, exactly as [routes](../guide/routing.md) are matched. Nothing
reorders a `RuleList`. Put the specific rules before the catch-alls:

```go
RuleList: []rewrite.Rule{
    {From: "/api/users", To: "/v2/users"},  // checked first
    {From: "/api/*", To: "/v2/$1"},         // takes everything else
}
```

Written the other way round, `/api/*` answers `/api/users` too and the second
rule never fires.

:::note
The deprecated `Rules` map has no order of its own, so Fiber sorts it: most path
text before the first `*` wins, then most path text overall, then fewest
asterisks, then the key itself. That is a rough reading of "most specific", not
an exact one, so a long wildcard rule can still outrank a shorter exact one.
`RuleList` is the exact control.
:::

## Examples

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/rewrite"
)

func main() {
    app := fiber.New()

    app.Use(rewrite.New(rewrite.Config{
      RuleList: []rewrite.Rule{
        {From: "/old", To: "/new"},
        {From: "/old/*", To: "/new/$1"},
      },
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
