# Application
The app object conventionally denotes the Fiber application.

#### Initialize
Creates an Fiber instance name "app"
```go
app := fiber.New()
// Optional fiber settings
// Sends the "Server" header, disabled by default
app.Server = ""
// Hides fiber banner, enabled by default
app.NoBanner = false
```

#### TLS
To enable TLS you need to provide a certkey and certfile.
```go
// Enable TLS
app := fiber.New()

app.CertKey("./cert.key")
app.CertFile("./cert.pem")

app.Listen(443)
```
#### Fasthttp
You can pass some Fasthttp server settings via the Fiber instance.  
Make sure that you set these settings before calling the [Listen](#listen) method. You can find the description of each property in [Fasthttp server settings](https://github.com/valyala/fasthttp/blob/master/server.go#L150)

!>Only change these settings if you know what you are doing.
```go
app := fiber.New()

app.Fasthttp.Concurrency = 256 * 1024
app.Fasthttp.DisableKeepAlive = false
app.Fasthttp.ReadBufferSize = 4096
app.Fasthttp.WriteBufferSize = 4096
app.Fasthttp.ReadTimeout = 0
app.Fasthttp.WriteTimeout = 0
app.Fasthttp.IdleTimeout = 0
app.Fasthttp.MaxConnsPerIP = 0
app.Fasthttp.MaxRequestsPerConn = 0
app.Fasthttp.TCPKeepalive = false
app.Fasthttp.TCPKeepalivePeriod = 0
app.Fasthttp.MaxRequestBodySize = 4 * 1024 * 1024
app.Fasthttp.ReduceMemoryUsage = false
app.Fasthttp.GetOnly = false
app.Fasthttp.DisableHeaderNamesNormalizing = false
app.Fasthttp.SleepWhenConcurrencyLimitsExceeded = 0
app.Fasthttp.NoDefaultContentType = false
app.Fasthttp.KeepHijackedConns = false
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
app.Listen(port int, addr ...string)


// Example
app.Listen(8080)
app.Listen(8080, "127.0.0.1")
```


*Caught a mistake? [Edit this page on GitHub!](https://github.com/Fenny/fiber/blob/master/docs/application.md)*
