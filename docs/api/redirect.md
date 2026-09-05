---
id: redirect
title: 🔄 Redirect
description: Fiber's built-in redirect package
sidebar_position: 5
toc_max_heading_level: 5
---

Redirect helpers send the client to another URL or route.

## Redirect Methods

### To

Redirects to a URL built from the given path. Optionally set an HTTP [status](#status).

:::info
If unspecified, status defaults to **303 See Other**.
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
  // => HTTP - GET 303 /foo/bar
  return c.Redirect().To("/foo/bar")
  // => HTTP - GET 303 ../login
  return c.Redirect().To("../login")
  // => HTTP - GET 303 http://example.com
  return c.Redirect().To("http://example.com")
  // => HTTP - GET 301 https://example.com
  return c.Redirect().Status(301).To("http://example.com")
})
```

### Route

Redirects to a named route with parameters and queries.

:::info
To send params and queries to a route, use the [`RedirectConfig`](#redirectconfig) struct.
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
  return c.Redirect().Route("user", fiber.RedirectConfig{
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

:::note
Each `Params` value is automatically percent-encoded with URL path-segment
rules. Delimiters such as `/`, `?`, and `#` therefore remain parameter data
instead of adding a path segment, query, or fragment, and cannot absorb or
discard `Queries`. [`Route.URL`](./app.md#getroute) and
[`GetRouteURL`](./ctx.md#getrouteurl) apply the same encoding.
:::

### Back

Redirects to the referer. If it's missing, fall back to the provided URL. You can also set the status code.

:::info
If unspecified, status defaults to **303 See Other**.
:::

:::note

The referer is only used when it is same-origin with the current request, and
the value sent in `Location` is a normalized form of it, not the raw header.
The check reads the referer the way a browser does — folding backslashes,
dropping ASCII tab, CR and LF, and collapsing a leading run of slashes — so a
value like `/\evil.com`, which a browser resolves to `//evil.com`, is not
mistaken for a local path. A referer that is missing, cross-origin, or
unparsable falls back to the provided URL, and `Back` returns an error if
there is no fallback.

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
Methods are **chainable**.
:::

### Status

Sets the HTTP status code for the redirect.

:::info
It is used in conjunction with [**To**](#to), [**Route**](#route), and [**Back**](#back) methods.
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
It is used in conjunction with the [**Route**](#route) method.
:::

```go title="Definition"
// RedirectConfig is a config to use with Redirect().Route()
type RedirectConfig struct {
  Params  fiber.Map         // Route parameters
  Queries map[string]string // Query map
}
```

### Flash Message

Similar to [Laravel](https://laravel.com/docs/11.x/redirects#redirecting-with-flashed-session-data), we can flash a message and retrieve it in the next request.

:::note

Flash messages travel in the `fiber_flash` cookie, so only a client that keeps cookies across the redirect (a browser, typically) receives them, and every request is scanned for the cookie. Deployments whose clients keep no cookies can turn the feature off with `fiber.Config{DisableFlashMessages: true}`: `With` and `WithInput` then set no cookie, `Messages` and `OldInput` report nothing, and the scan is skipped.

:::

#### Messages

Retrieve all flash messages. See [With](#with) for details.

```go title="Signature"
func (r *Redirect) Messages() []FlashMessage
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  messages := c.Redirect().Messages()
  return c.JSON(messages)
})
```

#### Message

Get a flash message by key; see [With](#with).

```go title="Signature"
func (r *Redirect) Message(key string) FlashMessage
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  message := c.Redirect().Message("status")
  return c.SendString(message.Value)
})
```

#### OldInputs

Retrieve stored input data. See [WithInput](#withinput).

```go title="Signature"
func (r *Redirect) OldInputs() []OldInputData
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  oldInputs := c.Redirect().OldInputs()
  return c.JSON(oldInputs)
})
```

#### OldInput

Get stored input data by key; see [WithInput](#withinput).

```go title="Signature"
func (r *Redirect) OldInput(key string) OldInputData
```

```go title="Example"
app.Get("/name", func(c fiber.Ctx) error {
  oldInput := c.Redirect().OldInput("name")
  return c.SendString(oldInput.Value)
})
```

#### With

Send flash messages with `With`.

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

Send input data with `WithInput`, which stores them in a cookie.

It captures form, multipart, or query data depending on the request content type.

:::caution

`WithInput` copies the whole submitted body into the `fiber_flash` cookie — a
rejected login carries the password the user just typed. The payload is
hex-encoded, not encrypted or signed, so anything sensitive in the request goes
back to the browser as a cookie value. Prefer `With` for the specific fields you
need to carry across the redirect.

The cookie is `HttpOnly`, and `Secure` when the request that produced it arrived
over TLS — so script cannot read it and it is not replayed in the clear. Neither
attribute protects the payload from whoever is at the browser.

:::

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
