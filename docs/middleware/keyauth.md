---
id: keyauth
---

# KeyAuth

The KeyAuth middleware implements API key authentication.

## Signatures

```go
func New(config ...Config) fiber.Handler
func TokenFromContext(c fiber.Ctx) string
```

## Examples

### Basic example

This example registers KeyAuth with an API key stored in a cookie.

```go
package main

import (
    "crypto/sha256"
    "crypto/subtle"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/keyauth"
)

var (
    apiKey = "correct horse battery staple"
)

func validateAPIKey(c fiber.Ctx, key string) (bool, error) {
    hashedAPIKey := sha256.Sum256([]byte(apiKey))
    hashedKey := sha256.Sum256([]byte(key))

    if subtle.ConstantTimeCompare(hashedAPIKey[:], hashedKey[:]) == 1 {
        return true, nil
    }
    return false, keyauth.ErrMissingOrMalformedAPIKey
}

func main() {
    app := fiber.New()

    // Register middleware before the routes that need it
    app.Use(keyauth.New(keyauth.Config{
        Extractor:  keyauth.FromCookie("access_token"),
        Validator:  validateAPIKey,
    }))

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Successfully authenticated!")
    })

    app.Listen(":3000")
}
```

**Test:**

```bash
# No API key specified -> 401 Missing or invalid API Key
curl http://localhost:3000
#> Missing or invalid API Key

# Correct API key -> 200 OK
curl --cookie "access_token=correct horse battery staple" http://localhost:3000
#> Successfully authenticated!

# Incorrect API key -> 401 Missing or invalid API Key
curl --cookie "access_token=Clearly A Wrong Key" http://localhost:3000
#> Missing or invalid API Key
```

