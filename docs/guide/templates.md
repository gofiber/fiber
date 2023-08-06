---
id: templates
title: 📝 Templates
description: Fiber supports server-side template engines.
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Template interfaces

Fiber provides a Views interface to provide your own template engine:

<Tabs>
<TabItem value="views" label="Views">

```go
type Views interface {
    Load() error
    Render(io.Writer, string, interface{}, ...string) error
}
```
</TabItem>
</Tabs>

`Views` interface contains a `Load` and `Render` method, `Load` is executed by Fiber on app initialization to load/parse the templates.

```go
// Pass engine to Fiber's Views Engine
app := fiber.New(fiber.Config{
    Views: engine,
    // Views Layout is the global layout for all template render until override on Render function.
    ViewsLayout: "layouts/main"
})
```

The `Render` method is linked to the [**ctx.Render\(\)**](../api/ctx.md#render) function that accepts a template name and binding data. It will use global layout if layout is not being defined in `Render` function.
If the Fiber config option `PassLocalsToViews` is enabled, then all locals set using `ctx.Locals(key, value)` will be passed to the template.

```go
app.Get("/", func(c *fiber.Ctx) error {
    return c.Render("index", fiber.Map{
        "hello": "world",
    });
})
```

## Engines

Fiber team maintains [templates](https://docs.gofiber.io/template) package that provides wrappers for multiple template engines:

* [ace](https://docs.gofiber.io/template/ace/)
* [amber](https://docs.gofiber.io/template/amber/)
* [django](https://docs.gofiber.io/template/django/)
* [handlebars](https://docs.gofiber.io/template/handlebars)
* [html](https://docs.gofiber.io/template/html)
* [jet](https://docs.gofiber.io/template/jet)
* [mustache](https://docs.gofiber.io/template/mustache)
* [pug](https://docs.gofiber.io/template/pug)
* [slim](https://docs.gofiber.io/template/slim)

<Tabs>
<TabItem value="example" label="Example">

```go
package main

import (
    "log"
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/template/html/v2"
)

func main() {
    // Initialize standard Go html template engine
    engine := html.New("./views", ".html")
    // If you want other engine, just replace with following
    // Create a new engine with django
	// engine := django.New("./views", ".django")

    app := fiber.New(fiber.Config{
        Views: engine,
    })
    app.Get("/", func(c *fiber.Ctx) error {
        // Render index template
        return c.Render("index", fiber.Map{
            "Title": "Hello, World!",
        })
    })

    log.Fatal(app.Listen(":3000"))
}
```
</TabItem>
<TabItem value="index" label="views/index.html">

```markup
<!DOCTYPE html>
<body>
    <h1>{{.Title}}</h1>
</body>
</html>
```
</TabItem>
</Tabs>
