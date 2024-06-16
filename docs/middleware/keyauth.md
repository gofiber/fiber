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
		KeyLookup:  "cookie:access_token",
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
# No api-key specified -> 400 missing 
curl http://localhost:3000
#> missing or malformed API Key

curl --cookie "access_token=correct horse battery staple" http://localhost:3000
#> Successfully authenticated!

curl --cookie "access_token=Clearly A Wrong Key" http://localhost:3000
#>  missing or malformed API Key
```

For a more detailed example, see also the [`github.com/gofiber/recipes`](https://github.com/gofiber/recipes) repository and specifically the `fiber-envoy-extauthz` repository and the [`keyauth example`](https://github.com/gofiber/recipes/blob/master/fiber-envoy-extauthz/authz/main.go) code.


### Authenticate only certain endpoints

If you want to authenticate only certain endpoints, you can use the `Config` of keyauth and apply a filter function (eg. `authFilter`) like so

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
			return false
		}
	}
	return true
}

func main() {
	app := fiber.New()

	app.Use(keyauth.New(keyauth.Config{
		Next:    authFilter,
		KeyLookup: "cookie:access_token",
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

Which results in this

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

Which results in this

```bash
# / does not need to be authenticated
curl http://localhost:3000
#> Welcome

# /allowed needs to be authenticated too
curl --header "Authorization: Bearer my-super-secret-key"  http://localhost:3000/allowed
#> Successfully authenticated!
```

## Config

| Property        | Type                                     | Description                                                                                            | Default                       |
|:----------------|:-----------------------------------------|:-------------------------------------------------------------------------------------------------------|:------------------------------|
| Next            | `func(fiber.Ctx) bool`                   | Next defines a function to skip this middleware when returned true.                                    | `nil`                         |
| SuccessHandler  | `fiber.Handler`                          | SuccessHandler defines a function which is executed for a valid key.                                   | `nil`                         |
| ErrorHandler    | `fiber.ErrorHandler`                     | ErrorHandler defines a function which is executed for an invalid key.                                  | `401 Invalid or expired key`  |
| KeyLookup       | `string`                                 | KeyLookup is a string in the form of "`<source>:<name>`" that is used to extract the key from the request. | "header:Authorization"        |
| CustomKeyLookup | `KeyauthKeyLookupFunc` aka `func(c fiber.Ctx) (string, error)` | If more complex logic is required to extract the key from the request, an arbitrary function to extract it can be specified here. Utility helper functions are described below. |  `nil` |
| AuthScheme      | `string`                                 | AuthScheme to be used in the Authorization header.                                                     | "Bearer"                      |
| Validator       | `func(fiber.Ctx, string) (bool, error)`  | Validator is a function to validate the key.                                                           | A function for key validation |

## Default Config

```go
var ConfigDefault = Config{
	SuccessHandler: func(c fiber.Ctx) error {
		return c.Next()
	},
	ErrorHandler: func(c fiber.Ctx, err error) error {
		if err == ErrMissingOrMalformedAPIKey {
			return c.Status(fiber.StatusUnauthorized).SendString(err.Error())
		}
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid or expired API Key")
	},
	KeyLookup:  "header:" + fiber.HeaderAuthorization,
	CustomKeyLookup: nil,
	AuthScheme: "Bearer",
}
```

## CustomKeyLookup

Two public utility functions are provided that may be useful when creating custom extraction:
* `DefaultKeyLookup(keyLookup string, authScheme string)`: This is the function that implements the default `KeyLookup` behavior, exposed to be used as a component of custom parsing logic
* `MultipleKeySourceLookup(keyLookups []string, authScheme string)`: Creates a CustomKeyLookup function that checks each listed source using the above function until a key is found or the options are all exhausted. For example, `MultipleKeySourceLookup([]string{"header:Authorization", "header:x-api-key", "cookie:apikey"}, "Bearer")` would first check the standard Authorization header, checks the `x-api-key` header next, and finally checks for a cookie named `apikey`. If any of these contain a valid API key, the request continues. Otherwise, an error is returned.
