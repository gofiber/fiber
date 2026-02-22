---
id: session
---

# Session

The Session middleware adds session management to Fiber apps through the [Storage](https://github.com/gofiber/storage) package, which offers a unified interface for multiple databases. By default, sessions live in memory, but you can plug in any storage backend.

## Table of Contents

- [Quick Start](#quick-start)
- [Usage Patterns](#usage-patterns)
- [Session Security](#session-security)
- [Session ID Extractors](#session-id-extractors)
- [Configuration](#configuration)
- [Migration Guide](#migration-guide)
- [API Reference](#api-reference)
- [Examples](#examples)

## Quick Start

```go
import (
    "fmt"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/session"
)

// Basic usage
app.Use(session.New())

app.Get("/", func(c fiber.Ctx) error {
    sess := session.FromContext(c)

    // Get and update visits count
    var visits int
    if v := sess.Get("visits"); v != nil {
        // Use type assertion with an ok check to prevent a panic
        if vInt, ok := v.(int); ok {
            visits = vInt
        }
    }
    visits++
    sess.Set("visits", visits)
    return c.SendString(fmt.Sprintf("Visits: %d", visits))
})
```

### Production Configuration

```go
import (
    "time"
    "github.com/gofiber/fiber/v3/extractors"
    "github.com/gofiber/storage/redis/v3"
)

storage := redis.New(redis.Config{
    Host: "localhost",
    Port: 6379,
})

app.Use(session.New(session.Config{
    Storage:           storage,
    CookieSecure:      true,              // HTTPS only
    CookieHTTPOnly:    true,              // Prevent XSS
    CookieSameSite:    "Lax",             // CSRF protection
    IdleTimeout:       30 * time.Minute,  // Session timeout
    AbsoluteTimeout:   24 * time.Hour,    // Maximum session life
    Extractor:         extractors.FromCookie("__Host-session_id"),
}))

Notes:

- AbsoluteTimeout must be greater than or equal to IdleTimeout; otherwise, the middleware panics during configuration.
- If CookieSameSite is set to "None", the middleware automatically forces CookieSecure=true when setting the cookie.
```

## Usage Patterns

### Middleware Pattern (Recommended)

This pattern automatically manages the session lifecycle and is recommended for most applications.

```go
// Setup middleware
app.Use(session.New())

// Use in handlers
app.Post("/login", func(c fiber.Ctx) error {
    sess := session.FromContext(c)

    // Session is automatically saved when handler returns
    sess.Set("user_id", 123)
    sess.Set("authenticated", true)

    return c.Redirect("/dashboard")
})
```

**Benefits:**

- Automatic session saving
- Automatic resource cleanup
- No manual lifecycle management
- Thread-safe operations

### Store Pattern (Advanced)

Use the store pattern for background tasks or when you need direct access to sessions.

```go
import (
    "context"
    "log"
    "time"
)

store := session.NewStore()

// In background tasks
func backgroundTask(sessionID string) {
    sess, err := store.GetByID(context.Background(), sessionID)
    if err != nil {
        return
    }
    defer sess.Release() // Important: Manual cleanup required

    // Modify session
    sess.Set("last_task", time.Now())

    // Manual save required
    if err := sess.Save(); err != nil {
        log.Printf("Failed to save session: %v", err)
    }
}
```

**Requirements:**

- Must call `sess.Release()` when done
- Must call `sess.Save()` to persist changes
- Handle errors manually

## Session Security

### Authentication Flow

Understanding session lifecycle during authentication is crucial for security.

#### Basic Login/Logout

```go
app.Post("/login", func(c fiber.Ctx) error {
    sess := session.FromContext(c)

    email := c.FormValue("email")
    password := c.FormValue("password")

    // Simple credential validation (use proper authentication in production)
    if email == "admin@example.com" && password == "secret" {
        // Important: Regenerate the session ID to prevent fixation
        // This changes the session ID while preserving existing data
        if err := sess.Regenerate(); err != nil {
            return c.Status(500).SendString("Session error")
        }

        // Add authentication data to existing session
        sess.Set("user_id", 1)
        sess.Set("authenticated", true)

        return c.Redirect("/dashboard")
    }

    return c.Status(401).SendString("Invalid credentials")
})

app.Post("/logout", func(c fiber.Ctx) error {
    sess := session.FromContext(c)

    // Complete session reset (clears all data + new session ID)
    if err := sess.Reset(); err != nil {
        return c.Status(500).SendString("Session error")
    }

    return c.Redirect("/")
})
```

#### Cart Preservation During Login

```go
app.Post("/login", func(c fiber.Ctx) error {
    sess := session.FromContext(c)

    // Validate credentials (implement your own validation)
    email := c.FormValue("email")
    password := c.FormValue("password")
    if !isValidUser(email, password) {
        return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
    }

    // Important: Regenerate the session ID to prevent fixation
    // This changes the session ID while preserving existing data
    if err := sess.Regenerate(); err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Session error"})
    }

    // Add authentication data to existing session
    sess.Set("user_id", getUserID(email))
    sess.Set("authenticated", true)
    sess.Set("login_time", time.Now())

    return c.JSON(fiber.Map{"status": "logged in"})
})
```

### Security Methods Comparison

| Method | Session ID | Session Data | Use Case |
|--------|------------|--------------|----------|
| `Regenerate()` | ✅ Changes | ✅ Preserved | Login, privilege escalation |
| `Reset()` | ✅ Changes | ❌ Cleared | Logout, security breach |
| `Destroy()` | ⚪ Unchanged | ❌ Cleared | Clear data only |

### Common Security Mistakes

❌ **Session Fixation Vulnerability:**

```go
// DANGEROUS: Keeping same session ID after login
app.Post("/login", func(c fiber.Ctx) error {
    sess := session.FromContext(c)
    // Validate user...
    sess.Set("user_id", userID) // Attacker can hijack this session!
    return c.Redirect("/dashboard")
})
```

✅ **Secure Implementation:**

```go
// SECURE: Always regenerate session ID after authentication
app.Post("/login", func(c fiber.Ctx) error {
    sess := session.FromContext(c)
    // Validate user...
    if err := sess.Regenerate(); err != nil { // Prevents session fixation
        return err
    }
    sess.Set("user_id", userID)
    return c.Redirect("/dashboard")
})
```

### Authentication Middleware

This is a basic example of an authentication middleware that checks if a user is logged in before accessing protected routes.

```go
// Authentication check middleware
func RequireAuth(c fiber.Ctx) error {
    sess := session.FromContext(c)
    if sess == nil {
        return c.Redirect("/login")
    }

    // Check if user is authenticated
    if sess.Get("authenticated") != true {
        return c.Redirect("/login")
    }

    return c.Next()
}

// Usage
app.Use("/dashboard", RequireAuth)
app.Use("/admin", RequireAuth)
```

### Automatic Session Expiration

Sessions automatically expire based on your configuration:

```go
app.Use(session.New(session.Config{
    IdleTimeout:     30 * time.Minute, // Auto-expire after 30 min of inactivity
    AbsoluteTimeout: 24 * time.Hour,   // Force expire after 24 hours regardless of activity
}))
```

**How it works:**

- `IdleTimeout`: Storage automatically removes sessions after inactivity period
  - Any route that uses the middleware will reset the idle timer
  - Calling `sess.Save()` will also reset the idle timer
- `AbsoluteTimeout`: Sessions are forcibly expired after maximum duration
- No manual cleanup required - the storage layer handles this

## Session ID Extractors

This middleware uses the shared extractors module for session ID extraction. See the [Extractors Guide](../guide/extractors) for more details.

### Built-in Extractors

```go
// Cookie-based (recommended for web apps)
extractors.FromCookie("session_id")

// Header-based (recommended for APIs)
extractors.FromHeader("X-Session-ID")

// Authorization header (read-only)
extractors.FromAuthHeader("Bearer")

// Form data
extractors.FromForm("session_id")

// URL query parameter
extractors.FromQuery("session_id")

// URL path parameter
extractors.FromParam("id")
```

**Session Response Behavior:**

- Cookie extractors: set cookie in the response
- Header extractors (non-Authorization): set header in the response
- Authorization header, Query, Form, Param, Custom: read-only (no response values are set)

### Multiple Sources with Fallback

```go
app.Use(session.New(session.Config{
    Extractor: extractors.Chain(
        extractors.FromCookie("session_id"),    // Try cookie first
        extractors.FromHeader("X-Session-ID"),  // Then header
        extractors.FromQuery("session_id"),     // Finally query
    ),
}))
```

**Response Behavior with Chained Extractors:**

Only cookie and non-Authorization header extractors contribute to response setting. Others are read-only.

- Cookie + Header (non-Auth) extractors: both cookie and header are set
- Only Cookie extractors: only cookie is set
- Only Header (non-Auth) extractors: only header is set
- Any mix that includes Authorization/Query/Form/Param/Custom: those sources are read-only

```go
// This will set both cookie and header in response
extractors.Chain(
    extractors.FromCookie("session_id"),
    extractors.FromHeader("X-Session-ID")
)

// This will set only cookie in response
extractors.Chain(
    extractors.FromCookie("session_id"),
    extractors.FromQuery("session_id")   // Ignored for response
)

// This will set nothing in response (read-only mode)
extractors.Chain(
    extractors.FromQuery("session_id"),
    extractors.FromForm("session_id")
)
```

### Custom Extractors (Session-specific)

Prefer the helper constructors from the extractors module. See the Extractors Guide for the full API; below are session-specific examples and notes.

```go
// Authorization Bearer tokens (read-only for sessions)
// The session middleware will NOT set Authorization back in the response.
app.Use(session.New(session.Config{
    Extractor: extractors.FromAuthHeader("Bearer"),
}))
```

```go
// Custom read-only header via FromCustom (read-only for sessions)
app.Use(session.New(session.Config{
    Extractor: extractors.FromCustom("X-Custom-Session", func(c fiber.Ctx) (string, error) {
        v := c.Get("X-Custom-Session")
        if v == "" { return "", extractors.ErrNotFound }
        return v, nil
    }),
}))
```

## Configuration

### Storage Options

```go
import (
    "github.com/gofiber/storage/redis/v3"
    "github.com/gofiber/storage/postgres/v3"
)

// Redis (recommended for production)
redisStorage := redis.New(redis.Config{
    Host:     "localhost",
    Port:     6379,
    Password: "",
    Database: 0,
})

// PostgreSQL
pgStorage := postgres.New(postgres.Config{
    Host:     "localhost",
    Port:     5432,
    Database: "sessions",
    Username: "user",
    Password: "pass",
})

app.Use(session.New(session.Config{
    Storage: redisStorage,
}))
```

### Production Security Settings

```go
import (
    "log"
    "time"
    "github.com/gofiber/utils/v2"
    "github.com/gofiber/fiber/v3/extractors"
)

app.Use(session.New(session.Config{
    // Storage
    Storage: redisStorage,

    // Security
    CookieSecure:      true,    // HTTPS only (required in production)
    CookieHTTPOnly:    true,    // No JavaScript access (prevents XSS)
    CookieSameSite:    "Lax",   // CSRF protection

    // Session Management
    IdleTimeout:       30 * time.Minute,  // Inactivity timeout
    AbsoluteTimeout:   24 * time.Hour,    // Maximum session duration

    // Cookie Settings
    CookiePath:        "/",
    CookieDomain:      "example.com",
    CookieSessionOnly: false,   // Persist across browser restarts

    // Session ID
    Extractor:         extractors.FromCookie("__Host-session_id"),
    KeyGenerator:      utils.SecureToken,

    // Error Handling
    ErrorHandler: func(c fiber.Ctx, err error) {
        log.Printf("Session error: %v", err)
    },
}))
```

### Custom Types

Session data supports basic Go types by default:

- `string`, `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `bool`, `float32`, `float64`
- `[]byte`, `complex64`, `complex128`
- `interface{}`

For custom types (structs, maps, slices), you must register them for encoding/decoding:

```go
import "fmt"

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Role string `json:"role"`
}

// Method 1: Using NewWithStore
func main() {
    app := fiber.New()

    sessionMiddleware, store := session.NewWithStore()
    store.RegisterType(User{}) // Register custom type

    app.Use(sessionMiddleware)

    app.Get("/", func(c fiber.Ctx) error {
        sess := session.FromContext(c)

        // Use custom type
        sess.Set("user", User{ID: 123, Name: "John", Role: "admin"})

        user, ok := sess.Get("user").(User)
        if ok {
            return c.JSON(fiber.Map{"user": user.Name, "role": user.Role})
        }
        return c.SendString("No user found")
    })

    app.Listen(":3000")
}
```

```go
// Method 2: Using separate store
store := session.NewStore()
store.RegisterType(User{})

app.Use(session.New(session.Config{
    Store: store,
}))

// Usage in handlers
sess.Set("user", User{ID: 123, Name: "John", Role: "admin"})
user, ok := sess.Get("user").(User)
if ok {
    fmt.Printf("User: %s (Role: %s)", user.Name, user.Role)
}
```

**Important Notes:**

- Custom types must be registered before using them in sessions
- Registration must happen during application startup
- All instances of the application must register the same types
- Types are encoded using Go's `gob` package

## Migration Guide

### v2 to v3 Breaking Changes

1. **Function Signature**: `session.New()` now returns middleware handler, not store
2. **Session ID Extraction**: `KeyLookup` replaced with `Extractor` functions
3. **Lifecycle Management**: Manual `Release()` required for store pattern
4. **Timeout Handling**: `Expiration` split into `IdleTimeout` and `AbsoluteTimeout`

### Migration Examples

**v2 Code:**

```go
store := session.New(session.Config{
    KeyLookup: "cookie:session_id",
})

app.Get("/", func(c fiber.Ctx) error {
    sess, err := store.Get(c)
    if err != nil {
        return err
    }
    // Session automatically saved and released
    sess.Set("key", "value")
    return nil
})
```

**v3 Middleware Pattern (Recommended):**

```go
app.Use(session.New(session.Config{
    Extractor: extractors.FromCookie("session_id"),
}))

app.Get("/", func(c fiber.Ctx) error {
    sess := session.FromContext(c)
    // Session automatically saved and released
    sess.Set("key", "value")
    return nil
})
```

**v3 Store Pattern (Advanced):**

```go
store := session.NewStore(session.Config{
    Extractor: extractors.FromCookie("session_id"),
})

app.Get("/", func(c fiber.Ctx) error {
    sess, err := store.Get(c)
    if err != nil {
        return err
    }
    defer sess.Release() // Manual cleanup required

    sess.Set("key", "value")
    return sess.Save() // Manual save required
})
```

### KeyLookup to Extractor Migration

| v2 KeyLookup                    | v3 Extractor                                                                       |
|---------------------------------|------------------------------------------------------------------------------------|
| `"cookie:session_id"`           | `extractors.FromCookie("session_id")`                                             |
| `"header:X-Session-ID"`         | `extractors.FromHeader("X-Session-ID")`                                           |
| `"query:session_id"`            | `extractors.FromQuery("session_id")`                                              |
| `"form:session_id"`             | `extractors.FromForm("session_id")`                                               |
| `"cookie:sid,header:X-Sid"`     | `extractors.Chain(extractors.FromCookie("sid"), extractors.FromHeader("X-Sid"))` |

## API Reference

### Middleware Methods (Recommended)

```go
sess := session.FromContext(c)

// Data operations
sess.Get(key any) any
sess.Set(key, value any)
sess.Delete(key any)
sess.Keys() []any

// Session management
sess.ID() string
sess.Fresh() bool
sess.Regenerate() error  // Change ID, keep data
sess.Reset() error       // Change ID, clear data
sess.Destroy() error     // Keep ID, clear data

// Store access
sess.Store() *session.Store
```

### Store Methods

```go
store := session.NewStore()

// Store operations
store.Get(c fiber.Ctx) (*session.Session, error)
store.GetByID(ctx context.Context, sessionID string) (*session.Session, error)
store.Reset(ctx context.Context) error
store.Delete(ctx context.Context, sessionID string) error

// Type registration
store.RegisterType(interface{})
```

### Session Methods (Store Pattern)

```go
sess, err := store.Get(c)
defer sess.Release() // Required!

// Same methods as middleware, plus:
sess.Save() error              // Manual save required
sess.SetIdleTimeout(duration)  // Per-session timeout
sess.Release()                 // Manual cleanup required
```

### Extractor Functions

```go
// Built-in extractors (import "github.com/gofiber/fiber/v3/extractors")
extractors.FromCookie(key string) extractors.Extractor
extractors.FromHeader(key string) extractors.Extractor
extractors.FromQuery(key string) extractors.Extractor
extractors.FromForm(key string) extractors.Extractor
extractors.FromParam(key string) extractors.Extractor

// Chaining
extractors.Chain(extractors ...extractors.Extractor) extractors.Extractor
```

### Config Properties

| Property            | Type                        | Description                 | Default                                    |
|---------------------|-----------------------------|-----------------------------|--------------------------------------------|
| `Store`             | `*session.Store`            | Pre-built session store (use when you need to share/register types) | `nil` (auto-created)                       |
| `Storage`           | `fiber.Storage`             | Session storage backend (used when creating a store if `Store` is nil) | `memory.New()`                             |
| `Extractor`         | `extractors.Extractor`      | Session ID extraction       | `extractors.FromCookie("session_id")`     |
| `KeyGenerator`      | `func() string`             | Session ID generator        | `utils.SecureToken`                             |
| `IdleTimeout`       | `time.Duration`             | Inactivity timeout          | `30 * time.Minute`                         |
| `AbsoluteTimeout`   | `time.Duration`             | Maximum session duration    | `0` (unlimited)                            |
| `CookieSecure`      | `bool`                      | HTTPS only                  | `false`                                    |
| `CookieHTTPOnly`    | `bool`                      | No JavaScript access        | `false`                                    |
| `CookieSameSite`    | `string`                    | SameSite attribute          | `"Lax"`                                    |
| `CookiePath`        | `string`                    | Cookie path                 | `""`                                       |
| `CookieDomain`      | `string`                    | Cookie domain               | `""`                                       |
| `CookieSessionOnly` | `bool`                      | Session cookie              | `false`                                    |
| `Next`              | `func(fiber.Ctx) bool`      | Skip middleware when returns true | `nil`                                  |
| `ErrorHandler`      | `func(fiber.Ctx, error)`    | Error callback              | `DefaultErrorHandler`                      |

## Examples

### E-commerce with Cart Persistence

```go
import (
    "time"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/session"
    "github.com/gofiber/fiber/v3/extractors"
    "github.com/gofiber/storage/redis/v3"
)

func main() {
    app := fiber.New()

    // Session middleware
    app.Use(session.New(session.Config{
        Storage:           redis.New(),
        CookieSecure:      true,
        CookieHTTPOnly:    true,
        CookieSameSite:    "Lax",
        IdleTimeout:       30 * time.Minute,
        AbsoluteTimeout:   24 * time.Hour,
        Extractor:         extractors.FromCookie("__Host-cart_session"),
    }))

    // Add to cart (anonymous user)
    app.Post("/cart/add", func(c fiber.Ctx) error {
        sess := session.FromContext(c)

        cart, _ := sess.Get("cart").([]string)
        cart = append(cart, c.FormValue("item_id"))
        sess.Set("cart", cart)

        return c.JSON(fiber.Map{"items": len(cart)})
    })

    // Login (preserve session data)
    app.Post("/login", func(c fiber.Ctx) error {
        sess := session.FromContext(c)

        // Simple validation (implement proper authentication)
        email := c.FormValue("email")
        password := c.FormValue("password")
        if email != "user@example.com" || password != "password" {
            return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
        }

        // Regenerate session ID for security
        // This changes the session ID while preserving existing data
        if err := sess.Regenerate(); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": "Session error"})
        }

        sess.Set("user_id", 1)
        sess.Set("authenticated", true)

        return c.JSON(fiber.Map{"status": "logged in"})
    })

    // Logout (clear everything)
    app.Post("/logout", func(c fiber.Ctx) error {
        sess := session.FromContext(c)

        // Reset clears all data and generates new session ID
        if err := sess.Reset(); err != nil {
            return c.Status(500).JSON(fiber.Map{"error": "Session error"})
        }

        return c.JSON(fiber.Map{"status": "logged out"})
    })

    app.Listen(":3000")
}

// Helper functions (implement these properly in production)
func isValidUser(email, password string) bool {
    return email == "user@example.com" && password == "password"
}

func getUserID(email string) int {
    return 1 // Return actual user ID from database
}
```

### API with Header-based Sessions

```go
import (
    "time"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/session"
    "github.com/gofiber/fiber/v3/extractors"
    "github.com/gofiber/storage/redis/v3"
)

func main() {
    app := fiber.New()

    // API session middleware with header extraction
    app.Use(session.New(session.Config{
        Storage:     redis.New(),
        Extractor:   extractors.FromHeader("X-Session-Token"),
        IdleTimeout: time.Hour,
    }))

    // API endpoint
    app.Post("/api/data", func(c fiber.Ctx) error {
        sess := session.FromContext(c)

        // Track API usage
        count, _ := sess.Get("api_calls").(int)
        count++
        sess.Set("api_calls", count)
        sess.Set("last_call", time.Now())

        return c.JSON(fiber.Map{
            "data":  "some data",
            "calls": count,
        })
    })

    app.Listen(":3000")
}
```

### Multi-source Session ID Support

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/session"
    "github.com/gofiber/fiber/v3/extractors"
)

func main() {
    app := fiber.New()

    // Support multiple sources with priority
    app.Use(session.New(session.Config{
        Extractor: extractors.Chain(
            extractors.FromCookie("session_id"),    // 1st: Cookie (web)
            extractors.FromHeader("X-Session-ID"),  // 2nd: Header (API)
            extractors.FromQuery("session_id"),     // 3rd: Query (fallback)
        ),
    }))

    app.Get("/", func(c fiber.Ctx) error {
        sess := session.FromContext(c)

        // Works with any of the above methods
        return c.JSON(fiber.Map{
            "session_id": sess.ID(),
            "source":     "multi-source",
        })
    })

    app.Listen(":3000")
}
```
