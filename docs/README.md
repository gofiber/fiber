<!--
docsify init ./docs
docsify serve ./docs
-->
<img src="logo.jpg" width="150" alt="accessibility text"><br><br>
[![GoDoc](https://godoc.org/github.com/fenny/fiber?status.svg)](http://godoc.org/github.com/fenny/fiber) [![fuzzit](https://app.fuzzit.dev/badge?org_id=fiber&branch=master)](https://fuzzit.dev) [![Go Report](https://goreportcard.com/badge/github.com/fenny/fiber)](https://goreportcard.com/report/github.com/fenny/fiber) [![Join the chat at https://gitter.im/FaradayRF/Lobby](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/fiber-chat/community)<br><br>
**Fiber** is a router framework build on top of [FastHTTP](https://github.com/valyala/fasthttp), the fastest HTTP package for **Go**.<br>
This library is inspired by [Expressjs](https://github.com/expressjs/fiber), one of the most populair and well known web framework for **Nodejs**.

### Getting started

#### Installing
Assuming you’ve already installed Golang, install the Fiber package by calling the following command:
```shell
$ go get -u github.com/fenny/fiber
```

#### Hello world
Embedded below is essentially the simplest Fiber app you can create.
```shell
$ create server.go
```
```go
package main

import "github.com/fenny/fiber"

func main() {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) {
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

Each route can have one handler function, that is executed when the route is matched.

Route definition takes the following structures:

```go
app.Method(handler)
app.Method(path, handler)
```

* **app** is an instance of **fiber**.
* **Method** is an [HTTP request method](https://en.wikipedia.org/wiki/Hypertext_Transfer_Protocol#Request_methods), in capitalization: Get, Put, Post etc
* **path** is a path on the server.
* **handler** is a function executed when the route is matched.

This tutorial assumes that an instance of fiber named app is created and the server is running. If you are not familiar with creating an app and starting it, see the [Hello world](#hello-world) example.

The following examples illustrate defining simple routes.  
Respond with Hello, World! on the homepage:
```go
app.Get("/", func(c *fiber.Ctx) {
  c.Send("Hello, World!")
})
```

Respond to POST request on the root route (/), the application’s home page:
```go
app.Post("/", func(c *fiber.Ctx) {
  c.Send("Got a POST request")
})
```

Respond to a PUT request to the /user route:
```go
app.Put("/user", func(c *fiber.Ctx) {
  c.Send("Got a PUT request at /user")
})
```

Respond to a DELETE request to the /user route:
```go
app.Delete("/user", func(c *fiber.Ctx) {
  c.Send("Got a DELETE request at /user")
})
```

#### Static files
To serve static files such as images, CSS files, and JavaScript files, use the express.static built-in middleware function in Express.

The function signature is:
```go
fiber.Static(root string)
```
The root argument specifies the root directory from which to serve static assets.  
You can also specify a single file instead of a directory, for example:

```go
app.Get("/", fiber.Static("./public"))
app.Get("/", fiber.Static("./public/index.html"))
```
Now, you can load the files that are in the public directory:
```bash
http://localhost:8080/images/kitten.jpg
http://localhost:8080/css/style.css
http://localhost:8080/js/app.js
http://localhost:8080/images/bg.png
http://localhost:8080/hello.html
http://localhost:8080 # => serves "index.html"
```

Fiber looks up the files relative to the static directory, so the name of the static directory is not part of the URL.  
To create a virtual path prefix (where the path does not actually exist in the file system) for files that are served by the fiber.Static function, specify a mount path for the static directory, as shown below:
```go
app.Get("/css", fiber.Static("./build/css/minified"))
```


### Application
The app object conventionally denotes the Fiber application.
#### Initialize
Creates an Fiber instance.
```go
app := fiber.New()
```
#### Settings
You can pass any of the Fasthttp server settings via the Fiber instance.  
Make sure that you set these settings before calling the .Listen() method.  

The following values are default.  
I suggest you only play with these settings if you know what you are doing.
```go
app := fiber.New()

app.TLSEnable = false,
app.CertKey = ""
app.CertFile = ""
app.Name = ""
app.Concurrency = 256 * 1024
app.DisableKeepAlive = false
app.ReadBufferSize = 4096
app.WriteBufferSize = 4096
app.WriteTimeout = 0
app.IdleTimeout = 0
app.MaxConnsPerIP = 0
app.MaxRequestsPerConn = 0
app.TCPKeepalive = false
app.TCPKeepalivePeriod = 0
app.MaxRequestBodySize = 4 * 1024 * 1024
app.ReduceMemoryUsage = false
app.GetOnly = false
app.DisableHeaderNamesNormalizing = false
app.SleepWhenConcurrencyLimitsExceeded = 0
app.NoDefaultServerHeader = true
app.NoDefaultContentType = false
KeepHijackedConns = false
```
#### Methods
Routes an HTTP request, where METHOD is the HTTP method of the request, such as GET, PUT, POST, and so on, in lowercase. Thus, the actual methods are app.get(), app.post(), app.put(), and so on. See Routing methods below for the complete list.
```go
app.Get("/",      func(c *fiber.Ctx) {})
app.Put("/",      func(c *fiber.Ctx) {})
app.Post("/",     func(c *fiber.Ctx) {})
app.Delete("/",   func(c *fiber.Ctx) {})
app.Head("/",     func(c *fiber.Ctx) {})
app.Patch("/",    func(c *fiber.Ctx) {})
app.Options("/",  func(c *fiber.Ctx) {})
app.Trace("/",    func(c *fiber.Ctx) {})
app.Connect("/",  func(c *fiber.Ctx) {})
// Matches all HTTP verbs
app.All("/",      func(c *fiber.Ctx) {})
```
#### Listen
Binds and listens for connections on the specified host and port.
```go
app.Listen(8080)
app.Listen("127.0.0.1", 8080)
```
### Routing

#### Paths
Route paths, in combination with a request method, define the endpoints at which requests can be made. Route paths can be strings, string patterns, or regular expressions.

The characters ?, +, "8", and () are subsets of their regular expression counterparts. The hyphen (-) and the dot (.) are interpreted literally by string-based paths.

If you need to use the dollar character ($) in a path string, enclose it escaped within ([ and ]). For example, the path string for requests at “/data/$book”, would be “/data/([\$])book”.

#### Parameters
Route parameters are named URL segments that are used to capture the values specified at their position in the URL. The captured values are populated in the req.params object, with the name of the route parameter specified in the path as their respective keys.

#### Middleware
You can provide multiple callback functions that behave like middleware to handle a request. The only exception is that these callbacks might invoke next('route') to bypass the remaining route callbacks. You can use this mechanism to impose pre-conditions on a route, then pass control to subsequent routes if there’s no reason to proceed with the current route.

Route handlers can be in the form of a function, an array of functions, or combinations of both, as shown in the following examples.

A single callback function can handle a route. For example:
### Context
The ctx object represents the HTTP request and response and has methods for the request query string, parameters, body, HTTP headers, and so on. In this documentation and by convention, the context is always referred to as c but its actual name is determined by the parameters to the callback function in which you’re working.
#### Accepts

#### Attachment
Sets the HTTP response Content-Disposition header field to “attachment”. If a filename is given, then it sets the Content-Type based on the extension name via res.type(), and sets the Content-Disposition “filename=” parameter.
```go
app.Get("/", func(c *fiber.Ctx) {
  c.Attachment()
  // Content-Disposition: attachment
  c.Attachment("./static/img/logo.png")
  // Content-Disposition: attachment; filename="logo.png"
  // Content-Type: image/png
})
```
#### Body
Contains key-value pairs of data submitted in the request body. By default, it is undefined, and is populated when you use body-parsing middleware such as express.json() or express.urlencoded().

The following example shows how to use body-parsing middleware to populate req.body.
```go
app.Post("/", func(c *fiber.Ctx) {
  c.Body()
  c.Body("param1")
  c.Body(func(key string, val string) {
    fmt.Printl(key, val)  
  })
})
```
#### ClearCookies
Clears all cookies, or a specific cookie by name.
```go
app.Get("/", func(c *fiber.Ctx) {
  // Delete all cookies from client side
  c.ClearCookies()

  // Delete specific cookie
  c.ClearCookies("name")
})
```

#### Cookies
Clears all cookies, or a specific cookie by name.
```go
app.Get("/", func(c *fiber.Ctx) {
  // Returns Cookie header value
  c.Cookies()

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
app.Get("/", func(c *fiber.Ctx) {
  c.Get("Content-Type")
  // "text/plain"

  c.Get("content-type")
  // "text/plain"

  c.Get("something")
  // ""
})
```
#### Hostname
Contains the hostname derived from the Host HTTP header.
```go
app.Get("/", func(c *fiber.Ctx) {
  // Host: "localhost:8080"
  c.Hostname()
  // => "localhost"
})
```
#### IP
Contains the remote IP address of the request.
```go
app.Get("/", func(c *fiber.Ctx) {
  c.IP()
  // "127.0.0.1"
})
```
#### Is
Returns the matching content type if the incoming request’s “Content-Type” HTTP header field matches the MIME type specified by the type parameter. If the request has no body, returns null. Returns false otherwise.
```go
app.Get("/", func(c *fiber.Ctx) {
  // Content-Type: text/html; charset=utf-8
  c.Is("html")  // => true
  c.Is(".html") // => true
  c.Is("json")  // => false
})
```
#### Json
#### Jsonp
#### Method
Contains a string corresponding to the HTTP method of the request: GET, POST, PUT, and so on.
```go
app.Post("/", func(c *fiber.Ctx) {
  c.Method() // => "POST"
})
```
#### OriginalUrl
#### Params
#### Path
#### Protocol
#### Query
This property is an object containing a property for each query string parameter in the route. If there is no query string, it returns an empty string
```go
app.Get("/", func(c *fiber.Ctx) {
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
app.Get("/", func(c *fiber.Ctx) {
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
app.Get("/", func(c *fiber.Ctx) {
  c.Set("Content-Type", "text/plain")
})
```
#### Status
Sets the HTTP status for the response. It is a chainable alias of Node’s response.statusCode.
```go
app.Get("/", func(c *fiber.Ctx) {
  c.Status(403)
  c.Status(400).Send("Bad Request")
  c.Status(404).SendFile("/absolute/path/to/404.png")
})
```
#### Type
Sets the Content-Type HTTP header to the MIME type as determined by mime.lookup() for the specified type. If type contains the “/” character, then it sets the Content-Type to type.
```go
app.Get("/", func(c *fiber.Ctx) {
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
