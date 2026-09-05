---
id: examples
title: 🍳 Examples
description: >-
  Client usage examples.
sidebar_position: 5
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Basic Auth

Clients send credentials via the `Authorization` header, while the server
stores hashed passwords as shown in the middleware example.

<Tabs>
<TabItem value="client" label="Client">

```go
package main

import (
    "encoding/base64"
    "fmt"

    "github.com/gofiber/fiber/v3/client"
)

func main() {
    cc := client.New()

    out := base64.StdEncoding.EncodeToString([]byte("john:doe"))
    resp, err := cc.Get("http://localhost:3000", client.Config{
        Header: map[string]string{
            "Authorization": "Basic " + out,
        },
    })
    if err != nil {
        panic(err)
    }

    fmt.Print(string(resp.Body()))
}
```

</TabItem>
<TabItem value="server" label="Server">

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/basicauth"
)

func main() {
    app := fiber.New()
    app.Use(
        basicauth.New(basicauth.Config{
            Users: map[string]string{
                // "doe" hashed using SHA-256
                "john": "{SHA256}eZ75KhGvkY4/t0HfQpNPO1aO0tk6wd908bjUGieTKm8=",
            },
        }),
    )

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    app.Listen(":3000")
}
```

</TabItem>
</Tabs>

## TLS

<Tabs>
<TabItem value="client" label="Client">

```go
package main

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "os"

    "github.com/gofiber/fiber/v3/client"
)

func main() {
    cc := client.New()

    certPool, err := x509.SystemCertPool()
    if err != nil {
        panic(err)
    }

    cert, err := os.ReadFile("ssl.cert")
    if err != nil {
        panic(err)
    }

    certPool.AppendCertsFromPEM(cert)
    cc.SetTLSConfig(&tls.Config{
        RootCAs: certPool,
    })

    resp, err := cc.Get("https://localhost:3000")
    if err != nil {
        panic(err)
    }

    fmt.Print(string(resp.Body()))
}
```

</TabItem>
<TabItem value="server" label="Server">

```go
package main

import (
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New()

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    err := app.Listen(":3000", fiber.ListenConfig{
        CertFile:    "ssl.cert",
        CertKeyFile: "ssl.key",
    })
    if err != nil {
        panic(err)
    }
}
```

</TabItem>
</Tabs>

## Reusing fasthttp transports

The Fiber client can wrap existing `fasthttp` clients so that you can reuse
connection pools, custom dialers, or load-balancing logic that is already tuned
for your infrastructure.

### HostClient

```go
package main

import (
    "log"
    "time"

    "github.com/gofiber/fiber/v3/client"
    "github.com/valyala/fasthttp"
)

func main() {
    hc := &fasthttp.HostClient{
        Addr:              "api.internal:443",
        IsTLS:             true,
        MaxConnDuration:   30 * time.Second,
        MaxIdleConnDuration: 10 * time.Second,
    }

    cc := client.NewWithHostClient(hc)

    resp, err := cc.Get("https://api.internal:443/status")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("status=%d body=%s", resp.StatusCode(), resp.Body())
}
```

### LBClient

```go
package main

import (
    "log"
    "time"

    "github.com/gofiber/fiber/v3/client"
    "github.com/valyala/fasthttp"
)

func main() {
    lb := &fasthttp.LBClient{
        Timeout: 2 * time.Second,
        Clients: []fasthttp.BalancingClient{
            &fasthttp.HostClient{Addr: "edge-1.internal:8080"},
            &fasthttp.HostClient{Addr: "edge-2.internal:8080"},
        },
    }

    cc := client.NewWithLBClient(lb)

    // Per-request overrides such as redirects, retries, TLS, and proxy dialers
    // are shared across every host client managed by the load balancer.
    resp, err := cc.Get("http://service.internal/api")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("status=%d body=%s", resp.StatusCode(), resp.Body())
}
```

## Cookie jar

The client can store and reuse cookies between requests by attaching a cookie jar.

The jar follows RFC 6265 for storage and retrieval:

- A `Set-Cookie` without a `Path` attribute is scoped to the **directory** of
  the request that set it, not to the whole host. A cookie set by a response to
  `/api/login` defaults to `Path=/api` and is not sent to `/`. Send an explicit
  `Path=/` to scope it host-wide.
- Cookies are identified by the triple (name, domain, path), so the same name
  can be stored at several paths at once. When more than one applies to a
  request, the one with the longest path wins.
- Storage is bounded: at most 1024 storage keys, and at most 64 cookies per
  key. A host-only cookie and a `Domain=` cookie are stored under different
  keys, so a single host can occupy more than one. When a key is full the jar
  drops expired entries first, then the least recently written — a session
  cookie the server re-sends on each response is not evicted by a flood of
  one-off cookies. A single request carries at most 64 cookies, the most
  specific first, so a host cannot inflate the `Cookie` header by spreading
  cookies across the `Domain=` keys of its parent labels.
- Cookies are attached once per request, before any redirect is followed, so a
  redirect chain carries the cookies selected for the original URL. A hop to an
  unrelated host drops them, but a hop to a subdomain keeps them — matching
  `net/http`, which permits `Cookie` from `foo.com` to `sub.foo.com`. That is
  wider than the jar's own rule: a cookie set without a `Domain=` attribute is
  host-only and the jar would not send it to `sub.foo.com`, yet a redirect
  there carries it. This only arises once redirects are being followed, which
  is not the default: leave `MaxRedirects` unset — or set `0`, via
  `Request.SetMaxRedirects` or `Config{MaxRedirects: 0}` — and issue each hop
  yourself, so the jar decides for each one.

### Request

```go
func main() {
    jar := client.AcquireCookieJar()
    defer client.ReleaseCookieJar(jar)

    cc := client.New()
    cc.SetCookieJar(jar)

    jar.SetKeyValueBytes("httpbin.org", []byte("john"), []byte("doe"))

    resp, err := cc.Get("https://httpbin.org/cookies")
    if err != nil {
        panic(err)
    }

    fmt.Println(string(resp.Body()))
}
```

<details>
<summary>Click here to see the result</summary>

```json
{
  "cookies": {
    "john": "doe"
  }
}
```

</details>

### Response

Read cookies set by the server directly from the jar.

```go
func main() {
    jar := client.AcquireCookieJar()
    defer client.ReleaseCookieJar(jar)

    cc := client.New()
    cc.SetCookieJar(jar)

    _, err := cc.Get("https://httpbin.org/cookies/set/john/doe")
    if err != nil {
        panic(err)
    }

    uri := fasthttp.AcquireURI()
    defer fasthttp.ReleaseURI(uri)

    uri.SetHost("httpbin.org")
    uri.SetPath("/cookies")
    fmt.Println(jar.Get(uri))
}
```

<details>
<summary>Click here to see the result</summary>

```plaintext
[john=doe; path=/]
```

</details>

### Response (follow-up request)

```go
func main() {
    jar := client.AcquireCookieJar()
    defer client.ReleaseCookieJar(jar)

    cc := client.New()
    cc.SetCookieJar(jar)

    _, err := cc.Get("https://httpbin.org/cookies/set/john/doe")
    if err != nil {
        panic(err)
    }

    resp, err := cc.Get("https://httpbin.org/cookies")
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.String())
}
```

<details>
<summary>Click here to see the result</summary>

```json
{
  "cookies": {
    "john": "doe"
  }
}
```

</details>
