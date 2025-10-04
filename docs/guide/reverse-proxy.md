---
id: reverse-proxy
title: reverse-proxy
description: >-
  Learn how to set up reverse proxies like Nginx or Traefik to enable modern
  HTTP capabilities in your Fiber application, including HTTP/2 and the
  experimental HTTP/3 (QUIC) support. This guide also covers basic reverse
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

Some Fiber features (like [`SendEarlyHints`](https://docs.gofiber.io/api/ctx#sendearlyhints)) require **HTTP/2 or newer**, which is easiest to enable using a reverse proxy.  


### Popular Reverse Proxies:

- [Nginx](https://nginx.org/)
- [Traefik](https://traefik.io/)
- [HA PROXY](https://www.haproxy.com/documentation/)
- [Caddy](https://caddyserver.com/docs/quick-starts/reverse-proxy)

### Enabling HTTP/2

Some features in Fiber, such as SendEarlyHints, require HTTP/2 or newer. If your app is served directly over HTTP/1.1, certain features may be ignored or not function as expected.

To enable HTTP/2 in production, run Fiber behind a reverse proxy that upgrades connections. Popular choices include Nginx and Traefik.

<details>
<summary>Nginx Example</summary>

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

This configuration enables HTTP/2 with TLS and proxies requests to your Fiber app on port 3000.
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

### HTTP/3 (QUIC) Support

Early Hints (103 responses) are defined for HTTP and can be delivered over HTTP/1.1 and HTTP/2/3. In practice, browsers process 103 most reliably over HTTP/2/3. Many reverse proxies also support HTTP/3 (QUIC):

- **Nginx**: Requires a recent build with QUIC/HTTP/3 patches.
- **Traefik**: Supports HTTP/3 via its entryPoint configuration.

Enabling HTTP/3 is optional but can provide lower latency and improved performance for clients that support it. If you enable HTTP/3, your Early Hints responses will still work as expected.
For more details, see the official documentation:

- [Nginx HTTP/2 Module](https://nginx.org/en/docs/http/ngx_http_v2_module.html)
- [Nginx QUIC / HTTP/3](https://nginx.org/en/docs/quic.html)
- [Traefik HTTP/3](https://doc.traefik.io/traefik/reference/install-configuration/entrypoints/#http3)
