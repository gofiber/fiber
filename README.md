# Fiber

Fiber is an Express-inspired web framework written in Go that focuses on developer ergonomics and zero-allocation performance. This repository contains the core framework along with documentation and supporting packages.

## Using existing net/http handlers

Fiber now adapts `net/http` handlers transparently, so you can plug in any existing standard library handler without bringing in additional middleware wrappers.

```go
package main

import (
    "net/http"

    "github.com/gofiber/fiber/v3"
)

func main() {
    httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("served by net/http"))
    })

    app := fiber.New()
    app.Get("/legacy", httpHandler)

    app.Listen(":8080")
}
```

The adapter also accepts bare functions with the `func(http.ResponseWriter, *http.Request)` signature, letting you keep your existing `net/http` endpoints while adopting Fiber for new routes.
