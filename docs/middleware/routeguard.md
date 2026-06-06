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

```text
goos: linux
goarch: amd64
pkg: github.com/gofiber/fiber/v3/middleware/routeguard
cpu: AMD Ryzen 7 9700X 8-Core Processor
BenchmarkTrieLookup-16               	12654866	        97.82 ns/op	       0 B/op	       0 allocs/op
BenchmarkTrieMiss-16                 	24624105	        50.00 ns/op	       0 B/op	       0 allocs/op
BenchmarkStaticShort-16              	38189272	        31.32 ns/op	       0 B/op	       0 allocs/op
BenchmarkStaticDeep-16               	18053446	        66.68 ns/op	       0 B/op	       0 allocs/op
BenchmarkRootPath-16                 	60027408	        19.61 ns/op	       0 B/op	       0 allocs/op
BenchmarkSingleParam-16              	16610433	        74.17 ns/op	       0 B/op	       0 allocs/op
BenchmarkMultipleParams-16           	 9953649	       118.8 ns/op	       0 B/op	       0 allocs/op
BenchmarkTripleParams-16             	11538085	       105.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkWildcardShort-16            	25320096	        48.02 ns/op	       0 B/op	       0 allocs/op
BenchmarkWildcardDeep-16             	33551408	        35.39 ns/op	       0 B/op	       0 allocs/op
BenchmarkNestedWildcard-16           	20941983	        58.10 ns/op	       0 B/op	       0 allocs/op
BenchmarkStaticVsParamPriority-16    	18148852	        67.55 ns/op	       0 B/op	       0 allocs/op
BenchmarkHeadFallback-16             	16708828	        73.61 ns/op	       0 B/op	       0 allocs/op
BenchmarkMethodVariation-16          	35123863	        33.92 ns/op	       0 B/op	       0 allocs/op
BenchmarkLongPath-16                 	18396319	        65.12 ns/op	       0 B/op	       0 allocs/op
BenchmarkEarlyMiss-16                	45314115	        26.26 ns/op	       0 B/op	       0 allocs/op
BenchmarkLateMiss-16                 	14878971	        83.03 ns/op	       0 B/op	       0 allocs/op
PASS
ok  	github.com/gofiber/fiber/v3/middleware/routeguard	21.652s
```
