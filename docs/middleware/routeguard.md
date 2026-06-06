---
id: routeguard
---

# RouteGuard

RouteGuard middleware for [Fiber](https://github.com/gofiber/fiber) validates incoming requests against registered routes before they reach the middleware chain. Unmatched routes are rejected immediately with a 404 response.

RouteGuard respects Fiber's `CaseSensitive` and `StrictRouting` configuration options.

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

| Property     | Type                   | Description                                                    | Default          |
| :----------- | :--------------------- | :------------------------------------------------------------- | :--------------- |
| Next         | `func(fiber.Ctx) bool` | Defines a function to skip this middleware when returned true. | `nil`            |
| ErrorHandler | `fiber.Handler`        | Custom handler for unmatched routes.                           | Returns 404 JSON |

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
goos: linux
goarch: amd64
pkg: github.com/gofiber/fiber/v3/middleware/routeguard
cpu: AMD Ryzen 7 9700X 8-Core Processor
BenchmarkTrieLookup-16                     11993240               103.9 ns/op             0 B/op          0 allocs/op
BenchmarkTrieMiss-16                       23933238                50.66 ns/op            0 B/op          0 allocs/op
BenchmarkStaticShort-16                    37706788                31.27 ns/op            0 B/op          0 allocs/op
BenchmarkStaticDeep-16                     17711882                66.27 ns/op            0 B/op          0 allocs/op
BenchmarkRootPath-16                       60888026                19.63 ns/op            0 B/op          0 allocs/op
BenchmarkSingleParam-16                    15804783                74.96 ns/op            0 B/op          0 allocs/op
BenchmarkMultipleParams-16                 10212600               119.7 ns/op             0 B/op          0 allocs/op
BenchmarkTripleParams-16                   10914847               108.5 ns/op             0 B/op          0 allocs/op
BenchmarkWildcardShort-16                  24468232                48.26 ns/op            0 B/op          0 allocs/op
BenchmarkWildcardDeep-16                   33341594                35.45 ns/op            0 B/op          0 allocs/op
BenchmarkNestedWildcard-16                 20657456                57.51 ns/op            0 B/op          0 allocs/op
BenchmarkStaticVsParamPriority-16          18075568                69.50 ns/op            0 B/op          0 allocs/op
BenchmarkHeadFallback-16                   16402279                74.89 ns/op            0 B/op          0 allocs/op
BenchmarkMethodVariation-16                35615319                33.65 ns/op            0 B/op          0 allocs/op
BenchmarkLongPath-16                       19078210                63.70 ns/op            0 B/op          0 allocs/op
BenchmarkEarlyMiss-16                      45598905                26.25 ns/op            0 B/op          0 allocs/op
BenchmarkLateMiss-16                       14966178                81.96 ns/op            0 B/op          0 allocs/op
PASS
ok         github.com/gofiber/fiber/v3/middleware/routeguard       21.570s)
```
