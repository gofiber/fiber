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
without adapters, but only to read values from it. `Ctx` is a context that can
never be canceled: `Deadline`, `Done`, and `Err` always report no deadline,
`nil`, and `nil`, whatever you set with `c.SetContext`. For cancellation and
deadlines, use `c.Context()`.

:::caution
The `fiber.Ctx` instance is only valid within the lifetime of the handler.
It is reused for subsequent requests, so avoid storing `c` or using it in
goroutines that outlive the handler. For asynchronous work, call
`c.Context()` inside the handler to obtain a `context.Context` that can safely
be used after the handler returns. By default, this returns `context.Background()`
unless a custom context was provided with `c.SetContext`. The context outlives
the handler, but a string you read from the request does not unless
[`Immutable`](../api/fiber.md#immutable) is set: copy such a value inside the
handler before you start the goroutine, as [Context Helpers](#context-helpers)
shows.
:::

```go title="Example"
func doSomething(ctx context.Context) {
    // ... your logic here
}

app.Get("/", func(c fiber.Ctx) error {
    doSomething(c) // c satisfies context.Context
    return nil
})
```

### Using context outside the handler

`fiber.Ctx` is recycled after each request. If you need a context that lives
longer—for example, for work performed in a new goroutine—obtain it with
`c.Context()` before returning from the handler.

```go title="Async work"
app.Get("/job", func(c fiber.Ctx) error {
    ctx := c.Context()
    go performAsync(ctx)
    return c.SendStatus(fiber.StatusAccepted)
})
```

You can customize the base context by calling `c.SetContext` before
requesting it. This changes what `c.Context()` returns for the rest of the
request; it does not make `c` itself cancelable.

```go
app.Get("/job", func(c fiber.Ctx) error {
    c.SetContext(context.WithValue(context.Background(), "requestID", "123"))
    ctx := c.Context()
    go performAsync(ctx)
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

## Working with `RequestCtx`

The underlying [`fasthttp.RequestCtx`](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx)
can be accessed via `c.RequestCtx()`.
This exposes low-level APIs and a `context.Context` implementation of its own.

```go title="Accessing RequestCtx"
app.Get("/raw", func(c fiber.Ctx) error {
    fctx := c.RequestCtx()
    // use fasthttp APIs directly
    fctx.Response.Header.Set("X-Engine", "fasthttp")
    return nil
})
```

`fasthttp.RequestCtx` satisfies the `context.Context` interface itself.
`Deadline` always reports no deadline. `Done` closes only when the server
shuts down, and `Err` then returns `context.Canceled`. Note that this is the
`fasthttp.RequestCtx`, not Fiber's `Ctx`, whose `Done` is always `nil`.

:::caution
`Done` does not fire when an individual client disconnects. To stop a
long-running handler at a deadline, wrap `c.Context()` with
`context.WithTimeout` as shown below.
:::

## Context Helpers

Fiber and its middleware expose a number of helper functions that
retrieve request-scoped values from the context.

Middleware stores these values with `fiber.StoreInContext`, and the helpers
read them back with `fiber.ValueFromContext`. The helpers accept a `fiber.Ctx`
as well as a plain `context.Context`.

`StoreInContext` always writes to `c.Locals()`. It also copies the value into
the context returned by `c.Context()` when
[`PassLocalsToContext`](../api/fiber.md#passlocalstocontext) is enabled, which
it is not by default. Reading from `c` itself therefore always works, but a
helper called with `c.Context()`, as you would in code that only receives a
`context.Context`, finds the value only when that setting is on:

```go
app := fiber.New(fiber.Config{PassLocalsToContext: true})
app.Use(requestid.New())

app.Get("/job", func(c fiber.Ctx) error {
    ctx := c.Context() // take it inside the handler
    // empty unless PassLocalsToContext is true; copy it, see the caution below
    id := strings.Clone(requestid.FromContext(ctx))
    go func() {
        _ = id
    }()
    return c.SendStatus(fiber.StatusAccepted)
})
```

:::caution
Read what you need from the context inside the handler and hand the goroutine a copy. A retained context keeps working after the handler returns, but what it holds may not stay valid:

- A string that comes from the request, such as the request ID when the client sent one or the key that `keyauth` extracted, points into a buffer that fasthttp reuses for the next request on the connection. Unless [`Immutable`](../api/fiber.md#immutable) is set, a goroutine that reads it later can see another request's ID or token. Copy it with `strings.Clone` before you hand it over; see [Zero Allocation](../intro.md#zero-allocation).
- A context you keep stops resolving a session when its request ends: `session.FromContext` returns `nil` for it from then on, so a goroutine that outlives the handler cannot reach the session, and never another user's. The `*session.Middleware` it returned while the request was running is only valid until the handler returns, so do not hand that pointer to a goroutine that may outlive the handler either. Read what you need inside the handler, for example the session ID, and hand that to the goroutine; see the [Store Pattern](../middleware/session.md#store-pattern-advanced) for background work on a session.

:::

Use the same pair for your own values:

```go
type userKey struct{}

fiber.StoreInContext(c, userKey{}, "john")
name, ok := fiber.ValueFromContext[string](c, userKey{})
```

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
context and accessible via `keyauth.TokenFromContext`. The middleware requires
a `Validator`, and reads the key from the `Authorization: Bearer <key>` header
unless you set another `Extractor`.

```go
app.Use(keyauth.New(keyauth.Config{
    Validator: func(c fiber.Ctx, key string) (bool, error) {
        return subtle.ConstantTimeCompare([]byte(key), []byte("secret")) == 1, nil
    },
}))
app.Get("/", func(c fiber.Ctx) error {
    token := keyauth.TokenFromContext(c)
    return c.SendString(token)
})
```

## Using `context.WithValue` and Friends

Standard helpers such as `context.WithValue`, `context.WithTimeout`, or
`context.WithCancel` can wrap the request context when needed. Derive them
from `c.Context()`, not from `c`: `c` can never be canceled, so a context
derived from it would never inherit a deadline or a cancellation.

```go
app.Get("/job", func(c fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
    defer cancel()

    // pass ctx to async operations that honor cancellation
    if err := doWork(ctx); err != nil {
        return err
    }
    return c.SendStatus(fiber.StatusOK)
})
```

### Context Cancellation with Goroutines in Fiber

When starting asynchronous work inside a handler, Fiber does not cancel the base `fiber.Ctx` automatically.
By wrapping the request context with `context.WithTimeout`, you can create a derived context that honors deadlines and cancellation signals.

The goroutine checks `ctx.Done()` before sending a result.
If the deadline elapses, the goroutine exits early and avoids leaking resources.

The handler then waits for either:

- a result from the goroutine, or
- the `context timeout` (which returns a 504 Gateway Timeout)

This pattern ensures that long-running operations (database queries, external API calls, background tasks) do not continue running after the request has ended.

```go
func Handler(c fiber.Ctx) error {
    ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
    defer cancel()

    resultChan := make(chan string, 1)

    go func() {
        select {
        case <-time.After(3 * time.Second):
            select {
            case <-ctx.Done():
                return
            case resultChan <- "done":
            }
        case <-ctx.Done():
            return
        }
    }()

    select {
    case res := <-resultChan:
        return c.SendString(res)
    case <-ctx.Done():
        return c.Status(fiber.StatusGatewayTimeout).SendString("timeout")
    }
}
```

This approach provides safe cancellation semantics for goroutine-based work while allowing you to integrate Fiber handlers with context-aware APIs.

## Summary

- `fiber.Ctx` satisfies `context.Context` but its `Deadline`, `Done`, and `Err`
  methods are currently no-ops.
- `RequestCtx` exposes the raw `fasthttp` context. Its `Done` channel
  closes only on server shutdown, not on client disconnect.
- Use `fiber.StoreInContext(c, key, value)` to store request-scoped values in
  `c.Locals()` and, when `fiber.Config.PassLocalsToContext` is enabled, in
  `c.Context()` as well, so they are available through either API.
- Middleware helpers like `requestid.FromContext` or `session.FromContext`
  make it easy to retrieve request-scoped data. Copy a string that comes from
  the request, or set `Immutable`, before a goroutine keeps it.
- Standard helpers such as `context.WithTimeout` should wrap `c.Context()`,
  not `fiber.Ctx`, to create derived contexts that honor deadlines and can
  outlive the handler.
- `fiber.Config.PassLocalsToContext` controls whether Fiber context helpers
  also propagate values into the request `context.Context` for Fiber-backed
  contexts when using `StoreInContext`. It defaults to `false` for backward
  compatibility, while `ValueFromContext` keeps reading from `c.Locals()`.
- Use `c.Context()` to obtain a `context.Context` that can outlive the handler,
  and `c.SetContext()` to customize it with additional values or deadlines.

With these tools, you can seamlessly integrate Fiber applications with
Go's context-based APIs and manage request-scoped data effectively.
