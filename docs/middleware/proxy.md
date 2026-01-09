---
id: proxy
---

# Proxy

The Proxy middleware forwards requests to one or more upstream servers.

## Signatures

```go
// Balancer creates a load balancer among multiple upstream servers.
func Balancer(config ...Config) fiber.Handler
// Forward performs the given http request and fills the given http response.
func Forward(addr string, clients ...*fasthttp.Client) fiber.Handler
// Do performs the given http request and fills the given http response.
func Do(c fiber.Ctx, addr string, clients ...*fasthttp.Client) error
// DoRedirects performs the given http request and fills the given http response while following up to maxRedirectsCount redirects.
func DoRedirects(c fiber.Ctx, addr string, maxRedirectsCount int, clients ...*fasthttp.Client) error
// DoDeadline performs the given request and waits for response until the given deadline.
func DoDeadline(c fiber.Ctx, addr string, deadline time.Time, clients ...*fasthttp.Client) error
// DoTimeout performs the given request and waits for response during the given timeout duration.
func DoTimeout(c fiber.Ctx, addr string, timeout time.Duration, clients ...*fasthttp.Client) error
// DomainForward performs the given http request based on the provided domain and fills the given http response.
func DomainForward(hostname string, addr string, clients ...*fasthttp.Client) fiber.Handler
// BalancerForward performs the given http request based round robin balancer and fills the given http response.
func BalancerForward(servers []string, clients ...*fasthttp.Client) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/proxy"
)
```

Once your Fiber app is initialized, you can use the middleware as shown:

```go
// Use proxy.WithClient to set a global custom client.
proxy.WithClient(&fasthttp.Client{
    NoDefaultUserAgentHeader: true,
    DisablePathNormalizing:   true,
    // Allow self-signed certificates when proxying to HTTPS targets.
    TLSConfig: &tls.Config{
        InsecureSkipVerify: true,
    },
})

// Forward requests for a specific domain with proxy.DomainForward.
app.Get("/payments", proxy.DomainForward("docs.gofiber.io", "http://localhost:8000"))

// Forward to a URL using a custom client
app.Get("/gif", proxy.Forward("https://i.imgur.com/IWaBepg.gif", &fasthttp.Client{
    NoDefaultUserAgentHeader: true,
    DisablePathNormalizing:   true,
}))

// Make a proxied request within a handler
app.Get("/:id", func(c fiber.Ctx) error {
    url := "https://i.imgur.com/" + c.Params("id") + ".gif"
    if err := proxy.Do(c, url); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Proxy requests while following redirects
app.Get("/proxy", func(c fiber.Ctx) error {
    if err := proxy.DoRedirects(c, "http://google.com", 3); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Proxy requests and wait up to five seconds before timing out
app.Get("/proxy", func(c fiber.Ctx) error {
    if err := proxy.DoTimeout(c, "http://localhost:3000", time.Second * 5); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Proxy requests with a deadline one minute from now
app.Get("/proxy", func(c fiber.Ctx) error {
    if err := proxy.DoDeadline(c, "http://localhost", time.Now().Add(time.Minute)); err != nil {
        return err
    }
    // Remove Server header from response
    c.Response().Header.Del(fiber.HeaderServer)
    return nil
})

// Minimal round-robin balancer
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://localhost:3001",
        "http://localhost:3002",
        "http://localhost:3003",
    },
}))

// Keep the Connection header when proxying
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://localhost:3001",
    },
    KeepConnectionHeader: true,
}))

// Or extend your balancer for customization
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://localhost:3001",
        "http://localhost:3002",
        "http://localhost:3003",
    },
    ModifyRequest: func(c fiber.Ctx) error {
        c.Request().Header.Add("X-Real-IP", c.IP())
        return nil
    },
    ModifyResponse: func(c fiber.Ctx) error {
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


// Make round robin balancer with IPv6 support.
app.Use(proxy.Balancer(proxy.Config{
    Servers: []string{
        "http://[::1]:3001",
        "http://127.0.0.1:3002",
        "http://localhost:3003",
    },
    // Enable TCP4 and TCP6 network stacks.
    DialDualStack: true,
}))
```

## Config

| Property        | Type                                           | Description                                                                                                                                                                                                                        | Default         |
|:----------------|:-----------------------------------------------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:----------------|
| Next            | `func(fiber.Ctx) bool`                        | Next defines a function to skip this middleware when it returns true.                                                                                                                                                                | `nil`           |
| Servers         | `[]string`                                     | Servers defines a list of `<scheme>://<host>` HTTP servers, which are used in a round-robin manner. i.e.: "[https://foobar.com](https://foobar.com), [http://www.foobar.com](http://www.foobar.com)"                                                        | (Required)      |
| ModifyRequest   | `fiber.Handler`                                | ModifyRequest allows you to alter the request.                                                                                                                                                                                     | `nil`           |
| ModifyResponse  | `fiber.Handler`                                | ModifyResponse allows you to alter the response.                                                                                                                                                                                   | `nil`           |
| Timeout         | `time.Duration`                                | Timeout is the request timeout used when calling the proxy client.                                                                                                                                                                 | 1 second        |
| ReadBufferSize  | `int`                                          | Per-connection buffer size for requests' reading. This also limits the maximum header size. Increase this buffer if your clients send multi-KB RequestURIs and/or multi-KB headers (for example, BIG cookies).                     | (Not specified) |
| WriteBufferSize | `int`                                          | Per-connection buffer size for responses' writing.                                                                                                                                                                                 | (Not specified) |
| KeepConnectionHeader | `bool` | Keeps the `Connection` header when set to `true`. By default the header is removed to comply with RFC 7230 §6.1 and avoid proxy loops. | `false` |
| TLSConfig       | `*tls.Config` | TLS config for the HTTP client. | `nil`           |
| DialDualStack   | `bool`                                         | Client will attempt to connect to both IPv4 and IPv6 host addresses if set to true.                                                                                                                                                | `false`         |
| Client          | `*fasthttp.LBClient`                           | Client is a custom client when client config is complex.                                                                                                                                                                           | `nil`           |

## Default Config

```go
var ConfigDefault = Config{
    Next:           nil,
    ModifyRequest:  nil,
    ModifyResponse: nil,
    Timeout:        fasthttp.DefaultLBClientTimeout,
    KeepConnectionHeader: false,
}
```
