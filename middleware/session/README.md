# Session

Session middleware for [Fiber](https://github.com/gofiber/fiber).

_NOTE: This middleware uses our [Storage](https://github.com/gofiber/storage) package to support various databases through a single interface. The default configuration for this middleware saves data to memory, see the examples below for other databases._

## Table of Contents

- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)

## Signatures

```go
func New(config ...Config) *Store
func (s *Store) RegisterType(i interface{})
func (s *Store) Get(c *fiber.Ctx) (*Session, error)
func (s *Store) Reset() error

func (s *Session) Get(key string) interface{}
func (s *Session) Set(key string, val interface{})
func (s *Session) Delete(key string)
func (s *Session) Destroy() error
func (s *Session) Regenerate() error
func (s *Session) Save() error
func (s *Session) Fresh() bool
func (s *Session) ID() string
```

**⚠ _Storing `interface{}` values are limited to built-ins Go types_**

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/session"
)
```

Then create a Fiber app with `app := fiber.New()`.

### Default Configuration

```go
// This stores all of your app's sessions
// Default middleware config
store := session.New()

// This panic will be catch by the middleware
app.Get("/", func(c *fiber.Ctx) error {
	// get session from storage
	sess, err := store.Get(c)
	if err != nil {
		panic(err)
	}

	// Get value
	name := sess.Get("name")

	// Set key/value
	sess.Set("name", "john")

	// Delete key
	sess.Delete("name")

	// Destry session
	if err := sess.Destroy(); err != nil {
		panic(err)
	}

	// save session
	if err := sess.Save(); err != nil {
		panic(err)
	}

	return fmt.Fprintf(ctx, "Welcome %v", name)
})
```

### Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3
store := session.New(session.Config{
	Storage: storage,
})
```

To use the the store, see the above example.

## Config

```go
// Config defines the config for middleware.
type Config struct {
	// Allowed session duration
	// Optional. Default value 24 * time.Hour
	Expiration time.Duration

	// Storage interface to store the session data
	// Optional. Default value memory.New()
	Storage fiber.Storage

	// Name of the session cookie. This cookie will store session key.
	// Optional. Default value "session_id".
	CookieName string

	// Domain of the CSRF cookie.
	// Optional. Default value "".
	CookieDomain string

	// Path of the CSRF cookie.
	// Optional. Default value "".
	CookiePath string

	// Indicates if CSRF cookie is secure.
	// Optional. Default value false.
	CookieSecure bool

	// Indicates if CSRF cookie is HTTP only.
	// Optional. Default value false.
	CookieHTTPOnly bool

	// Indicates if CSRF cookie is HTTP only.
	// Optional. Default value false.
	CookieSameSite string

	// KeyGenerator generates the session key.
	// Optional. Default value utils.UUID
	KeyGenerator func() string
}
```

## Default Config

```go
var ConfigDefault = Config{
	Expiration:   24 * time.Hour,
	CookieName:   "session_id",
	KeyGenerator: utils.UUID,
}
```
