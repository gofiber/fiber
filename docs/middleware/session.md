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
```

Notes:

- AbsoluteTimeout must be greater than or equal to IdleTimeout; otherwise, the middleware panics during configuration.
- CookieSameSite accepts `"Disabled"`, `"Lax"`, `"Strict"`, or `"None"` case-insensitively; other values panic during configuration.
- If CookieSameSite is set to `"Disabled"`, the cookie omits the SameSite attribute and preserves `CookieSecure`.
- If CookieSameSite is set to `"None"`, the middleware automatically forces `CookieSecure=true` when setting the cookie.

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

    return c.Redirect().To("/dashboard")
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
    sess.Set("last_task", time.Now().Unix())

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

### Multiple Sessions per Request

A request can hold more than one session, each with its own cookie, storage and lifetime. A typical case is a short-lived session for a login flow next to the session that keeps the user signed in. Give every session its own middleware over its own cookie or header, and reach each one through the store it was built with:

```go
import (
    "time"
    "github.com/gofiber/fiber/v3/extractors"
)

loginHandler, loginStore := session.NewWithStore(session.Config{
    Extractor:   extractors.FromCookie("login_sid"),
    IdleTimeout: 10 * time.Minute,
})
appHandler, appStore := session.NewWithStore(session.Config{
    Extractor:       extractors.FromCookie("app_sid"),
    AbsoluteTimeout: 24 * time.Hour,
})
app.Use(loginHandler, appHandler)

app.Get("/auth/callback", func(c fiber.Ctx) error {
    login := loginStore.FromContext(c)
    user := appStore.FromContext(c)

    state, ok := session.GetAs[string](login, "state")
    if !ok || state != c.Query("state") {
        return fiber.ErrBadRequest
    }

    // The login flow is over: drop its session and start the signed-in one.
    if err := login.Destroy(); err != nil {
        return err
    }
    if err := user.Regenerate(); err != nil {
        return err
    }
    user.Set("user_id", 123)

    return c.Redirect().To("/")
})
```

- `store.FromContext(c)` returns the middleware that store installed for the request, or `nil` if it did not run, for example because its `Next` skipped it. The package-level `session.FromContext(c)` keeps returning the innermost middleware, the last one to run.
- Each middleware saves its own session when the handler returns, the inner one first, and reports a failed save to its own `ErrorHandler`. `Destroy()` removes only its own session; the others are saved as usual.
- Use a different cookie or header for every session. Stores that read the same cookie or header resolve the same session. That is how the session middleware and the store you give another middleware that runs before it, such as CSRF, end up with one session for a visitor who has no cookie yet: the store that runs first saves its session, and the other one finds it. Two session middleware over the same cookie do not: each saves its own session when its handler returns, and the cookie keeps the ID of the one that saves last, so the first request of a new visitor loses the session of the other. Stores whose extractors only partly overlap, such as a chain of the cookies `a` and `b` next to a store over `b` alone, count as sharing too, and a new visitor gets the same ID in both: give two stores either every source in common or none. A store that reads its ID only from a source it cannot write back to, such as the query string, an `Authorization` header, a form field, a route parameter or a custom extractor, cannot be told apart from the others and keeps sharing their ID, as it did before: a store behind it can end up with that ID instead of the one in its own cookie. The client chooses that ID, so a link or a form that carries a session ID in the query string, a form field or the path makes the stores behind it serve that session for the request, and sets no cookie for it. Do not mix such a store with cookie or header stores in one request.
- Sessions can share one storage if their stores have different [`StorageKeyPrefix`](#storage-keys) values, none of which is the start of another. Separate storages work as well, and are the better choice when the data of one store must never be reachable through another.
- A `timeout` middleware answers the request while the handlers behind it can still be running. When the session middleware in front of it has finished by then, `store.FromContext` returns `nil` for its store in those handlers, and never the session of another request, as long as you give it the Go context (`c.Context()` with `PassLocalsToContext`) that the handler took before it was abandoned: the `fiber.Ctx` itself must not be used after the request ended, with sessions or without them. A session such a handler still holds can be saved: the record is written, but no cookie is set, since the response was already sent.
- `store.Get(c)` serves several stores without the middleware too. Call it once per store and request and keep the session: asking a store again after a store over another cookie was asked, for a visitor who has no cookie yet, issues a second new ID.

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

        return c.Redirect().To("/dashboard")
    }

    return c.Status(401).SendString("Invalid credentials")
})

app.Post("/logout", func(c fiber.Ctx) error {
    sess := session.FromContext(c)

    // Complete session reset (clears all data + new session ID)
    if err := sess.Reset(); err != nil {
        return c.Status(500).SendString("Session error")
    }

    return c.Redirect().To("/")
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
    sess.Set("login_time", time.Now().Unix())

    return c.JSON(fiber.Map{"status": "logged in"})
})
```

