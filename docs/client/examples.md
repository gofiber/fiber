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
