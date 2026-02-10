---
id: basicauth
---

# BasicAuth

Basic Authentication middleware for [Fiber](https://github.com/gofiber/fiber) that provides HTTP basic auth. It calls the next handler for valid credentials and returns [`401 Unauthorized`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/401) for missing or invalid credentials, [`400 Bad Request`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/400) for malformed `Authorization` headers, or [`431 Request Header Fields Too Large`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/431) when the header exceeds size limits. Credentials may omit Base64 padding as permitted by RFC 7235's `token68` syntax.

The default unauthorized response includes the header `WWW-Authenticate: Basic realm="Restricted", charset="UTF-8"`, sets `Cache-Control: no-store`, and adds a `Vary: Authorization` header. Only the `UTF-8` charset is supported; any other value will panic.

## Signatures

```go
func New(config Config) fiber.Handler
func UsernameFromContext(ctx any) string
```

`UsernameFromContext` accepts a `fiber.Ctx`, a `*fasthttp.RequestCtx`, `fiber.CustomCtx`, or a `context.Context`.

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/basicauth"
)
```

Once your Fiber app is initialized, choose one of the following approaches:

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

### Password hashes

Passwords must be supplied in pre-hashed form. The middleware detects the
hashing algorithm from a prefix:

- `"{SHA512}"` or `"{SHA256}"` followed by a base64-encoded digest
- standard bcrypt strings beginning with `$2`

If no prefix is present, the value is interpreted as a SHA-256 digest encoded in
hex or base64. Plaintext passwords are rejected.

#### Generating SHA-256 and SHA-512 passwords

Create a digest, encode it in base64, and prefix it with `{SHA256}` or
`{SHA512}` before adding it to `Users`:

```bash
# SHA-256
printf 'secret' | openssl dgst -binary -sha256 | base64

# SHA-512
printf 'secret' | openssl dgst -binary -sha512 | base64
```

Include the prefix in your config:

```go
Users: map[string]string{
    "john":  "{SHA256}K7gNU3sdo+OL0wNhqoVWhr3g6s1xYv72ol/pe/Unols=",
    "admin": "{SHA512}vSsar3708Jvp9Szi2NWZZ02Bqp1qRCFpbcTZPdBhnWgs5WtNZKnvCXdhztmeD2cmW192CF5bDufKRpayrW/isg==",
}
```

## Config

| Property        | Type                        | Description                                                                                                                                                           | Default               |
|:----------------|:----------------------------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------|:----------------------|
| Next            | `func(fiber.Ctx) bool`     | Next defines a function to skip this middleware when it returns true.                                                                                                   | `nil`                 |
| Users           | `map[string]string`         | Users maps usernames to **hashed** passwords (e.g. bcrypt, `{SHA256}`). | `map[string]string{}` |
| Realm           | `string`                    | Realm is a string to define the realm attribute of BasicAuth. The realm identifies the system to authenticate against and can be used by clients to save credentials. | `"Restricted"`        |
| Charset         | `string`                    | Charset sent in the `WWW-Authenticate` header. Only `"UTF-8"` is supported (case-insensitive). | `"UTF-8"` |
| HeaderLimit     | `int`                       | Maximum allowed length of the `Authorization` header. Requests exceeding this limit are rejected. | `8192` |
| Authorizer      | `func(string, string, fiber.Ctx) bool` | Authorizer defines a function to check the credentials. It will be called with a username, password, and the current context and is expected to return true or false to indicate approval.  | `nil`                 |
| Unauthorized    | `fiber.Handler`             | Unauthorized defines the response body for unauthorized responses.                                                                                                    | `nil`                 |
| BadRequest      | `fiber.Handler`             | BadRequest defines the response for malformed `Authorization` headers.                                                                                     | `nil`                 |

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
    BadRequest:      nil,
}
```