### Security Methods Comparison

| Method | Session ID | Session Data | Use Case |
|--------|------------|--------------|----------|
| `Regenerate()` | ✅ Changes | ✅ Preserved | Login, privilege escalation |
| `Reset()` | ✅ Changes | ❌ Cleared | Logout, security breach |
| `Destroy()` | ⚪ Unchanged, cookie expired | ❌ Cleared | End the session without issuing a new ID |

These methods, `store.Delete` and `store.Reset` also hold against other requests of the same session that are still running: a request that loaded the session before its ID was deleted neither stores it again nor sends its cookie when it finishes, and its changes to the session are dropped. This holds when that request calls `Regenerate()` as well: the ID still changes, so the old one is never written again, but the new one is neither stored nor sent. Of two requests that rotate one session at the same time, only the first stores and sends a new ID, and a login that rotates a session a parallel logout just ended is dropped too. A request that left the session unchanged does not write it back over a change another request stored meanwhile, so a role taken away while it ran stays taken away; of two requests that both change the session, the one that saves last stores its whole session. This is guaranteed within one process. Processes that share an external `Storage` are not coordinated, because `fiber.Storage` has no conditional write, so a request still running on another instance can store a session it loaded before the delete. Where a logout must hold across instances, also keep a server-side revocation marker (for example, a per-user "sessions valid after" timestamp) that every request checks.

### Common Security Mistakes

❌ **Session Fixation Vulnerability:**

```go
// DANGEROUS: Keeping same session ID after login
app.Post("/login", func(c fiber.Ctx) error {
    sess := session.FromContext(c)
    // Validate user...
    sess.Set("user_id", userID) // Attacker can hijack this session!
    return c.Redirect().To("/dashboard")
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
    return c.Redirect().To("/dashboard")
})
```

### Authentication Middleware

This is a basic example of an authentication middleware that checks if a user is logged in before accessing protected routes.

```go
// Authentication check middleware
func RequireAuth(c fiber.Ctx) error {
    sess := session.FromContext(c)
    if sess == nil {
        return c.Redirect().To("/login")
    }

    // Check if user is authenticated
    if sess.Get("authenticated") != true {
        return c.Redirect().To("/login")
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
  - Near the end, a save keeps the record and the cookie only for the time that is left, so neither outlives the deadline. A save made less than a second before the deadline, or after it, keeps them for one second, because a storage may read a shorter lifetime as "never expires"
  - `sess.AbsoluteExpiry()` reports when the session ends, for example to warn the user
  - The cookie's `Max-Age` counts whole seconds and is rounded down, so the cookie can expire up to a second before the deadline. With an `AbsoluteTimeout` equal to the `IdleTimeout`, the first cookie of a session says one second less than the idle timeout
  - A session that was stored before the option was set has no deadline: it ends with the idle timeout alone, until it is reset
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

These rules describe a request that carries no session ID, for example a first visit. For an existing session, the ID is written back only through the extractor that supplied it, and only when that extractor is a cookie or a non-Authorization header. An ID that arrived through a read-only source (Authorization header, Query, Form, Param, or Custom) is never copied into a cookie or header, which prevents session fixation. When the middleware issues a new ID for a request that carried one, because `Regenerate()` or `Reset()` was called or because the supplied ID is not in the storage, the new ID goes back through the same cookie or header, or through every cookie and header extractor of the chain when the old ID came from a read-only source.

```go
// This will set both cookie and header in response
extractors.Chain(
    extractors.FromCookie("session_id"),
    extractors.FromHeader("X-Session-ID"),
)

// This will set only cookie in response
extractors.Chain(
    extractors.FromCookie("session_id"),
    extractors.FromQuery("session_id"),  // Ignored for response
)

