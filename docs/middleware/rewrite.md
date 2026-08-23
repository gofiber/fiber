---
id: rewrite
---

# Rewrite

The Rewrite middleware remaps the request path using custom rules, helping with backward compatibility and cleaner URLs.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Config

| Property | Type                  | Description                                           | Default    |
|:---------|:----------------------|:------------------------------------------------------|:-----------|
| Next     | `func(fiber.Ctx) bool` | Skip when function returns `true`.                    | `nil`      |
| RuleList | `[]Rule`              | Rules tried in order; the first match wins. Use `$1`, `$2` for wildcard captures.| (Required) |
| Rules    | `map[string]string`   | Deprecated. Same rules as a map, ranked most specific first.| `nil`      |

A rule's `From` is path text: `*` matches a run of any length and every other
byte stands for itself, so `/preis-1.000-euro` rewrites that path and not
`/preis-1X000-euro`.

Put the specific rules before the catch-alls. Setting both `RuleList` and
`Rules` panics.

:::note
`Rules` is deprecated because a map has no order, so which rule answered a path
two rules both matched was decided by map iteration, which Go randomizes per
run. It keeps working for the whole of v3: its keys are ranked most path text
before the first `*`, then most path text overall, then fewest asterisks, then
the key itself.
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
