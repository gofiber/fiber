---
id: templates
title: 📝 Templates
description: Fiber supports server-side template engines.
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Templates render dynamic content without requiring a separate frontend framework.

## Template Engines

Fiber accepts a custom template engine during app initialization.

```go
app := fiber.New(fiber.Config{
    // Provide a template engine
    Views: engine,

    // Default layout for views, overridden by the layouts passed to Render()
    ViewsLayout: "layouts/main",

    // Enables/Disables access to `ctx.Locals()` entries in rendered views
    // (defaults to false)
    PassLocalsToViews: false,
})
```

### Supported Engines

Fiber maintains a [templates](https://docs.gofiber.io/template) package that wraps several engines:

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
Custom engines implement the `Views` interface to work with Fiber.
:::

```go title="Views interface"
type Views interface {
    // Fiber executes Load() in fiber.New() to load/parse the templates;
    // an error is logged as a warning, not returned
    Load() error

    // Outputs a template to the provided buffer using the provided template,
    // template name, and bound data
    Render(io.Writer, string, any, ...string) error
}
```

:::note
The `Render` method powers [**ctx.Render\(\)**](../api/ctx.md#render), which accepts a template name and data to bind.
:::

## Rendering Templates

After configuring an engine, handlers call [**ctx.Render\(\)**](../api/ctx.md#render) with a template name and data to send the rendered output.

```go title="Signature"
func (c fiber.Ctx) Render(name string, bind any, layouts ...string) error
```

:::info
By default, [**ctx.Render\(\)**](../api/ctx.md#render) wraps the template in the layout named by `ViewsLayout`. Pass layout names in the `layouts` argument to use another layout for that call.
:::

Without a `Views` engine, or with a nil engine pointer, `Render` reads the file at `name` and parses it with Go's `html/template`.

<Tabs>
<TabItem value="example" label="Example">

```go
app.Get("/", func(c fiber.Ctx) error {
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
When `PassLocalsToViews` is enabled, all values set using `ctx.Locals(key, value)` are passed to the template. Use unique keys to avoid collisions.
:::

Locals, and the values you add with [`c.ViewBind`](../api/ctx.md#viewbind), are merged into the data only when you pass a `fiber.Map` or `nil` to `Render`. A key in the map you pass wins over a local or a `ViewBind` value with the same name. The merge goes into a copy, so the map you pass is left unchanged and can be shared across requests.

### Rendering Without a Request

[`app.Render`](../api/app.md#render) writes a template through the configured engine to any `io.Writer`, applying `ViewsLayout` when you pass no layout. [`app.ReloadViews`](../api/app.md#reloadviews) runs `Load` again on the engine of the app and of every mounted sub-app, for example after a template changed on disk, and returns `fiber.ErrNoViewEngineConfigured` only when none of them has an engine. `app.Render` returns it when the app itself has none. A nil engine pointer counts as none for both.

### Mounted Apps

A sub-app mounted with [`app.Use`](../api/app.md#mounting) renders its templates with its own `Views` for requests under its mount path. A sub-app without an engine falls back to the nearest app above it that has one. The default layout is the `ViewsLayout` of the innermost app that sets one, and the search stops at the app whose engine renders the template. This needs a literal mount path, taken from the top-level app down: under a parameter, as in `app.Use("/t/:tenant", tenantApp)`, the routes still match and run but the `Views` and `ViewsLayout` of the sub-app, and of the apps mounted inside it, are not used. The nearest enclosing mount with a literal path supplies them, or the top-level app when there is none, and without an engine anywhere `Render` reads `name` as a file path. A sub-app mounted through [`Domain`](../api/app.md#domain) matches a parameterized mount path as a pattern and keeps its own `Views`. `PassLocalsToViews` is read from the top-level app, whatever the sub-app sets.

## Advanced Templating

### Custom Functions

Fiber supports adding custom functions to templates.

#### AddFunc

Adds a global function to all templates.

```go title="Signature"
func (e *Engine) AddFunc(name string, fn any) IEngineCore
```

<Tabs>
<TabItem value="add-func-example" label="AddFunc Example">

```go
// Add `ToUpper` to engine
engine := html.New("./views", ".html")
engine.AddFunc("ToUpper", func(s string) string {
    return strings.ToUpper(s)
})

// Initialize Fiber App
app := fiber.New(fiber.Config{
    Views: engine,
})

app.Get("/", func (c fiber.Ctx) error {
    return c.Render("index", fiber.Map{
        "Content": "hello, World!",
    })
})
```

</TabItem>
<TabItem value="add-func-template" label="views/index.html">

```html
<!DOCTYPE html>
<html>
    <body>
        <p>This will be in {{ToUpper "all caps"}}:</p>
        <p>{{ToUpper .Content}}</p>
    </body>
</html>
```

</TabItem>
</Tabs>

#### AddFuncMap

Adds a Map of functions (keyed by name) to all templates.

```go title="Signature"
func (e *Engine) AddFuncMap(m map[string]any) IEngineCore
```

<Tabs>
<TabItem value="add-func-map-example" label="AddFuncMap Example">

```go
// Add `ToUpper` to engine
engine := html.New("./views", ".html")
engine.AddFuncMap(map[string]any{
    "ToUpper": func(s string) string {
        return strings.ToUpper(s)
    },
})

// Initialize Fiber App
app := fiber.New(fiber.Config{
    Views: engine,
})

app.Get("/", func (c fiber.Ctx) error {
    return c.Render("index", fiber.Map{
        "Content": "hello, world!",
    })
})
```

</TabItem>
<TabItem value="add-func-map-template" label="views/index.html">

```html
<!DOCTYPE html>
<html>
    <body>
        <p>This will be in {{ToUpper "all caps"}}:</p>
        <p>{{ToUpper .Content}}</p>
    </body>
</html>
```

</TabItem>
</Tabs>

* For more advanced template documentation, please visit the [gofiber/template GitHub Repository](https://github.com/gofiber/template).

## Full Example

<Tabs>
<TabItem value="example" label="Example">

```go
package main

import (
    "log"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/template/html/v2"
)

func main() {
    // Initialize standard Go html template engine
    engine := html.New("./views", ".html")
    // If you want to use another engine,
    // just replace with following:
    // Create a new engine with django
    // engine := django.New("./views", ".django")

    app := fiber.New(fiber.Config{
        Views: engine,
    })
    app.Get("/", func(c fiber.Ctx) error {
        // Render index template
        return c.Render("index", fiber.Map{
            "Title": "Go Fiber Template Example",
            "Description": "An example template",
            "Greeting": "Hello, World!",
        });
    })

    log.Fatal(app.Listen(":3000"))
}
```

</TabItem>
<TabItem value="index" label="views/index.html">

```html
<!DOCTYPE html>
<html>
    <head>
        <title>{{.Title}}</title>
        <meta name="description" content="{{.Description}}">
    </head>
<body>
    <h1>{{.Title}}</h1>
        <p>{{.Greeting}}</p>
</body>
</html>
```

</TabItem>
</Tabs>
