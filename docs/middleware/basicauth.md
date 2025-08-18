---
id: basicauth
---

# BasicAuth

Basic Authentication middleware for [Fiber](https://github.com/gofiber/fiber) that provides an HTTP basic authentication. It calls the next handler for valid credentials and [401 Unauthorized](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/401) or a custom response for missing or invalid credentials.

The default unauthorized response includes the header `WWW-Authenticate: Basic realm="Restricted", charset="UTF-8"`, sets `Cache-Control: no-store`, and adds a `Vary: Authorization` header.

## Signatures

```go
func New(config Config) fiber.Handler
func UsernameFromContext(c fiber.Ctx) string
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/basicauth"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Provide a minimal config
app.Use(basicauth.New(basicauth.Config{
    Users: map[string]string{
        // "doe" hashed using SHA-256
        "john":  "{SHA256}eZ75KhGvkY4/t0HfQpNPO1aO0tk6wd908bjUGieTKm8=",
        // "123456" hashed using bcrypt
        "admin": "$2a$10$gTYwCN66/tBRoCr3.TXa1.v1iyvwIF7GRBqxzv7G.AHLMt/owXrp.",
    },
}))

// Or extend your config for customization
app.Use(basicauth.New(basicauth.Config{
    Users: map[string]string{
        // "doe" hashed using SHA-256
        "john":  "{SHA256}eZ75KhGvkY4/t0HfQpNPO1aO0tk6wd908bjUGieTKm8=",
        // "123456" hashed using bcrypt
        "admin": "$2a$10$gTYwCN66/tBRoCr3.TXa1.v1iyvwIF7GRBqxzv7G.AHLMt/owXrp.",
    },
    Realm: "Forbidden",
    Authorizer: func(user, pass string, c fiber.Ctx) bool {
        // custom validation logic
        return (user == "john" || user == "admin")
    },
    Unauthorized: func(c fiber.Ctx) error {
        return c.SendFile("./unauthorized.html")
    },
}))
```

Getting the username and password

### Password hashes

Passwords must be supplied in pre-hashed form. The middleware detects the
hashing algorithm from a prefix:

- `"{SHA512}"` or `"{SHA256}"` followed by a base64 encoded digest
- standard bcrypt strings beginning with `$2`

If no prefix is present the value is interpreted as a SHA-256 digest encoded in
hex or base64. Plaintext passwords are rejected.

## Config

| Property        | Type                        | Description                                                                                                                                                           | Default               |
|:----------------|:----------------------------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------|:----------------------|
| Next            | `func(fiber.Ctx) bool`     | Next defines a function to skip this middleware when returned true.                                                                                                   | `nil`                 |
| Users           | `map[string]string`         | Users maps usernames to **hashed** passwords (e.g. bcrypt, `{SHA256}`). | `map[string]string{}` |
| Realm           | `string`                    | Realm is a string to define the realm attribute of BasicAuth. The realm identifies the system to authenticate against and can be used by clients to save credentials. | `"Restricted"`        |
| Charset         | `string`                    | Charset sent in the `WWW-Authenticate` header, so clients know how credentials are encoded. | `"UTF-8"` |
| HeaderLimit     | `int`                       | Maximum allowed length of the `Authorization` header. Requests exceeding this limit are rejected. | `8192` |
| Authorizer      | `func(string, string, fiber.Ctx) bool` | Authorizer defines a function to check the credentials. It will be called with a username, password, and the current context and is expected to return true or false to indicate approval.  | `nil`                 |
| Unauthorized    | `fiber.Handler`             | Unauthorized defines the response body for unauthorized responses.                                                                                                    | `nil`                 |

## Default Config

```go
var ConfigDefault = Config{
    Next:            nil,
    Users:           map[string]string{},
    Realm:           "Restricted",
    Charset:         "UTF-8",
    HeaderLimit:     8192,
    Authorizer:      nil,
    Unauthorized:    nil,
}
```
