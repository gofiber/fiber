---
id: proxy
---

# Proxy

Proxy middleware for [Fiber](https://github.com/gofiber/fiber) that allows you to proxy requests to multiple servers.

## Signatures

```go
// Balancer create a load balancer among multiple upstrem servers.
func Balancer(config Config) fiber.Handler
// Forward performs the given http request and fills the given http response.
func Forward(addr string, clients ...*fasthttp.Client) fiber.Handler
// Do performs the given http request and fills the given http response.
func Do(c *fiber.Ctx, addr string, clients ...*fasthttp.Client) error
// DoRedirects performs the given http request and fills the given http response while following up to maxRedirectsCount redirects.
func DoRedirects(c *fiber.Ctx, addr string, maxRedirectsCount int, clients ...*fasthttp.Client) error
// DoDeadline performs the given request and waits for response until the given deadline.
func DoDeadline(c *fiber.Ctx, addr string, deadline time.Time, clients ...*fasthttp.Client) error
// DoTimeout performs the given request and waits for response during the given timeout duration.
func DoTimeout(c *fiber.Ctx, addr string, timeout time.Duration, clients ...*fasthttp.Client) error
// DomainForward the given http request based on the given domain and fills the given http response
func DomainForward(hostname string, addr string, clients ...*fasthttp.Client) fiber.Handler
// BalancerForward performs the given http request based round robin balancer and fills the given http response
func BalancerForward(servers []string, clients ...*fasthttp.Client) fiber.Handler
```

## Examples

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

// If you want to forward with a specific domain. You have to use proxy.DomainForward.
app.Get("/payments", proxy.DomainForward("docs.gofiber.io", "http://localhost:8000"))

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

// Make proxy requests while following redirects
app.Get("/proxy", func(c *fiber.Ctx) error {
    if err := proxy.DoRedirects(c, "http://google.com", 3); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Make proxy requests and wait up to 5 seconds before timing out
app.Get("/proxy", func(c *fiber.Ctx) error {
    if err := proxy.DoTimeout(c, "http://localhost:3000", time.Second * 5); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Make proxy requests, timeout a minute from now
app.Get("/proxy", func(c *fiber.Ctx) error {
    if err := proxy.DoDeadline(c, "http://localhost", time.Now().Add(time.Minute)); err != nil {
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

// Or this way if the balancer is using https and the destination server is only using http.
app.Use(proxy.BalancerForward([]string{
    "http://localhost:3001",
    "http://localhost:3002",
    "http://localhost:3003",
}))
```

## Config

| Property        | Type                                           | Description                                                                                                                                                                                                    | Default         |
|:----------------|:-----------------------------------------------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:----------------|
| Next            | `func(*fiber.Ctx) bool`                        | Next defines a function to skip this middleware when returned true.                                                                                                                                            | `nil`           |
| Servers         | `[]string`                                     | Servers defines a list of `<scheme>://<host>` HTTP servers, which are used in a round-robin manner. i.e.: "https://foobar.com, http://www.foobar.com"                                                            | (Required)      |
| ModifyRequest   | `fiber.Handler`                                | ModifyRequest allows you to alter the request.                                                                                                                                                                 | `nil`           |
| ModifyResponse  | `fiber.Handler`                                | ModifyResponse allows you to alter the response.                                                                                                                                                               | `nil`           |
| Timeout         | `time.Duration`                                | Timeout is the request timeout used when calling the proxy client.                                                                                                                                             | 1 second        |
| ReadBufferSize  | `int`                                          | Per-connection buffer size for requests' reading. This also limits the maximum header size. Increase this buffer if your clients send multi-KB RequestURIs and/or multi-KB headers (for example, BIG cookies). | (Not specified) |
| WriteBufferSize | `int`                                          | Per-connection buffer size for responses' writing.                                                                                                                                                             | (Not specified) |
| TlsConfig       | `*tls.Config` (or `*fasthttp.TLSConfig` in v3) | TLS config for the HTTP client.                                                                                                                                                                                | `nil`           |
| Client          | `*fasthttp.LBClient`                           | Client is a custom client when client config is complex.                                                                                                                                                       | `nil`           |

## Default Config

```go
var ConfigDefault = Config{
    Next:           nil,
    ModifyRequest:  nil,
    ModifyResponse: nil,
    Timeout:        fasthttp.DefaultLBClientTimeout,
}
```
