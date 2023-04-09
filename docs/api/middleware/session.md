---
id: session
title: Session
---

Session middleware for [Fiber](https://github.com/gofiber/fiber).

:::note
This middleware uses our [Storage](https://github.com/gofiber/storage) package to support various databases through a single interface. The default configuration for this middleware saves data to memory, see the examples below for other databases.
:::

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
func (s *Session) Keys() []string
```

:::caution
Storing `interface{}` values are limited to built-ins Go types.
:::

## Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/session"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
// This stores all of your app's sessions
store := session.New()

app.Get("/", func(c *fiber.Ctx) error {
    // Get session from storage
    sess, err := store.Get(c)
    if err != nil {
        panic(err)
    }

    // Get value
    name := sess.Get("name")

    // Set key/value
    sess.Set("name", "john")

    // Get all Keys
    keys := sess.Keys()

    // Delete key
    sess.Delete("name")

    // Destroy session
    if err := sess.Destroy(); err != nil {
        panic(err)
    }

	// Sets a specific expiration for this session
	sess.SetExpiry(time.Second * 2)

    // Save session
    if err := sess.Save(); err != nil {
		panic(err)
	}

	return c.SendString(fmt.Sprintf("Welcome %v", name))
})
```

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

	// KeyLookup is a string in the form of "<source>:<name>" that is used
	// to extract session id from the request.
	// Possible values: "header:<name>", "query:<name>" or "cookie:<name>"
	// Optional. Default value "cookie:session_id".
	KeyLookup string

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

	// Value of SameSite cookie.
	// Optional. Default value "Lax".
	CookieSameSite string

	// Decides whether cookie should last for only the browser sesison.
	// Ignores Expiration if set to true
	// Optional. Default value false.
	CookieSessionOnly bool

	// KeyGenerator generates the session key.
	// Optional. Default value utils.UUIDv4
	KeyGenerator func() string

	// Deprecated: Please use KeyLookup
	CookieName string

	// Source defines where to obtain the session id
	source Source

	// The session name
	sessionName string
}
```

## Default Config

```go
var ConfigDefault = Config{
	Expiration:   24 * time.Hour,
	KeyLookup:    "cookie:session_id",
	KeyGenerator: utils.UUIDv4,
	source:       "cookie",
	sessionName:  "session_id",
}
```

## Constants

```go
const (
	SourceCookie   Source = "cookie"
	SourceHeader   Source = "header"
	SourceURLQuery Source = "query"
)
```

### Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3
store := session.New(session.Config{
	Storage: storage,
})
```

To use the store, see the [Examples](#examples).