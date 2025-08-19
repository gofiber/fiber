---
id: keyauth
---

# Keyauth

Key auth middleware provides a key based authentication.

## Signatures

```go
func New(config ...Config) fiber.Handler
func TokenFromContext(c fiber.Ctx) string
```

## Examples

### Basic Example

This example shows how to use the KeyAuth middleware with an API key passed in a cookie.

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

    // note that the keyauth middleware needs to be defined before the routes are defined!
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
# No api-key specified -> 401 Invalid or expired API Key
curl http://localhost:3000
#> Invalid or expired API Key

# Correct API key -> 200 OK
curl --cookie "access_token=correct horse battery staple" http://localhost:3000
#> Successfully authenticated!

# Incorrect API key -> 401 Invalid or expired API Key
curl --cookie "access_token=Clearly A Wrong Key" http://localhost:3000
#> Invalid or expired API Key
```

For a more detailed example, see also the [`github.com/gofiber/recipes`](https://github.com/gofiber/recipes) repository and specifically the `fiber-envoy-extauthz` repository and the [`keyauth example`](https://github.com/gofiber/recipes/blob/master/fiber-envoy-extauthz/authz/main.go) code.

### Authenticate only certain endpoints

If you want to authenticate only certain endpoints, you can use the `Next` function in the config to skip the middleware for specific routes.

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
# / does not need to be authenticated
curl http://localhost:3000
#> Welcome

# /authenticated needs to be authenticated
curl --cookie "access_token=correct horse battery staple" http://localhost:3000/authenticated
#> Successfully authenticated!

# /auth2 needs to be authenticated too
curl --cookie "access_token=correct horse battery staple" http://localhost:3000/auth2
#> Successfully authenticated 2!
```

### Specifying middleware in the handler

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
# / does not need to be authenticated
curl http://localhost:3000
#> Welcome

# /allowed needs to be authenticated
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

You can use `keyauth.Chain` to try multiple extractors in order until one succeeds. The first successful extraction will be used.

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
| Next            | `func(fiber.Ctx) bool`                   | Next defines a function to skip this middleware when returned true.                                    | `nil`                         |
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
    ErrorHandler: func(c fiber.Ctx, err error) error {
        return c.Status(fiber.StatusUnauthorized).SendString("Invalid or expired API Key")
    },
    Realm:     "Restricted",
    Extractor: FromAuthHeader(fiber.HeaderAuthorization, "Bearer"),
}
```
