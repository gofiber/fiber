<!--
docsify init ./docs
docsify serve ./docs
-->
<img src="logo.jpg" width="150" alt="accessibility text"><br><br>
[![Latest Release](https://img.shields.io/github/release/fenny/fiber.svg)](https://github.com/fenny/fiber/releases/latest)
[![GoDoc](https://godoc.org/github.com/fenny/fiber?status.svg)](http://godoc.org/github.com/fenny/fiber)
[![Go Report](https://goreportcard.com/badge/github.com/fenny/fiber)](https://goreportcard.com/report/github.com/fenny/fiber)
[![GitHub license](https://img.shields.io/github/license/fenny/fiber.svg)](https://github.com/fenny/fiber/blob/master/LICENSE)
[![Join the chat at https://gitter.im/FiberGo/community](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/FiberGo/community)
<br><br>
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
```go
app.Method(static string)
app.Method(path string, static string)
```

To serve static files such as images, CSS files, and JavaScript files, replace your function handler with a file or directory string.

The function signature is:
```go
app.Method(static)
app.Method(path, static)
```
The root argument specifies the root directory from which to serve static assets.  
You can also specify a single file instead of a directory, for example:

```go
app.Get("./public")
// http://localhost:8080/images/kitten.jpg
// http://localhost:8080/css/style.css
// http://localhost:8080/js/app.js
// http://localhost:8080/images/bg.png
// http://localhost:8080/hello.html
app.Get("/static", "./public")
// http://localhost:8080/static/images/kitten.jpg
// http://localhost:8080/static/css/style.css
// etc
app.Get("/specific/path", "./public/index.html")
// http://localhost:8080/specific/path
// => ./public/index.html
app.Get("*", "./public/index.html")
// http://localhost:8080/my/name/is/jeff
// => ./public/index.html

```
Fiber looks up the files relative to the static directory, so the name of the static directory is not part of the URL.  
To create a virtual path prefix (where the path does not actually exist in the file system) for files that are served by the fiber.Static function, specify a mount path for the static directory, as shown below:
```go
app.Get("/css", "./build/css/minified")
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
// Function signature
app.Method(static string)
app.Method(func(*fiber.Ctx))
app.Method(path string, static string)
app.Method(path string, func(*fiber.Ctx))

// Example
app.Connect("/", handler)
app.Delete("/", handler)
app.Get("/", handler)
app.Head("/", handler)
app.Options("/", handler)
app.Patch("/", handler)
app.Post("/", handler)
app.Put("/", handler)
app.Trace("/", handler)
// Matches all HTTP verbs
app.All("/", handler)
```
#### Listen
Binds and listens for connections on the specified host and port.
```go
// Function signature
app.Listen(port int)
app.Listen(addr string, port int)

// Example
app.Listen(8080)
app.Listen("127.0.0.1", 8080)
```
### Routing

#### Paths
Route paths, in combination with a request method, define the endpoints at which requests can be made. Route paths can be strings, string patterns, or regular expressions.

The characters ?, +, "8", and () are subsets of their regular expression counterparts. The hyphen (-) and the dot (.) are interpreted literally by string-based paths.

Here are some examples of route paths based on strings.

```go
// This route path will match requests to the root route, /.
app.Get("/", func(c *fiber.Ctx) {
  c.Send("root")
})

// This route path will match requests to /about.
app.Get("/about", func(c *fiber.Ctx) {
  c.Send("about")
})

// This route path will match requests to /random.text.
app.Get("/random.text", func(c *fiber.Ctx) {
  c.Send("random.text")
})
```
Here are some examples of route paths based on string patterns.
```go
// This route path will match acd and abcd.
app.Get("/ab?cd", func(c *fiber.Ctx) {
  c.Send("/ab?cd")
})

// This route path will match abcd, abbcd, abbbcd, and so on.
app.Get("/ab+cd", func(c *fiber.Ctx) {
  c.Send("ab+cd")
})

// This route path will match abcd, abxcd, abRANDOMcd, ab123cd, and so on.
app.Get("/ab*cd", func(c *fiber.Ctx) {
  c.Send("ab*cd")
})

// This route path will match /abe and /abcde.
app.Get("/ab(cd)?e", func(c *fiber.Ctx) {
  c.Send("ab(cd)?e")
})
```


#### Parameters
Route parameters are named URL segments that are used to capture the values specified at their position in the URL. The captured values are populated in the req.params object, with the name of the route parameter specified in the path as their respective keys.

#### Middleware
You can provide multiple callback functions that behave like middleware to handle a request. The only exception is that these callbacks might invoke next('route') to bypass the remaining route callbacks. You can use this mechanism to impose pre-conditions on a route, then pass control to subsequent routes if there’s no reason to proceed with the current route.

Route handlers can be in the form of a function, an array of functions, or combinations of both, as shown in the following examples.

A single callback function can handle a route. For example:
### Context
The ctx object represents the HTTP request and response and has methods for the request query string, parameters, body, HTTP headers, and so on. In this documentation and by convention, the context is always referred to as c but its actual name is determined by the parameters to the callback function in which you’re working.

#### Accepts
!> Planned for V2

#### Attachment
Sets the HTTP response Content-Disposition header field to “attachment”. If a filename is given, then it sets the Content-Type based on the extension name via res.type(), and sets the Content-Disposition “filename=” parameter.
```go
// Function signature
c.Attachment()
c.Attachment(file string)

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Attachment()
  // => Content-Disposition: attachment

  c.Attachment("./static/img/logo.png")
  // => Content-Disposition: attachment; filename="logo.png"
  // => Content-Type: image/png
})
```

#### Body
Contains the raw post body submitted in the request.  
Calling a key in body returns a string value if exist or you loop trough the cookies using a function.

The following example shows how to use the body function.
```go
// Function signature
c.Body()
c.Body(key string)
c.Body(func(key string, value string))

// Example
app.Post("/", func(c *fiber.Ctx) {
	// Get the raw body post
  c.Body() // => user=john

	// Get the body value using the key
  c.Body("user") // => "john"

	// Loop trough all body params
  c.Body(func(key string, val string) {
    fmt.Printl(key, val)  // => "user", "john"
  })
})
```

#### ClearCookies
Clears all client cookies, or a specific cookie by name.
```go
// Function signature
c.ClearCookies()
c.ClearCookies(key string)

// Example
app.Get("/", func(c *fiber.Ctx) {
  // Delete all cookies from client side
  c.ClearCookies()

  // Delete specific cookie
  c.ClearCookies("user")
})
```

#### Cookies
Clears all cookies from client, or a specific cookie by name by adjusting the expiration.
```go
// Function signature
c.Cookies()
c.Cookies(key string)
c.Cookies(key string, value string)
c.Cookies(func(key string, value string))

// Example
app.Get("/", func(c *fiber.Ctx) {
	// Create cookie with key, value
	c.Cookies("name", "john") // => Cookie: name=john

	// Get cookie by key
	c.Cookies("name") // => "john"

	// Get raw cookie header
	c.Cookies() // => name=john;

	// Show all cookies
	c.Cookies(func(key string, val string) {
		fmt.Println(key, val) // => "name", "john"
	})
})
```

#### Download
Transfers the file at path as an “attachment”. Typically, browsers will prompt the user for download. By default, the Content-Disposition header “filename=” parameter is path (this typically appears in the browser dialog). Override this default with the filename parameter.
```go
// Function signature
c.Download(path string)
c.Download(path string, filename string)

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Download("./files/report-12345.pdf")
	// => Download report-12345.pdf

  c.Download("./files/report-12345.pdf", "report.pdf")
	// => Download report.pdf
})
```

#### Get
Returns the HTTP response header specified by field. The match is case-insensitive.
```go
// Function signature
c.Get(field string)

// Example
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
// Function signature
c.Hostname()

// Example
app.Get("/", func(c *fiber.Ctx) {
  // Host: "localhost:8080"
  c.Hostname()
  // => "localhost"
})
```

#### IP
Contains the remote IP address of the request.
```go
// Function signature
c.IP()

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.IP()
  // => "127.0.0.1"
})
```

#### Is
Returns the matching content type if the incoming request’s “Content-Type” HTTP header field matches the MIME type specified by the type parameter. If the request has no body, returns false.
```go
// Function signature
c.IP(typ string)

// Example
app.Get("/", func(c *fiber.Ctx) {
	// Content-Type: text/html; charset=utf-8
	c.Is("html")  
	// => true

	c.Is(".html")
	// => true

	c.Is("json")  
	// => false
})
```
#### Json
!> Planned for V2

#### Jsonp
!> Planned for V2

#### Method
Contains a string corresponding to the HTTP method of the request: GET, POST, PUT, and so on.
```go
// Function signature
c.Method()

// Example
app.Post("/", func(c *fiber.Ctx) {
	c.Method()
	// => "POST"
})
```
#### OriginalURL
Contains the original request URL.
```go
// Function signature
c.OriginalURL()

// Example
app.Get("/", func(c *fiber.Ctx) {
	// GET /search?q=something
	c.OriginalURL()
	// => '/search?q=something'
})
```

#### Params
This method can be used to get the route parameters. For example, if you have the route /user/:name, then the “name” property is available as c.Params("name"). This method defaults "".
```go
// Function signature
c.Params(param string)

// Example
app.Get("/user/:name", func(c *fiber.Ctx) {
	// GET /user/tj
	c.Params("name")
	// => "tj"
})
```

#### Path
Contains the path part of the request URL.
```go
// Function signature
c.Path()

// Example
app.Get("/users", func(c *fiber.Ctx) {
	// example.com/users?sort=desc
	c.Path()
	// => "/users"
})
```

#### Protocol
Contains the request protocol string: either http or (for TLS requests) https.

```go
// Function signature
c.Protocol()

// Example
app.Get("/", func(c *fiber.Ctx) {
	c.Protocol()
	// => "http"
})
```
#### Query
This property is an object containing a property for each query string parameter in the route. If there is no query string, it returns an empty string
```go
// Function signature
c.Query(parameter string)

// Example
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
// Function signature
c.Redirect(path string)
c.Redirect(status int, path string)

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Redirect("/foo/bar")
  c.Redirect("http://example.com")
  c.Redirect(301, "http://example.com")
  c.Redirect("../login")
})
```

#### Send
Sends the HTTP response.

The Send parameter can be a buffer or string
```go
// Function signature
c.Send(body string)
c.Send(body []byte)

// Example
app.Get("/", func(c *fiber.Ctx) {
	c.Send("Hello, World!")

	c.Send([]byte("Hello, World!"))
})
```
#### SendFile
Transfers the file at the given path. Sets the Content-Type response HTTP header field based on the filename’s extension.
```go
// Function signature
c.SendFile(path string)

// Example
app.Get("/not-found", func(c *fiber.Ctx) {
	c.SendFile("./public/404.html")
})
```

#### Set
Sets the response’s HTTP header field to value. To set multiple fields at once, pass an object as the parameter.
```go
// Function signature
c.Set(key string, value string)

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Set("Content-Type", "text/plain")
	// => "Content-type: text/plain"
})
```

#### Status
Sets the HTTP status for the response. It is a chainable alias of Node’s response.statusCode.
```go
// Function signature
c.Status(status int)

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Status(200)
  c.Status(400).Send("Bad Request")
  c.Status(404).SendFile("./public/gopher.png")
})
```

#### Type
Sets the Content-Type HTTP header to the MIME type as determined by mime.lookup() for the specified type. If type contains the “/” character, then it sets the Content-Type to type.
```go
// Function signature
c.Type(typ string)

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Type(".html")
  // => 'text/html'

  c.Type("html")
  // => 'text/html'

  c.Type("json")
  // => 'application/json'

  c.Type("png")
  // => 'image/png'
})
```

#### Xhr
A Boolean property that is true if the request’s X-Requested-With header field is “XMLHttpRequest”, indicating that the request was issued by a client library such as jQuery.
```go
// Function signature
c.Xhr()

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Xhr()
  // => true
})
```

### License

MIT © [Fenny](https://github.com/fenny/fiber/blob/master/LICENSE)
