# Coregex Adapter for Fiber

This package provides an adapter to use [coregex](https://github.com/coregx/coregex), a high-performance regex engine, with Fiber's routing system.

## Features

- Drop-in replacement for Go's standard `regexp` package
- 3-3000x performance improvement in many use cases
- Zero-allocation iterators for pattern matching
- SIMD prefilters for fast candidate rejection
- O(n) time complexity guarantee (no ReDoS vulnerabilities)

## Installation

```bash
go get github.com/coregx/coregex
```

## Usage

```go
package main

import (
    "log"
    "github.com/gofiber/fiber/v3"
    "github.com/coregx/coregex"
)

// CoregexEngine implements fiber.RegexEngine using coregex
type CoregexEngine struct{}

func (CoregexEngine) MustCompile(pattern string) fiber.RegexCompiler {
    return &CoregexCompiler{
        Regex: coregex.MustCompile(pattern),
    }
}

// CoregexCompiler implements fiber.RegexCompiler using coregex.Regex
type CoregexCompiler struct {
    *coregex.Regex
}

func main() {
    app := fiber.New(fiber.Config{
        RegexEngine: CoregexEngine{},
    })

    // Routes with regex constraints will now use coregex
    app.Get("/api/v1/:id<regex(\\d+)>", func(c fiber.Ctx) error {
        return c.SendString("ID: " + c.Params("id"))
    })

    log.Fatal(app.Listen(":3000"))
}
```

## Performance Benefits

Coregex excels at:
- Multi-pattern matching (`foo|bar|baz`)
- Suffix patterns (`.*\.log`, `.*\.txt`)
- Inner literals (`.*error.*`, `.*@example\.com`)
- IP/phone patterns with digit prefiltering
- Multiline patterns (`(?m)^/.*\.php`)

## Compatibility

The adapter maintains full compatibility with Fiber's existing regex constraint syntax:

```go
// Single constraint
app.Get("/:id<regex(\\d+)>", handler)

// Multiple constraints
app.Get("/:param<int;max(3000)>", handler)

// Complex patterns
app.Get("/date/:date<regex(\\d{4}-\\d{2}-\\d{2})>", handler)
```

## Notes

- Coregex requires Go 1.25+
- The adapter automatically handles all regex operations including `MatchString` and `FindAllStringSubmatch`
- Falls back gracefully if the pattern cannot be compiled
