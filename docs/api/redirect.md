---
id: redirect
title: 🔄 Redirect
description: Fiber's built-in redirect package
sidebar_position: 5
toc_max_heading_level: 5
---

Fiber's built-in redirect package provides methods to redirect the client to a different URL or route. This can be useful for various purposes, such as redirecting users after form submissions, handling outdated links, or structuring a more user-friendly navigation flow.

## Redirect Methods

### To

Redirects the client to the URL derived from the specified path, with an optional HTTP status code.

:::info
If the status is **not** specified, it defaults to  **302 Found**.
:::

```go title="Signature"
func (r *Redirect) To(location string) error
```

```go title="Example"
app.Get("/coffee", func(c fiber.Ctx) error {
    // => HTTP - GET 301 /teapot 
    return c.Redirect().Status(fiber.StatusMovedPermanently).To("/teapot")
})

app.Get("/teapot", func(c fiber.Ctx) error {
    return c.Status(fiber.StatusTeapot).Send("🍵 short and stout 🍵")
})
```

```go title="More examples"
app.Get("/", func(c fiber.Ctx) error {
    // => HTTP - GET 302 /foo/bar 
    return c.Redirect().To("/foo/bar")
    // => HTTP - GET 302 ../login
    return c.Redirect().To("../login")
    // => HTTP - GET 302 http://example.com
    return c.Redirect().To("http://example.com")
    // => HTTP - GET 301 https://example.com
    return c.Redirect().Status(301).To("http://example.com")
})
```

### Route

Redirects the client to a specific route, along with any parameters and queries.

:::info
To send queries and parameters to the route, use the [**RedirectConfig**](#redirectconfig) struct.
:::

```go title="Signature"
func (r *Redirect) Route(name string, config ...RedirectConfig) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    // /user/fiber
    return c.Redirect().Route("user", fiber.RedirectConfig{
        Params: fiber.Map{
          "name": "fiber",
        },
    })
})

app.Get("/with-queries", func(c fiber.Ctx) error {
    // /user/fiber?data[0][name]=john&data[0][age]=10&test=doe
    return c.Route("user", RedirectConfig{
        Params: fiber.Map{
          "name": "fiber",
        },
        Queries: map[string]string{
          "data[0][name]": "john",
          "data[0][age]":  "10",
          "test":          "doe",
        },
  })
})

app.Get("/user/:name", func(c fiber.Ctx) error {
    return c.SendString(c.Params("name"))
}).Name("user")
```

### Back

Redirects the client back to the referring URL. If the referer header does not exist, it redirects to a specified fallback URL, with an optional HTTP status code.

:::info
If the status is **not** specified, it defaults to **302 Found**.
:::

```go title="Signature"
func (r *Redirect) Back(fallback string) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Home page")
})
app.Get("/test", func(c fiber.Ctx) error {
    c.Set("Content-Type", "text/html")
    return c.SendString(`<a href="/back">Back</a>`)
})

app.Get("/back", func(c fiber.Ctx) error {
    return c.Redirect().Back("/")
})
```

## Controls

:::info
The control methods are **chainable**.
:::

### Status

Sets the HTTP status code for the redirect.

:::info
This method is used in conjunction with [**To**](#to), [**Route**](#route) and [**Back**](#back) methods.
:::

```go title="Signature"
func (r *Redirect) Status(status int) *Redirect
```

```go title="Example"
app.Get("/coffee", func(c fiber.Ctx) error {
    // => HTTP - GET 301 /teapot 
    return c.Redirect().Status(fiber.StatusMovedPermanently).To("/teapot")
})
```

### RedirectConfig

Sets the configuration for the redirect, including route parameters, query strings and cookie configuration.

:::info
Is used in conjunction with the [**Route**](#route) method.
:::

```go
// RedirectConfig A config to use with Redirect().Route()
type RedirectConfig struct {
    Params          fiber.Map           // Route parameters
    Queries         map[string]string   // Query map
    CookieConfig    CookieConkie        // Cookie configuration
}
```

### Cookie Configuration

The `CookieConfig` struct holds the configuration for cookies used in redirects, particularly for flash messages and old input data.

```go
type CookieConfig struct {
    Name     string
    HTTPOnly bool
    Secure   bool
    SameSite string
}
```

#### Default Configuration

The default cookie configuration is as follows:

```go
var CookieConfigDefault = CookieConfig{
    Name:     "fiber_flash",
    HTTPOnly: true,
    Secure:   false,
    SameSite: "Lax",
}
```

### Flash Message

Similar to [Laravel](https://laravel.com/docs/11.x/redirects#redirecting-with-flashed-session-data), Fiber allows you to flash messages and retrieve them in the next request.

#### Messages

Retrieves all the messages.

```go title="Signature"
func (r *Redirect) Messages() map[string]string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    messages := c.Redirect().Messages()
    return c.JSON(messages)
})
```

#### Message

Retrieve a flash message by key.

```go title="Signature"
func (r *Redirect) Message(key string) *Redirect
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    message := c.Redirect().Message("status")
    return c.SendString(message)
})
```

#### OldInputs

Retrieves all old input data.

```go title="Signature"
func (r *Redirect) OldInputs() map[string]string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    oldInputs := c.Redirect().OldInputs()
    return c.JSON(oldInputs)
})
```

#### OldInput

Retrieves old input data by key.

```go title="Signature"
func (r *Redirect) OldInput(key string) string
```

```go title="Example"
app.Get("/name", func(c fiber.Ctx) error {
    soldInput := c.Redirect().OldInput("name")
    return c.SendString(oldInput)
})
```

#### With

Sends flash messages using `With()`.

```go title="Signature"
func (r *Redirect) With(key, value string) *Redirect
```

```go title="Example"
app.Get("/login", func(c fiber.Ctx) error {
    return c.Redirect().With("status", "Logged in successfully").To("/")
})

app.Get("/", func(c fiber.Ctx) error {
    // => Logged in successfully
    return c.SendString(c.Redirect().Message("status"))
})
```

#### WithInput

Sends input data using `WithInput()`. This method can send form, multipart form, or query data to the redirected route, depending on the request content type. The default backend is using cookies.

```go title="Signature"
func (r *Redirect) WithInput() *Redirect
```

```go title="Example"
// curl -X POST http://localhost:3000/login -d "name=John"
app.Post("/login", func(c fiber.Ctx) error {
    return c.Redirect().WithInput().Route("name")
})

app.Get("/name", func(c fiber.Ctx) error {
    // => John
    return c.SendString(c.Redirect().OldInput("name"))
}).Name("name")
```
