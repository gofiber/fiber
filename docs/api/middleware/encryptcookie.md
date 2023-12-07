---
id: encryptcookie
---

# Encrypt Cookie

Encrypt Cookie is a middleware for [Fiber](https://github.com/gofiber/fiber) that secures your cookie values through encryption. 

:::note
This middleware encrypts cookie values and not the cookie names.
:::

## Signatures

```go
// Intitializes the middleware
func New(config ...Config) fiber.Handler

// Returns a random 32 character long string
func GenerateKey() string
```

## Examples

To use the Encrypt Cookie middleware, first, import the middleware package as part of the Fiber web framework:

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/encryptcookie"
)
```

Once you've imported the middleware package, you can use it inside your Fiber app:

```go
// Provide a minimal configuration
app.Use(encryptcookie.New(encryptcookie.Config{
    Key: "secret-thirty-2-character-string",
}))

// Retrieve the encrypted cookie value
app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("value=" + c.Cookies("test"))
})

// Create an encrypted cookie
app.Post("/", func(c *fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:  "test",
        Value: "SomeThing",
    })
    return nil
})
```

:::note
`Key` must be a 32 character string. It's used to encrypt the values, so make sure it is random and keep it secret.
You can run `openssl rand -base64 32` or call `encryptcookie.GenerateKey()` to create a random key for you.
Make sure not to set `Key` to `encryptcookie.GenerateKey()` because that will create a new key every run.
:::

## Config

| Property  | Type                                                | Description                                                                                           | Default                      |
|:----------|:----------------------------------------------------|:------------------------------------------------------------------------------------------------------|:-----------------------------|
| Next      | `func(*fiber.Ctx) bool`                             | A function to skip this middleware when returned true.                                                | `nil`                        |
| Except    | `[]string`                                          | Array of cookie keys that should not be encrypted.                                                    | `[]`                         |
| Key       | `string`                                            | A base64-encoded unique key to encode & decode cookies. Required. Key length should be 32 characters. | (No default, required field) |
| Encryptor | `func(decryptedString, key string) (string, error)` | A custom function to encrypt cookies.                                                                 | `EncryptCookie`              |
| Decryptor | `func(encryptedString, key string) (string, error)` | A custom function to decrypt cookies.                                                                 | `DecryptCookie`              |

## Default Config

```go
var ConfigDefault = Config{
	Next:      nil,
	Except:    []string{},
	Key:       "",
	Encryptor: EncryptCookie,
	Decryptor: DecryptCookie,
}
```

## Usage With Other Middlewares That Reads Or Modify Cookies
Place the encryptcookie middleware before any other middleware that reads or modifies cookies. For example, if you are using the CSRF middleware, ensure that the encryptcookie middleware is placed before it. Failure to do so may prevent the CSRF middleware from reading the encrypted cookie.

You may also choose to exclude certain cookies from encryption. For instance, if you are using the CSRF middleware with a frontend framework like Angular, and the framework reads the token from a cookie, you should exclude that cookie from encryption. This can be achieved by adding the cookie name to the Except array in the configuration:

```go
app.Use(encryptcookie.New(encryptcookie.Config{
	Key:    "secret-thirty-2-character-string",
	Except: []string{csrf.ConfigDefault.CookieName}, // exclude CSRF cookie
}))
app.Use(csrf.New(csrf.Config{
	KeyLookup:      "header:" + csrf.HeaderName,
	CookieSameSite: "Lax",
	CookieSecure:   true,
	CookieHTTPOnly: false,
}))
```
