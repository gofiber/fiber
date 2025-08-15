---
id: context
title: "\U0001F9E0 Context"
description: >-
  Learn how Fiber's Ctx integrates with Go's context.Context,
  how to interact with the underlying fasthttp RequestCtx,
  and how to use the available context helpers.
sidebar_position: 6
toc_max_heading_level: 4
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Fiber Context as `context.Context`

Fiber's [`Ctx`](../api/ctx.md) now implements Go's
[`context.Context`](https://pkg.go.dev/context#Context) interface.
This means you can pass the context directly to functions that expect
`context.Context` without any adapters.
However, due to current limitations in `fasthttp`, the
`Deadline`, `Done`, and `Err` methods are implemented as no-ops【F:docs/api/ctx.md†L44-L53】.

```go title="Example"
func doSomething(ctx context.Context) {
    // ... your logic here
}

app.Get("/", func(c fiber.Ctx) error {
    doSomething(c) // c satisfies context.Context
    return nil
})
```

### Retrieving Values

`Ctx.Value` is backed by [Locals](../api/ctx.md#locals). Values stored
with `c.Locals` can be read back via the `Value` method or through
`context.WithValue` helpers【F:docs/api/ctx.md†L66-L74】.

```go title="Locals and Value"
app.Get("/", func(c fiber.Ctx) error {
    c.Locals("role", "admin")
    role := c.Value("role") // returns "admin"
    return c.SendString(role.(string))
})
```

## Working with `RequestCtx` and `fasthttpctx`

The underlying [`fasthttp.RequestCtx`](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx)
can be accessed via `c.RequestCtx()`【F:ctx.go†L104-L108】.
This exposes low level APIs and the context support provided by the
`fasthttpctx` layer.

```go title="Accessing RequestCtx"
app.Get("/raw", func(c fiber.Ctx) error {
    fctx := c.RequestCtx()
    // use fasthttp APIs directly
    fctx.Response.Header.Set("X-Engine", "fasthttp")
    return nil
})
```

`fasthttpctx` enables `fasthttp` to satisfy the `context.Context` interface.
`Deadline` always reports no deadline, `Done` is closed when the client
connection ends, and once it fires `Err` reports `context.Canceled`. This
means handlers can detect client disconnects while still passing
`c.RequestCtx()` into APIs that expect a `context.Context`.

## Context Helpers

Fiber and its middleware expose a number of helper functions that
retrieve request-scoped values from the context.

### Request ID

The RequestID middleware stores the generated identifier in the context.
Use `requestid.FromContext` to read it later【F:docs/middleware/requestid.md†L11-L14】【F:docs/middleware/requestid.md†L44-L51】.

```go
app.Use(requestid.New())
app.Get("/", func(c fiber.Ctx) error {
    id := requestid.FromContext(c)
    return c.SendString(id)
})
```

### CSRF

The CSRF middleware provides helpers to fetch the token or the handler
attached to the current context【F:docs/middleware/csrf.md†L111-L133】【F:docs/middleware/csrf.md†L452-L475】.

```go
app.Use(csrf.New())
app.Get("/form", func(c fiber.Ctx) error {
    token := csrf.TokenFromContext(c)
    return c.SendString(token)
})
```

```go title="Deleting a token"
app.Post("/logout", func(c fiber.Ctx) error {
    handler := csrf.HandlerFromContext(c)
    if handler != nil {
        // Invalidate the token on logout
        _ = handler.DeleteToken(c)
    }
    // ... other logout logic
    return c.SendString("Logged out")
})
```

### Session

Sessions are stored on the context and can be retrieved via
`session.FromContext`【F:docs/middleware/session.md†L22-L46】.

```go
app.Use(session.New())
app.Get("/", func(c fiber.Ctx) error {
    sess := session.FromContext(c)
    count := sess.Get("visits")
    return c.JSON(fiber.Map{"visits": count})
})
```

### Basic Authentication

After successful authentication, the username is available with
`basicauth.UsernameFromContext`【F:docs/middleware/basicauth.md†L14-L18】.

```go
app.Use(basicauth.New(basicauth.Config{Users: map[string]string{"admin": "secret"}}))
app.Get("/", func(c fiber.Ctx) error {
    user := basicauth.UsernameFromContext(c)
    return c.SendString(user)
})
```

### Key Authentication

For API key authentication, the extracted token is stored in the
context and accessible via `keyauth.TokenFromContext`【F:docs/middleware/keyauth.md†L9-L14】.

```go
app.Use(keyauth.New())
app.Get("/", func(c fiber.Ctx) error {
    token := keyauth.TokenFromContext(c)
    return c.SendString(token)
})
```

## Using `context.WithValue` and Friends

Since `fiber.Ctx` conforms to `context.Context`, standard helpers such as
`context.WithValue`, `context.WithTimeout`, or `context.WithCancel`
can wrap the request context when needed.

```go
app.Get("/job", func(c fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(c, 5*time.Second)
    defer cancel()

    // pass ctx to async operations that honor cancellation
    if err := doWork(ctx); err != nil {
        return err
    }
    return c.SendStatus(fiber.StatusOK)
})
```

Even though the base `fiber.Ctx` never cancels on its own, wrapping it with
helpers like `context.WithTimeout` creates a derived context that honors
deadlines and cancellation for any operations you launch from the handler. This
common pattern lets you coordinate work with external APIs or databases while
keeping a familiar API.

## Summary

- `fiber.Ctx` satisfies `context.Context` but its `Deadline`, `Done`, and `Err`
  methods are currently no-ops.
- `RequestCtx` exposes the raw `fasthttp` context, whose `Done` channel closes
  when the client connection ends.
- Middleware helpers like `requestid.FromContext` or `session.FromContext`
  make it easy to retrieve request-scoped data.
- Standard helpers such as `context.WithTimeout` can wrap `fiber.Ctx` to create
  fully featured derived contexts inside handlers.

With these tools, you can seamlessly integrate Fiber applications with
Go's context-based APIs and manage request-scoped data effectively.
