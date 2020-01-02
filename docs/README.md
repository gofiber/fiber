<!--
docsify init ./docs
docsify serve ./docs
-->

[![GoDoc](https://godoc.org/github.com/fenny/fastex?status.svg)](http://godoc.org/github.com/fenny/fastex) [![fuzzit](https://app.fuzzit.dev/badge?org_id=fastex&branch=master)](https://fuzzit.dev) [![Go Report](https://goreportcard.com/badge/github.com/fenny/fastex)](https://goreportcard.com/report/github.com/fenny/fastex) [![Join the chat at https://gitter.im/FaradayRF/Lobby](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/fastex-chat/community)<br>  
<img src="https://i.imgur.com/31gxky7.jpg" width="150" alt="accessibility text"><br>  
**Fiber** is a router framework build on top of [FastHTTP](https://github.com/valyala/fasthttp), the fastest HTTP package for **Go**.<br>
This library is inspired by [fiber](https://github.com/fiberjs/fiber), one of the most populair and well known web frameworks for **Nodejs**.

### Getting started

#### Installing

```shell
$ go get -u github.com/fenny/fiber
```

#### Hello world
```shell
$ create server.go
```
```go
package main

import "github.com/fenny/fiber"

func main {
  app := fiber.New()
  app.Get("/", func(c *fiber.Context) {
    c.Send("Hello, World!")
  })
  app.Listen(8080)
}
```
```shell
$ go run server.go
```
Browse to http://localhost:8080 and you should see Hello, World! on the page.

#### Basic routing

Routing refers to determining how an application responds to a client request to a particular endpoint, which is a URI (or path) and a specific HTTP request method (GET, POST, and so on).

Each route can have one or more handler functions, which are executed when the route is matched.

Route definition takes the following structures:

```go
app.Method(Handler)
app.Method(Path, Handler)
```

* **app** is an instance of **fastex**.
* **Method** is an [HTTP request method](https://en.wikipedia.org/wiki/Hypertext_Transfer_Protocol#Request_methods), in capitalization: Get, Put, Post etc
* **Path** is a path on the server.
* **Handler** is a function executed when the route is matched.

This tutorial assumes that an instance of fiber named app is created and the server is running. If you are not familiar with creating an app and starting it, see the [Hello world](#hello-world) example.

The following examples illustrate defining simple routes.  
Respond with Hello World! on the homepage:
```go
app.Get("/", func(c *fiber.Context) {
  c.Send("Hello, World!")
})
```

Respond to POST request on the root route (/), the application’s home page:
```go
app.Post("/", func(c *fiber.Context) {
  c.Send("Got a POST request")
})
```

Respond to a PUT request to the /user route:
```go
app.Put("/user", func(c *fiber.Context) {
  c.Send("Got a PUT request at /user")
})
```

Respond to a DELETE request to the /user route:
```go
app.Delete("/user", func(c *fiber.Context) {
  c.Send("Got a DELETE request at /user")
})
```

#### Static files
Input a folder or file
```go
app.Get("/", fiber.Static("./static"))
app.Get("/css", fiber.Static("./public/compiled/css"))

// Or if you work with pushstates
app.Get("*", fiber.Static("./static/index.html"))
```
### Application

#### Initialize
#### Settings
#### Methods
#### Listen

### Routing

### Context

#### Accepts

#### Attachment
Sets the HTTP response Content-Disposition header field to “attachment”. If a filename is given, then it sets the Content-Type based on the extension name via res.type(), and sets the Content-Disposition “filename=” parameter.
```go
app.Get("/", func(c *fiber.Context) {
  c.Attachment()
  // Content-Disposition: attachment
  c.Attachment("./static/img/logo.png")
  // Content-Disposition: attachment; filename="logo.png"
  // Content-Type: image/png
})
```
#### BaseUrl

#### Body

#### ClearCookies
```go
app.Get("/", func(c *fiber.Context) {
  // Delete all cookies from client side
  c.ClearCookies()

  // Delete specific cookie
  c.ClearCookies("name")
})
```

#### Cookies
```go
app.Get("/", func(c *fiber.Context) {
  // Create cookie name=john
  c.Cookies("name", "john")

  // Get cookie value by key
  c.Cookies("name")

  // Show all cookies
  c.Cookies(func(key string, val string) {
    fmt.Println(key, val)
  })
})
```
#### Download
#### Get
Returns the HTTP response header specified by field. The match is case-insensitive.
```go
app.Get("/", func(c *fiber.Context) {
  c.Get("Content-Type")
  // "text/plain"
})
```
#### Hostname
#### Ip
Contains the remote IP address of the request.

When the trust proxy setting does not evaluate to false, the value of this property is derived from the left-most entry in the X-Forwarded-For header. This header can be set by the client or by the proxy.
```go
app.Get("/", func(c *fiber.Context) {
  c.Send(c.Ip())
  // "127.0.0.1"
})
```
#### Is
#### Json
#### Jsonp
#### Method
#### OriginalUrl
#### Params
#### Path
#### Protocol
#### Query
This property is an object containing a property for each query string parameter in the route. If there is no query string, it returns an empty string
```go
app.Get("/", func(c *fiber.Context) {
  // GET /search?q=tobi+ferret
  c.Query("q")
  // => "tobi ferret"

  // GET /shoes?order=desc&shoe[color]=blue&shoe[type]=converse
  c.Query("order")
  // => "desc"
})
```
#### Redirect
Redirects to the URL derived from the specified path, with specified status, a positive integer that corresponds to an HTTP status code . If not specified, status defaults to “302 “Found”.
```go
app.Get("/", func(c *fiber.Context) {
  c.Redirect("/foo/bar")
  c.Redirect("http://example.com")
  c.Redirect(301, "http://example.com")
  c.Redirect("../login")
})
```
#### Jsonp
#### Send
#### SendBytes
#### SendFile
#### SendString
#### Set
Sets the response’s HTTP header field to value. To set multiple fields at once, pass an object as the parameter.
```go
app.Get("/", func(c *fiber.Context) {
  c.Set("Content-Type", "text/plain")
})
```
#### Status
Sets the HTTP status for the response. It is a chainable alias of Node’s response.statusCode.
```go
app.Get("/", func(c *fiber.Context) {
  c.Status(403)
  c.Status(400).Send("Bad Request")
  c.Status(404).SendFile("/absolute/path/to/404.png")
})
```
#### Type
Sets the Content-Type HTTP header to the MIME type as determined by mime.lookup() for the specified type. If type contains the “/” character, then it sets the Content-Type to type.
```go
app.Get("/", func(c *fiber.Context) {
  c.Type(".html")
  // => 'text/html'
  c.Type("html")
  // => 'text/html'
  c.Type("json")
  // => 'application/json'
  c.Type("application/json")
  // => 'application/json'
  c.Type("png")
  // => 'image/png'
})
```
#### WriteBytes
#### WriteString
#### Xhr

### License

MIT © [Kiko Beats](https://kikobeats.com)
