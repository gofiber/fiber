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
                "john": "doe",
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

## Cookiejar

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

### Response 2

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
