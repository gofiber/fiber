---
id: faq
title: 🤔 FAQ
description: >-
  Frequently asked questions. Open an issue if you have another question to add.
sidebar_position: 1
---

## How should I structure my application?

There's no single answer; the ideal structure depends on your application's scale and team. Fiber makes no assumptions about project layout.

Routes and other application logic can live in any files or directories. For inspiration, see:

* [gofiber/boilerplate](https://github.com/gofiber/boilerplate)
* [thomasvvugt/fiber-boilerplate](https://github.com/thomasvvugt/fiber-boilerplate)
* [Youtube - Building a REST API using Gorm and Fiber](https://www.youtube.com/watch?v=Iq2qT0fRhAA)
* [embedmode/fiberseed](https://github.com/embedmode/fiberseed)

## How do I handle custom 404 responses?

If you're using v2.32.0 or later, implement a custom error handler as shown below or read more at [Error Handling](../guide/error-handling.md#custom-error-handler).

If you're using v2.31.0 or earlier, the error handler will not capture 404 errors. Instead, add a middleware function at the very bottom of the stack \(below all other functions\) to handle a 404 response:

```go title="Example"
app.Use(func(c fiber.Ctx) error {
    return c.Status(fiber.StatusNotFound).SendString("Sorry can't find that!")
})
```

## How can I use live reload?

[Air](https://github.com/air-verse/air) automatically restarts your Go application when source files change, speeding development.

To use Air in a Fiber project, follow these steps:

* Install Air by downloading the appropriate binary for your operating system from the GitHub release page or by building the tool from source.
* Create a configuration file for Air in your project directory, such as `.air.toml` or `air.conf`. Here's a sample configuration file that works with Fiber:

```toml
# .air.toml
root = "."
tmp_dir = "tmp"
[build]
  cmd = "go build -o ./tmp/main ."
  bin = "./tmp/main"
  delay = 1000 # ms
  exclude_dir = ["assets", "tmp", "vendor"]
  include_ext = ["go", "tpl", "tmpl", "html"]
  exclude_regex = ["_test\\.go"]
```

* Start your Fiber application with Air by running the following command:

```sh
air
```

As you edit source files, Air detects the changes and restarts the application.

A complete example is available in the [Fiber Recipes repository](https://github.com/gofiber/recipes/tree/master/air) and shows how to configure Air for a Fiber project.

## How do I set up an error handler?

To override the default error handler, provide a custom one in the [Config](../api/fiber.md#errorhandler) when creating a new [Fiber instance](../api/fiber.md#new).

```go title="Example"
app := fiber.New(fiber.Config{
    ErrorHandler: func(c fiber.Ctx, err error) error {
        return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
    },
})
```

We have a dedicated page explaining how error handling works in Fiber, see [Error Handling](../guide/error-handling.md).

## Which template engines does Fiber support?

Fiber currently supports 9 template engines in our [gofiber/template](https://docs.gofiber.io/template/) middleware:

* [ace](https://docs.gofiber.io/template/ace/)
* [amber](https://docs.gofiber.io/template/amber/)
* [django](https://docs.gofiber.io/template/django/)
* [handlebars](https://docs.gofiber.io/template/handlebars/)
* [html](https://docs.gofiber.io/template/html/)
* [jet](https://docs.gofiber.io/template/jet/)
* [mustache](https://docs.gofiber.io/template/mustache/)
* [pug](https://docs.gofiber.io/template/pug/)
* [slim](https://docs.gofiber.io/template/slim/)

To learn more about using Templates in Fiber, see [Templates](../guide/templates.md).

## Does Fiber have a community chat?

Yes, we have a [Discord](https://gofiber.io/discord) server with rooms for every topic.
If you have questions or just want to chat, join us via this [invite link](https://gofiber.io/discord).

![](/img/support-discord.png)

## Does Fiber support subdomain routing?

Yes, we do. Here are some examples:

<details>
<summary>Example</summary>

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/logger"
)

type Host struct {
    Fiber *fiber.App
}

func main() {
    // Hosts
    hosts := map[string]*Host{}
    //-----
    // API
    //-----
    api := fiber.New()
    api.Use(logger.New(logger.Config{
        Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
    }))
    hosts["api.localhost:3000"] = &Host{api}
    api.Get("/", func(c fiber.Ctx) error {
        return c.SendString("API")
    })
    //------
    // Blog
    //------
    blog := fiber.New()
    blog.Use(logger.New(logger.Config{
        Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
    }))
    hosts["blog.localhost:3000"] = &Host{blog}
    blog.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Blog")
    })
    //---------
    // Website
    //---------
    site := fiber.New()
    site.Use(logger.New(logger.Config{
        Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
    }))

    hosts["localhost:3000"] = &Host{site}
    site.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Website")
    })
    // Server
    app := fiber.New()
    app.Use(func(c fiber.Ctx) error {
        host := hosts[c.Hostname()]
        if host == nil {
            return c.SendStatus(fiber.StatusNotFound)
        } else {
            host.Fiber.Handler()(c.Context())
            return nil
        }
    })
    log.Fatal(app.Listen(":3000"))
}
```

</details>

For more information, see issue [#750](https://github.com/gofiber/fiber/issues/750).

## How can I handle conversions between Fiber and net/http?

Fiber can register common `net/http` handlers directly—just pass an
`http.Handler`, `http.HandlerFunc`, or compatible function to your routing
method. For other interoperability scenarios, the `adaptor` middleware provides
utilities for converting between Fiber and `net/http`. It allows seamless
integration of `net/http` handlers, middleware, and requests into Fiber
applications, and vice versa.

For details on how to:

* Convert `net/http` handlers to Fiber handlers
* Convert Fiber handlers to `net/http` handlers
* Convert `fiber.Ctx` to `http.Request`

See the dedicated documentation: [Adaptor Documentation](../middleware/adaptor.md).