For a more detailed example, see the [`fiber-envoy-extauthz`](https://github.com/gofiber/recipes/tree/master/fiber-envoy-extauthz) recipe in the `gofiber/recipes` repository.

### Authenticate only certain endpoints

Use the `Next` function to run KeyAuth only on selected routes.

```go
package main

import (
    "crypto/sha256"
    "crypto/subtle"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/keyauth"
    "regexp"
    "strings"
)

var (
    apiKey        = "correct horse battery staple"
    protectedURLs = []*regexp.Regexp{
        regexp.MustCompile("^/authenticated$"),
        regexp.MustCompile("^/auth2$"),
    }
)

func validateAPIKey(c fiber.Ctx, key string) (bool, error) {
    hashedAPIKey := sha256.Sum256([]byte(apiKey))
    hashedKey := sha256.Sum256([]byte(key))

    if subtle.ConstantTimeCompare(hashedAPIKey[:], hashedKey[:]) == 1 {
        return true, nil
    }
    return false, keyauth.ErrMissingOrMalformedAPIKey
}

func authFilter(c fiber.Ctx) bool {
    originalURL := strings.ToLower(c.OriginalURL())

    for _, pattern := range protectedURLs {
        if pattern.MatchString(originalURL) {
            // Run middleware for protected routes
            return false
        }
    }
    // Skip middleware for non-protected routes
    return true
}

func main() {
    app := fiber.New()

    app.Use(keyauth.New(keyauth.Config{
        Next:      authFilter,
        Extractor: keyauth.FromCookie("access_token"),
        Validator: validateAPIKey,
    }))

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Welcome")
    })
    app.Get("/authenticated", func(c fiber.Ctx) error {
        return c.SendString("Successfully authenticated!")
    })
    app.Get("/auth2", func(c fiber.Ctx) error {
        return c.SendString("Successfully authenticated 2!")
    })

    app.Listen(":3000")
}
```

**Test:**

```bash
# / doesn't require authentication
curl http://localhost:3000
#> Welcome

# /authenticated requires authentication
curl --cookie "access_token=correct horse battery staple" http://localhost:3000/authenticated
#> Successfully authenticated!

# /auth2 requires authentication too
curl --cookie "access_token=correct horse battery staple" http://localhost:3000/auth2
#> Successfully authenticated 2!
```

### Apply middleware in the handler

You can apply the middleware to specific routes or groups instead of globally. This example uses the default extractor (`FromAuthHeader`).

```go
package main

import (
    "crypto/sha256"
    "crypto/subtle"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/keyauth"
)

const (
  apiKey = "my-super-secret-key"
)

func main() {
    app := fiber.New()

    authMiddleware := keyauth.New(keyauth.Config{
        Validator:  func(c fiber.Ctx, key string) (bool, error) {
            hashedAPIKey := sha256.Sum256([]byte(apiKey))
            hashedKey := sha256.Sum256([]byte(key))

            if subtle.ConstantTimeCompare(hashedAPIKey[:], hashedKey[:]) == 1 {
                return true, nil
            }
            return false, keyauth.ErrMissingOrMalformedAPIKey
        },
    })

    app.Get("/", func(c fiber.Ctx) error {
        return c.SendString("Welcome")
    })

    app.Get("/allowed",  authMiddleware, func(c fiber.Ctx) error {
        return c.SendString("Successfully authenticated!")
    })

    app.Listen(":3000")
}
```

**Test:**

```bash
# / doesn't require authentication
curl http://localhost:3000
#> Welcome

# /allowed requires authentication
curl --header "Authorization: Bearer my-super-secret-key"  http://localhost:3000/allowed
#> Successfully authenticated!
```

## Key Extractors

The middleware extracts the API key from the request using an `Extractor`. You can specify one or more extractors in the configuration.

### Built-in Extractors

The following extractors are available:

- `keyauth.FromHeader(header string)`: Extracts the key from the specified header.
- `keyauth.FromAuthHeader(header, authScheme string)`: Extracts the key from an authorization header (e.g., `Authorization: Bearer <key>`).
- `keyauth.FromQuery(param string)`: Extracts the key from a URL query parameter.
- `keyauth.FromParam(param string)`: Extracts the key from a URL path parameter.
- `keyauth.FromCookie(name string)`: Extracts the key from a cookie.
- `keyauth.FromForm(name string)`: Extracts the key from a form field.

### Chaining Extractors

You can use `keyauth.Chain` to try multiple extractors until one succeeds. The first successful extraction is used.

```go
// This will try to extract the key from:
// 1. The "X-API-Key" header
// 2. The "api_key" query parameter
app.Use(keyauth.New(keyauth.Config{
    Extractor: keyauth.Chain(
        keyauth.FromHeader("X-API-Key"),
        keyauth.FromQuery("api_key"),
    ),
    Validator: validateAPIKey,
}))
```

## Config

| Property        | Type                                     | Description                                                                                            | Default                       |
|:----------------|:-----------------------------------------|:-------------------------------------------------------------------------------------------------------|:------------------------------|
| Next            | `func(fiber.Ctx) bool`                   | Next defines a function to skip this middleware when it returns true.                                    | `nil`                         |
| SuccessHandler  | `fiber.Handler`                          | SuccessHandler defines a function which is executed for a valid key.                                   | `c.Next()`                         |
| ErrorHandler    | `fiber.ErrorHandler`                     | ErrorHandler defines a function which is executed for an invalid key. By default a 401 response with a `WWW-Authenticate` challenge is sent. | Default error handler  |
| Validator       | `func(fiber.Ctx, string) (bool, error)`  | **Required.** Validator is a function to validate the key.                                                           | `nil` (panic) |
| Extractor       | `keyauth.Extractor`                    | Extractor defines how to retrieve the key from the request. Use helper functions like `keyauth.FromAuthHeader` or `keyauth.FromCookie`. | `keyauth.FromAuthHeader("Authorization", "Bearer")` |
| Realm           | `string`                                 | Realm specifies the protected area name used in the `WWW-Authenticate` header. | `"Restricted"` |

## Default Config

```go
var ConfigDefault = Config{
    SuccessHandler: func(c fiber.Ctx) error {
        return c.Next()
    },
    ErrorHandler: func(c fiber.Ctx, _ error) error {
        return c.Status(fiber.StatusUnauthorized).SendString(ErrMissingOrMalformedAPIKey.Error())
    },
    Realm:     "Restricted",
    Extractor: FromAuthHeader(fiber.HeaderAuthorization, "Bearer"),
}
```
