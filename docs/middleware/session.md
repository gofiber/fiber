---
id: session
---

# Session Middleware for [Fiber](https://github.com/gofiber/fiber)

The `session` middleware provides session management for Fiber applications, utilizing the [Storage](https://github.com/gofiber/storage) package for multi-database support via a unified interface. By default, session data is stored in memory, but custom storage options are easily configurable (see examples below).

As of v3, we recommend using the middleware handler for session management. However, for backward compatibility, v2's session methods are still available, allowing you to continue using the session management techniques from earlier versions of Fiber. Both methods are demonstrated in the examples.

## Table of Contents

- [Migration Guide](#migration-guide)
  - [v2 to v3](#v2-to-v3)
- [Types](#types)
  - [Config](#config)
  - [Middleware](#middleware)
  - [Session](#session)
  - [Store](#store)
- [Signatures](#signatures)
  - [Session Package Functions](#session-package-functions)
  - [Config Methods](#config-methods)
  - [Middleware Methods](#middleware-methods)
  - [Session Methods](#session-methods)
  - [Store Methods](#store-methods)
- [Examples](#examples)
  - [Middleware Handler (Recommended)](#middleware-handler-recommended)
  - [Custom Storage Example](#custom-storage-example)
  - [Session Without Middleware Handler](#session-without-middleware-handler)
  - [Custom Types in Session Data](#custom-types-in-session-data)
- [Config](#config)
- [Default Config](#default-config)

## Migration Guide

### v2 to v3

- **Function Signature Change**: In v3, the `New` function now returns a middleware handler instead of a `*Store`. To access the store, use the `Store` method on `*Middleware` (obtained from `session.FromContext(c)` in a handler) or use `NewStore` or `NewWithStore`.

- **Session Lifecycle Management**: The `*Store.Save` method no longer releases the instance automatically. You must manually call `sess.Release()` after using the session to manage its lifecycle properly.

- **Expiration Handling**: Previously, the `Expiration` field represented the maximum session duration before expiration. However, it would extend every time the session was saved, making its behavior a mix between session duration and session idle timeout. The `Expiration` field has been removed and replaced with `IdleTimeout` and `AbsoluteTimeout` fields, which explicitly defines the session's idle and absolute timeout periods.

For more details about Fiber v3, see [What’s New](https://github.com/gofiber/fiber/blob/main/docs/whats_new.md).

### Migrating v2 to v3 Example (Legacy Approach)

To convert a v2 example to use the v3 legacy approach, follow these steps:

1. **Initialize with Store**: Use `session.NewStore()` to obtain a store.
2. **Retrieve Session**: Access the session store using the `store.Get(c)` method.
3. **Release Session**: Ensure that you call `sess.Release()` after you are done with the session to manage its lifecycle.

:::note
When using the legacy approach, the IdleTimeout will be updated when the session is saved.
:::

#### Example Conversion

**v2 Example:**

```go
store := session.New()

app.Get("/", func(c *fiber.Ctx) error {
    sess, err := store.Get(c)
    if err != nil {
        return err
    }

    key, ok := sess.Get("key").(string)
    if !ok {
        return c.SendStatus(fiber.StatusInternalServerError)
    }

    sess.Set("key", "value")

    err = sess.Save()
    if err != nil {
        return c.SendStatus(fiber.StatusInternalServerError)
    }

    return nil
})
```

**v3 Legacy Approach:**

```go
store := session.NewStore()

app.Get("/", func(c *fiber.Ctx) error {
    sess, err := store.Get(c)
    if err != nil {
        return err
    }
    defer sess.Release() // Important: Release the session

    key, ok := sess.Get("key").(string)
    if !ok {
        return c.SendStatus(fiber.StatusInternalServerError)
    }

    sess.Set("key", "value")

    err = sess.Save()
    if err != nil {
        return c.SendStatus(fiber.StatusInternalServerError)
    }

    return nil
})
```

### v3 Example (Recommended Middleware Handler)

Do not call `sess.Release()` when using the middleware handler. `sess.Save()` is also not required, as the middleware automatically saves the session data.

For the recommended approach, use the middleware handler. See the [Middleware Handler (Recommended)](#middleware-handler-recommended) section for details.

## Types

### Config

Defines the configuration options for the session middleware.

```go
type Config struct {
    Storage           fiber.Storage
    Next              func(c *fiber.Ctx) bool
    Store             *Store
    ErrorHandler      func(*fiber.Ctx, error)
    KeyGenerator      func() string
    KeyLookup         string
    CookieDomain      string
    CookiePath        string
    CookieSameSite    string
    IdleTimeout       time.Duration
    AbsoluteTimeout   time.Duration
    CookieSecure      bool
    CookieHTTPOnly    bool
    CookieSessionOnly bool
}
```

### Middleware

The `Middleware` struct encapsulates the session middleware configuration and storage, created via `New` or `NewWithStore`.

```go
type Middleware struct {
    Session *Session
}
```

### Session

Represents a user session, accessible through `FromContext` or `Store.Get`.

```go
type Session struct {}
```

### Store

Handles session data management and is created using `NewStore`, `NewWithStore` or by accessing the `Store` method of a middleware instance.

```go
type Store struct {
    Config
}
```

## Signatures

### Session Package Functions

```go
func New(config ...Config) *Middleware
func NewWithStore(config ...Config) (fiber.Handler, *Store)
func FromContext(c fiber.Ctx) *Middleware
```

### Config Methods

```go
func DefaultErrorHandler(c *fiber.Ctx, err error)
```

### Middleware Methods

```go
func (m *Middleware) Set(key string, value any)
func (m *Middleware) Get(key string) any
func (m *Middleware) Delete(key string)
func (m *Middleware) Destroy() error
func (m *Middleware) Reset() error
func (m *Middleware) Store() *Store
```

### Session Methods

```go
func (s *Session) Fresh() bool
func (s *Session) ID() string
func (s *Session) Get(key string) any
func (s *Session) Set(key string, val any)
func (s *Session) Destroy() error
func (s *Session) Regenerate() error
func (s *Session) Release()
func (s *Session) Reset() error
func (s *Session) Save() error
func (s *Session) Keys() []string
func (s *Session) SetIdleTimeout(idleTimeout time.Duration)
```

### Store Methods

```go
func (*Store) RegisterType(i any)
func (s *Store) Get(c fiber.Ctx) (*Session, error)
func (s *Store) GetByID(id string) (*Session, error)
func (s *Store) Reset() error
func (s *Store) Delete(id string) error
```

## Examples

:::note
**Security Notice**: For robust security, especially during sensitive operations like account changes or transactions, consider using CSRF protection. Fiber provides a [CSRF Middleware](https://docs.gofiber.io/api/middleware/csrf) that can be used with sessions to prevent CSRF attacks.
:::

:::note
**Middleware Order**: The order of middleware matters. The session middleware should come before any handler or middleware that uses the session (for example, the CSRF middleware).
:::

### Middleware Handler (Recommended)

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/csrf"
    "github.com/gofiber/fiber/v3/middleware/session"
)

func main() {
    app := fiber.New()

    sessionMiddleware, sessionStore := session.NewWithStore()

    app.Use(sessionMiddleware)
    app.Use(csrf.New(csrf.Config{
        Store: sessionStore,
    }))

    app.Get("/", func(c *fiber.Ctx) error {
        sess := session.FromContext(c)
        if sess == nil {
            return c.SendStatus(fiber.StatusInternalServerError)
        }

        name, ok := sess.Get("name").(string)
        if !ok {
            return c.SendString("Welcome anonymous user!")
        }

        return c.SendString("Welcome " + name)
    })

    app.Listen(":3000")
}
```

### Custom Storage Example

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/storage/sqlite3"
    "github.com/gofiber/fiber/v3/middleware/csrf"
    "github.com/gofiber/fiber/v3/middleware/session"
)

func main() {
    app := fiber.New()

    storage := sqlite3.New()
    sessionMiddleware, sessionStore := session.NewWithStore(session.Config{
        Storage: storage,
    })

    app.Use(sessionMiddleware)
    app.Use(csrf.New(csrf.Config{
        Store: sessionStore,
    }))

    app.Listen(":3000")
}
```

### Session Without Middleware Handler

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/csrf"
    "github.com/gofiber/fiber/v3/middleware/session"
)

func main() {
    app := fiber.New()

    sessionStore := session.NewStore()

    app.Use(csrf.New(csrf.Config{
        Store: sessionStore,
    }))

    app.Get("/", func(c *fiber.Ctx) error {
        sess, err := sessionStore.Get(c)
        if err != nil {
            return c.SendStatus(fiber.StatusInternalServerError)
        }
        defer sess.Release()

        name, ok := sess.Get("name").(string)
        if !ok {
            return c.SendString("Welcome anonymous user!")
        }

        return c.SendString("Welcome " + name)
    })

    app.Post("/login", func(c *fiber.Ctx) error {
        sess, err := sessionStore.Get(c)
        if err != nil {
            return c.SendStatus(fiber.StatusInternalServerError)
        }
        defer sess.Release()

        if !sess.Fresh() {
            if err := sess.Regenerate(); err != nil {
                return c.SendStatus(fiber.StatusInternalServerError)
            }
        }

        sess.Set("name", "John Doe")

        err = sess.Save()
        if err != nil {
            return c.SendStatus(fiber.StatusInternalServerError)
        }

        return c.SendString("Logged in!")
    })

    app.Listen(":3000")
}
```

### Custom Types in Session Data

Session data can only be of the following types by default:

- `string`
- `int`
- `int8`
- `int16`
- `int32`
- `int64`
- `uint`
- `uint8`
- `uint16`
- `uint32`
- `uint64`
- `bool`
- `float32`
- `float64`
- `[]byte`
- `complex64`
- `complex128`
- `interface{}`

To support other types in session data, you can register custom types. Here is an example of how to register a custom type:

```go
package main

import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/session"
)

type User struct {
    Name string
    Age  int
}

func main() {
    app := fiber.New()

    sessionMiddleware, sessionStore := session.NewWithStore()
    sessionStore.RegisterType(User{})

    app.Use(sessionMiddleware)

    app.Listen(":3000")
}
```

## Config

| Property              | Type                           | Description                                                                                | Default                   |
|-----------------------|--------------------------------|--------------------------------------------------------------------------------------------|---------------------------|
| **Storage**           | `fiber.Storage`                | Defines where session data is stored.                                                      | `nil` (in-memory storage) |
| **Next**              | `func(c fiber.Ctx) bool`       | Function to skip this middleware under certain conditions.                                 | `nil`                     |
| **ErrorHandler**      | `func(c fiber.Ctx, err error)` | Custom error handler for session middleware errors.                                        | `nil`                     |
| **KeyGenerator**      | `func() string`                | Function to generate session IDs.                                                          | `UUID()`                  |
| **KeyLookup**         | `string`                       | Key used to store session ID in cookie or header.                                          | `"cookie:session_id"`     |
| **CookieDomain**      | `string`                       | The domain scope of the session cookie.                                                    | `""`                      |
| **CookiePath**        | `string`                       | The path scope of the session cookie.                                                      | `"/"`                     |
| **CookieSameSite**    | `string`                       | The SameSite attribute of the session cookie.                                              | `"Lax"`                   |
| **IdleTimeout**       | `time.Duration`                | Maximum duration of inactivity before session expires.                                     | `30 * time.Minute`        |
| **AbsoluteTimeout**   | `time.Duration`                | Maximum duration before session expires.                                                   | `0` (no expiration)       |
| **CookieSecure**      | `bool`                         | Ensures session cookie is only sent over HTTPS.                                            | `false`                   |
| **CookieHTTPOnly**    | `bool`                         | Ensures session cookie is not accessible to JavaScript (HTTP only).                        | `true`                    |
| **CookieSessionOnly** | `bool`                         | Prevents session cookie from being saved after the session ends (cookie expires on close). | `false`                   |

## Default Config

```go
session.Config{
    Storage:           memory.New(),
    Next:              nil,
    Store:             nil,
    ErrorHandler:      nil,
    KeyGenerator:      utils.UUIDv4,
    KeyLookup:         "cookie:session_id",
    CookieDomain:      "",
    CookiePath:        "",
    CookieSameSite:    "Lax",
    IdleTimeout:       30 * time.Minute,
    AbsoluteTimeout:   0,
    CookieSecure:      false,
    CookieHTTPOnly:    false,
    CookieSessionOnly: false,
}
```
