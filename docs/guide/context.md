---
id: go-context
title: "\U0001F9E0 Go Context"
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

Fiber's [`Ctx`](../api/ctx.md) implements Go's
[`context.Context`](https://pkg.go.dev/context#Context) interface.
You can pass `c` directly to functions that expect a `context.Context`
without adapters.
However, `fasthttp` doesn't support cancellation yet, so
`Deadline`, `Done`, and `Err` are no-ops.

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

`Ctx.Value` is backed by [Locals](../api/ctx.md#locals).
Values stored with `c.Locals` are accessible through `Value` or
standard `context.WithValue` helpers.

```go title="Locals and Value"
app.Get("/", func(c fiber.Ctx) error {
    c.Locals("role", "admin")
    role := c.Value("role") // returns "admin"
    return c.SendString(role.(string))
})
```

## Working with `RequestCtx` and `fasthttpctx`

The underlying [`fasthttp.RequestCtx`](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx)
can be accessed via `c.RequestCtx()`.
This exposes low-level APIs and the extra context support provided by
`fasthttpctx`.

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
Use `requestid.FromContext` to read it later.

```go
app.Use(requestid.New())
app.Get("/", func(c fiber.Ctx) error {
    id := requestid.FromContext(c)
    return c.SendString(id)
})
```

### CSRF

The CSRF middleware provides helpers to fetch the token or the handler
attached to the current context.

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
`session.FromContext`.

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
`basicauth.UsernameFromContext`. Passwords in `Users` must be pre-hashed.

```go
app.Use(basicauth.New(basicauth.Config{
    Users: map[string]string{
        // "secret" hashed using SHA-256
        "admin": "{SHA256}K7gNU3sdo+OL0wNhqoVWhr3g6s1xYv72ol/pe/Unols=",
    },
}))
app.Get("/", func(c fiber.Ctx) error {
    user := basicauth.UsernameFromContext(c)
    return c.SendString(user)
})
```

### Key Authentication

For API key authentication, the extracted token is stored in the
context and accessible via `keyauth.TokenFromContext`.

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

The base `fiber.Ctx` never cancels on its own. Wrapping it with helpers
like `context.WithTimeout` yields a derived context that honors deadlines
and cancellation for operations started from the handler. This pattern lets
you coordinate work with external APIs or databases while using Fiber's API.

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
