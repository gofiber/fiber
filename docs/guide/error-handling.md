---
id: error-handling
title: 🐛 Error Handling
description: >-
  Fiber supports centralized error handling by returning an error to the handler
  which allows you to log errors to external services or send a customized HTTP
  response to the client.
sidebar_position: 4
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Catching Errors

Ensure that Fiber catches all errors from route handlers and middleware by returning them to the handler function for processing.

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

Fiber does not handle [panics](https://go.dev/blog/defer-panic-and-recover) by default. To recover from a panic thrown by any handler in the stack, you need to include the `Recover` middleware below:

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

Use Fiber's custom error struct to pass a status code with `fiber.NewError()`. The message parameter is optional; if omitted, it defaults to the standard status text (for example, `404` becomes `Not Found`).

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    // 503 Service Unavailable
    return fiber.ErrServiceUnavailable

    // 503 On vacation!
    return fiber.NewError(fiber.StatusServiceUnavailable, "On vacation!")
})
```

## Default Error Handler

Fiber provides an error handler by default. For a standard error, the response is sent as **500 Internal Server Error**. If the error is of type [fiber.Error](https://godoc.org/github.com/gofiber/fiber#Error), the response is sent with the provided status code and message.

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

A custom error handler can be set using a [Config](../api/fiber.md#errorhandler) when initializing a [Fiber instance](../api/fiber.md#new).

In most cases, the default error handler is sufficient. However, a custom handler is useful when you need to capture different error types and respond accordingly—for example, sending notification emails or logging to a centralized system. You can also send custom responses to the client, such as an error page or a JSON message.

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

> Special thanks to the [Echo](https://echo.labstack.com/) & [Express](https://expressjs.com/) framework for inspiration regarding error handling.
