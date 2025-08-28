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

Use `fiber.NewError()` to create an error with a status code. If you omit the message, Fiber uses the standard status text (for example, `404` becomes `Not Found`).

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    // 503 Service Unavailable
    return fiber.ErrServiceUnavailable

    // 503 On vacation!
    return fiber.NewError(fiber.StatusServiceUnavailable, "On vacation!")
})
```

## Default Error Handler

Fiber ships with a default error handler that sends **500 Internal Server Error** for generic errors. If the error is a [fiber.Error](https://godoc.org/github.com/gofiber/fiber#Error), the response uses the embedded status code and message.

```go title="Example"
// Default error handler
var DefaultErrorHandler = func(c fiber.Ctx, err error) error {
    // Status code defaults to 500
    code := fiber.StatusInternalServerError

    // Retrieve the custom status code if it's a *fiber.Error
    var e *fiber.Error
    if errors.As(err, &e) {
        code = e.Code
    }

    // Set Content-Type: text/plain; charset=utf-8
    c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)

    // Return status code with error message
    return c.Status(code).SendString(err.Error())
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
        if errors.As(err, &e) {
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

> Special thanks to the [Echo](https://echo.labstack.com/) and [Express](https://expressjs.com/) frameworks for inspiring parts of this error-handling approach.
