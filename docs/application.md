# Application
The app object conventionally denotes the Fiber application.

## Initialize
Creates an Fiber instance name "app"
```go
app := fiber.New()
```

## Settings
You can pass some [Fasthttp server settings](https://github.com/valyala/fasthttp/blob/master/server.go#L150) via the Fiber instance.  
Make sure that you set these settings before calling the [Listen](#listen) method.  

!>Only change these settings if you know what you are doing.
```go
app := fiber.New()

// Server name for sending in response headers.
//
// No server header is send if left empty.
app.Name = ""

// Clears the console when you run app
app.ClearConsole = false

// Hides the "Fiber" banner when you launch your application.
app.HideBanner = false

// Enables TLS, you need to provide a certificate key and file
app.TLSEnable = false

// Cerficate key
app.CertKey = ""

// Certificate file
app.CertFile = ""

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

## Methods
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

## Listen
Binds and listens for connections on the specified host and port.
```go
// Function signature
app.Listen(port int)
app.Listen(addr string, port int)

// Example
app.Listen(8080)
app.Listen("127.0.0.1", 8080)
```


*Caught a mistake? [Edit this page on GitHub!](https://github.com/Fenny/fiber/blob/master/docs/application.md)*
