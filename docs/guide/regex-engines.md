---
id: regex-engines
title: 🔎 Alternative Regex Engines
description: >-
  Configure Fiber's regex() route constraints to use an alternative regex
  compiler such as coregex.
sidebar_position: 8
toc_max_heading_level: 4
---

Fiber v3+ lets you configure the compiler used for `regex()` parameter
constraints through the `RegexHandler` option. This allows you to use
high-performance regex engines like
[coregex](https://github.com/coregx/coregex) as a drop-in replacement for Go's
standard library `regexp` package when compiling those constraint patterns.

## Why Use an Alternative Regex Engine?

The default Go `regexp` package is intentionally simple and guarantees O(n) time complexity, but it leaves performance on the table. Alternative engines like coregex can provide:

- **3-3000x speedup** in many use cases
- SIMD prefilters for fast candidate rejection
- Multiple regex engine strategies optimized for different patterns
- Zero-allocation iterators
- O(n) time complexity guarantee (no ReDoS vulnerabilities)

## RegexHandler Configuration

The `RegexHandler` field in `fiber.Config` accepts any regex compile function
directly:

```go
type Config struct {
    // ... other fields ...

    // RegexHandler is a function that compiles regex patterns for route constraints.
    // Both regexp.MustCompile and coregex.MustCompile can be assigned directly.
    RegexHandler any
}
```

**Default:** `regexp.MustCompile` (Go's standard library)

## Using Coregex

Coregex provides excellent performance improvements for most regex patterns.
Here's how to integrate it:

### 1. Install Coregex

```bash
go get github.com/coregx/coregex
```

**Note:** Coregex requires Go 1.25+

### 2. Configure Fiber to Use Coregex

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/coregx/coregex"
)

func main() {
    app := fiber.New(fiber.Config{
        RegexHandler: coregex.MustCompile,
    })

    // All regex() constraints will now use coregex
    app.Get("/api/v1/:id<regex(\\d+)>", func(c fiber.Ctx) error {
        return c.SendString("ID: " + c.Params("id"))
    })

    app.Listen(":3000")
}
```

That's it! No adapter types or wrappers needed—just pass
`coregex.MustCompile` directly.

## Using Standard Library (Explicit)

If you want to explicitly set the standard library regex handler:

```go
import "regexp"

app := fiber.New(fiber.Config{
    RegexHandler: regexp.MustCompile,
})
```

## Performance Benefits

Coregex excels at these pattern types:

| Pattern Type | Example | Speedup |
|--------------|---------|---------|
| Multi-pattern alternation | `foo\|bar\|baz` | 100-300x |
| Suffix patterns | `.*\\.log`, `.*\\.txt` | 100-1100x |
| Inner literals | `.*error.*`, `.*@example\\.com` | 100-900x |
| IP/phone patterns | `\\d+\\.\\d+\\.\\d+\\.\\d+` | 300-700x |
| Multiline patterns | `(?m)^/.*\\.php` | 100-550x |
| Email validation | Complex email patterns | 400-600x |

## Compatibility

Both `regexp.MustCompile` and `coregex.MustCompile` work seamlessly with
Fiber's regex constraint syntax:

```go
// Single constraint
app.Get("/:id<regex(\\d+)>", handler)

// Multiple constraints
app.Get("/:param<int;max(3000)>", handler)

// Complex patterns
app.Get("/date/:date<regex(\\d{4}-\\d{2}-\\d{2})>", handler)

// Email validation
app.Get("/user/:email<regex([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})>", handler)
```

## Notes

- Regex patterns are compiled once during route registration, so the performance improvement is in the matching phase
- The regex handler is used for all `regex()` constraints in route patterns
- Invalid regex patterns still panic during route registration because Fiber uses `MustCompile`-style semantics
- The compiled matcher is reused across requests, so custom matchers must be safe for concurrent use
- No changes required to existing route definitions
