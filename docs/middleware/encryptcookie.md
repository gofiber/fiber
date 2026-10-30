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

// EncryptCookie and DecryptCookie are the default Encryptor and Decryptor.
func EncryptCookie(name, value, key string) (string, error)
func DecryptCookie(name, value, key string) (string, error)
```

`GenerateKey` returns the base64 encoding of `length` random bytes, which is the form `Config.Key` expects, and panics with `ErrInvalidKeyLength` for any other length. `New` panics when `Key` is empty, is not valid standard base64 (padding included), or does not decode to 16, 24, or 32 bytes (`ErrInvalidKeyLength`). It checks the key even when you supply your own `Encryptor` and `Decryptor`.

## Examples

Import the middleware package:

```go
import (
    "os"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/encryptcookie"
)
```

Once your Fiber app is initialized, register the middleware:

```go
// Provide a minimal configuration
app.Use(encryptcookie.New(encryptcookie.Config{
    // A base64-encoded key of 16, 24, or 32 bytes, for example from `openssl rand -base64 32`
    Key: os.Getenv("COOKIE_KEY"),
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

| Property  | Type                                                | Description                                                                                           | Default                      |
|:----------|:----------------------------------------------------|:------------------------------------------------------------------------------------------------------|:-----------------------------|
| Next      | `func(fiber.Ctx) bool`                             | A function to skip this middleware when it returns true.                                                | `nil`                        |
| Except    | `[]string`                                          | Array of cookie keys that should not be encrypted.                                                    | `[]`                         |
| Key       | `string`                                            | A base64-encoded unique key to encode & decode cookies. Required. The decoded key must be 16, 24, or 32 bytes long; `New` panics otherwise. | (No default, required field) |
| Encryptor | `func(name, decryptedString, key string) (string, error)` | A custom function to encrypt cookies.                                                                 | `EncryptCookie`              |
| Decryptor | `func(name, encryptedString, key string) (string, error)` | A custom function to decrypt cookies.                                                                 | `DecryptCookie`              |

### Encryptor and Decryptor parameters

Custom encryptor and decryptor functions receive three arguments:

- `name`: The cookie name. The default helpers bind this value as additional authenticated data (AAD) so encrypted values can only be decrypted for the same cookie.
- `string`: The cookie payload. `EncryptCookie` accepts the decrypted value and returns ciphertext, while `DecryptCookie` receives ciphertext and must return the decrypted value.
- `key`: The base64-encoded key pulled from the middleware configuration. Use it to derive or validate any encryption keys your implementation requires.

Every `Set-Cookie` line of the response is encrypted on its own, however it was written (`c.Cookie`, `c.Add`, a proxied or adapted response), also when a name repeats. Only the value is replaced, without the double quotes around it, and the attributes are sent as they were written.

A request cookie for which the `Decryptor` returns an error, such as a tampered value, a cookie that was never encrypted, or one sealed with another key, is removed from the request, so handlers see it as absent. When a request carries the same cookie name more than once, for example set under two paths or domains, the handler sees a single entry for that name holding the first value that decrypts; the other entries, including any that do not decrypt, are dropped. Names listed in `Except` keep every entry as it arrived. When the `Encryptor` returns an error for a response cookie, the middleware removes every response cookie that is not listed in `Except` and returns that error, joined with any error from the rest of the chain. The default helpers return `ErrInvalidEncryptedValue` for a payload too short to hold the nonce and authentication tag, and `ErrInvalidKeyLength` for a key that does not decode to 16, 24, or 32 bytes.

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
import (
    "os"

    "github.com/gofiber/fiber/v3/extractors"
    "github.com/gofiber/fiber/v3/middleware/csrf"
    "github.com/gofiber/fiber/v3/middleware/encryptcookie"
)

app.Use(encryptcookie.New(encryptcookie.Config{
    Key:    os.Getenv("COOKIE_KEY"),
    Except: []string{csrf.ConfigDefault.CookieName}, // exclude CSRF cookie
}))
app.Use(csrf.New(csrf.Config{
    Extractor:      extractors.FromHeader(csrf.HeaderName),
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
