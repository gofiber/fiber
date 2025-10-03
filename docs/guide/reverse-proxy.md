---
id: reverse-proxy
title: 🔄 Reverse Proxy Configuration
description: Configure reverse proxies to enable HTTP/2 and HTTP/3 support for your Fiber applications
sidebar_position: 12
---

Many Fiber features like `SendEarlyHints` require HTTP/2 or newer protocols to function properly. Since Go's standard HTTP server has limitations with HTTP/2 in certain configurations, using a reverse proxy is often the most practical solution to enable these modern protocol features.

## Why Use a Reverse Proxy?

- **Protocol Upgrade**: Convert HTTP/1.1 connections to HTTP/2 or HTTP/3
- **TLS Termination**: Handle SSL/TLS certificates and encryption
- **Load Balancing**: Distribute traffic across multiple Fiber instances
- **Caching**: Improve performance with reverse proxy caching
- **Security**: Add additional security layers and request filtering

## Popular Reverse Proxies

### Nginx

Nginx is a widely used reverse proxy that provides excellent HTTP/2 and HTTP/3 support.

#### Basic HTTP/2 Configuration

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /path/to/your/certificate.crt;
    ssl_certificate_key /path/to/your/private.key;

    # Modern SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers on;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

#### HTTP/3 Configuration (Experimental)

```nginx
server {
    listen 443 ssl http2;
    listen 443 quic reuseport;
    
    server_name your-domain.com;
    
    ssl_certificate /path/to/your/certificate.crt;
    ssl_certificate_key /path/to/your/private.key;
    
    # HTTP/3 specific settings
    ssl_protocols TLSv1.3;
    
    # Add Alt-Svc header for HTTP/3 discovery
    add_header Alt-Svc 'h3=":443"; ma=86400';
    
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### Traefik

Traefik is a modern reverse proxy with automatic service discovery and excellent HTTP/2 support.

#### Docker Compose Configuration

```yaml
version: '3.8'

services:
  traefik:
    image: traefik:v3.0
    command:
      - --api.dashboard=true
      - --entrypoints.websecure.address=:443
      - --entrypoints.websecure.http.tls=true
      - --entrypoints.websecure.http2.maxConcurrentStreams=250
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --certificatesresolvers.letsencrypt.acme.email=your-email@domain.com
      - --certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json
      - --certificatesresolvers.letsencrypt.acme.tlschallenge=true
    ports:
      - "443:443"
      - "8080:8080" # Dashboard access - secure this in production!
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./letsencrypt:/letsencrypt

  fiber-app:
    image: your-fiber-app:latest
    expose:
      - "3000"
    labels:
      - traefik.enable=true
      - traefik.http.routers.fiber.rule=Host(`your-domain.com`)
      - traefik.http.routers.fiber.entrypoints=websecure
      - traefik.http.routers.fiber.tls.certresolver=letsencrypt
      - traefik.http.services.fiber.loadbalancer.server.port=3000
```

#### File-based Configuration

```yaml
# traefik.yml
api:
  dashboard: true

entryPoints:
  websecure:
    address: ":443"
    http:
      tls: {}
      middlewares:
        - security-headers@file
    http2:
      maxConcurrentStreams: 250

providers:
  file:
    filename: /etc/traefik/dynamic.yml

certificatesResolvers:
  letsencrypt:
    acme:
      email: your-email@domain.com
      storage: /letsencrypt/acme.json
      tlsChallenge: {}
```

```yaml
# dynamic.yml
http:
  middlewares:
    security-headers:
      headers:
        customResponseHeaders:
          X-Frame-Options: "DENY"
          X-Content-Type-Options: "nosniff"

  routers:
    fiber-router:
      rule: "Host(`your-domain.com`)"
      entryPoints:
        - websecure
      service: fiber-service
      tls:
        certResolver: letsencrypt

  services:
    fiber-service:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:3000"
```

### Caddy

Caddy automatically handles HTTPS and HTTP/2, making it one of the simplest options for enabling modern protocols.

#### Basic Caddyfile

```caddy
your-domain.com {
    reverse_proxy 127.0.0.1:3000
    
    # Automatically enables HTTP/2 and HTTPS
    # HTTP/3 is experimental but can be enabled:
    # protocols h1 h2 h3
}
```

#### Advanced Configuration

```caddy
your-domain.com {
    # Enable HTTP/3 (experimental)
    protocols h1 h2 h3
    
    # Caddy automatically sets the required headers for Fiber (Host, X-Real-IP, etc.)
    reverse_proxy 127.0.0.1:3000
    
    # Additional security headers
    header {
        # Security headers
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "strict-origin-when-cross-origin"
    }
}
```

## Configuring Fiber for Reverse Proxy

When using a reverse proxy, configure your Fiber application to trust proxy headers:

```go
package main

import (
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New(fiber.Config{
        // Trust proxy headers
        TrustProxy: true,
        TrustProxyConfig: &fiber.TrustProxyConfig{
            Proxies: []string{
                "127.0.0.1",
                "10.0.0.0/8",
                "172.16.0.0/12",
                "192.168.0.0/16",
            },
        },
        ProxyHeader: fiber.HeaderXForwardedFor,
    })

    // Your routes here
    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    // Example using SendEarlyHints (requires HTTP/2)
    app.Get("/early-hints", func(c fiber.Ctx) error {
        hints := []string{
            "<https://cdn.example.com/app.js>; rel=preload; as=script",
            "<https://cdn.example.com/style.css>; rel=preload; as=style",
        }
        
        if err := c.SendEarlyHints(hints); err != nil {
            return err
        }
        
        return c.SendString("Page with early hints")
    })

    app.Listen(":3000")
}
```

## Testing HTTP/2 Configuration

You can verify that HTTP/2 is working correctly using curl or browser developer tools:

```bash
# Test with curl
curl -I --http2 https://your-domain.com

# Check protocol version
curl -w "%{http_version}\n" -o /dev/null -s https://your-domain.com
```

Look for the HTTP/2 protocol indicator in the response or browser network tab.

## Performance Considerations

- **Connection Multiplexing**: HTTP/2 allows multiple requests over a single connection
- **Server Push**: Some reverse proxies support HTTP/2 server push
- **Compression**: Enable compression at the reverse proxy level for better performance
- **Keep-Alive**: Configure appropriate keep-alive settings for persistent connections

## Security Notes

When using reverse proxies:

1. **Always validate trusted proxy ranges** to prevent IP spoofing
2. **Use TLS between proxy and Fiber** in production environments
3. **Configure proper timeout values** to prevent resource exhaustion
4. **Monitor proxy logs** for suspicious activity
5. **Keep proxy software updated** for security patches

## References

- [Nginx HTTP/2 Module](https://nginx.org/en/docs/http/ngx_http_v2_module.html)
- [Nginx HTTP/3 Support](https://nginx.org/en/docs/http/ngx_http_v3_module.html)
- [Traefik HTTP/2 Configuration](https://doc.traefik.io/traefik/routing/entrypoints/#http2)
- [Traefik HTTP/3 Support](https://doc.traefik.io/traefik/routing/entrypoints/#http3)
- [Caddy Reverse Proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
