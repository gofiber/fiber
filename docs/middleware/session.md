---
id: session
---

# Session Middleware for [Fiber](https://github.com/gofiber/fiber)

The `session` middleware provides session handling for Fiber applications. It leverages the [Storage](https://github.com/gofiber/storage) package to offer support for multiple databases through a unified interface. By default, session data is stored in memory, but you can easily switch to other storage options, as shown in the examples below.

:::note
We recommend using the `Middleware` handler for better integration with other middleware. See the [As a Middleware Handler (Recommended)](#as-a-middleware-handler-recommended) section for details.
:::

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
  - [As a Middleware Handler (Recommended)](#as-a-middleware-handler-recommended)
  - [Using a Custom Storage](#using-a-custom-storage)
  - [Session Without Middleware Handler](#session-without-middleware-handler)
  - [Using Custom Types in Session Data](#using-custom-types-in-session-data)
- [Config](#config)
- [Default Config](#default-config)

## Migration Guide

### v2 to v3
- In version 3, the `New` function signature has been updated. It now returns a Fiber middleware handler instead of a `*Store`. To access the store, you can use the `Store` method on the `*Middleware` (obtained by calling `session.FromContext(c)` in a handler where the middleware is applied) or utilize the `NewWithStore` function.

While it's still possible to work with the `*Store` directly, we recommend using the `Middleware` handler for better integration with other Fiber middlewares.

For more information about changes in Fiber v3, see [What's New](https://github.com/gofiber/fiber/blob/main/docs/whats_new.md).

#### v2 Example

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

#### v3 Example (Using Store)

```go
_, store := session.NewWithStore()

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

#### v3 Example (Using Middleware)

See the [As a Middleware Handler (Recommended)](#as-a-middleware-handler-recommended) section for details.

## Types

### Config

The configuration for the session middleware.

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
    Expiration        time.Duration
    CookieSecure      bool
    CookieHTTPOnly    bool
    CookieSessionOnly bool
}
```

### Middleware

The `Middleware` struct encapsulates the session middleware configuration and storage. It is created using the `New` or `NewWithStorage` function and used as a `fiber.Handler`.

```go
type Middleware struct {
    Session *Session
}
```

### Session

The `Session` struct is used to interact with session data. You can retrieve it from the `Middleware` using the `FromContext` method or from the `Store` using the `Get` method.

```go
type Session struct {}
```

### Store

The `Store` struct is used to manage session data. It is created using the `NewWithStore` function or by calling the `Store` method on a `Middleware`.

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

Used to interact with session data when using the middleware handler.

```go
func (m *Middleware) Set(key string, value any)
func (m *Middleware) Get(key string) any
func (m *Middleware) Delete(key string)
func (m *Middleware) Destroy() error
func (m *Middleware) Reset() error
func (m *Middleware) Store() *Store
```

### Session Methods

If using the middleware handler, you generally won't need to use these methods directly.

```go
func (s *Session) Fresh() bool
func (s *Session) ID() string
func (s *Session) Get(key string) any
func (s *Session) Set(key string, val any)
func (s *Session) Destroy() error
func (s *Session) Regenerate() error
func (s *Session) Reset() error
func (s *Session) Save() error
func (s *Session) Keys() []string
func (s *Session) SetIdleTimeout(idleTimeout time.Duration)
```

### Store Methods

```go
func (*Store) RegisterType(i any)
func (s *Store) Get(c fiber.Ctx) (*Session, error)
func (s *Store) Reset() error
func (s *Store) Delete(id string) error
func (s *Store) GetSessionByID(id string) (*Session, error)
```

## Examples

:::note
**Security Note**: Fiber’s session middleware uses cookies with `SameSite=Lax` by default, which provides basic CSRF protection for most GET requests. However, for comprehensive security—especially for POST requests or sensitive operations (e.g., account changes, transactions, form submissions)—it is strongly recommended to use CSRF protection middleware. Fiber provides a `csrf` middleware that can be used in conjunction with the `session` middleware for robust protection. Find more information in the [CSRF Middleware](https://docs.gofiber.io/api/middleware/csrf) documentation.

### Recommendations

1. **Session Middleware Without CSRF**:
   - You can use the `session` middleware without the `csrf` middleware or rely solely on `SameSite=Lax` for basic protection in low-risk scenarios.

2. **Double Submit Cookie Pattern**:
   - You can implement the **double submit cookie pattern** (via custom, third-party, or built-in middleware), where the CSRF token is stored in a cookie, and the request includes the token in a hidden field or header. In this approach, there is no need to pass the `session.Store` to the `csrf` middleware. Simply apply the `session.New()` and `csrf.New()` middleware to the routes you want to protect.

3. **Recommended Approach**:
   - For stronger protection, especially in high-risk scenarios, use the `csrf` middleware with the session store. This method implements the **Synchronizer Token Pattern**, providing robust defense by associating the CSRF token with the user’s session. This approach requires passing the `session.Store` to the `csrf` middleware.
   - Ensure the CSRF token is embedded in forms or included in a header for POST requests and verified on the server side for incoming requests. This adds a crucial security layer for state-changing actions.

:::

### As a Middleware Handler (Recommended)

```go
package main

import (
    "fmt"
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/session/v3"
    "github.com/gofiber/session/v3/middleware/csrf"
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

        return c.SendString(fmt.Sprintf("Welcome %v", name))
    })

    log.Fatal(app.Listen(":3000"))
}
```

### Using a Custom Storage

This example shows how to use the `sqlite3` storage from the [Fiber storage package](https://github.com/gofiber/storage).

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/storage/sqlite3"
    "github.com/gofiber/session/v3"
    "github.com/gofiber/session/v3/middleware/csrf"
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

    log.Fatal(app.Listen(":3000"))
}
```

### Session Without Middleware Handler

This example shows how to work with sessions directly without the middleware handler.

```go
package main

import (
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/session/v3"
    "github.com/gofiber/session/v3/middleware/csrf"
)

func main() {
    app := fiber.New()

    _, sessionStore := session.NewWithStore()

    app.Use(csrf.New(csrf.Config{
        Store: sessionStore,
    }))

    app.Get("/", func(c *fiber.Ctx) error {
        sess, err := sessionStore.Get(c)
        if err != nil {
            return c.SendStatus(fiber.StatusInternalServerError)
        }

        name, ok := sess.Get("name").(string)
        if !ok {
            return c.SendString("Welcome anonymous user!")
        }

        return c.SendString(fmt.Sprintf("Welcome %v", name))
    })

    log.Fatal(app.Listen(":3000

"))
}
```

### Using Custom Types in Session Data

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
    "log"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/session/v3"
    "github.com/gofiber/session/v3/middleware/session"
)

type User struct {
    Name string
    Age  int
}

func main() {
    // Create a new Fiber app
    app := fiber.New()

    // Initialize custom session config
    sessionMiddleware, sessionStore := session.NewWithStore()

    // Register custom type
    sessionStore.RegisterType(User{})

    // Use the session middleware
    app.Use(sessionMiddleware)

    ...
}
```

## Config

| Property                | Type            | Description                                                                                                   | Default               |
|:------------------------|:----------------|:--------------------------------------------------------------------------------------------------------------|:----------------------|
| Storage                 | `fiber.Storage` | Storage interface to store the session data.                                                                  | `memory.New()`        |
| Next                    | `func(c fiber.Ctx) bool` | Function to skip this middleware when returned true.                                                 | `nil`                 |
| Store                   | `*Store`        | Defines the session store.                                                                                    | `nil` (Required)      |
| ErrorHandler            | `func(*fiber.Ctx, error)` | Function executed for errors.                                                                       | `nil`                 |
| KeyGenerator            | `func() string` | KeyGenerator generates the session key.                                                                       | `utils.UUIDv4`        |
| KeyLookup               | `string`        | KeyLookup is a string in the form of "`<source>:<name>`" that is used to extract session id from the request. | `"cookie:session_id"` |
| CookieDomain            | `string`        | Domain of the cookie.                                                                                         | `""`                  |
| CookiePath              | `string`        | Path of the cookie.                                                                                           | `""`                  |
| CookieSameSite          | `string`        | Value of SameSite cookie.                                                                                     | `"Lax"`               |
| IdleTimeout             | `time.Duration` | Allowed session idle duration.                                                                                | `24 * time.Hour`      |
| Expiration              | `time.Duration` | Allowed session duration.                                                                                     | `24 * time.Hour`      |
| CookieSecure            | `bool`          | Indicates if cookie is secure.                                                                                | `false`               |
| CookieHTTPOnly          | `bool`          | Indicates if cookie is HTTP only.                                                                             | `false`               |
| CookieSessionOnly       | `bool`          | Decides whether cookie should last for only the browser session. Ignores Expiration if set to true.           | `false`               |

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
    IdleTimeout:       24 * time.Hour,
    Expiration:        24 * time.Hour,
    CookieSecure:      false,
    CookieHTTPOnly:    false,
    CookieSessionOnly: false,
}
```
