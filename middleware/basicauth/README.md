# Basic Authentication
Basic Authentication middleware for [Fiber](https://github.com/gofiber/fiber) that provides an HTTP basic authentication. It calls the next handler for valid credentials and [401 Unauthorized](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/401) or a custom response for missing or invalid credentials.

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)


### Signatures
```go
func New(config Config) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/basicauth"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
// Provide a minimal config
app.Use(basicauth.New(basicauth.Config{
	Users: map[string]string{
		"john":  "doe",
		"admin": "123456",
	},
}))

// Or extend your config for customization
app.Use(basicauth.New(basicauth.Config{
	Users: map[string]string{
		"john":  "doe",
		"admin": "123456",
	},
	Realm: "Forbidden",
	Authorizer: func(user, pass string) bool {
		if user == "john" && pass == "doe" {
			return true
		}
		if user == "admin" && pass == "123456" {
			return true
		}
		return false
	},
	Unauthorized: func(c *fiber.Ctx) error {
		return c.SendFile("./unauthorized.html")
	},
	ContextUsername: "_user",
	ContextPassword: "_pass",
}))
```

### Config
```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Users defines the allowed credentials
	//
	// Required. Default: map[string]string{}
	Users map[string]string

	// Realm is a string to define realm attribute of BasicAuth.
	// the realm identifies the system to authenticate against
	// and can be used by clients to save credentials
	//
	// Optional. Default: "Restricted".
	Realm string

	// Authorizer defines a function you can pass
	// to check the credentials however you want.
	// It will be called with a username and password
	// and is expected to return true or false to indicate
	// that the credentials were approved or not.
	//
	// Optional. Default: nil.
	Authorizer func(string, string) bool

	// Unauthorized defines the response body for unauthorized responses.
	// By default it will return with a 401 Unauthorized and the correct WWW-Auth header
	//
	// Optional. Default: nil
	Unauthorized fiber.Handler

	// ContextUser is the key to store the username in Locals
	//
	// Optional. Default: "username"
	ContextUsername string

	// ContextPass is the key to store the password in Locals
	//
	// Optional. Default: "password"
	ContextPassword string
}
```

### Default Config
```go
var ConfigDefault = Config{
	Next:            nil,
	Users:           map[string]string{},
	Realm:           "Restricted",
	Authorizer:      nil,
	Unauthorized:    nil,
	ContextUsername: "username",
	ContextPassword: "password",
}
```
