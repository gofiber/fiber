---
id: csrf
---

# CSRF

CSRF middleware for [Fiber](https://github.com/gofiber/fiber) that provides [Cross-site request forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) protection by passing a csrf token via cookies. This cookie value will be used to compare against the client csrf token on requests, other than those defined as "safe" by RFC7231 \(GET, HEAD, OPTIONS, or TRACE\). When the csrf token is invalid, this middleware will return the `fiber.ErrForbidden` error. 

CSRF Tokens are generated on GET requests. You can retrieve the CSRF token with `c.Locals(contextKey)`, where `contextKey` is the string you set in the config (see Custom Config below).

When no `csrf_` cookie is set, or the token has expired, a new token will be generated and `csrf_` cookie set.

:::note
This middleware uses our [Storage](https://github.com/gofiber/storage) package to support various databases through a single interface. The default configuration for this middleware saves data to memory, see the examples below for other databases.
:::

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/csrf"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
app.Use(csrf.New())

// Or extend your config for customization
app.Use(csrf.New(csrf.Config{
    KeyLookup:      "header:X-Csrf-Token",
    CookieName:     "csrf_",
	CookieSameSite: "Lax",
    Expiration:     1 * time.Hour,
    KeyGenerator:   utils.UUID,
    Extractor:      func(c *fiber.Ctx) (string, error) { ... },
}))
```

:::note
KeyLookup will be ignored if Extractor is explicitly set.
:::

## Config

### Config

| Property          | Type                               | Description                                                                                                                                                                                                                                                                      | Default                      |
|:------------------|:-----------------------------------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:-----------------------------|
| Next              | `func(*fiber.Ctx) bool`            | Next defines a function to skip this middleware when returned true.                                                                                                                                                                                                              | `nil`                        |
| KeyLookup         | `string`                           | KeyLookup is a string in the form of "<source>:<key>" that is used to create an Extractor that extracts the token from the request. Possible values: "header:<name>", "query:<name>", "param:<name>", "form:<name>", "cookie:<name>". Ignored if an Extractor is explicitly set. | "header:X-CSRF-Token"        |
| CookieName        | `string`                           | Name of the session cookie. This cookie will store the session key.                                                                                                                                                                                                              | "csrf_"                      |
| CookieDomain      | `string`                           | Domain of the CSRF cookie.                                                                                                                                                                                                                                                       | ""                           |
| CookiePath        | `string`                           | Path of the CSRF cookie.                                                                                                                                                                                                                                                         | ""                           |
| CookieSecure      | `bool`                             | Indicates if the CSRF cookie is secure.                                                                                                                                                                                                                                          | false                        |
| CookieHTTPOnly    | `bool`                             | Indicates if the CSRF cookie is HTTP-only.                                                                                                                                                                                                                                       | false                        |
| CookieSameSite    | `string`                           | Value of SameSite cookie.                                                                                                                                                                                                                                                        | "Lax"                        |
| CookieSessionOnly | `bool`                             | Decides whether the cookie should last for only the browser session. Ignores Expiration if set to true.                                                                                                                                                                          | false                        |
| Expiration        | `time.Duration`                    | Expiration is the duration before the CSRF token will expire.                                                                                                                                                                                                                    | 1 * time.Hour                |
| Storage           | `fiber.Storage`                    | Store is used to store the state of the middleware.                                                                                                                                                                                                                              | memory.New()                 |
| ContextKey        | `string`                           | Context key to store the generated CSRF token into the context. If left empty, the token will not be stored in the context.                                                                                                                                                      | ""                           |
| KeyGenerator      | `func() string`                    | KeyGenerator creates a new CSRF token.                                                                                                                                                                                                                                           | utils.UUID                   |
| CookieExpires     | `time.Duration` (Deprecated)       | Deprecated: Please use Expiration.                                                                                                                                                                                                                                               | 0                            |
| Cookie            | `*fiber.Cookie` (Deprecated)       | Deprecated: Please use Cookie* related fields.                                                                                                                                                                                                                                   | nil                          |
| TokenLookup       | `string` (Deprecated)              | Deprecated: Please use KeyLookup.                                                                                                                                                                                                                                                | ""                           |
| ErrorHandler      | `fiber.ErrorHandler`               | ErrorHandler is executed when an error is returned from fiber.Handler.                                                                                                                                                                                                           | DefaultErrorHandler          |
| Extractor         | `func(*fiber.Ctx) (string, error)` | Extractor returns the CSRF token. If set, this will be used in place of an Extractor based on KeyLookup.                                                                                                                                                                         | Extractor based on KeyLookup |

## Default Config

```go
var ConfigDefault = Config{
	KeyLookup:      "header:" + HeaderName,
	CookieName:     "csrf_",
	CookieSameSite: "Lax",
	Expiration:     1 * time.Hour,
	KeyGenerator:   utils.UUID,
	ErrorHandler:   defaultErrorHandler,
	Extractor:      CsrfFromHeader(HeaderName),
}
```

## Constants

```go
const (
    HeaderName = "X-Csrf-Token"
)
```

### Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3
app.Use(csrf.New(csrf.Config{
	Storage: storage,
}))
```