// This will set nothing in response (read-only mode)
extractors.Chain(
    extractors.FromQuery("session_id"),
    extractors.FromForm("session_id"),
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

Return `extractors.ErrNotFound` when the request carries no session ID, so the visitor gets a new session. Any other error from an extractor is not treated as a new visitor: it goes to `ErrorHandler`, which answers `500` by default, or is returned by `store.Get`. This lets a custom or chain-level `Extract` that validates the ID refuse a forged one. A validating child inside a built-in `extractors.Chain` cannot: the chain keeps walking after an error, and a later child's value, or its `ErrNotFound`, replaces the refusal, so put the validation in the chain-level `Extract` or use the validating extractor alone. Such an `Extract` must return the ID unchanged, because the same value is written back and looked up in the storage.

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

A session is stored under `session:<session ID>`, so a session ID sent by a client cannot name an entry another middleware keeps in the same `Storage`. `store.GetByID` and `store.Delete` take the bare session ID. `store.Reset` clears the whole `Storage`, the entries of other middleware included. Sessions stored by an earlier version under the bare ID are not found after the upgrade, so users sign in again; those entries expire on their own in an external `Storage`.

A `Storage` that is a typed nil, such as an unset `*redis.Storage`, panics when the store is created, instead of on the first request, even if its methods would cope with a nil receiver. A plain `nil` stands for the default in-memory storage, and a `Storage` next to a `Config.Store` is not used, so it is not checked.

### Skipping Empty Sessions

By default, every request that arrives without a valid session creates one: a record in the storage that lives until the idle timeout, and a `Set-Cookie` header on the response. A client that never sends cookies, such as a health check, a monitoring probe or a crawler, therefore leaves a record behind with every request. `SkipEmptySessions` keeps a new session that nothing was stored in out of the storage and out of the response:

```go
app.Use(session.New(session.Config{
    SkipEmptySessions: true,
}))
```

- A session that holds data is saved exactly as before. A handler that wants a session to exist for a visitor stores something in it, if only a flag. Do not use `sess.ID()` as a visitor identifier while nothing is stored: without a cookie, the next request gets another one.
- A session that already exists is still saved on every request that goes through the middleware, so its idle timeout keeps sliding.
- A `sess.Reset()` that leaves nothing behind, such as a logout, expires the cookie and stores no new session. `sess.Fresh()` is `true` for a new session either way.
- The option applies to the middleware. A session from `store.Get(c)` is saved when you call `Save`. With `Config.Store`, set it on the config of the middleware; a warning is logged when only the store has it.

It is off by default so that existing applications keep their behavior. It is recommended for any application that clients without cookies can reach.

### Storage Keys

Two options change the key that every read, write and delete of a session uses, next to the `session:` namespace above:

```go
app.Use(session.New(session.Config{
    Storage:          redisStorage,
    StorageKeyPrefix: "myapp:session:", // end it with a separator
    HashStorageKeys:  true,
}))
```

- `StorageKeyPrefix` goes in front of `session:`, so that several applications, environments or stores can share one storage without their sessions meeting: the key is `myapp:session:<id>`. End it with a separator such as `:`, and choose prefixes so that none, followed by `session:`, is the start of another: `app:` and `admin:` stay apart, `app:` and `app:admin:` too, but `app:` and `app:session:admin:` do not, and a store without a prefix reaches the stores whose prefix starts with `session:`. The ID comes from the client and is appended as it is, so a store with the prefix `app:` reads the key `app:session:admin:session:<id>` when the client sends it the ID `admin:session:<id>`, which is the key of a store with the prefix `app:session:admin:`. An ID is unguessable, so this does not reach a session whose ID the client does not hold, but it does mean a prefix alone does not keep stores apart for a client that holds an ID of the other.
- `HashStorageKeys` stores each session under `session:` and the SHA-256 digest of its ID (base64url-encoded, 43 characters) instead of the ID. Whoever can read the storage, through a backup, a replica or a leaked dump, then holds digests that cannot be sent as a session cookie. The ID the client holds does not change, and neither do `store.GetByID` and `store.Delete`, which take that ID. It does not make a guessable ID from a custom `KeyGenerator` safe.
- Changing either option changes the key of every session, so sessions stored before the change are not found afterwards and their visitors get new ones, as after the upgrade that added the `session:` namespace. Plan the switch like any other invalidation of all sessions. Do not run a hashed and an unhashed store over the same storage and prefix, as during a rolling deployment that turns `HashStorageKeys` on: the unhashed store takes a digest read from the storage for a session ID. Switch to a new prefix instead.
- `store.Reset` clears the whole storage, not only the keys under the prefix.
- Like `Storage` and `Extractor`, both options belong to the store. With `Config.Store`, set them on the config the store was built with; a warning is logged when the config of the middleware sets them and the store does not.

### Production Security Settings

```go
import (
    "log"
    "time"
    "github.com/gofiber/fiber/v3"
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
    // CookieDomain stays empty: browsers reject a __Host- cookie that sets a Domain
    CookieSessionOnly: false,   // Persist across browser restarts

    // Session ID
    Extractor:         extractors.FromCookie("__Host-session_id"),
    KeyGenerator:      utils.SecureToken,

    // Error Handling
    ErrorHandler: func(c fiber.Ctx, err error) {
        log.Printf("Session error: %v", err)

        // Write the response: when the session cannot be loaded, the request is
        // not passed on, so nothing else answers it.
        if sendErr := c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Session error"}); sendErr != nil {
            log.Printf("Failed to send error response: %v", sendErr)
        }
    },
}))
```

A custom `ErrorHandler` has to write the response itself. When the session cannot be loaded from the storage, the request is not passed on, so a handler that only logs answers with an empty `200`. When the session cannot be saved, the route handler has already run: the status and body that `ErrorHandler` writes replace its own, while its headers stay, and a handler that only logs leaves that response, usually a success, for a session the storage did not keep.

Browsers, not the middleware, enforce the rules of the `__Host-` and `__Secure-` cookie name prefixes, and they drop a cookie that breaks them without a word, which shows up as sessions that never persist. A `__Host-` cookie needs `CookieSecure`, the path `/` and no `CookieDomain`; the default `CookiePath` is sent as `/`, so it meets that rule. A `__Secure-` cookie needs `CookieSecure` (`CookieSameSite: "None"` sets it too). The prefixes are matched case-insensitively. When the store is created, a warning is logged for each cookie of the `Extractor` that breaks its rules. The configuration is accepted as before.

### Custom Types

Session data supports basic Go types by default:

- `string`, `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `bool`, `float32`, `float64`
- `[]byte`, `complex64`, `complex128`
- `any`

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
- `time.Time` is a struct, so it needs `store.RegisterType(time.Time{})` as well. The store registers it by itself only when `AbsoluteTimeout` is greater than zero. Saving a session that holds an unregistered `time.Time` fails, and the middleware answers with its `ErrorHandler` (`500` by default). Store `time.Now().Unix()` when a plain timestamp is enough
- Reading a struct back out with `Get` makes the save re-encode, even for an all-scalar struct like `User`, because every struct is treated as possibly sharing memory with the session. A request that only reads scalars or `time.Time` values writes the stored bytes back unchanged

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

### Constructors and Helpers

```go
func New(config ...Config) fiber.Handler
func NewWithStore(config ...Config) (fiber.Handler, *Store)
func NewStore(config ...Config) *Store
func FromContext(ctx any) *Middleware
func GetAs[V any](src interface{ Get(key any) any }, key any) (V, bool)
func DefaultErrorHandler(c fiber.Ctx, err error)
```

`NewWithStore` returns the middleware together with the `Store` it uses, for example to call `RegisterType`. `FromContext` returns `nil` when the request has no session middleware. With several session middleware it returns the innermost one, the last to run; see [Multiple Sessions per Request](#multiple-sessions-per-request).

`GetAs` reads a value as a given type without a type assertion: `ok` is `false` when the key is absent or holds a value of another type, and the zero value is returned with it. `src` is a `*session.Middleware` or a `*session.Session`. A `nil` one finds nothing, so `session.GetAs[int](session.FromContext(c), "visits")` is safe even for a request without the session middleware.

### Middleware Methods (Recommended)

```go
sess := session.FromContext(c)

// Data operations
sess.Get(key any) any
sess.Set(key, value any)
sess.Delete(key any)
sess.Keys() []any

// Typed read: ok is false when the key is absent or holds another type
visits, ok := session.GetAs[int](sess, "visits")

// Session management
sess.ID() string
sess.Fresh() bool
sess.IdleTimeout() time.Duration         // Idle timeout of the session
sess.AbsoluteExpiry() (time.Time, bool)  // When the session ends however active it is
sess.Regenerate() error  // Change ID, keep data
sess.Reset() error       // Change ID, clear data
sess.Destroy() error     // Delete from storage, expire the cookie

// Context-aware variants propagate cancellation/deadlines to storage I/O.
// Pass a context.Context to control timeouts; a nil context is treated as
// context.Background().
sess.RegenerateWithContext(ctx context.Context) error
sess.ResetWithContext(ctx context.Context) error
sess.DestroyWithContext(ctx context.Context) error

// Store access
sess.Store() *session.Store
```

`FromContext` accepts a `fiber.CustomCtx`, `fiber.Ctx`, a `*fasthttp.RequestCtx`, or a `context.Context`.

The returned `*session.Middleware` is only valid while the request is being handled. A context you kept from `c.Context()` (with `PassLocalsToContext` enabled) stops resolving it when the request ends: `FromContext` returns `nil` for that context from then on, so a goroutine that outlives the handler cannot reach the session, and never another request's. Do not hand the pointer itself to such a goroutine. Pass it the session ID and use the [Store Pattern](#store-pattern-advanced) instead.

A handler the [Timeout](./timeout.md) middleware gave up on keeps its `*session.Middleware` and its session until it returns, and no other request is handed them. The session of such a request is not saved: the timeout response is already fixed and cannot carry a cookie, and the handler may still be changing the data.

A session that was only read is written back to storage without being re-encoded. `Get` and `Keys` give that up when the value or key they return could share memory with the session (a map, slice, array, pointer or struct; scalars and `time.Time` are safe), because the caller could then change it in place without calling `Set`.

### Store Methods

```go
store := session.NewStore()

// Store operations
store.Get(c fiber.Ctx) (*session.Session, error)
store.GetByID(ctx context.Context, sessionID string) (*session.Session, error)
store.Reset(ctx context.Context) error
store.Delete(ctx context.Context, sessionID string) error

// This store's middleware in the current request, for several sessions per request
store.FromContext(ctx any) *session.Middleware

// Type registration
store.RegisterType(User{})
```

`store.Reset` clears the whole storage, including keys that are not sessions and the sessions of other stores that share it. A `StorageKeyPrefix` does not limit it, because the storage interface cannot list keys.

`sess.IdleTimeout()` is the timeout set with `SetIdleTimeout`, or else the store's. A save applies it, cut down to the time left until the absolute deadline. A timeout set with `SetIdleTimeout` holds for every later save of that session in the request, unless the session is reset, and is not kept for the next request, so set it again where it matters. `sess.AbsoluteExpiry()` reports `false` for a store without an `AbsoluteTimeout`, and for a session that was stored before the store had one.

Like the other methods of the middleware, these take a lock. `store.FromContext` and `sess.Save` take the read lock of the middleware, which is held while it destroys, resets or regenerates its session. `sess.Save` and `sess.IdleTimeout()` take the lock of the session, which is held while it saves. Call them from handlers, not from inside a `Storage` implementation, which the session calls during those operations.

### Errors

```go
var (
    ErrEmptySessionID                   = errors.New("session ID cannot be empty")
    ErrSessionAlreadyLoadedByMiddleware = errors.New("session already loaded by middleware")
    ErrSessionIDNotFoundInStore         = errors.New("session ID not found in session store")
)
```

- `store.Get(c)` returns `ErrSessionAlreadyLoadedByMiddleware` when the session middleware already loaded the session for the request. Use `session.FromContext(c)` there.
- `store.GetByID` and `store.Delete` return `ErrEmptySessionID` for an empty session ID.
- `store.GetByID` returns `ErrSessionIDNotFoundInStore` when the storage has no such session, or when the session has outlived its `AbsoluteTimeout` (it is deleted).

### Session Methods (Store Pattern)

```go
sess, err := store.Get(c)
defer sess.Release() // Required!

// Same methods as middleware (except Store()), plus:
sess.Save() error              // Manual save required
sess.SetIdleTimeout(duration)  // Per-session timeout
sess.Release()                 // Manual cleanup required

// Context-aware variants propagate cancellation/deadlines to storage I/O.
// A nil context is treated as context.Background().
sess.DestroyWithContext(ctx context.Context) error
sess.RegenerateWithContext(ctx context.Context) error
sess.ResetWithContext(ctx context.Context) error
sess.SaveWithContext(ctx context.Context) error
```

### Session with Context (timeouts/cancellation)

The `*WithContext` variants let you propagate a `context.Context` to the underlying storage call so that slow or unresponsive backends can be bounded by a deadline or canceled. This mirrors the `Storage` and `SharedState` `WithContext` convention.

```go
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()

// Persist the session, bounding the storage write by the deadline.
if err := sess.SaveWithContext(ctx); err != nil {
    // deadline exceeded, cancellation, or storage error
}

// Destroy is also covered:
// if err := sess.DestroyWithContext(ctx); err != nil { ... }
```

A `nil` context is treated as `context.Background()`, so `sess.SaveWithContext(nil)` is equivalent to `sess.Save()` when no request context is available.

### Extractor Functions

```go
// Built-in extractors (import "github.com/gofiber/fiber/v3/extractors")
extractors.FromCookie(key string) extractors.Extractor
extractors.FromHeader(key string) extractors.Extractor
extractors.FromAuthHeader(authScheme string) extractors.Extractor
extractors.FromQuery(key string) extractors.Extractor
extractors.FromForm(key string) extractors.Extractor
extractors.FromParam(key string) extractors.Extractor
extractors.FromCustom(key string, fn func(fiber.Ctx) (string, error)) extractors.Extractor

// Chaining
extractors.Chain(extractors ...extractors.Extractor) extractors.Extractor
```

### Config Properties

| Property            | Type                        | Description                 | Default                                    |
|---------------------|-----------------------------|-----------------------------|--------------------------------------------|
| `Store`             | `*session.Store`            | Pre-built session store (use when you need to share/register types). The store's own options decide how sessions are stored and sent; this config supplies only `Next`, `ErrorHandler` and `SkipEmptySessions` | `nil` (auto-created)                       |
| `Storage`           | `fiber.Storage`             | Session storage backend (used when creating a store if `Store` is nil) | `memory.New()`                             |
| `Extractor`         | `extractors.Extractor`      | Session ID extraction       | `extractors.FromCookie("session_id")`     |
| `KeyGenerator`      | `func() string`             | Session ID generator        | `utils.SecureToken`                             |
| `IdleTimeout`       | `time.Duration`             | Inactivity timeout          | `30 * time.Minute`                         |
| `AbsoluteTimeout`   | `time.Duration`             | Maximum session duration    | `0` (unlimited)                            |
| `CookieSecure`      | `bool`                      | HTTPS only                  | `false`                                    |
| `CookieHTTPOnly`    | `bool`                      | No JavaScript access        | `false`                                    |
| `CookieSameSite`    | `string`                    | SameSite mode: `Disabled`, `Lax`, `Strict`, or `None` | `"Lax"`                    |
| `CookiePath`        | `string`                    | Cookie path                 | `""`                                       |
| `CookieDomain`      | `string`                    | Cookie domain               | `""`                                       |
| `CookieSessionOnly` | `bool`                      | Session cookie              | `false`                                    |
| `SkipEmptySessions` | `bool`                      | Do not store, or set a cookie for, a new session that holds no data. See [Skipping Empty Sessions](#skipping-empty-sessions) | `false` |
| `StorageKeyPrefix`  | `string`                    | Prefix of the key every session is stored under. See [Storage Keys](#storage-keys) | `""`                 |
| `HashStorageKeys`   | `bool`                      | Store each session under the SHA-256 digest of its ID instead of the ID. See [Storage Keys](#storage-keys) | `false`   |
| `Next`              | `func(fiber.Ctx) bool`      | Skip middleware when returns true | `nil`                                  |
| `ErrorHandler`      | `func(fiber.Ctx, error)`    | Called when the session cannot be loaded from the storage, in which case the request is not passed on, or cannot be saved after the handler returns. A custom handler has to write the response: one that only logs answers an empty `200` after a load failure and leaves the route handler's response in place after a save failure. The default logs the error and responds with `500 Internal Server Error` | `DefaultErrorHandler`                      |

### Default Config

```go
var ConfigDefault = Config{
    IdleTimeout:    30 * time.Minute,
    KeyGenerator:   utils.SecureToken,
    Extractor:      extractors.FromCookie("session_id"),
    CookieSameSite: fiber.CookieSameSiteLaxMode,
}
```

An unset `IdleTimeout`, `Extractor`, `KeyGenerator`, or `CookieSameSite` is filled from `ConfigDefault`. A `Storage` that is `nil` becomes an in-memory store when the store is created.

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
        sess.Set("last_call", time.Now().Unix())

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
