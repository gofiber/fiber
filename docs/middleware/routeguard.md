---
id: routeguard
---

# RouteGuard

RouteGuard middleware for [Fiber](https://github.com/gofiber/fiber) validates incoming requests against registered routes before they reach the middleware chain. Unmatched routes are rejected immediately with a 404 response.

## Signatures

```go
func New(config ...Config) fiber.Handler
func Build(app *fiber.App)
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/routeguard"
)
```

### Basic Usage

```go
app := fiber.New()

// Add RouteGuard as the first middleware
app.Use(routeguard.New())

app.Get("/api/users", func(c fiber.Ctx) error {
    return c.SendString("Users")
})
app.Get("/api/users/:id", func(c fiber.Ctx) error {
    return c.SendString("User: " + c.Params("id"))
})

// Build must be called after all routes are registered
routeguard.Build(app)

app.Listen(":3000")
```

### Custom Error Handler

```go
app.Use(routeguard.New(routeguard.Config{
    ErrorHandler: func(c fiber.Ctx) error {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "endpoint not found",
        })
    },
}))
```

### Skip Middleware

```go
app.Use(routeguard.New(routeguard.Config{
    Next: func(c fiber.Ctx) bool {
        return c.Path() == "/health"
    },
}))
```

## Config

| Property | Type | Description | Default |
|:---------|:-----|:------------|:--------|
| Next | `func(fiber.Ctx) bool` | Defines a function to skip this middleware when returned true. | `nil` |
| ErrorHandler | `fiber.Handler` | Custom handler for unmatched routes. | Returns 404 JSON |

## Default Config

```go
var ConfigDefault = Config{
    Next:         nil,
    ErrorHandler: defaultErrorHandler,
}

func defaultErrorHandler(c fiber.Ctx) error {
    return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
        "error": "not found",
    })
}
```

## Benchmarks

RouteGuard uses a trie-based lookup with zero allocations:

```
BenchmarkRootPath-16              65475970    18.30 ns/op    0 B/op    0 allocs/op
BenchmarkStaticShort-16           48553028    24.82 ns/op    0 B/op    0 allocs/op
BenchmarkStaticDeep-16            24663532    50.09 ns/op    0 B/op    0 allocs/op
BenchmarkSingleParam-16           22362229    53.24 ns/op    0 B/op    0 allocs/op
BenchmarkMultipleParams-16        16619850    72.28 ns/op    0 B/op    0 allocs/op
BenchmarkWildcardShort-16         39936240    30.16 ns/op    0 B/op    0 allocs/op
BenchmarkWildcardDeep-16          42687034    28.26 ns/op    0 B/op    0 allocs/op
BenchmarkEarlyMiss-16             53622060    22.34 ns/op    0 B/op    0 allocs/op
BenchmarkLateMiss-16              21642680    55.49 ns/op    0 B/op    0 allocs/op
```
