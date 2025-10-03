---
id: reverse-proxy
title: Reverse Proxies
description: >-
  Learn how to set up reverse proxies like Nginx or Traefik to enable modern
  HTTP capabilities in your Fiber application, including HTTP/2 and the
  experimental HTTP/3 (QUIC) support. This guide also covers basic reverse
  proxy configuration and links to external documentation.
sidebar_position: 4
---

# Reverse Proxies

## Enabling HTTP/2

Some features in Fiber, such as SendEarlyHints, require HTTP/2 or newer. If your app is served directly over HTTP/1.1, certain features may be ignored or not function as expected.

To enable HTTP/2 in production, run Fiber behind a reverse proxy that upgrades connections. Popular choices include Nginx and Traefik.

<details>
<summary>Nginx Example</summary>

```nginx title="nginx.conf"
server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate     /etc/ssl/certs/example.crt;
    ssl_certificate_key /etc/ssl/private/example.key;

    location / {
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
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

## HTTP/3 (QUIC) Support

Early Hints (103 responses) are officially part of HTTP/2 and newer. Many reverse proxies also support HTTP/3 (QUIC):

- **Nginx**: Requires a recent build with QUIC/HTTP3 patches.
- **Traefik**: Supports HTTP/3 via its entryPoint configuration.

Enabling HTTP/3 is optional but can provide lower latency and improved performance for clients that support it. If you enable HTTP/3, your Early Hints responses will still work as expected.

For more details, see the official documentation:

- [Nginx HTTP/2 Module](https://nginx.org/en/docs/http/ngx_http_v2_module.html)
- [Nginx QUIC / HTTP/3](https://nginx.org/en/docs/quic.html)
- [Traefik HTTP/3](https://doc.traefik.io/traefik/reference/install-configuration/entrypoints/#http3)

Popular Reverse Proxies
- [Nginx](https://nginx.org/)
- [Traefik](https://traefik.io/)
- [HAProxy](https://www.haproxy.org/)
