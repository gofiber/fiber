# Encrypt Cookie Middleware

Encrypt middleware for [Fiber](https://github.com/gofiber/fiber) which encrypts cookie values. Note: this middleware does not encrypt cookie names.


## Signaures

```go
// Intitializes the middleware
func New(config ...Config) fiber.Handler

// Returns a random 32 character long string
func GenerateKey() string
```

## Setup

First import the middleware from Fiber,

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/cache"
)

Then create a Fiber app with `app := fiber.New()`.

## Minimum Config

```go
// `Key` must be a 32 character string. It's used to encrpyt the values, so make sure it is random and keep it secret.
// You can call `encryptcookie.GenerateKey()` to create a random key for you.
// Make sure not to set `Key` to `encryptcookie.GenerateKey()` because that will create a new key every run.
app.Use(encryptcookie.New(encryptcookie.Config{
    Key: "secret-thirty-2-character-string",
}))
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
	// Optional. Default: []
	Except []string

	// Base64 encoded unique key to encode & decode cookies.
	//
	// Required. Key length should be 32 characters.
	// You may use `encryptcookie.GenerateKey()` to generate a new key.
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
