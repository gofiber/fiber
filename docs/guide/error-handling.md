---
id: error-handling
title: 🐛 Error Handling
description: >-
  Fiber supports centralized error handling: handlers return errors so you can
  log them or send a custom HTTP response to the client.
sidebar_position: 4
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Catching Errors

Return errors from route handlers and middleware so Fiber can handle them centrally.

<Tabs>
<TabItem value="example" label="Example">

```go
app.Get("/", func(c fiber.Ctx) error {
    // Pass error to Fiber
    return c.SendFile("file-does-not-exist")
})
```

</TabItem>
</Tabs>

Fiber does not recover from [panics](https://go.dev/blog/defer-panic-and-recover) by default. Add the `Recover` middleware to catch panics in any handler:

```go title="Example"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/recover"
)

func main() {
    app := fiber.New()

    app.Use(recover.New())

    app.Get("/", func(c fiber.Ctx) error {
        panic("This panic is caught by fiber")
    })

    log.Fatal(app.Listen(":3000"))
}
```

A recovered panic reaches the error handler as an ordinary error, so the default handler answers **500** with the panic message as the response body, unless the panic value is itself a `*fiber.Error`, which keeps its status. Set the middleware's `PanicHandler` to return a generic error such as `fiber.ErrInternalServerError` instead; see [Recover](../middleware/recover.md).

Use `fiber.NewError()` to create an error with a status code. If you omit the message, Fiber uses the standard status text (for example, `404` becomes `Not Found`). [`fiber.NewErrorf()`](../api/fiber.md#newerrorf) formats the message for you.

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    // 503 Service Unavailable
    return fiber.ErrServiceUnavailable

    // 503 On vacation!
    return fiber.NewError(fiber.StatusServiceUnavailable, "On vacation!")
})
```

:::note
The status code has to fit a three-digit status line. When Fiber writes a response with a code outside 100-999, it logs an error and sends **500** instead.
:::

## Default Error Handler

Fiber ships with a default error handler that sends **500 Internal Server Error** for generic errors. If the error is a [fiber.Error](https://pkg.go.dev/github.com/gofiber/fiber/v3#Error), the response uses the embedded status code and message.

```go title="Example"
// Default error handler (fiber.DefaultErrorHandler); utils is github.com/gofiber/utils/v2
func DefaultErrorHandler(c fiber.Ctx, err error) error {
    // Status code defaults to 500
    code := fiber.StatusInternalServerError
    var e *fiber.Error
    matched := errors.As(err, &e)
    if matched && e != nil {
        code = e.Code
    }
    message := utils.StatusMessage(code)
    if err != nil && (!matched || e != nil) {
        message = err.Error()
    }

    // Set Content-Type: text/plain; charset=utf-8
    c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)

    // Return status code with error message
    return c.Status(code).SendString(message)
}
```

## Custom Error Handler

Set a custom error handler in [`fiber.Config`](../api/fiber.md#errorhandler) when creating a new app.

The default handler covers most cases, but a custom handler lets you react to specific error types—for example, by logging to a service or sending a tailored JSON or HTML response.

The following example shows how to display error pages for different types of errors.

```go title="Example"
// Create a new fiber instance with custom config
app := fiber.New(fiber.Config{
    // Override default error handler
    ErrorHandler: func(ctx fiber.Ctx, err error) error {
        // Status code defaults to 500
        code := fiber.StatusInternalServerError

        // Retrieve the custom status code if it's a *fiber.Error
        var e *fiber.Error
        if errors.As(err, &e) && e != nil {
            code = e.Code
        }

        // Send custom error page
        err = ctx.Status(code).SendFile(fmt.Sprintf("./%d.html", code))
        if err != nil {
            // In case the SendFile fails
            return ctx.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
        }

        // Return from handler
        return nil
    },
})

// ...
```

Call `fiber.DefaultErrorHandler(c, err)` from your handler to fall back to the default response for the errors you do not handle yourself.

A sub-app mounted with `app.Use` can set its own `ErrorHandler`. It answers every error raised under the mount path, including the **404** for a path that matches none of the sub-app's routes. An app mounted inside it without a handler of its own uses it too, and any other request falls back to the parent's handler. This needs a literal mount path, taken from the top-level app down: the lookup compares the request path with the mount path as text, so under a parameter, as in `app.Use("/t/:tenant", tenantApp)`, the sub-app's routes still match and run but its handler is not found, and neither is the handler of an app mounted inside it. The handler of the nearest enclosing mount with a literal path answers instead, or the top-level app's handler when there is none. A sub-app mounted through [`Domain`](../api/app.md#domain) matches a parameterized mount path as a pattern and keeps its own handler.

## Errors Before a Handler Runs

The error handler also answers requests that never reach a route handler. A request that matches no route reaches it as `fiber.ErrNotFound` (**404**), or as `fiber.ErrMethodNotAllowed` (**405**, with an `Allow` header) when the path exists for other methods.

The HTTP server rejects some requests while it reads them. Fiber maps those errors to a `*fiber.Error` before it calls the error handler:

| Cause                                                                    | Status                                               |
| ------------------------------------------------------------------------ | ---------------------------------------------------- |
| Headers larger than [`ReadBufferSize`](../api/fiber.md#readbuffersize)   | `431 Request Header Fields Too Large`                |
| Body larger than [`BodyLimit`](../api/fiber.md#bodylimit)                | `413 Request Entity Too Large`                       |
| A method other than `GET` or `HEAD` while [`GETOnly`](../api/fiber.md#getonly) is enabled | `405 Method Not Allowed`, with `Allow: GET, HEAD`    |
| A method token the parser rejects, such as `GE(T`                        | `501 Not Implemented`                                |
| A `Transfer-Encoding` naming a coding the server cannot decode           | `501 Not Implemented`                                |
| A timeout while reading the request, or another network error            | `408 Request Timeout`, or `502 Bad Gateway`          |
| Any other malformed request                                              | `400 Bad Request`, with the parser's message         |

When the request method is a known one, and not one that `GETOnly` rejects, middleware registered with `Use` runs first and already sees the mapped status on the response, while route handlers are skipped, a guard registered as a route among them. The body the chain produced, such as the answer of a catch-all `Use` handler, is dropped before the error handler writes the response, and the headers the chain set, such as the ones `cors` adds, are kept. The cache and idempotency middleware neither serve nor store such a request, the proxy forwarders do not send it upstream, and `static` serves no file for it. A guard that has to hold for your own `Use` handlers as well belongs in `Use`.

A well-formed method that is not in [`RequestMethods`](../api/fiber.md#requestmethods), such as `FOO`, or a known one in another letter case, such as `get`, does not get that far: Fiber answers `501 Not Implemented` itself, and neither middleware nor the error handler runs.

> Special thanks to the [Echo](https://echo.labstack.com/) and [Express](https://expressjs.com/) frameworks for inspiring parts of this error-handling approach.
