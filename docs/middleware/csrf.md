---
id: csrf
---

# CSRF

The CSRF middleware provides protection against [Cross-Site Request Forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) attacks. It validates tokens on unsafe HTTP methods (POST, PUT, DELETE, etc.) and returns 403 Forbidden if an attack is detected.

## Table of Contents

- [Quick Start](#quick-start)
- [Best Practices & Production Requirements](#best-practices--production-requirements)
- [Configuration by Application Type](#configuration-by-application-type)
- [Recipes for Common Use Cases](#recipes-for-common-use-cases)
- [Using CSRF Tokens](#using-csrf-tokens)
- [Security Model](#security-model)
- [Token Extractors](#token-extractors)
- [Advanced Configuration](#advanced-configuration)
- [API Reference](#api-reference)
- [Config Properties](#config-properties)
- [Error Types](#error-types)
- [Constants](#constants)

## Quick Start

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/csrf"
)

// Default config (development only)
app.Use(csrf.New())

// Production config
app.Use(csrf.New(csrf.Config{
    CookieName:        "__Host-csrf_",
    CookieSecure:      true,
    CookieHTTPOnly:    true,  // false for SPAs
    CookieSameSite:    "Lax",
    CookieSessionOnly: true,
    Extractor:         csrf.FromHeader("X-Csrf-Token"),
    Session:           sessionStore,
}))
```

## Best Practices & Production Requirements

:::danger Production Requirements

- `CookieSecure: true` (HTTPS only)
- `CookieSameSite: "Lax"` or `"Strict"`
- Use `Session` store for better security

:::

1. **Always use HTTPS** in production
2. **Use sessions** for authenticated applications
3. **Set `CookieSecure: true`** and appropriate SameSite values
4. **Implement XSS protection** alongside CSRF
5. **Regenerate tokens** after auth changes
6. **Use `__Host-` cookie prefix** when possible

:::warning BREACH Protection
To mitigate BREACH attacks, ensure your pages are served over HTTPS, disable HTTP compression, and implement rate limiting for requests. The CSRF token is sent as a header on every request, so if you include the token in a page that is vulnerable to BREACH, an attacker may be able to extract the token.
:::

## Configuration by Application Type

### Server-Side Rendered Apps

```go
app.Use(csrf.New(csrf.Config{
    CookieName:        "__Host-csrf_",
    CookieSecure:      true,
    CookieHTTPOnly:    true,        // Secure - blocks JavaScript
    CookieSameSite:    "Lax",
    CookieSessionOnly: true,
    Extractor:         csrf.FromForm("_csrf"),
    Session:           sessionStore,
}))
```

### Single Page Applications (SPAs)

```go
app.Use(csrf.New(csrf.Config{
    CookieName:        "__Host-csrf_",
    CookieSecure:      true,
    CookieHTTPOnly:    false,       // Required for JavaScript access to tokens
    CookieSameSite:    "Lax",
    CookieSessionOnly: true,
    Extractor:         csrf.FromHeader("X-Csrf-Token"),
    Session:           sessionStore,
}))
```

:::warning SPA Security Trade-off
SPAs require `CookieHTTPOnly: false` to access tokens via JavaScript. This slightly increases XSS risk but is necessary for SPA functionality.
:::

## Recipes for Common Use Cases

- **Without Sessions**: [CSRF Recipe](https://github.com/gofiber/recipes/tree/master/csrf) - Simple Double Submit Cookie pattern
- **With Sessions**: [CSRF with Session Recipe](https://github.com/gofiber/recipes/tree/master/csrf-with-session) - More secure Synchronizer Token pattern

## Using CSRF Tokens

### Server-Side Forms

```go
func formHandler(c fiber.Ctx) error {
    token := csrf.TokenFromContext(c)
    
    return c.SendString(fmt.Sprintf(`
        <form method="POST" action="/submit">
            <input type="hidden" name="_csrf" value="%s">
            <input type="text" name="message" required>
            <button type="submit">Submit</button>
        </form>
    `, token))
}
```

### Single Page Applications

```go
func apiHandler(c fiber.Ctx) error {
    token := csrf.TokenFromContext(c)
    
    return c.JSON(fiber.Map{
        "csrf_token": token,
        "data":       "your data",
    })
}
```

```javascript
// Get CSRF token from cookie
function getCsrfToken() {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; __Host-csrf_=`);
    if (parts.length === 2) return parts.pop().split(';').shift();
}

// Use with fetch API
async function makeRequest(url, data) {
    const csrfToken = getCsrfToken();
    
    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-Csrf-Token': csrfToken
        },
        body: JSON.stringify(data)
    });
    
    if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    
    return response.json();
}
```

## Security Model

The middleware employs a robust, defense-in-depth strategy to protect against CSRF attacks. The primary defense is token-based validation, which operates in one of two modes depending on your configuration. This is supplemented by a mandatory secondary check on the request's origin.

### 1. Token Validation Patterns

#### Double Submit Cookie (Default Mode)

This is the default pattern, used when a `Session` store is **not** configured. It is a "semi-stateless" approach; while it doesn't tie tokens to a specific user session, the server still maintains a record of all validly issued tokens.

- **How it Works:**
  1. On a user's first visit (or a safe request like `GET`), the middleware generates a unique token.
  2. This token is sent to the client in a `Set-Cookie` header.
  3. A record of this token is also kept on the server (in-memory by default, or in your configured `Storage`). This proves the token was generated by the server and is not expired, but does not link it to a specific user.
  4. For any subsequent unsafe request (e.g., `POST`, `PUT`), the client application must read the token from the cookie and send it back in a different location, such as the `X-Csrf-Token` header.

- **Validation:** The middleware validates three things: that the token from the header/form **exactly matches** the token from the cookie, that the token **exists** in the server-side storage, and that it **has not expired**.
- **Why it's Secure:** An attacker on a malicious domain cannot read the victim's cookie to forge a matching header. Furthermore, they cannot invent a token, because it wouldn't exist in the server's storage registry.

#### Synchronizer Token (Session-Based Mode)

This is a more secure, stateful pattern that is **automatically enabled** when you provide a `Session` store in the configuration.

- **How it Works:**
  1. A unique token is generated and stored directly within the user's session data on the server.
  2. The token is also sent to the client as a cookie.
  3. For unsafe requests, the client sends the token back in a header or form field.

- **Validation:** The middleware performs a multi-step validation:
  1. It first performs the standard **Double Submit Cookie check**: the token from the header/form must exactly match the token from the cookie. This is a fast and efficient first line of defense, and there is little benefit of skipping it.
  2. It then validates that this token exists and is valid within the user's **server-side session**. This is the authoritative check that ties the token to the authenticated user.

- **Why it's More Secure:** Tying the token to the server-side session provides the strongest CSRF protection, as the token is then guaranteed to have been generated for the specific user. While browsers handle sending the required cookie automatically, it's important to note that custom API clients must also remember to send the cookie with their requests for validation to succeed.

```go
// Enable the more secure Synchronizer Token pattern
app.Use(csrf.New(csrf.Config{
    Session: sessionStore, // Providing a session store activates this mode
}))
```

### 2. Origin & Referer Validation

As a crucial second layer of defense, the middleware **always** performs `Origin` and `Referer` header checks for unsafe requests (when the connection is HTTPS).

- The request's `Origin` (for cross-origin requests) or `Referer` (for same-origin requests) header **must** match the application's `Host` header or be explicitly allowed in the `TrustedOrigins` list.
- This check is performed *in addition* to token validation and provides strong protection because these headers are reliably set by browsers and cannot be programmatically controlled by an attacker from a malicious site.

## Token Extractors

### Built-in Extractors

**Most Secure (Recommended):**

- `csrf.FromHeader("X-Csrf-Token")` - Headers are not logged and cannot be manipulated via URL
- `csrf.FromForm("_csrf")` - Form data is secure and not typically logged

**Less Secure (Use with caution):**

- `csrf.FromQuery("csrf_token")` - URLs may be logged by servers, proxies, browsers
- `csrf.FromParam("csrf")` - URLs may be logged by servers, proxies, browsers

**Advanced:**

- `csrf.Chain(...)` - Try multiple extractors in sequence

:::note What about cookies?
**Cookies are generally not a secure source for CSRF tokens.** The middleware does not provide a built-in cookie extractor because reading the CSRF token from a cookie with the same name as the CSRF cookie defeats CSRF protection.

**Advanced usage:**  
In rare cases, you may securely extract a CSRF token from a cookie if:

- You read from a different cookie (not the CSRF cookie itself)
- You use multiple cookies for custom validation
- You implement custom logic across different cookie sources

If you do this, set the extractor’s `Source` to `SourceCookie` and allow the middleware to check that the cookie name is different from your CSRF cookie. It will panic if this is the case.

**Warning:**  
We strongly discourage cookie-based extraction, as it is easy to misconfigure and creates security risks. Prefer extracting tokens from headers or form fields for robust CSRF protection.
:::

### Extractor Metadata

Each extractor returns an `Extractor` struct with metadata about its behavior:

```go
extractor := csrf.FromHeader("X-Csrf-Token")
fmt.Printf("Source: %v, Key: %s", extractor.Source, extractor.Key)
// Output: Source: 0, Key: X-Csrf-Token

// Available source types:
// - csrf.SourceHeader (0): Most secure, not logged
// - csrf.SourceForm (1): Secure, not typically logged  
// - csrf.SourceQuery (2): Less secure, URLs may be logged
// - csrf.SourceParam (3): Less secure, URLs may be logged
// - csrf.SourceCookie (4): Not recommended for CSRF, no built-in extractor for this source
// - csrf.SourceCustom (5): Security depends on implementation

// Check source type
if extractor.Source == csrf.SourceHeader {
    fmt.Println("Using secure header extraction")
}
```

#### Using Route-Specific Extractors

There are cases where you might want to use different extractors for different routes:

```go
// API routes - header only
api := app.Group("/api")
api.Use(csrf.New(csrf.Config{
    Extractor: csrf.FromHeader("X-Csrf-Token"),
}))

// Form routes - form only  
forms := app.Group("/forms")
forms.Use(csrf.New(csrf.Config{
    Extractor: csrf.FromForm("_csrf"),
}))
```

### Custom Extractor

You can create a custom extractor to handle specific cases by creating an `Extractor` struct:

:::danger Never Extract from Cookies
**NEVER create custom extractors that read from cookies using the same `CookieName` as your CSRF configuration.** This completely defeats CSRF protection by making the extracted token always match the cookie value, allowing any CSRF attack to succeed.

```go
// ❌ NEVER DO THIS - Completely defeats CSRF protection
badExtractor := csrf.Extractor{
    Extract: func(c fiber.Ctx) (string, error) {
        return c.Cookies("csrf_"), nil  // Always passes validation!
    },
    Source: csrf.SourceCustom,
    Key:    "csrf_",
}

// ✅ DO THIS - Extract from different source than cookie
app.Use(csrf.New(csrf.Config{
    CookieName: "csrf_",
    Extractor: csrf.FromHeader("X-Csrf-Token"), // Header vs cookie comparison
}))
```

The middleware uses the **Double Submit Cookie** pattern - it compares the extracted token against the cookie value. If your extractor reads from the same cookie, they will always match and provide zero CSRF protection.
:::

#### Bearer Token Embedding

```go
// Extract CSRF token embedded in JWT Authorization header
// Useful for APIs that combine JWT auth with CSRF protection
func BearerTokenExtractor() csrf.Extractor {
    return csrf.Extractor{
        Extract: func(c fiber.Ctx) (string, error) {
            // Extract from "Authorization: Bearer <jwt>:<csrf>"
            auth := c.Get("Authorization")
            if !strings.HasPrefix(auth, "Bearer ") {
                return "", csrf.ErrTokenNotFound
            }
            
            parts := strings.SplitN(strings.TrimPrefix(auth, "Bearer "), ":", 2)
            if len(parts) != 2 || parts[1] == "" {
                return "", csrf.ErrTokenNotFound
            }
            
            return parts[1], nil
        },
        Source: csrf.SourceCustom,
        Key:    "Authorization",
    }
}

// Usage
app.Use(csrf.New(csrf.Config{
    Extractor: BearerTokenExtractor(),
}))
```

#### Custom JSON Body Extractor

```go
// Extract CSRF token from JSON request body
// Useful for APIs that need token in request payload
func JSONBodyExtractor(field string) csrf.Extractor {
    return csrf.Extractor{
        Extract: func(c fiber.Ctx) (string, error) {
            var body map[string]interface{}
            if err := c.BodyParser(&body); err != nil {
                return "", csrf.ErrTokenNotFound
            }
            
            token, ok := body[field].(string)
            if !ok || token == "" {
                return "", csrf.ErrTokenNotFound
            }
            
            return token, nil
        },
        Source: csrf.SourceCustom,
        Key:    field,
    }
}

// Usage
app.Use(csrf.New(csrf.Config{
    Extractor: JSONBodyExtractor("csrf_token"),
}))
```

### Chain Extractor (Advanced)

For specific cases requiring fallback behavior:

```go
// Try header first, fallback to form
app.Use(csrf.New(csrf.Config{
    Extractor: csrf.Chain(
        csrf.FromHeader("X-Csrf-Token"),
        csrf.FromForm("_csrf"),
    ),
}))

// Check chain metadata
chained := csrf.Chain(
    csrf.FromHeader("X-Csrf-Token"),
    csrf.FromForm("_csrf"),
)
fmt.Printf("Primary source: %v, Chain length: %d", chained.Source, len(chained.Chain))
// Output: Primary source: 0, Chain length: 2
```

:::danger Security Risk
Chaining extractors increases attack surface and complexity. Most applications should use a single, appropriate extractor for their use case.
:::

## Advanced Configuration

### Trusted Origins

```go
app.Use(csrf.New(csrf.Config{
    TrustedOrigins: []string{
        "https://trusted.example.com",
        "https://*.example.com", // Wildcard subdomains
    },
}))
```

### Custom Error Handler

```go
app.Use(csrf.New(csrf.Config{
    ErrorHandler: func(c fiber.Ctx, err error) error {
        accepts := c.Accepts("html", "json")
        path := c.Path()
        if accepts == "json" || strings.HasPrefix(path, "/api/") {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
                "error": "Forbidden",
            })
        }
        return c.Status(fiber.StatusForbidden).Render("error", fiber.Map{
            "Title": "Forbidden",
            "Status": fiber.StatusForbidden,
        }, "layouts/main")
    },
}))
```

### Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3
app.Use(csrf.New(csrf.Config{
    Storage: storage,
}))
```

### Token Management

```go
// Delete token (e.g., on logout)
handler := csrf.HandlerFromContext(c)
if handler != nil {
    if err := handler.DeleteToken(c); err != nil {
        // handle error, e.g. log it
    }
}

// With session middleware
session.Destroy()  // Also deletes CSRF token
```

## API Reference

```go
// Create middleware
func New(config ...csrf.Config) fiber.Handler

// Get token from context
func TokenFromContext(c fiber.Ctx) string

// Get handler from context
func HandlerFromContext(c fiber.Ctx) *csrf.Handler

// Delete token
func (h *csrf.Handler) DeleteToken(c fiber.Ctx) error
```

## Config Properties

| Property          | Type                               | Description                                                                                                                   | Default                      |
|:------------------|:-----------------------------------|:------------------------------------------------------------------------------------------------------------------------------|:-----------------------------|
| Next              | `func(fiber.Ctx) bool`             | Skip middleware when returns true                                                                                             | `nil`                        |
| CookieName        | `string`                           | CSRF cookie name                                                                                                              | `"csrf_"`                    |
| CookieDomain      | `string`                           | CSRF cookie domain                                                                                                            | `""`                         |
| CookiePath        | `string`                           | CSRF cookie path                                                                                                              | `""`                         |
| CookieSecure      | `bool`                             | HTTPS only cookie (**required for production**)                                                                               | `false`                      |
| CookieHTTPOnly    | `bool`                             | Prevent JavaScript access (**use `false` for SPAs**)                                                                          | `false`                      |
| CookieSameSite    | `string`                           | SameSite attribute (**use "Lax" or "Strict"**)                                                                                | `"Lax"`                      |
| CookieSessionOnly | `bool`                             | Session-only cookie (expires on browser close)                                                                                | `false`                      |
| IdleTimeout       | `time.Duration`                    | Token expiration time                                                                                                         | `30 * time.Minute`           |
| KeyGenerator      | `func() string`                    | Token generation function                                                                                                     | `utils.UUIDv4`               |
| ErrorHandler      | `fiber.ErrorHandler`               | Custom error handler                                                                                                          | `defaultErrorHandler`        |
| Extractor         | `csrf.Extractor`                   | Token extraction method with metadata                                                                                         | `FromHeader("X-Csrf-Token")` |
| Session           | `*session.Store`                   | Session store (**recommended for production**)                                                                                | `nil`                        |
| Storage           | `fiber.Storage`                    | Token storage (overridden by Session)                                                                                         | `nil`                        |
| TrustedOrigins    | `[]string`                         | Trusted origins for cross-origin requests                                                                                     | `[]`                         |
| SingleUseToken    | `bool`                             | Generate new token after each use                                                                                             | `false`                      |

## Error Types

```go
var (
    ErrTokenNotFound   = errors.New("csrf: token not found")
    ErrTokenInvalid    = errors.New("csrf: token invalid")
    ErrRefererNotFound = errors.New("csrf: referer header missing")
    ErrRefererInvalid  = errors.New("csrf: referer header invalid")
    ErrRefererNoMatch  = errors.New("csrf: referer does not match host or trusted origins")
    ErrOriginInvalid   = errors.New("csrf: origin header invalid")
    ErrOriginNoMatch   = errors.New("csrf: origin does not match host or trusted origins")
)
```

## Constants

```go
const (
    HeaderName = "X-Csrf-Token"
)

// Source types for extractor metadata
const (
    SourceHeader Source = iota  // 0 - Most secure
    SourceForm                  // 1 - Secure
    SourceQuery                 // 2 - Less secure
    SourceParam                 // 3 - Less secure  
    SourceCookie                // 4 - Not recommended for CSRF, no built-in extractor for this source
    SourceCustom                // 5 - Security depends on implementation
)
```
