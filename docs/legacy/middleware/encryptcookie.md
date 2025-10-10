---
id: encryptcookie
---

# Encrypt Cookie

The Encrypt Cookie middleware for [Fiber](https://github.com/gofiber/fiber) encrypts cookie values for secure storage.

:::note
This middleware encrypts cookie values but not cookie names.
:::

## Signatures

```go
// Initializes the middleware
func New(config ...Config) fiber.Handler

// GenerateKey returns a random string of 16, 24, or 32 bytes.
// The length of the key determines the AES encryption algorithm used:
// 16 bytes for AES-128, 24 bytes for AES-192, and 32 bytes for AES-256-GCM.
func GenerateKey(length int) string
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/encryptcookie"
)
```

Once your Fiber app is initialized, register the middleware:

```go
// Provide a minimal configuration
app.Use(encryptcookie.New(encryptcookie.Config{
    Key: "secret-32-character-string",
}))

// Retrieve the encrypted cookie value
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("value=" + c.Cookies("test"))
})

// Create an encrypted cookie
app.Post("/", func(c fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:  "test",
        Value: "SomeThing",
    })
    return nil
})
```

:::note
Use an encoded key of 16, 24, or 32 bytes to select AES‑128, AES‑192, or AES‑256‑GCM. Generate a stable key with `openssl rand -base64 32` or `encryptcookie.GenerateKey(32)` and store it securely. Generating a new key on each startup renders existing cookies unreadable.
:::

## Config

| Property        | Type                                                 | Description                                                                                                 | Default                      |
|:----------------|:-----------------------------------------------------|:------------------------------------------------------------------------------------------------------------|:-----------------------------|
| Next            | `func(fiber.Ctx) bool`                               | A function to skip this middleware when it returns true.                                                    | `nil`                        |
| Except          | `[]string`                                           | Array of cookie keys that should not be encrypted.                                                          | `[]`                         |
| Key             | `string`                                             | A base64-encoded unique key to encode & decode cookies. Required. Key length should be 16, 24, or 32 bytes. | (No default, required field) |
| EncryptorFunc   | `func(decryptedString, key string) (string, error)`  | A custom function to encrypt cookies.                                                                       | `EncryptCookie`              |
| DecryptorFunc   | `func(encryptedString, key string) (string, error)`  | A custom function to decrypt cookies.                                                                       | `DecryptCookie`              |

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

## Use with Other Middleware That Reads or Modifies Cookies

Place `encryptcookie` before middleware that reads or writes cookies. If you use the CSRF middleware, register `encryptcookie` first so it can read the token.

Exclude cookies from encryption by listing them in `Except`. If a frontend framework such as Angular reads the CSRF token from a cookie, add that name to the `Except` array:

```go
const csrfCookieName = "csrf_"
app.Use(encryptcookie.New(encryptcookie.Config{
    Key:    "secret-thirty-2-character-string",
    Except: []string{csrfCookieName}, // exclude CSRF cookie
}))
app.Use(csrf.New(csrf.Config{
    Extractor:      csrf.FromHeader(csrf.HeaderName),
    CookieSameSite: "Lax",
    CookieSecure:   true,
    CookieHTTPOnly: false,
}))
```

## Encryption Algorithms

The default Encryptor and Decryptor functions use `AES-256-GCM` for encryption and decryption. If you need to use `AES-128` or `AES-192` instead, you can do so by changing the length of the key when calling `encryptcookie.GenerateKey(length)` or by providing a key of one of the following lengths:

- AES-128 requires a 16-byte key.
- AES-192 requires a 24-byte key.
- AES-256 requires a 32-byte key.

For example, to generate a key for AES-128:

```go
key := encryptcookie.GenerateKey(16)
```

And for AES-192:

```go
key := encryptcookie.GenerateKey(24)
```
