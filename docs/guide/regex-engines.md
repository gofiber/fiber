# Alternative Regex Engines

Fiber v3+ supports using alternative regex implementations for route pattern matching through the `RegexEngine` configuration option. This allows you to use high-performance regex engines like [coregex](https://github.com/coregx/coregex) as a drop-in replacement for Go's standard library `regexp` package.

## Why Use an Alternative Regex Engine?

The default Go `regexp` package is intentionally simple and guarantees O(n) time complexity, but it leaves performance on the table. Alternative engines like coregex can provide:

- **3-3000x speedup** in many use cases
- SIMD prefilters for fast candidate rejection
- Multiple regex engine strategies optimized for different patterns
- Zero-allocation iterators
- O(n) time complexity guarantee (no ReDoS vulnerabilities)

## RegexEngine Configuration

The `RegexEngine` field in `fiber.Config` allows you to specify a custom regex implementation:

```go
type Config struct {
    // ... other fields ...

    // RegexEngine allows using alternative regex implementations for route pattern matching.
    // Custom engines must implement the RegexEngine interface.
    RegexEngine RegexEngine
}
```

**Default:** `DefaultRegexEngine` (uses Go's standard library `regexp`)

## Using Coregex

Coregex provides excellent performance improvements for most regex patterns. Here's how to integrate it:

### 1. Install Coregex

```bash
go get github.com/coregx/coregex
```

**Note:** Coregex requires Go 1.25+

### 2. Create a Coregex Adapter

```go
package main

import (
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
```

### 3. Configure Fiber to Use Coregex

```go
func main() {
    app := fiber.New(fiber.Config{
        RegexEngine: CoregexEngine{},
    })

    // All regex constraints will now use coregex
    app.Get("/api/v1/:id<regex(\\d+)>", func(c fiber.Ctx) error {
        return c.SendString("ID: " + c.Params("id"))
    })

    app.Listen(":3000")
}
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

The adapter maintains full compatibility with Fiber's existing regex constraint syntax:

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

## Custom Regex Engine Implementation

You can implement your own regex engine by implementing the `RegexEngine` and `RegexCompiler` interfaces:

```go
// RegexEngine provides methods for creating compiled regex patterns
type RegexEngine interface {
    // MustCompile compiles a regex pattern and panics if invalid
    MustCompile(pattern string) RegexCompiler
}

// RegexCompiler defines methods for regex pattern matching
type RegexCompiler interface {
    // MatchString reports whether the string contains any match
    MatchString(s string) bool

    // FindAllStringSubmatch returns all successive matches
    FindAllStringSubmatch(s string, n int) [][]string
}
```

### Example: Custom Engine with Caching

```go
type CachedRegexEngine struct {
    cache map[string]fiber.RegexCompiler
    mu    sync.RWMutex
}

func (e *CachedRegexEngine) MustCompile(pattern string) fiber.RegexCompiler {
    e.mu.RLock()
    if cached, ok := e.cache[pattern]; ok {
        e.mu.RUnlock()
        return cached
    }
    e.mu.RUnlock()

    e.mu.Lock()
    defer e.mu.Unlock()

    // Double-check after acquiring write lock
    if cached, ok := e.cache[pattern]; ok {
        return cached
    }

    compiled := /* your implementation */
    e.cache[pattern] = compiled
    return compiled
}
```

## Notes

- Regex patterns are compiled once during route registration, so the performance improvement is in the matching phase
- The regex engine is used for all `regex()` constraints in route patterns
- Falls back gracefully if pattern compilation fails
- No changes required to existing route definitions
