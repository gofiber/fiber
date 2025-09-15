---
id: csrf
---

# CSRF

The CSRF middleware protects against [Cross-Site Request Forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) attacks by validating tokens on unsafe HTTP methods such as POST, PUT, and DELETE. It responds with 403 Forbidden when validation fails.

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
    "github.com/gofiber/fiber/v3/extractors"
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
    Extractor:         extractors.FromHeader("X-Csrf-Token"),
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
    Extractor:         extractors.FromForm("_csrf"),
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
    Extractor:         extractors.FromHeader("X-Csrf-Token"),
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
  3. The server also stores this token (in memory by default or in the configured `Storage`). It confirms the token is server-generated and still valid, but it is not tied to a specific user.
  4. For subsequent unsafe requests (e.g., `POST`, `PUT`), the client must read the token from the cookie and echo it in a different location, such as the `X-Csrf-Token` header.

- **Validation:** The middleware validates three things: that the token from the header/form **exactly matches** the token from the cookie, that the token **exists** in the server-side storage, and that it **has not expired**.
- **Why it is secure:** Attackers on a malicious domain cannot read the victim's cookie to forge a matching header. They also cannot invent a token because it wouldn't exist in the server's storage registry.

#### Synchronizer Token (Session-Based Mode)

This is a more secure, stateful pattern that is **automatically enabled** when you provide a `Session` store in the configuration.

- **How it Works:**
  1. A unique token is generated and stored directly within the user's session data on the server.
  2. The token is also sent to the client as a cookie.
  3. For unsafe requests, the client sends the token back in a header or form field.

- **Validation:** The middleware performs a multi-step validation:
  1. It first performs the standard **Double Submit Cookie check**: the token from the header/form must exactly match the token from the cookie. This is a fast and efficient first line of defense, and there is little benefit of skipping it.
  2. It then validates that this token exists and is valid within the user's **server-side session**. This is the authoritative check that ties the token to the authenticated user.

- **Why it is more secure:** Tying the token to the server-side session provides the strongest CSRF protection, as the token is then guaranteed to have been generated for the specific user. While browsers automatically send the required cookie, custom API clients must remember to include the cookie with their requests for validation to succeed.

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

This middleware uses the shared `extractors` package for token extraction. For full details on extractor types, chaining, security, and advanced usage, see the [Extractors Guide](https://docs.gofiber.io/guide/extractors).

**Extractor Source Constants:**
Extractor source constants (such as `SourceHeader`, `SourceForm`, etc.) are defined in the shared extractors package, not in the CSRF middleware itself. Refer to the Extractors Guide for their definitions and usage.

### CSRF-Specific Extractor Notes

For CSRF protection, prefer secure extraction methods:

- **Headers** (`extractors.FromHeader("X-Csrf-Token")`) – Most secure, not logged in URLs
- **Form data** (`extractors.FromForm("_csrf")`) – Secure for form submissions
- **Avoid URL parameters** – Query/param extractors expose tokens in logs and browser history

:::note What about cookies?
**Cookies are generally not a secure source for CSRF tokens.** The middleware will panic if you configure an extractor that reads from cookies with the same name as your CSRF cookie. This is because reading the CSRF token from a cookie with the same name as the CSRF cookie defeats CSRF protection entirely, as the extracted token will always match the cookie value, allowing any CSRF attack to succeed.

**Advanced usage:**
In rare cases, you may securely extract a CSRF token from a cookie if:

- You read from a different cookie (not the CSRF cookie itself)
- You use multiple cookies for custom validation
- You implement custom logic across different cookie sources

If you do this, set the extractor’s `Source` to `SourceCookie` and allow the middleware to check that the cookie name is different from your CSRF cookie. It will panic if this is the case.

**Warning:**
Cookie-based extraction is strongly discouraged, as it is easy to misconfigure and creates security risks. Prefer extracting tokens from headers or form fields for robust CSRF protection. See the [Extractors Guide](https://docs.gofiber.io/guide/extractors#security-considerations) for more details.
:::

### Route-Specific Configuration

You can configure different extraction methods for different routes:

```go
// API routes - header extraction for AJAX/fetch requests
api := app.Group("/api")
api.Use(csrf.New(csrf.Config{
    Extractor: extractors.FromHeader("X-Csrf-Token"),
}))

// Form routes - form field extraction for traditional forms
forms := app.Group("/forms")
forms.Use(csrf.New(csrf.Config{
    Extractor: extractors.FromForm("_csrf"),
}))
```


### Custom CSRF Extractors

For specialized CSRF token extraction needs, you can create custom extractors. See the [Extractors Guide](https://docs.gofiber.io/guide/extractors#custom-extractors) for advanced patterns and security notes.

:::danger Never Extract from Cookies
**NEVER create custom extractors that read from cookies using the same `CookieName` as your CSRF configuration.** This completely defeats CSRF protection by making the extracted token always match the cookie value, allowing any CSRF attack to succeed.

```go
// ❌ NEVER DO THIS - Completely defeats CSRF protection
badExtractor := csrf.Extractor{
    Extract: func(c fiber.Ctx) (string, error) {
        return c.Cookies("csrf_"), nil  // Always passes validation!
    },
    Source: csrf.SourceCustom, // See extractors.SourceCustom in shared package
    Key:    "csrf_",
}

// ✅ DO THIS - Extract from different source than cookie
app.Use(csrf.New(csrf.Config{
    CookieName: "csrf_",
    Extractor: extractors.FromHeader("X-Csrf-Token"), // Header vs cookie comparison
}))
```

The middleware uses the **Double Submit Cookie** pattern – it compares the extracted token against the cookie value. If you configure an extractor that reads from the same cookie, it will panic because they will always match and provide zero CSRF protection.
:::


#### Bearer Token Embedding & Custom Extractors

You can create advanced extractors for use cases like JWT embedding or JSON body parsing. See the [Extractors Guide](https://docs.gofiber.io/guide/extractors#custom-extractors) for secure implementation patterns and more examples.


### Fallback Extraction

For applications that need to support both AJAX and form submissions:

```go
// Try header first (AJAX), fallback to form (traditional forms)
app.Use(csrf.New(csrf.Config{
    Extractor: extractors.Chain(
        extractors.FromHeader("X-Csrf-Token"),
        extractors.FromForm("_csrf"),
    ),
}))
```

:::warning
Chaining extractors increases complexity. Use only when you need to support multiple client types. See the [Extractors Guide](https://docs.gofiber.io/guide/extractors#chaining-extractors) for details and security notes.
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
// Destroying the session will also remove the CSRF token if using session-based CSRF.
session.Destroy()
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
| Extractor         | `extractors.Extractor`             | Token extraction method with metadata                                                                                         | `FromHeader("X-Csrf-Token")` |
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
```
