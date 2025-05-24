---
id: security
title: 🔒 Security Helpers
sidebar_position: 8
---

Fiber provides helper functions for common security tasks like extracting API keys or credentials from a request.
These utilities can be used with your middleware or handlers.

## API Key helpers

```go
import "github.com/gofiber/fiber/v3/security"

func handler(c fiber.Ctx) error {
    key, err := security.APIKeyHeader(c, "X-API-Key")
    if err != nil {
        return err
    }
    // use key
    return nil
}
```

Available helpers:

- `APIKeyCookie(c fiber.Ctx, name string)`
- `APIKeyHeader(c fiber.Ctx, header string)`
- `APIKeyQuery(c fiber.Ctx, name string)`

Each returns the key or `fiber.ErrUnauthorized` when the key is missing.

```go
// Cookie
key, _ := security.APIKeyCookie(c, "session")

// Query parameter
key, _ = security.APIKeyQuery(c, "api_key")
```

## Authorization helpers

```go
cred, err := security.GetAuthorizationCredentials(c)
```

Use `HTTPBearer`, `HTTPBasic`, or `HTTPDigest` to parse common Authorization schemes.

```go
bearer, err := security.HTTPBearer(c)
```

```go
user, err := security.HTTPBasic(c)
```

```go
digest, err := security.HTTPDigest(c)
```

`HTTPBasic` returns `HTTPBasicCredentials` containing the parsed username and password.
