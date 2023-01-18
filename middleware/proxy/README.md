# Proxy

Proxy middleware for [Fiber](https://github.com/gofiber/fiber) that allows you to proxy requests to multiple servers.

### Table of Contents

- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)

### Signatures

```go
func Balancer(config Config) fiber.Handler
func Forward(addr string, clients ...*fasthttp.Client) fiber.Handler
func Do(c *fiber.Ctx, addr string, clients ...*fasthttp.Client) error
```

### Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// if target https site uses a self-signed certificate, you should
// call WithTlsConfig before Do and Forward
proxy.WithTlsConfig(&tls.Config{
    InsecureSkipVerify: true,
})

// if you need to use global self-custom client, you should use proxy.WithClient.
proxy.WithClient(&fasthttp.Client{
	NoDefaultUserAgentHeader: true, 
	DisablePathNormalizing:   true,
})

// Forward to url
app.Get("/gif", proxy.Forward("https://i.imgur.com/IWaBepg.gif"))

// Forward to url with local custom client
app.Get("/gif", proxy.Forward("https://i.imgur.com/IWaBepg.gif", &fasthttp.Client{
	NoDefaultUserAgentHeader: true, 
	DisablePathNormalizing:   true,
}))

// Make request within handler
app.Get("/:id", func(c *fiber.Ctx) error {
	url := "https://i.imgur.com/"+c.Params("id")+".gif"
	if err := proxy.Do(c, url); err != nil {
		return err
	}
	// Remove Server header from response
	c.Response().Header.Del(fiber.HeaderServer)
	return nil
})

// Minimal round robin balancer
app.Use(proxy.Balancer(proxy.Config{
	Servers: []string{
		"http://localhost:3001",
		"http://localhost:3002",
		"http://localhost:3003",
	},
}))

// Or extend your balancer for customization
app.Use(proxy.Balancer(proxy.Config{
	Servers: []string{
		"http://localhost:3001",
		"http://localhost:3002",
		"http://localhost:3003",
	},
	ModifyRequest: func(c *fiber.Ctx) error {
		c.Request().Header.Add("X-Real-IP", c.IP())
		return nil
	},
	ModifyResponse: func(c *fiber.Ctx) error {
		c.Response().Header.Del(fiber.HeaderServer)
		return nil
	},
}))
```

### Config

```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Servers defines a list of <scheme>://<host> HTTP servers,
	//
	// which are used in a round-robin manner.
	// i.e.: "https://foobar.com, http://www.foobar.com"
	//
	// Required
	Servers []string

	// ModifyRequest allows you to alter the request
	//
	// Optional. Default: nil
	ModifyRequest fiber.Handler

	// ModifyResponse allows you to alter the response
	//
	// Optional. Default: nil
	ModifyResponse fiber.Handler
	
	// Timeout is the request timeout used when calling the proxy client
	//
	// Optional. Default: 1 second
	Timeout time.Duration

	// Per-connection buffer size for requests' reading.
	// This also limits the maximum header size.
	// Increase this buffer if your clients send multi-KB RequestURIs
	// and/or multi-KB headers (for example, BIG cookies).
	ReadBufferSize int
    
	// Per-connection buffer size for responses' writing.
	WriteBufferSize int

	// tls config for the http client.
	TlsConfig *tls.Config 
	
	// Client is custom client when client config is complex. 
	// Note that Servers, Timeout, WriteBufferSize, ReadBufferSize and TlsConfig 
	// will not be used if the client are set.
	Client *fasthttp.LBClient
}
```

### Default Config

```go
// ConfigDefault is the default config
var ConfigDefault = Config{
    Next:           nil,
    ModifyRequest:  nil,
    ModifyResponse: nil,
    Timeout:        fasthttp.DefaultLBClientTimeout,
}
```
