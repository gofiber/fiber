---
id: expvar
---

# ExpVar

The ExpVar middleware exposes runtime variables over HTTP in JSON. Using it (e.g., `app.Use(expvarmw.New())`) registers handlers on `/debug/vars`.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "expvar"
    "fmt"

    "github.com/gofiber/fiber/v3"
    expvarmw "github.com/gofiber/fiber/v3/middleware/expvar"
)
```

Once your Fiber app is initialized, use the middleware as shown:

```go
var count = expvar.NewInt("count")

app.Use(expvarmw.New())
app.Get("/", func(c fiber.Ctx) error {
    count.Add(1)

    return c.SendString(fmt.Sprintf("hello expvar count %d", count.Value()))
})
```

Visit `/debug/vars` to see all variables, and append `?r=<regexp>` to keep only the variables whose names match the regular expression; an invalid expression is answered with `400 Bad Request`. Any path beneath `/debug/vars/` is redirected to `/debug/vars` with `303 See Other`, and every other path continues down the handler chain.

```bash
curl 127.0.0.1:3000
hello expvar count 1

curl 127.0.0.1:3000/debug/vars
{
    "cmdline": ["xxx"],
    "count": 1,
    "expvarHandlerCalls": 33,
    "expvarRegexpErrors": 0,
    "memstats": {...}
}

curl 127.0.0.1:3000/debug/vars?r=c
{
    "cmdline": ["xxx"],
    "count": 1
}
```

## Config

| Property | Type                    | Description                                                         | Default |
|:---------|:------------------------|:--------------------------------------------------------------------|:--------|
| Next     | `func(fiber.Ctx) bool` | Next defines a function to skip this middleware when it returns true. | `nil`   |

## Default Config

```go
var ConfigDefault = Config{
    Next: nil,
}
```
