---
id: reverse-proxy
title: 🔄 Reverse Proxy Configuration
description: >-
  Learn how to set up reverse proxies like Nginx or Traefik to enable modern
  HTTP capabilities in your Fiber application, including HTTP/2 and
  HTTP/3 (QUIC) support. This guide also covers basic reverse
  proxy configuration and links to external documentation.
sidebar_position: 4
---

## Reverse Proxies

Running Fiber behind a reverse proxy is a common production setup.
Reverse proxies can handle:

- **HTTPS/TLS termination** (offloading SSL certificates)
- **Protocol upgrades** (HTTP/2, HTTP/3 support)
- **Request routing & load balancing**
- **Caching & compression**
- **Security features** (rate limiting, WAF, DDoS mitigation)

Some Fiber features (like [`SendEarlyHints`](../api/ctx.md#sendearlyhints)) require **HTTP/2 or newer**, which is easiest to enable using a reverse proxy.

### Popular Reverse Proxies

- [Nginx](https://nginx.org/)
- [Traefik](https://traefik.io/)
- [HA PROXY](https://www.haproxy.com/)
- [Caddy](https://caddyserver.com/)

## Getting the Real Client IP Address

When your Fiber application is behind a reverse proxy, the TCP connection comes from the proxy server, not the actual client. To get the real client IP address, you need to configure Fiber to read it from proxy headers like `X-Forwarded-For`.

:::warning Security Warning
Proxy headers can be easily spoofed by malicious clients. **Always** configure `TrustProxyConfig` to validate the proxy IP address, otherwise attackers can forge headers to bypass IP-based access controls, rate limiting, or geolocation features.

In addition, your reverse proxy should be configured to **set or overwrite** the forwarding header you choose (for example, `X-Forwarded-For`) based on the real client connection, or to use its real IP / PROXY protocol features. Do not simply pass through client-supplied forwarding headers, or `c.IP()` may still be controlled by an attacker even when `TrustProxyConfig` is correct.
:::

### Configuration

To enable reading the client IP from proxy headers, you must configure **three settings**:

1. **`TrustProxy`** - Enable proxy header trust (must be `true`)
2. **`ProxyHeader`** - Specify which header contains the client IP
3. **`TrustProxyConfig`** - Define which proxy IPs to trust

```go title="Example - App Behind Nginx"
app := fiber.New(fiber.Config{
    // Enable proxy support
    TrustProxy: true,

    // Read client IP from X-Forwarded-For header
    ProxyHeader: fiber.HeaderXForwardedFor,

    // Trust requests from your Nginx proxy
    TrustProxyConfig: fiber.TrustProxyConfig{
        // Option 1: Trust specific proxy IPs
        Proxies: []string{"10.10.0.58", "192.168.1.0/24"},

        // Option 2: Or trust all private IPs (useful for internal load balancers)
        // Private: true,
    },
})
```

### Common Proxy Headers

Different proxies use different headers:

| Proxy/Service | Recommended Header | Config Value |
|---------------|-------------------|--------------|
| Nginx, HAProxy, Apache | X-Forwarded-For | `fiber.HeaderXForwardedFor` |
| Cloudflare | CF-Connecting-IP | `"CF-Connecting-IP"` |
| Fastly | Fastly-Client-IP | `"Fastly-Client-IP"` |
| Generic | X-Real-IP | `"X-Real-IP"` |

### TrustProxyConfig Options

The `TrustProxyConfig` struct provides multiple ways to specify trusted proxies:

```go
TrustProxyConfig: fiber.TrustProxyConfig{
    // Specific IPs or CIDR ranges
    Proxies: []string{
        "10.10.0.58",           // Single IP
        "192.168.0.0/24",       // CIDR range
        "2001:db8::/32",        // IPv6 range
    },

    // Or use convenience flags:
    Loopback:   true,  // Trust 127.0.0.0/8, ::1/128
    LinkLocal:  true,  // Trust 169.254.0.0/16, fe80::/10
    Private:    true,  // Trust 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7
    UnixSocket: true,  // Trust Unix domain socket connections
},
```

### Complete Example with Nginx

```nginx title="nginx.conf"
server {
    listen 443 ssl;
    http2 on;
    server_name example.com;

    ssl_certificate     /etc/ssl/certs/example.crt;
    ssl_certificate_key /etc/ssl/private/example.key;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

```go title="main.go"
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New(fiber.Config{
        TrustProxy:        true,
        ProxyHeader:       fiber.HeaderXForwardedFor,
        EnableIPValidation: true,
        TrustProxyConfig: fiber.TrustProxyConfig{
            // Trust localhost since Nginx is on the same machine
            Loopback: true,
        },
    })

    app.Get("/", func(c fiber.Ctx) error {
        // This will now return the real client IP from X-Forwarded-For
        // instead of 127.0.0.1
        return c.SendString("Your IP: " + c.IP())
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Testing Your Configuration

You can verify your configuration is working:

```go
app.Get("/debug", func(c fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "c.IP()":           c.IP(),                    // Should show real client IP
        "X-Forwarded-For":  c.Get("X-Forwarded-For"),  // Raw header value
        "IsProxyTrusted":   c.IsProxyTrusted(),        // Should be true
        "RemoteIP":         c.RequestCtx().RemoteIP().String(), // Proxy IP
    })
})
```

## Enabling HTTP/2

Popular choices include Nginx and Traefik.

<details>
<summary>Nginx Example</summary>

See the [Complete Example with Nginx](#complete-example-with-nginx) above for a full configuration with HTTP/2 enabled.
</details>
<details>
<summary>Traefik Example</summary>

```yaml title="traefik.yaml"
entryPoints:
  websecure:
    address: ":443"

http:
  routers:
    app:
      rule: "Host(`example.com`)"
      entryPoints:
        - websecure
      service: app
      tls: {}

  services:
    app:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:3000"
```

With this configuration, Traefik terminates TLS and serves your app over HTTP/2.
</details>

## HTTP/3 (QUIC) Support

Early Hints (103 responses) are defined for HTTP and can be delivered over HTTP/1.1 and HTTP/2/3. In practice, browsers process 103 most reliably over HTTP/2/3. Many reverse proxies also support HTTP/3 (QUIC):

- **Nginx**
- **Traefik**

Enabling HTTP/3 is optional but can provide lower latency and improved performance for clients that support it. If you enable HTTP/3, your Early Hints responses will still work as expected.
For more details, see the official documentation:

- [Nginx QUIC / HTTP/3](https://nginx.org/en/docs/quic.html)
- [Traefik HTTP/3](https://doc.traefik.io/traefik/reference/install-configuration/entrypoints/#http3)
