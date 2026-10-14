---
id: pprof
---

# Pprof

Pprof middleware exposes runtime profiling data for analysis with the Go `pprof` tool. Once you add it with `app.Use`, it serves the profiles under `/debug/pprof/`.

The middleware claims a request only when the path is the prefix plus `/debug/pprof` itself or continues it at a segment boundary, so an application route such as `/debug/pprof-old` still reaches its own handler. Within its paths, `303 See Other` redirects settle the spelling: `/debug/pprof` goes to `/debug/pprof/`, a trailing slash after a profile name (`/debug/pprof/heap/`) is dropped, and a name that is not a profile (`/debug/pprof/nope`) goes to the index.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/pprof"
)
```

Once your Fiber app is initialized, use the middleware as shown:

```go
// Initialize default config
app.Use(pprof.New())

// Or customize the config

// For multi-ingress systems, add a URL prefix:
app.Use(pprof.New(pprof.Config{Prefix: "/endpoint-prefix"}))

// The resulting URL is "/endpoint-prefix/debug/pprof/"
```

## Config

| Property | Type                    | Description                                                                                                                                                                          | Default |
|:---------|:------------------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:-------:|
| Next     | `func(fiber.Ctx) bool` | Next defines a function to skip this middleware when it returns true.                                                                                                                  | `nil`   |
| Prefix   | `string`                | Prefix adds a segment before `/debug/pprof`; it must start with a slash and omit the trailing slash. Example: `/federated-fiber` | `""`   |

## Default Config

```go
var ConfigDefault = Config{
    Next:   nil,
    Prefix: "",
}
```
