---
id: csrf
---

# CSRF

The CSRF middleware provides protection against [Cross-Site Request Forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) attacks. It validates tokens on unsafe HTTP methods (POST, PUT, DELETE, etc.) and returns 403 Forbidden if an attack is detected.

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
| Extractor         | `func(fiber.Ctx) (string, error)`  | Token extraction method                                                                                                       | `FromHeader("X-Csrf-Token")` |
| Session           | `*session.Store`                   | Session store (**recommended for production**)                                                                                | `nil`                        |
| Storage           | `fiber.Storage`                    | Token storage (overridden by Session)                                                                                         | `nil`                        |
| TrustedOrigins    | `[]string`                         | Trusted origins for cross-origin requests                                                                                     | `[]`                         |
| SingleUseToken    | `bool`                             | Generate new token after each use                                                                                             | `false`                      |

## API Reference

```go
// Create middleware
func New(config ...Config) fiber.Handler

// Get token from context
func TokenFromContext(c fiber.Ctx) string

// Get handler from context
func HandlerFromContext(c fiber.Ctx) *Handler

// Delete token
func (h *Handler) DeleteToken(c fiber.Ctx) error
```

## Constants

```go
const (
    HeaderName = "X-Csrf-Token"
)
```

## Error Types

```go
var (
    ErrTokenNotFound   = errors.New("csrf token not found")
    ErrTokenInvalid    = errors.New("csrf token invalid")
    ErrRefererNotFound = errors.New("referer not supplied")
    ErrRefererInvalid  = errors.New("referer invalid")
    ErrRefererNoMatch  = errors.New("referer does not match host")
    ErrOriginInvalid   = errors.New("origin invalid")
    ErrOriginNoMatch   = errors.New("origin does not match host")
)
```

## Token Extractors

### Built-in Extractors

**Secure (Recommended):**
- `csrf.FromHeader("X-Csrf-Token")` - Most secure, preferred for APIs
- `csrf.FromForm("_csrf")` - Secure for form submissions

**Acceptable:**
- `csrf.FromQuery("csrf_token")` - URL parameters
- `csrf.FromParam("csrf")` - Route parameters

**Avoid:**
- `csrf.FromCookie("csrf_token")` - Vulnerable to subdomain attacks

### Custom Extractor
You can create a custom extractor to handle specific cases:

#### Smart Content-Type Extractor

```go
func SmartExtractor(c fiber.Ctx) (string, error) {
    contentType := c.Get("Content-Type")
    
    // JSON/API requests use header
    if strings.Contains(contentType, "application/json") {
        return csrf.FromHeader("X-Csrf-Token")(c)
    }
    
    // Form requests use form field
    if strings.Contains(contentType, "application/x-www-form-urlencoded") ||
       strings.Contains(contentType, "multipart/form-data") {
        return csrf.FromForm("_csrf")(c)
    }
    
    // Default to header
    return csrf.FromHeader("X-Csrf-Token")(c)
}
```

#### Bearer Token Embedding

```go
func BearerTokenExtractor(c fiber.Ctx) (string, error) {
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
}
```

## Security Patterns

### Double Submit Cookie (Default)
- Stores tokens in memory/database
- Compares cookie value with submitted token
- No session required

### Synchronizer Token (with Session)
- Stores tokens in user session
- More secure, prevents login CSRF
- Requires session middleware

```go
// Enable synchronizer pattern
app.Use(csrf.New(csrf.Config{
    Session: sessionStore,
}))
```

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
    handler.DeleteToken(c)
}

// With session middleware
session.Destroy()  // Also deletes CSRF token
```

## Security Features

- **Referer checking** for HTTPS requests
- **Origin validation** for cross-origin requests  
- **Token expiration** with configurable timeout
- **Single-use tokens** for maximum security
- **BREACH attack protection** recommendations (see note below)

:::note BREACH Protection
To mitigate BREACH attacks, ensure your pages are served over HTTPS, disable HTTP compression, and implement rate limiting for requests. The CSRF token is sent as a header on every request, so if you include the token in a page that is vulnerable to BREACH, an attacker may be able to extract the token.
:::

## Best Practices

1. **Always use HTTPS** in production
2. **Use sessions** for authenticated applications
3. **Set `CookieSecure: true`** and appropriate SameSite values
4. **Implement XSS protection** alongside CSRF
5. **Regenerate tokens** after auth changes
6. **Use `__Host-` cookie prefix** when possible

:::danger Production Requirements
- `CookieSecure: true` (HTTPS only)
- `CookieSameSite: "Lax"` or `"Strict"`
- Use `Session` store for better security
:::