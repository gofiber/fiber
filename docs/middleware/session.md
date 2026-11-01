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

- `store.FromContext(c)` returns the middleware that store installed for the request, or `nil` if it did not run, for example because its `Next` skipped it. The package-level `session.FromContext(c)` returns the innermost middleware that is running; once that one has finished, code that still runs inside the outer one finds the outer middleware, where it used to find none.
- Each middleware saves its own session when the handler returns, the inner one first, and reports a failed save to its own `ErrorHandler`. `Destroy()` removes only its own session; the others are saved as usual.
- Use a different cookie or header for every session. Stores that read the same cookie or header resolve the same session. That is how the session middleware and the store you give another middleware that runs before it, such as CSRF, end up with one session for a visitor who has no cookie yet: the store that runs first saves its session, and the other one finds it. Two session middleware over the same cookie do not: each saves its own session when its handler returns, and the cookie keeps the ID of the one that saves last, so the first request of a new visitor loses the session of the other. Stores whose extractors only partly overlap, such as a chain of the cookies `a` and `b` next to a store over `b` alone, count as sharing too, and a new visitor gets the same ID in both: give two stores either every source in common or none. Stores are told apart by every source they read, by kind and name: a store that reads its ID from the query string, a form field, a route parameter, an `Authorization` scheme or a custom extractor shares the ID with a store that reads the same source, and with no other, so a link or a form that carries a session ID reaches the stores that read that source and leaves the cookie stores next to them to their own cookies. Two custom extractors are told apart by their key: give the ones that read the same place the same key. Whatever the stores share, each writes a session ID only to its own cookies and headers: a store never writes to the cookie of another, with its own attributes, not even after a `Regenerate` or `Reset` of a session it took over.
- Sessions can share one storage if their stores have different [`StorageKeyPrefix`](#storage-keys) values, none of which, followed by `session:`, is the start of another. Separate storages work as well, and are the better choice when the data of one store must never be reachable through another.
- A `timeout` middleware answers the request while the handlers behind it can still be running. Those handlers keep their middleware and their session until they return, and no other request is handed them: the middleware does not save the session, since the response is already fixed (see [Middleware Methods](#middleware-methods-recommended)), and neither refers to the request any more, whose context is reused once the handlers are done. A `Save` that such a handler calls itself writes the record and sets no cookie. A request that is abandoned any other way, as the `sse` middleware does, is saved when its handler returns, and its middleware is finished from then on: `store.FromContext` returns `nil` for its store in the handlers that still run, and never the session of another request, as long as you give it the Go context (`c.Context()` with `PassLocalsToContext`) that the handler took before it was abandoned: the `fiber.Ctx` itself must not be used after the request ended, with sessions or without them. A `*session.Session` such a handler still holds can be saved: the record is written, but no cookie is set, since the response was already sent. A panic that unwinds the session middleware, recovered by a middleware in front of it, leaves the session unsaved. The recovering middleware can still reach the middleware with `session.FromContext(c)` or `store.FromContext(c)`, and the one it runs inside, and a `Destroy`, `Reset` or `Regenerate` from there changes the storage and sets the cookies as before the panic, until the request ends. A `Save` from there saves the session and sets its cookie, where it does nothing while the middleware is in charge of the session. The middleware and the session let go of the `fiber.Ctx` at once, since it goes back to the pool as soon as the handlers return, and of the request when fasthttp resets it, before it is reused: a goroutine of the handler that still holds the session writes into no other request. A Go context kept past the request does not find the middleware.
- `csrf` keeps its token in the session of the innermost session middleware, the last to run, whichever store you give it as `Session`: with several session middleware in a request, put the one that should hold the token last.
- While a session middleware is running for the request, `store.Get(c)` returns `ErrSessionAlreadyLoadedByMiddleware` whichever store it is called on, as it always has: reach the sessions of the middleware with `store.FromContext(c)`.
- `store.Get(c)` serves several stores without the middleware too, and it works after the session middleware has finished with the request, for example in a middleware in front of it once `c.Next()` returns: it finds the session that an earlier `store.Get` of the request saved, and otherwise reads the session ID from the request, as for a request the middleware did not serve. Call it once per store and request and keep the session. A store that is asked again finds the session it saved, whatever other stores were asked in between, and under the new ID if `Regenerate` or `Reset` gave the session one; so does a store that shares a cookie or header with it over the same storage and key settings. The ID that one store issues never takes the place of the ID of a store that reads other cookies and headers. A session that was not saved is not found: an ID is not a session until it is saved, so asking again before a `Save` issues another new ID.

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

`Regenerate()` and `Reset()` also tell the request about the new ID, so a store that is asked again with `store.Get(c)` finds the session under it once it is saved. Like `Save()`, they use the request of the session: call them from the goroutine that serves the request, not from one of your own.

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
  - A session that was stored with a deadline keeps it after the option is turned off: once the deadline has passed, the session starts over, without one, and `store.GetByID` no longer finds it
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
- A store with a `Codec` needs no registration, and a read never makes the save encode again; see [Codecs and Typed Reads](#codecs-and-typed-reads)

### Codecs and Typed Reads

By default a session is stored as one gob stream: the whole session is decoded when a request loads it, a custom type must be registered with `RegisterType`, and every change encodes the whole session again. A `Codec` stores each value on its own instead:

```go
store := session.NewStore(session.Config{
    Storage: redisStorage,
    Codec:   session.JSONCodec{},
})
app.Use(session.New(session.Config{Store: store}))

app.Get("/me", func(c fiber.Ctx) error {
    sess := session.FromContext(c)

    var user User // no RegisterType
    if err := sess.GetInto("user", &user); err != nil {
        return c.SendStatus(fiber.StatusUnauthorized) // errors.Is(err, session.ErrKeyNotFound) when there is none
    }
    visits, _ := session.GetAs[int](sess, "visits")
    sess.Set("visits", visits+1)

    return c.JSON(user)
})
```

`session.JSONCodec{}` uses `encoding/json`. For another format, such as msgpack or CBOR, give `NewCodec` the functions of the library you use, the way you plug a library into the `MsgPackEncoder` and `CBOREncoder` of the app. The name is written into every record:

```go
codec := session.NewCodec("msgpack", msgpack.Marshal, msgpack.Unmarshal)
```

Any type that has the three methods of `session.Codec` works too: `Name() string`, `Marshal(v any) ([]byte, error)` and `Unmarshal(data []byte, v any) error`. A codec is called from many requests at once. Change its `Name()` when you change the options it runs with, since a store reads the records of the codec of that name. `Marshal` runs while `Set` on the middleware holds the lock of the middleware, so it must not use the session. `Unmarshal` gets a copy of the encoded value, which it may keep, and runs without a lock.

What a request pays for its session, for a signed-in user with five values, one of them a struct, on one CPU of the machine the benchmarks in `codec_benchmark_test.go` ran on:

| Request                  | gob                           | `JSONCodec`                 |
|--------------------------|-------------------------------|-----------------------------|
| reads one value          | 280 allocations, 12.9 KB      | 24 allocations, 1.2 KB      |
| reads all five values    | 319 allocations, 15.8 KB      | 52 allocations, 2.6 KB      |
| changes one value        | 319 allocations, 15.8 KB      | 26 allocations, 1.5 KB      |

The allocations are exact; the time follows them, about 30 µs against 2.4 µs for the first row on that machine. Most of what gob costs is building a decoder for the stream, which every request pays. With a codec, a request that only reads a session writes the record back as it was, whatever it read; without one, the save encodes the session again after a read of a struct, a map or a slice.

#### What changes with a codec

|                      | Without a codec                                 | With a codec                                                                                 |
|----------------------|-------------------------------------------------|----------------------------------------------------------------------------------------------|
| Stored as            | one gob stream                                  | one record, each value encoded on its own                                                    |
| Custom types         | registered with `RegisterType`                  | no registration                                                                              |
| Keys                 | any comparable value                            | strings. A `Set` with another key fails the save; a `Get` of one finds nothing               |
| `Set`                | keeps the value, encoded when it is saved       | encodes the value now: changing it afterwards is not stored                                  |
| `Get`                | the value that was stored                       | what the codec decodes into an `any`; see below                                              |
| `GetInto`, `GetAs`   | assign, or assert, the stored value             | decode only that value: `GetInto` into your variable, `GetAs` into a new value               |
| A read               | of a struct, map or slice makes the save encode again | never changes the session                                                              |
| `Keys`               | all keys (with `AbsoluteTimeout`, also an internal one) | the keys you set                                                                     |

- **`GetInto` and `GetAs` have the same signatures in both modes**, so code that uses them compiles with either, but they do not always give the same result, and they check different things. Without a codec they check the Go type of what was stored. With one they decode what was stored into the type you ask for, which works for every type whose fields fit: a `User` stored and read as an `Admin` with the same fields decodes, an `int` reads as a `float64` or an `int64`, and an object none of whose fields fit decodes to a zero value without an error. Do not use the Go type of a stored value to tell kinds of values apart, in an authorization check for one: store a field that says what it is and check that. An interface type such as `fmt.Stringer` works as a target without a codec and fails with one. `GetInto(key, &target)` returns `ErrKeyNotFound` for a missing key and leaves the target alone when it fails; with a codec it replaces what the target held, it does not merge into it, and a target that is still empty is decoded into directly, which copies nothing. `GetAs[V]` reports `false` instead of an error. A stored `nil` (JSON `null`) reads as the zero value, and `GetAs` says `true` for it with a codec and `false` without. `GetAs` takes a `*session.Session` or a `*session.Middleware` and nothing else compiles: a type that embeds one passes the one it embeds.
- **`Get` returns the generic form of the value**, and it differs between codecs: `JSONCodec` gives `float64` for every number, `string` for a `[]byte` or a `time.Time`, `[]any` and `map[string]any`; a msgpack library gives integers of other widths. A number above 2^53 does not survive as a `float64`, and a map you change after `Get` is not stored. Use `GetInto` or `GetAs` for anything but a string or a flag. A value that does not decode is returned as its encoded bytes, so that it is never taken for a missing key.
- **A codec keeps what its format keeps.** `JSONCodec` replaces the bytes of a string that are not valid UTF-8 with the Unicode replacement character, cannot encode `NaN` or `±Inf` (the save fails), gives back a number held by an `any` field as a `float64`, and leaves out the fields that are not exported, as gob does. A round trip of a value through the codec in a test shows what it loses before you store it.
- **A value that cannot be encoded**, or a key that is not a string, is remembered with its key, and `Save` fails with it while it is not made up for by another `Set` or a `Delete` of the key, as it does for gob until the value is replaced. The key keeps the value it had until then, which `Get` still returns. A new session whose `Set` failed is not "empty": `SkipEmptySessions` does not skip it.
- **Every `Get`, `GetAs` and `GetInto` decodes again**; read a value once and keep it. A variable that you reuse for `GetInto` is safe: the old value is gone, not merged.

#### When gob is the better choice

A codec decodes a value every time it is read and encodes it every time it is set, where gob decodes the whole session once per request and encodes it once per save. For a session of a few small values, of which a request reads or changes a few, that is much less work (the table above), and the record is about half the size of the gob stream. It is more work when a request:

- **reads the same value again and again.** A request that reads the same struct 20 times takes about 2.5 times as long as with gob, and one that reads it 100 times about 13 times. Read it once and keep it.
- **sets the same key again and again.** A request that sets the same struct 1000 times takes about 10 times as long as with gob, which encodes the last value once.
- **handles large strings, `[]byte` or arrays of numbers.** `JSONCodec` takes about 5 times as long to read a string or `[]byte` of 100 KiB, 16 times when the string is full of characters that JSON escapes, and about 3 times for 1000 floats, whose record is also about 1.8 times as large. Keep such data out of the session, or use a store without a codec for it.

These are rough figures of `JSONCodec`, measured on one machine with benchmarks that are not part of the repository, and a codec of another library will differ. Measure with a session of the size of yours: `Benchmark_Codec_Request` in `codec_benchmark_test.go` is a start.

#### Turning it on, and off

- **Sessions that exist stay as they are.** A session that was stored by a store without a codec is read as before, with typed values from `Get`, and is written as gob until it ends or is reset: `Reset` and `Destroy` start it over as a session of the store, and so does a visitor who comes without one. Nothing is converted, since a conversion cannot keep every value: a string with bytes that are not UTF-8, a field that JSON hides, a key that is not a string. A session that stays in use, or that is only rotated with `Regenerate`, stays gob until it ends, so read values with `GetAs` and `GetInto` meanwhile, which work for both, and keep `RegisterType` for the custom types of sessions that exist: a session stored as gob is decoded with gob, and a type that is not registered fails to load with a `500`.
- **A store that cannot read a record treats it as a session that does not exist.** A store without a codec meets a record when the option was turned off again or has not reached every instance yet; a store meets a record of another codec after the codec was changed. The visitor gets a new session, which is a logout and not an outage, the record is left alone for the instance that can read it, and one warning is logged per store. If the warning keeps coming, applications with different codecs share a storage and a session cookie.
- **Roll out in two steps.** A release that does not have this feature cannot read a record, and answers every request that carries one with a `500`, without a new cookie, until the record's time to live has passed: a rolling deployment that still runs such instances, or a rollback of the release and not only of the option, locks those visitors out. Deploy a release that has the feature with the option off everywhere first, and turn the option on after that. Then turn it on for every instance at once, or give the new instances their own `StorageKeyPrefix` or cookie name: while instances with and without the option run together, a visitor whose request lands on one that cannot read the session is logged out, and a record that has been written is not damaged by it.
- **`csrf` cannot use a session store with a codec**: it keeps its token under a key that is not a string. `csrf.New` panics when `csrf.Config.Session` is such a store, and a session middleware with a codec in front of `csrf` fails the save of every request that sets a token. Keep the token in `csrf.Config.Storage`, or give `csrf` and the session middleware a store without a codec.
- A record that is damaged is an error for a store with a codec, with the cause in the message. A store without a codec cannot tell it from a record it cannot read, and treats it as a session that does not exist. `store.GetByID` returns `ErrSessionIDNotFoundInStore` for a record its store cannot read.

#### The record

A record is `00 'F' 'S' 'R'`, a version, the name of the codec, the absolute deadline if there is one, and the keys in ascending order, each followed by its encoded value. Writing the same session twice gives the same bytes. A record is told from a gob stream by its first byte, which a gob stream never has. The deadline is part of the record, not one of the keys.

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
func GetAs[V any, S *Session | *Middleware](src S, key any) (V, bool)
func NewCodec(name string, marshal func(v any) ([]byte, error), unmarshal func(data []byte, v any) error) Codec
func DefaultErrorHandler(c fiber.Ctx, err error)
```

`NewWithStore` returns the middleware together with the `Store` it uses, for example to call `RegisterType`. `FromContext` returns `nil` when the request has no session middleware. With several session middleware it returns the innermost one, the last to run; see [Multiple Sessions per Request](#multiple-sessions-per-request).

`GetAs` reads a value as a given type without a type assertion: `ok` is `false` when the key is absent or holds a value of another type, and the zero value is returned with it. `src` is a `*session.Middleware` or a `*session.Session`, and nothing else compiles. A `nil` one finds nothing, so `session.GetAs[int](session.FromContext(c), "visits")` is safe even for a request without the session middleware. With a [codec](#codecs-and-typed-reads) the value is decoded into a `V`. `GetAs` returns its value, so a struct of a kilobyte or more is copied more than once on the way out; `GetInto` fills a variable of yours and copies it once. A pointer in the session, without a codec, copies nothing but the pointer.

`Codec` is the interface of a format for session values, `JSONCodec` is the one of `encoding/json`, and `NewCodec` makes one of a name and the two functions of a library. See [Codecs and Typed Reads](#codecs-and-typed-reads).

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

// Typed read into a variable of yours: ErrKeyNotFound when there is no such key
var user User
err := sess.GetInto("user", &user)

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

The returned `*session.Middleware` is only valid while the request is being handled. A context you kept from `c.Context()` (with `PassLocalsToContext` enabled) stops resolving it when the request ends, except for a handler the [Timeout](./timeout.md) middleware gave up on (below): `FromContext` returns `nil` for that context from then on, so a goroutine that outlives the handler cannot reach the session, and never another request's. Do not hand the pointer itself to such a goroutine. Pass it the session ID and use the [Store Pattern](#store-pattern-advanced) instead.

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
    ErrKeyNotFound                      = errors.New("session key not found")
)
```

- `store.Get(c)` returns `ErrSessionAlreadyLoadedByMiddleware` when the session middleware already loaded the session for the request. Use `session.FromContext(c)` there.
- `store.GetByID` and `store.Delete` return `ErrEmptySessionID` for an empty session ID.
- `store.GetByID` returns `ErrSessionIDNotFoundInStore` when the storage has no such session, or when the session has outlived the absolute deadline it was stored with, whatever the store's `AbsoluteTimeout` is now (it is deleted), or when it is a record that the store cannot read (see [Turning it on, and off](#turning-it-on-and-off)).
- `sess.GetInto` returns `ErrKeyNotFound`, wrapped with the key, when the session holds nothing under it.

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
| `Store`             | `*session.Store`            | Pre-built session store (use when you need to share/register types). The store's own options (including `Codec`) decide how sessions are stored and sent; this config supplies only `Next`, `ErrorHandler` and `SkipEmptySessions` | `nil` (auto-created)                       |
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
| `Codec`             | `session.Codec`             | Store each value encoded on its own, and read them into your types, instead of one gob stream. See [Codecs and Typed Reads](#codecs-and-typed-reads) | `nil` (gob) |
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
