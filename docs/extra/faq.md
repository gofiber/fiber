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

Fiber currently supports 8 template engines in our [gofiber/template](https://github.com/gofiber/template) middleware:

* [Ace](https://github.com/yosssi/ace)
* [Amber](https://github.com/eknkc/amber)
* [Django](https://github.com/flosch/pongo2)
* [Handlebars](https://github.com/aymerick/raymond)
* [HTML](https://pkg.go.dev/html/template/)
* [Jet](https://github.com/CloudyKit/jet)
* [Mustache](https://github.com/cbroglie/mustache)
* [Pug](https://github.com/Joker/jade)

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
If more information is needed, please refer to this issue https://github.com/gofiber/fiber/issues/750
