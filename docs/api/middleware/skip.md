---
id: skip
---

# Skip

Skip middleware for [Fiber](https://github.com/gofiber/fiber) that skips a wrapped handler if a predicate is true.

## Signatures
```go
func New(handler fiber.Handler, exclude func(c *fiber.Ctx) bool) fiber.Handler
```

## Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/skip"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
func main() {
	app := fiber.New()

	app.Use(skip.New(BasicHandler, func(ctx *fiber.Ctx) bool {
		return ctx.Method() == fiber.MethodGet
	}))

	app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendString("It was a GET request!")
	})

	log.Fatal(app.Listen(":3000"))
}

func BasicHandler(ctx *fiber.Ctx) error {
	return ctx.SendString("It was not a GET request!")
}
```

:::tip
app.Use will handle requests from any route, and any method. In the example above, it will only skip if the method is GET.
:::
