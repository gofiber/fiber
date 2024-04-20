---
id: redirect
title: ↪️ Redirect
description: Fiber's built-in redirect package
sidebar_position: 5
toc_max_heading_level: 5
---

Is used to redirect the ctx(request) to a different URL/Route.

## Redirect Methods

### To

Redirects to the URL derived from the specified path, with specified [status](#status), a positive integer that
corresponds to an HTTP status code.

:::info
If **not** specified, status defaults to **302 Found**.
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

Redirects to the specific route along with the parameters and queries.

:::info
If you want to send queries and params to route, you must use the [**RedirectConfig**](#redirectconfig) struct.
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

Redirects back to refer URL. It redirects to fallback URL if refer header doesn't exists, with specified status, a
positive integer that corresponds to an HTTP status code.

:::info
If **not** specified, status defaults to **302 Found**.
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
Method are **chainable**.
:::

### Status

Sets the HTTP status code for the redirect.

:::info
Is used in conjunction with [**To**](#to), [**Route**](#route) and [**Back**](#back) methods.
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

Sets the configuration for the redirect.

:::info
Is used in conjunction with the [**Route**](#route) method.
:::

```go
// RedirectConfig A config to use with Redirect().Route()
type RedirectConfig struct {
  Params  fiber.Map         // Route parameters
  Queries map[string]string // Query map
}
```

### Flash Message

Similar to [Laravel](https://laravel.com/docs/11.x/redirects#redirecting-with-flashed-session-data) we can flash a message and retrieve it in the next request.

#### Messages

Get flash messages. Check [With](#with) for more information.

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

Get flash message by key. Check [With](#with) for more information.

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

Get old input data. Check [WithInput](#withinput) for more information.

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

Get old input data by key. Check [WithInput](#withinput) for more information.

```go title="Signature"
func (r *Redirect) OldInput(key string) string
```

```go title="Example"
app.Get("/name", func(c fiber.Ctx) error {
  oldInput := c.Redirect().OldInput("name")
  return c.SendString(oldInput)
})
```

#### With

You can send flash messages by using `With()`.

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

You can send input data by using `WithInput()`.
They will be sent as a cookie.

This method can send form, multipart form, query data to redirected route depending on the request content type.

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
