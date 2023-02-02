# Encrypt Cookie Middleware

Encrypt middleware for [Fiber](https://github.com/gofiber/fiber) which encrypts cookie values. Note: this middleware does not encrypt cookie names.

## Table of Contents

* [Signatures](encryptcookie.md#signatures)
* [Setup](encryptcookie.md#setup)
* [Config](encryptcookie.md#config)
* [Default Config](encryptcookie.md#default-config)

## Signatures

```go
// Intitializes the middleware
func New(config ...Config) fiber.Handler

// Returns a random 32 character long string
func GenerateKey() string
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/encryptcookie"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Default middleware config
app.Use(encryptcookie.New(encryptcookie.Config{
    Key: "secret-thirty-2-character-string",
}))

// Get / reading out the encrypted cookie
app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("value=" + c.Cookies("test"))
})

// Post / create the encrypted cookie
app.Post("/", func(c *fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:  "test",
        Value: "SomeThing",
    })
    return nil
})
```

## Config

```go
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Array of cookie keys that should not be encrypted.
	//
	// Optional. Default: ["csrf_"]
	Except []string

	// Base64 encoded unique key to encode & decode cookies.
	//
	// Required. The key should be 32 bytes of random data in base64-encoded form.
	// You may run `openssl rand -base64 32` or use `encryptcookie.GenerateKey()` to generate a new key.
	Key string

	// Custom function to encrypt cookies.
	//
	// Optional. Default: EncryptCookie
	Encryptor func(decryptedString, key string) (string, error)

	// Custom function to decrypt cookies.
	//
	// Optional. Default: DecryptCookie
	Decryptor func(encryptedString, key string) (string, error)
}
```

## Default Config

```go
// `Key` must be a 32 character string. It's used to encrpyt the values, so make sure it is random and keep it secret.
// You can run `openssl rand -base64 32` or call `encryptcookie.GenerateKey()` to create a random key for you.
// Make sure not to set `Key` to `encryptcookie.GenerateKey()` because that will create a new key every run.
app.Use(encryptcookie.New(encryptcookie.Config{
    Key: "secret-thirty-2-character-string",
}))
```

## Usage of CSRF and Encryptcookie Middlewares with Custom Cookie Names
Normally, encryptcookie middleware skips `csrf_` cookies. However, it won't work when you use custom cookie names for CSRF. You should update `Except` config to avoid this problem. For example:

```go
app.Use(encryptcookie.New(encryptcookie.Config{
	Key: "secret-thirty-2-character-string",
	Except: []string{"csrf_1"}, // exclude CSRF cookie
}))

app.Use(csrf.New(csrf.Config{
	KeyLookup:      "form:test",
	CookieName:     "csrf_1", 
	CookieHTTPOnly: true,
}))
```
