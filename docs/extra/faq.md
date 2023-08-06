---
id: faq
title: 🤔 FAQ
description: >-
  List of frequently asked questions. Feel free to open an issue to add your
  question to this page.
sidebar_position: 1
---

## How should I structure my application?

There is no definitive answer to this question. The answer depends on the scale of your application and the team that is involved. To be as flexible as possible, Fiber makes no assumptions in terms of structure.

Routes and other application-specific logic can live in as many files as you wish, in any directory structure you prefer. View the following examples for inspiration:

* [gofiber/boilerplate](https://github.com/gofiber/boilerplate)
* [thomasvvugt/fiber-boilerplate](https://github.com/thomasvvugt/fiber-boilerplate)
* [Youtube - Building a REST API using Gorm and Fiber](https://www.youtube.com/watch?v=Iq2qT0fRhAA)
* [embedmode/fiberseed](https://github.com/embedmode/fiberseed)

## How do I handle custom 404 responses?

If you're using v2.32.0 or later, all you need to do is to implement a custom error handler. See below, or see a more detailed explanation at [Error Handling](../guide/error-handling.md#custom-error-handler). 

If you're using v2.31.0 or earlier, the error handler will not capture 404 errors. Instead, you need to add a middleware function at the very bottom of the stack \(below all other functions\) to handle a 404 response:

```go title="Example"
app.Use(func(c *fiber.Ctx) error {
    return c.Status(fiber.StatusNotFound).SendString("Sorry can't find that!")
})
```

## How can i use live reload ?

[Air](https://github.com/cosmtrek/air) is a handy tool that automatically restarts your Go applications whenever the source code changes, making your development process faster and more efficient.

To use Air in a Fiber project, follow these steps:

1. Install Air by downloading the appropriate binary for your operating system from the GitHub release page or by building the tool directly from source.
2. Create a configuration file for Air in your project directory. This file can be named, for example, .air.toml or air.conf. Here's a sample configuration file that works with Fiber:
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
3. Start your Fiber application using Air by running the following command in the terminal:
```sh
air
```

As you make changes to your source code, Air will detect them and automatically restart the application.

A complete example demonstrating the use of Air with Fiber can be found in the [Fiber Recipes repository](https://github.com/gofiber/recipes/tree/master/air). This example shows how to configure and use Air in a Fiber project to create an efficient development environment.


## How do I set up an error handler?

To override the default error handler, you can override the default when providing a [Config](../api/fiber.md#config) when initiating a new [Fiber instance](../api/fiber.md#new).

```go title="Example"
app := fiber.New(fiber.Config{
    ErrorHandler: func(c *fiber.Ctx, err error) error {
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
* [handlebars](https://docs.gofiber.io/template/handlebars)
* [html](https://docs.gofiber.io/template/html)
* [jet](https://docs.gofiber.io/template/jet)
* [mustache](https://docs.gofiber.io/template/mustache)
* [pug](https://docs.gofiber.io/template/pug)
* [slim](https://docs.gofiber.io/template/pug)

To learn more about using Templates in Fiber, see [Templates](../guide/templates.md).

## Does Fiber have a community chat?

Yes, we have our own [Discord ](https://gofiber.io/discord)server, where we hang out. We have different rooms for every subject.  
If you have questions or just want to have a chat, feel free to join us via this **&gt;** [**invite link**](https://gofiber.io/discord) **&lt;**.

![](/img/support-discord.png)

## Does fiber support sub domain routing ?

Yes we do, here are some examples: 
This example works v2
```go
package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
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
	api.Get("/", func(c *fiber.Ctx) error {
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
	blog.Get("/", func(c *fiber.Ctx) error {
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
	site.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Website")
	})
	// Server
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
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
If more information is needed, please refer to this issue [#750](https://github.com/gofiber/fiber/issues/750)
