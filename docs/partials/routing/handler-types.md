---
id: handler-types
title: Handler types
---

Fiber's adapter converts a variety of handler shapes into native `func(fiber.Ctx) error` callbacks. The 17 supported shapes are grouped below; any other signature, and a `nil` handler, makes registering the route panic. This lets you mix Fiber-style handlers with Express-style callbacks and even reuse `net/http` or `fasthttp` functions.

### Fiber-native handlers (cases 1-2)

- **Case 1.** `fiber.Handler` - the canonical `func(fiber.Ctx) error` form.
- **Case 2.** `func(fiber.Ctx)` - Fiber runs the function and treats it as if it returned `nil`.

### Express-style request handlers (cases 3-12)

- **Case 3.** `func(fiber.Req, fiber.Res) error`
- **Case 4.** `func(fiber.Req, fiber.Res)`
- **Case 5.** `func(fiber.Req, fiber.Res, func() error) error`
- **Case 6.** `func(fiber.Req, fiber.Res, func() error)`
- **Case 7.** `func(fiber.Req, fiber.Res, func()) error`
- **Case 8.** `func(fiber.Req, fiber.Res, func())`
- **Case 9.** `func(fiber.Req, fiber.Res, func(error))`
- **Case 10.** `func(fiber.Req, fiber.Res, func(error)) error`
- **Case 11.** `func(fiber.Req, fiber.Res, func(error) error)`
- **Case 12.** `func(fiber.Req, fiber.Res, func(error) error) error`

The adapter injects a `next` callback when your signature accepts one. Fiber propagates downstream errors from `c.Next()` back through the wrapper, so returning those errors remains optional. If you never call the injected `next` function, the handler chain stops, matching Express semantics.

When you accept `next` callbacks that take an `error`, calling `next(nil)` continues the chain and passing a non-nil error short-circuits with that error. If the handler itself returns an error, Fiber prioritizes that value over any recorded `next` error.

Fiber has no Express-style four-argument error handler (`func(err, req, res, next)`); a non-nil error propagates to the app's central `ErrorHandler` instead.

### net/http handlers (cases 13-15)

- **Case 13.** `http.HandlerFunc`
- **Case 14.** `http.Handler`
- **Case 15.** `func(http.ResponseWriter, *http.Request)`

:::caution Compatibility overhead
Fiber adapts these handlers through `fasthttpadaptor`. They do not receive `fiber.Ctx`, cannot call `c.Next()`, and therefore always terminate the handler chain. The compatibility layer also adds more overhead than running a native Fiber handler, so prefer the other forms when possible.
:::

These handlers get the same routed-path handling as `adaptor.HTTPHandler`: they are given the path the router matched, `c.Path()`, followed by the query, not the request line as it arrived, so `/a/../admin/x` reaches them as `/admin/x`, after the middleware on `/admin` has run. A routed path that `net/http` would read as another one, such as `/admin%2Fx`, `//admin/x` or `/public/..%2Fadmin/x`, is answered with `404 Not Found`.

### fasthttp handlers (cases 16-17)

- **Case 16.** `fasthttp.RequestHandler` - the named fasthttp type; a plain `func(*fasthttp.RequestCtx)` has to be converted to it first.
- **Case 17.** `func(*fasthttp.RequestCtx) error`

fasthttp handlers run with full access to the underlying `fasthttp.RequestCtx`. They are expected to manage the response directly. Fiber will propagate any error returned by the `func(*fasthttp.RequestCtx) error` variant but otherwise does not inspect the context state.

```go title="Examples"
// Reuse an existing net/http handler without manual adaptation
httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
})

app.Get("/foo", httpHandler)

// Align with Express-style handlers using fiber.Req and fiber.Res helpers (works
// for middleware and routes alike)
app.Use(func(req fiber.Req, res fiber.Res, next func() error) error {
    if req.IP() == "192.168.1.254" {
        return res.SendStatus(fiber.StatusForbidden)
    }
    return next()
})

app.Get("/express", func(req fiber.Req, res fiber.Res) error {
    return res.SendString("Hello from Express-style handlers!")
})

// Mount a fasthttp.RequestHandler directly (case 16). A bare function
// literal is not a fasthttp.RequestHandler, so Fiber rejects it: declare the
// variable with that type, or convert the literal.
var bar fasthttp.RequestHandler = func(ctx *fasthttp.RequestCtx) {
    ctx.SetStatusCode(fiber.StatusAccepted)
}
app.Get("/bar", bar)

// ...or the error-returning variant (case 17)
app.Get("/baz", func(ctx *fasthttp.RequestCtx) error {
    ctx.SetStatusCode(fiber.StatusAccepted)
    return nil
})
```
