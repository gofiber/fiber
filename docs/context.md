# Context
The ctx object represents the HTTP request and response and has methods for the request query string, parameters, body, HTTP headers, and so on. In this documentation and by convention, the context is always referred to as c but its actual name is determined by the parameters to the callback function in which you’re working.

## Accepts
!> Planned for V2

## Attachment
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

## BasicAuth
BasicAuth returns the username and password provided in the request's Authorization header, if the request uses HTTP Basic Authentication.
```go
// Function signature
user, pass, ok := c.BasicAuth()

// Example
// curl --user john:doe http://localhost:8080
app.Get("/", func(c *fiber.Ctx) {

	user, pass, ok := c.BasicAuth()

	if !ok || user != "john" || pass != "doe" {
		c.Status(403).Send("Forbidden")
    return
	}

  c.Send("Welcome " + user)
})
```


## Body
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

## ClearCookies
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

## Cookies
Clears all cookies from client, or a specific cookie by name by adjusting the expiration.
```go
// Function signature
c.Cookies() string
c.Cookies(key string) string
c.Cookies(key string, value string) string
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

## Download
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

## Fasthttp
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

## FormFile
MultipartForm files can be retrieved by name, the first file from the given key is returned.
```go
// Function signature
c.FormFile(name string) (*multipart.FileHeader, error)

// Example
app.Post("/", func(c *fiber.Ctx) {
	file, err := c.FormFile("document")
})
```

## FormValue
MultipartForm values can be retrieved by name, the first value from the given key is returned.
```go
// Function signature
c.FormValue(name string) string

// Example
app.Post("/", func(c *fiber.Ctx) {
	c.FormValue("name")
})
```


## Get
Returns the HTTP response header specified by field. The match is case-insensitive.
```go
// Function signature
c.Get(field string) string

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

## Hostname
Contains the hostname derived from the Host HTTP header.
```go
// Function signature
c.Hostname() string

// Example
app.Get("/", func(c *fiber.Ctx) {
  // Host: "localhost:8080"
  c.Hostname()
  // => "localhost"
})
```

## IP
Contains the remote IP address of the request.
```go
// Function signature
c.IP() string

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.IP()
  // => "127.0.0.1"
})
```

## Is
Returns the matching content type if the incoming request’s “Content-Type” HTTP header field matches the MIME type specified by the type parameter. If the request has no body, returns false.
```go
// Function signature
c.Is(typ string) bool

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
## Json
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

## Jsonp
!> Planned for V2

## Method
Contains a string corresponding to the HTTP method of the request: GET, POST, PUT, and so on.
```go
// Function signature
c.Method() string

// Example
app.Post("/", func(c *fiber.Ctx) {
	c.Method()
	// => "POST"
})
```

## MultipartForm
To access multipart form entries, you can parse the binary with .Form().  
This returns a map[string][]string, so given a key the value will be a string slice.  
So accepting multiple files or values is easy, as shown below!
```go
// Function signature
c.MultipartForm() (*multipart.Form, error)

// Example
app.Post("/", func(c *fiber.Ctx) {
	// Parse the multipart form
	if form, err := c.MultipartForm(); err == nil {
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

## Next
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

## OriginalURL
Contains the original request URL.
```go
// Function signature
c.OriginalURL() string

// Example
app.Get("/", func(c *fiber.Ctx) {
	// GET /search?q=something
	c.OriginalURL()
	// => '/search?q=something'
})
```

## Params
This method can be used to get the route parameters. For example, if you have the route /user/:name, then the “name” property is available as c.Params("name"). This method defaults "".
```go
// Function signature
c.Params(param string) string

// Example
app.Get("/user/:name", func(c *fiber.Ctx) {
	// GET /user/tj
	c.Params("name")
	// => "tj"
})
```

## Path
Contains the path part of the request URL.
```go
// Function signature
c.Path() string

// Example
app.Get("/users", func(c *fiber.Ctx) {
	// example.com/users?sort=desc
	c.Path()
	// => "/users"
})
```

## Protocol
Contains the request protocol string: either http or (for TLS requests) https.

```go
// Function signature
c.Protocol() string

// Example
app.Get("/", func(c *fiber.Ctx) {
	c.Protocol()
	// => "http"
})
```
## Query
This property is an object containing a property for each query string parameter in the route. If there is no query string, it returns an empty string
```go
// Function signature
c.Query(parameter string) string

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
## Redirect
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

## SaveFile
This function is used to save any multipart file to disk.  
You can see a working example here: [Multiple file upload](#multipartform)

```go
// Function signature
c.SaveFile(fh *multipart.FileHeader, path string)
```

## Send
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
## SendFile
Transfers the file at the given path. Sets the Content-Type response HTTP header field based on the filename’s extension.
```go
// Function signature
c.SendFile(path string)

// Example
app.Get("/not-found", func(c *fiber.Ctx) {
	c.SendFile("./public/404.html")
})
```

## Set
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

## Status
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

## Type
Sets the Content-Type HTTP header to the MIME type as determined by mime.lookup() for the specified type. If type contains the “/” character, then it sets the Content-Type to type.
```go
// Function signature
c.Type(typ string) string

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

## Write
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

## Xhr
A Boolean property that is true if the request’s X-Requested-With header field is “XMLHttpRequest”, indicating that the request was issued by a client library such as jQuery.
```go
// Function signature
c.Xhr() bool

// Example
app.Get("/", func(c *fiber.Ctx) {
  c.Xhr()
  // => true
})
```

*Caught a mistake? [Edit this page on GitHub!](https://github.com/Fenny/fiber/blob/master/docs/context.md)*
