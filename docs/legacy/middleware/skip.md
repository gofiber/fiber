---
id: skip
---

# Skip

The Skip middleware wraps a handler and bypasses it when the predicate returns `true` for the current request.

## Signatures

```go
func New(handler fiber.Handler, exclude func(c fiber.Ctx) bool) fiber.Handler
```

## Examples

Import the package:

```go
import (
    "log"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/skip"
)
```

`skip.New` accepts the handler to wrap and a predicate function. The predicate
runs for every request, and returning `true` skips the wrapped handler and
executes the next middleware in the chain.

After you initialize your Fiber app, use `skip.New` like this:

```go
func main() {
    app := fiber.New()

    app.Use(skip.New(BasicHandler, func(ctx fiber.Ctx) bool {
        return ctx.Method() == fiber.MethodGet
    }))

    app.Get("/", func(ctx fiber.Ctx) error {
        return ctx.SendString("It was a GET request!")
    })

    log.Fatal(app.Listen(":3000"))
}

func BasicHandler(ctx fiber.Ctx) error {
    return ctx.SendString("It was not a GET request!")
}
```

:::tip
`app.Use` processes requests on any route and method. In the example above, the handler is skipped only for `GET`.
:::
