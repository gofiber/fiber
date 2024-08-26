---
id: templates
title: 📝 Templates
description: Fiber supports server-side template engines.
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Templates are a great tool to render dynamic content without using a separate frontend framework.

## Template Engines

Fiber allows you to provide a custom template engine at app initialization.

```go
app := fiber.New(fiber.Config{
    // Pass in Views Template Engine
    Views: engine,

    // Default global path to search for views (can be overriden when calling Render())
    ViewsLayout: "layouts/main",

    // Enables/Disables access to `ctx.Locals()` entries in rendered views
    // (defaults to false)
    PassLocalsToViews: false,
})
```


### Supported Engines

The Fiber team maintains a [templates](https://docs.gofiber.io/template) package that provides wrappers for multiple template engines:

* [ace](https://docs.gofiber.io/template/ace/)
* [amber](https://docs.gofiber.io/template/amber/)
* [django](https://docs.gofiber.io/template/django/)
* [handlebars](https://docs.gofiber.io/template/handlebars)
* [html](https://docs.gofiber.io/template/html)
* [jet](https://docs.gofiber.io/template/jet)
* [mustache](https://docs.gofiber.io/template/mustache)
* [pug](https://docs.gofiber.io/template/pug)
* [slim](https://docs.gofiber.io/template/slim)

:::info
Custom template engines can implement the `Views` interface to be supported in Fiber.
:::


```go title="Views interface"
type Views interface {
    // Fiber executes Load() on app initialization to load/parse the templates
    Load() error

    // Outputs a template to the provided buffer using the provided template,
    // template name, and binded data
    Render(io.Writer, string, interface{}, ...string) error
}
```

:::note
The `Render` method is linked to the [**ctx.Render\(\)**](../api/ctx.md#render) function that accepts a template name and binding data.
:::


## Rendering Templates

Once an engine is set up, a route handler can call the [**ctx.Render\(\)**](../api/ctx.md#render) function with a template name and binded data to send the rendered template.

```go title="Signature"
func (c *Ctx) Render(name string, bind Map, layouts ...string) error
```

:::info
By default, [**ctx.Render\(\)**](../api/ctx.md#render) searches for the template name in the `ViewsLayout` path. To override this setting, provide the path(s) in the `layouts` argument
:::


<Tabs>

<TabItem value="example" label="Example">

```go
app.Get("/", func(c *fiber.Ctx) error {
    return c.Render("index", fiber.Map{
        "Title": "Hello, World!",
    })

})
```

</TabItem>

<TabItem value="index" label="layouts/index.html">

```html
<!DOCTYPE html>
<html>
    <body>
        <h1>{{.Title}}</h1>
    </body>
</html>
```

</TabItem>

</Tabs>

:::caution
If the Fiber config option `PassLocalsToViews` is enabled, then all locals set using `ctx.Locals(key, value)` will be passed to the template. It is important to avoid clashing keys when using this setting.
:::

