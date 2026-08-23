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
| RuleList | `[]Rule`          | Rules tried in order, first match wins; `$1`, `$2` insert wildcard captures. | Required |
| Rules    | `map[string]string`   | **Deprecated.** Use `RuleList`. A map has no order, so precedence is decided by a heuristic. | nil |

A rule's `From` is path text: `*` matches a run of any bytes but a newline, and
every other byte stands for itself, so `/preis-1.000-euro` rewrites that path
and not `/preis-1X000-euro`. There is no escape for a literal `*`.

Put the specific rules before the catch-alls. Setting both `RuleList` and
`Rules` panics.

:::note
`Rules` keeps working for the whole of v3. Its keys are ranked by a heuristic
rather than by the author: most path text before the first `*`, then most path
text overall, then fewest asterisks, then the key itself. That is a rough
reading of "most specific", not an exact one, so a long wildcard rule can still
outrank a shorter exact one. `RuleList` gives exact control.
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
