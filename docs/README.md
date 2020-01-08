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
**[Fiber](https://github.com/fenny/fiber)** is a router framework build on top of [FastHTTP](https://github.com/valyala/fasthttp), the fastest HTTP package for **[Go](https://golang.org/doc/)**.<br>
This library is inspired by [Express](https://expressjs.com/en/4x/api.html), one of the most populair and well known web framework for **[Nodejs](https://nodejs.org/en/about/)**.

### Getting started

#### Installing
Assuming you’ve already installed [Go](https://golang.org/doc/), install the [Fiber](https://github.com/fenny/fiber) package by calling the following command:
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
// Function signature
app.Method(func(*fiber.Ctx))
app.Method(path string, func(*fiber.Ctx))
app.Method(static string)
app.Method(path string, static string)
```

* **app** is an instance of **[Fiber](#hello-world)**.
* **Method** is an [HTTP request method](https://en.wikipedia.org/wiki/Hypertext_Transfer_Protocol#Request_methods), in capitalization: Get, Put, Post etc
* **path string** is a path or prefix (for static files) on the server.
* **static string** is a file path or directory.
* **func(*fiber.Ctx)** is a function executed when the route is matched.

This tutorial assumes that an instance of fiber named app is created and the server is running. If you are not familiar with creating an app and starting it, see the [Hello world](#hello-world) example.

The following examples illustrate defining simple routes.  
```go
// Respond with Hello, World! on the homepage:
app.Get("/", func(c *fiber.Ctx) {
  c.Send("Hello, World!")
})

//Respond to POST request on the root route (/), the application’s home page:
app.Post("/", func(c *fiber.Ctx) {
  c.Send("Got a POST request")
})

// Respond to a PUT request to the /user route:
app.Put("/user", func(c *fiber.Ctx) {
  c.Send("Got a PUT request at /user")
})

// Respond to a DELETE request to the /user route:
app.Delete("/user", func(c *fiber.Ctx) {
  c.Send("Got a DELETE request at /user")
})
```

#### Static files
To serve static files such as images, CSS files, and JavaScript files, replace your function handler with a file or directory string.
```go
// Function signature
app.Method(static string)
app.Method(path string, static string)
```
For example, use the following code to serve images, CSS files, and JavaScript files in a directory named public:

```go
app.Get("./public")
```
Now, you can load the files that are in the public directory:
```shell
http://localhost:8080/images/kitten.jpg
http://localhost:8080/css/style.css
http://localhost:8080/js/app.js
http://localhost:8080/images/bg.png
http://localhost:8080/hello.html
```
To use multiple static assets directories, call the express.static middleware function multiple times:
```go
app.Get("./public")
app.Get("./files")
```
?>For best results, use a reverse proxy cache like [NGINX](https://www.nginx.com/resources/wiki/start/topics/examples/reverseproxycachingexample/) to improve performance of serving static assets.  

To create a virtual path prefix (where the path does not actually exist in the file system) for files that are served by the express.static function, specify a mount path for the static directory, as shown below:
```go
app.Get("/static", "./public")
```
Now, you can load the files that are in the public directory from the /static path prefix.
```shell
http://localhost:8080/static/images/kitten.jpg
http://localhost:8080/static/css/style.css
http://localhost:8080/static/js/app.js
http://localhost:8080/static/images/bg.png
http://localhost:8080/static/hello.html
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

// Enables TLS, you need to provide a certificate key and file
app.TLSEnable = false,

// Cerficate key
app.CertKey = ""

// Certificate file
app.CertFile = ""

// Server name for sending in response headers.
//
// No server header is send if left empty.

app.Name = ""
// The maximum number of concurrent connections the server may serve.

app.Concurrency = 256 * 1024

// Whether to disable keep-alive connections.
//
// The server will close all the incoming connections after sending
// the first response to client if this option is set to true.
//
// By default keep-alive connections are enabled.
app.DisableKeepAlive = false

// Per-connection buffer size for requests' reading.
// This also limits the maximum header size.
//
// Increase this buffer if your clients send multi-KB RequestURIs
// and/or multi-KB headers (for example, BIG cookies).
app.ReadBufferSize = 4096

// Per-connection buffer size for responses' writing.
app.WriteBufferSize = 4096

// WriteTimeout is the maximum duration before timing out
// writes of the response. It is reset after the request handler
// has returned.
//
// By default response write timeout is unlimited.
app.WriteTimeout = 0

// IdleTimeout is the maximum amount of time to wait for the
// next request when keep-alive is enabled. If IdleTimeout
// is zero, the value of ReadTimeout is used.
app.IdleTimeout = 0

// Maximum number of concurrent client connections allowed per IP.
//
// By default unlimited number of concurrent connections
// may be established to the server from a single IP address.
app.MaxConnsPerIP = 0

// Maximum number of requests served per connection.
//
// The server closes connection after the last request.
// 'Connection: close' header is added to the last response.
//
// By default unlimited number of requests may be served per connection.
app.MaxRequestsPerConn = 0

// Whether to enable tcp keep-alive connections.
//
// Whether the operating system should send tcp keep-alive messages on the tcp connection.
//
// By default tcp keep-alive connections are disabled.
app.TCPKeepalive = false

// Period between tcp keep-alive messages.
//
// TCP keep-alive period is determined by operation system by default.
app.TCPKeepalivePeriod = 0

// Maximum request body size.
//
// The server rejects requests with bodies exceeding this limit.
//
// Request body size is limited by DefaultMaxRequestBodySize by default.
app.MaxRequestBodySize = 4 * 1024 * 1024

// Aggressively reduces memory usage at the cost of higher CPU usage
// if set to true.
//
// Try enabling this option only if the server consumes too much memory
// serving mostly idle keep-alive connections. This may reduce memory
// usage by more than 50%.
//
// Aggressive memory usage reduction is disabled by default.
app.ReduceMemoryUsage = false

// Rejects all non-GET requests if set to true.
//
// This option is useful as anti-DoS protection for servers
// accepting only GET requests. The request size is limited
// by ReadBufferSize if GetOnly is set.
//
// Server accepts all the requests by default.
app.GetOnly = false

// By default request and response header names are normalized, i.e.
// The first letter and the first letters following dashes
// are uppercased, while all the other letters are lowercased.
// Examples:
//
//     * HOST -> Host
//     * content-type -> Content-Type
//     * cONTENT-lenGTH -> Content-Length
app.DisableHeaderNamesNormalizing = false

// SleepWhenConcurrencyLimitsExceeded is a duration to be slept of if
// the concurrency limit in exceeded (default [when is 0]: don't sleep
// and accept new connections immidiatelly).
app.SleepWhenConcurrencyLimitsExceeded = 0

// NoDefaultContentType, when set to true, causes the default Content-Type
// header to be excluded from the Response.
//
// The default Content-Type header value is the internal default value. When
// set to true, the Content-Type will not be present.
app.NoDefaultContentType = false

// KeepHijackedConns is an opt-in disable of connection
// close by fasthttp after connections' HijackHandler returns.
// This allows to save goroutines, e.g. when fasthttp used to upgrade
// http connections to WS and connection goes to another handler,
// which will close it when needed.
app.KeepHijackedConns = false
```
#### Methods
Routes an HTTP request, where METHOD is the HTTP method of the request, such as GET, PUT, POST, and so on capitalized. Thus, the actual methods are app.Get(), app.Post(), app.Put(), and so on. See Routing methods below for the complete list.
```go
// Function signature
app.Connect(...)
app.Delete(...)
app.Get(...)
app.Head(...)
app.Options(...)
app.Patch(...)
app.Post(...)
app.Put(...)
app.Trace(...)
// Matches all HTTP verbs
app.Use(...)
app.All(...)
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
Route parameters are named URL segments that are used to capture the values specified at their position in the URL. The captured values can be retrieved using the [Params()](#params) function, with the name of the route parameter specified in the path as their respective keys.

To define routes with route parameters, simply specify the route parameters in the path of the route as shown below.

```go
app.Get("/user/:name/books/:title", func(c *fiber.Ctx) {
	c.Write(c.Params("name"))
	c.Write(c.Params("title"))
})

app.Get("/user/*", func(c *fiber.Ctx) {
	c.Send(c.Params("*"))
})

app.Get("/user/:name?", func(c *fiber.Ctx) {
	c.Send(c.Params("name"))
})
```
?>The name of route parameters must be made up of “word characters” ([A-Za-z0-9_]).

!> The hyphen (-) and the dot (.) are not interpreted literally yet, planned for V2

#### Middleware
The [Next()](#next) function is a function in the [Fiber](https://github.com/fenny/fiber) router which, when called, executes the next function that matches the current route.

Functions that are designed to make changes to the request or response are called middleware functions.

Here is a simple example of a middleware function that prints "ip > path" when a request to the app passes through it.

```go
app := fiber.New()
app.Get(func(c *fiber.Ctx) {
	fmt.Println(c.IP(), "=>", c.Path())
	// => Prints ip and path

	c.Set("X-Logged", "true")
	// => Sets response header

	c.Next()
	// Go to next middleware
})
app.Get("/", func(c *fiber.Ctx) {
	c.Send("Hello, World!")
})
app.Listen(8080)
```

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

#### BasicAuth
BasicAuth returns the username and password provided in the request's Authorization header, if the request uses HTTP Basic Authentication.
```go
// Function signature
user, pass, ok := c.BasicAuth()

// Example
// curl --user john:doe http://localhost:8080
app.Get("/", func(c *fiber.Ctx) {

	user, pass, ok := c.BasicAuth()

	if ok && user == "john" && pass == "doe" {
		c.Send("Welcome " + user)
		return
	}

	c.Status(403).Send("Forbidden")
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

#### Fasthttp
You can still access and use all Fasthttp methods and properties.  
Please read the [Fasthttp Documentation](https://godoc.org/github.com/valyala/fasthttp) for more information
```go
// Function signature
c.Fasthttp...

// Example
app.Get("/", func(c *fiber.Ctx) {
	string(c.Fasthttp.Request.Header.Method())
	// => "GET"

	c.Fasthttp.Response.Write([]byte("Hello, World!"))
	// => "Hello, World!"
})
```

#### Form
To access multipart form entries, you can parse the binary with .Form().  
This returns a map[string][]string, so given a key the value will be a string slice.  
So accepting multiple files or values is easy, as shown below!
```go
// Function signature
c.Form()

// Example
app.Post("/", func(c *fiber.Ctx) {
	// Parse the multipart form
	if form := c.Form(); form != nil {
		// => *multipart.Form

		if token := form.Value["token"]; len(token) > 0 {
			// Get key value
			fmt.Println(token[0])
		}

		// Get all files from "documents" key
		files := form.File["documents"]
		// => []*multipart.FileHeader

		// Loop trough files
		for _, file := range files {
			fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
			// => "tutorial.pdf" 360641 "application/pdf"

			// Save the files to disk
			c.SaveFile(file, fmt.Sprintf("./%s", file.Filename))
		}
	}
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
Converts any interface to json using [FFJson](https://github.com/pquerna/ffjson), this functions also sets the content header to application/json.
```go
// Function signature
err := c.Json(v interface{})

// Example
type SomeData struct {
	Name string
	Age  uint8
}

app := fiber.New()
app.Get("/json", func(c *fiber.Ctx) {
	data := SomeData{
		Name: "Grame",
		Age:  20,
	}
	c.Json(data)
	// or
	err := c.Json(data)
	if err != nil {
		// etc
	}
})
app.Listen(8080)
```

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

#### Next
When Next() is called, it executes the next function in the stack that matches the current route.
```go
// Function signature
c.Next()

// Example
app.Get("/", func(c *fiber.Ctx) {
	fmt.Printl("1st route!")
	c.Next()
})
app.Get("*", func(c *fiber.Ctx) {
	fmt.Printl("2nd route!")
	c.Next()
})
app.Get("/", func(c *fiber.Ctx) {
	fmt.Printl("3rd route!")
	c.Send("Hello, World!")
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

#### SaveFile
This function is used to save any multipart file to disk.  
You can see a working example here: [Multiple file upload](#form)

```go
// Function signature
c.SaveFile(fh *multipart.FileHeader, path string)
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

#### Write
Appends to the HTTP response.

The Write parameter can be a buffer or string
```go
// Function signature
c.Write(body string)
c.Write(body []byte)

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Write("Hello, ")
	c.Write([]byte("World!"))
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

### Examples

#### Multiple File Upload
```go

package main

import "github.com/fenny/fiber"

func main() {
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) {
		// Parse the multipart form
		if form := c.Form(); form != nil {
			// => *multipart.Form

			if token := form.Value["token"]; len(token) > 0 {
				// Get key value
				fmt.Println(token[0])
			}

			// Get all files from "documents" key
			files := form.File["documents"]
			// => []*multipart.FileHeader

			// Loop trough files
			for _, file := range files {
				fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
				// => "tutorial.pdf" 360641 "application/pdf"

				// Save the files to disk
				c.SaveFile(file, fmt.Sprintf("./%s", file.Filename))
			}
		}
	})
	app.Listen(8080)
}
```
#### 404 Handling
```go
package main

import "github.com/fenny/fiber"

func main() {
	app := fiber.New()

	app.Get("./static")
	app.Get(notFound)

	app.Listen(8080)
}

func notFound(c *fiber.Ctx) {
	c.Status(404).Send("Not Found")
}
```
#### Static Caching
```go
package main

import "github.com/fenny/fiber"

func main() {
	app := fiber.New()
	app.Get(cacheControl)
	app.Get("./static")
	app.Listen(8080)
}

func cacheControl(c *fiber.Ctx) {
	c.Set("Cache-Control", "max-age=2592000, public")
	c.Next()
}
```
#### Enable CORS
```go
package main

import "./fiber"

func main() {
	app := fiber.New()

	app.All("/api", enableCors)
	app.Get("/api", apiHandler)

	app.Listen(8080)
}

func enableCors(c *fiber.Ctx) {
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "X-Requested-With")
	c.Next()
}
func apiHandler(c *fiber.Ctx) {
	c.Send("Hi, I'm API!")
}
```
#### Returning JSON

### License & Thanks
Special thanks to some amazing people and organizations:  

[@valyala](https://github.com/valyala)  
[@julienschmidt](https://github.com/julienschmidt)  
[@savsgio](https://github.com/savsgio)  
[@vincentLiuxiang](https://github.com/vincentLiuxiang)  
[@pillarjs](https://github.com/pillarjs)  


MIT © [Fiber](https://github.com/fenny/fiber/blob/master/LICENSE)  
MIT © [Fasthttp](https://github.com/valyala/fasthttp/blob/master/LICENSE)  
MIT © [Express](https://github.com/expressjs/express/blob/master/LICENSE)  
MIT © [Lu](https://github.com/vincentLiuxiang/lu/blob/master/LICENSE)  
MIT © [Path-to-regexp](https://github.com/pillarjs/path-to-regexp/blob/master/LICENSE)  
Apache © [Atreugo](https://github.com/savsgio/atreugo/blob/master/LICENSE)  
