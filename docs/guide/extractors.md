---
id: extractors
title: 🔬 Extractors
description: Understanding how middleware extracts values from HTTP requests
sidebar_position: 8.5
toc_max_heading_level: 4
---

The extractors package provides shared value extraction utilities for Fiber middleware packages. It helps reduce code duplication across middleware packages while ensuring consistent behavior and security practices.

## Overview

The `github.com/gofiber/fiber/v3/extractors` package serves as a centralized location for value extraction logic that is common across multiple middleware packages. This approach:

- **Reduces Code Duplication**: Eliminates redundant extractor implementations across middleware packages
- **Ensures Consistency**: Maintains identical behavior and security practices across all extractors
- **Simplifies Maintenance**: Changes to extraction logic only need to be made in one place
- **Enables Direct Usage**: Middleware can import and use extractors directly
- **Improves Performance**: Shared, optimized extraction functions reduce overhead

## Installation

```bash
go get github.com/gofiber/fiber/v3/extractors
```

## What Are Extractors?

Extractors are utilities that middleware uses to get values from different parts of HTTP requests:

### Available Extractors

- `FromAuthHeader(authScheme string)`: Extract from Authorization header with optional scheme
- `FromCookie(key string)`: Extract from HTTP cookies
- `FromParam(param string)`: Extract from URL path parameters
- `FromForm(param string)`: Extract from form data
- `FromHeader(header string)`: Extract from custom HTTP headers
- `FromQuery(param string)`: Extract from URL query parameters
- `FromCustom(key string, fn func(fiber.Ctx) (string, error))`: Define custom extraction logic with metadata
- `Chain(extractors ...Extractor)`: Chain multiple extractors with fallback logic

### Extractor Structure

Each `Extractor` contains:

```go
type Extractor struct {
    Extract    func(fiber.Ctx) (string, error)  // Extraction function
    Key        string                           // Parameter/header name
    Source     Source                           // Source type for inspection
    AuthScheme string                           // Auth scheme (FromAuthHeader)
    Chain      []Extractor                      // Chained extractors
}
```

- **Headers**: `Authorization`, `X-API-Key`, custom headers
- **Cookies**: Session cookies, authentication tokens
- **Query Parameters**: URL parameters like `?token=abc123`
- **Form Data**: POST body form fields
- **URL Parameters**: Route parameters like `/users/:id`

### Chain Behavior

The `Chain` function creates extractors that try multiple sources in order:

- Returns the first successful extraction (non-empty value with no error)
- If all extractors fail, returns the last error encountered or `ErrNotFound`
- **Robust error handling**: Skips extractors with `nil` Extract functions
- Preserves the source and key from the first extractor for metadata
- Stores a defensive copy of all chained extractors for introspection via the `Chain` field

## Why Middleware Uses Extractors

Middleware needs to extract values from requests for authentication, authorization, and other purposes. Extractors provide:

- **Security Awareness**: Different sources have different security implications
- **Fallback Support**: Try multiple sources if the first one doesn't have the value
- **Consistency**: Same extraction logic across all middleware packages
- **Source Tracking**: Know where values came from for security decisions

## Usage Examples

### Basic Usage

```go
// KeyAuth middleware extracts key from header
app.Use(keyauth.New(keyauth.Config{
    Extractor: extractors.FromHeader("Middleware-Key"),
}))
```

### Fallback Chains

```go
// Try multiple sources in order
tokenExtractor := extractors.Chain(
    extractors.FromHeader("Middleware-Key"),  // Try header first
    extractors.FromCookie("middleware_key"),  // Then cookie
    extractors.FromQuery("middleware_key"),   // Finally query param
)

app.Use(keyauth.New(keyauth.Config{
    Extractor: tokenExtractor,
}))
```

## Configuring Middleware That Uses Extractors

### Authentication Middleware

```go
// KeyAuth middleware (default: FromAuthHeader)
app.Use(keyauth.New(keyauth.Config{
    // Default extracts from Authorization header
    // Extractor: extractors.FromAuthHeader("Bearer"),
}))

// Custom header extraction
app.Use(keyauth.New(keyauth.Config{
    Extractor: extractors.FromHeader("X-API-Key"),
}))

// Multiple sources with secure fallback
app.Use(keyauth.New(keyauth.Config{
    Extractor: extractors.Chain(
        extractors.FromAuthHeader("Bearer"),  // Secure first
        extractors.FromHeader("X-API-Key"),   // Then custom header
        extractors.FromQuery("api_key"),      // Least secure last
    ),
}))
```

### Session Middleware

```go
// Session middleware (default: FromCookie)
app.Use(session.New(session.Config{
    // Default extracts from session_id cookie
    // Extractor: extractors.FromCookie("session_id"),
}))

// Custom cookie name
app.Use(session.New(session.Config{
    Extractor: extractors.FromCookie("my_session"),
}))
```

### CSRF Middleware

```go
// CSRF middleware (default: FromHeader)
app.Use(csrf.New(csrf.Config{
    // Default extracts from X-CSRF-Token header
    // Extractor: extractors.FromHeader("X-CSRF-Token"),
}))

// Form-based CSRF (less secure, use only if needed)
app.Use(csrf.New(csrf.Config{
    Extractor: extractors.Chain(
        extractors.FromHeader("X-CSRF-Token"), // Secure first
        extractors.FromForm("_csrf"),          // Form fallback
    ),
}))
```

## Security Considerations

### Source Characteristics

Different extraction sources have different security properties and use cases:

#### Headers (Generally Preferred)

- **Authorization Header**: Standard for authentication tokens, widely supported
- **Custom Headers**: Application-specific, less likely to be logged by default
- **Considerations**: Can be intercepted without HTTPS, may be stripped by proxies

#### Cookies (Good for Sessions)

- **Session Cookies**: Designed for secure client-side storage
- **Considerations**: Require proper `Secure`, `HttpOnly`, and `SameSite` flags
- **Best for**: Session management, remember-me tokens

#### Query Parameters (Use Sparingly)

- **Query parameters**: Convenient for simple APIs and debugging
- **Considerations**: Always visible in URLs, logged by servers/proxies, stored in browser history
- **Best for**: Non-sensitive parameters, public identifiers

#### Form Data (Context Dependent)

- **POST Bodies**: Suitable for form submissions and API requests
- **Considerations**: Avoid putting sensitive data in query strings; ensure request bodies aren’t logged and use the correct content type
- **Best for**: User-generated content, file uploads

### Security Best Practices

1. **Use HTTPS**: Encrypt all traffic to protect extracted values in transit
2. **Validate Input**: Always validate and sanitize extracted values
3. **Log Carefully**: Avoid logging sensitive values from any source
4. **Choose Appropriate Sources**: Match the source to your security requirements
5. **Test Thoroughly**: Verify extraction works in your environment
6. **Monitor Security**: Watch for extraction failures or unusual patterns

### Chain Ordering Strategy

When using multiple sources, order them by your security preferences:

```go
// Example: Prefer headers, fallback to cookies, then query
extractors.Chain(
    extractors.FromAuthHeader("Bearer"),    // Standard auth
    extractors.FromCookie("auth_token"),    // Secure storage
    extractors.FromQuery("token"),          // Public fallback
)
```

The "best" source depends on your specific use case, security requirements, and application architecture.

### Common Security Issues

#### Leaky URLs

```go
// ❌ DON'T: API keys in URLs (visible in logs, history, bookmarks)
app.Use(keyauth.New(keyauth.Config{
    Extractor: extractors.FromQuery("api_key"), // PROBLEMATIC
}))

// ✅ DO: API keys in headers (not visible in URLs)
app.Use(keyauth.New(keyauth.Config{
    Extractor: extractors.FromHeader("X-API-Key"), // BETTER
}))
```

#### Session Tokens in Query Parameters

```go
// ❌ DON'T: Session tokens in URLs (can be bookmarked, leaked)
app.Use(session.New(session.Config{
    Extractor: extractors.FromQuery("session"), // PROBLEMATIC
}))

// ✅ DO: Session tokens in cookies (designed for this purpose)
app.Use(session.New(session.Config{
    Extractor: extractors.FromCookie("session_id"), // BETTER
}))
```

#### Form-Only CSRF Tokens

While the default extractor uses headers, some implementations use form fields, which is fine if you don't have AJAX or API clients:

```go
// ❌ DON'T: CSRF tokens only in forms (breaks AJAX, API calls)
app.Use(csrf.New(csrf.Config{
    Extractor: extractors.FromForm("_csrf"), // LIMITED
}))

// ✅ DO: Header-first with form fallback (works everywhere)
app.Use(csrf.New(csrf.Config{
    Extractor: extractors.Chain(
        extractors.FromHeader("X-CSRF-Token"), // PREFERRED
        extractors.FromForm("_csrf"),          // FALLBACK
    ),
}))
```

### Understanding Trade-offs

**No extractor is universally "secure" - security depends on:**

- Whether you're using HTTPS
- How you configure cookies (Secure, HttpOnly, SameSite flags)
- Your logging and monitoring setup
- The sensitivity of the data being extracted
- Your threat model and security requirements

Choose extractors based on your specific use case and security needs, not blanket "secure" vs "insecure" labels.

## Standards Compliance

### RFC 7235 Authorization Header Support

The `FromAuthHeader` extractor is fully compliant with RFC 7235 HTTP Authentication:

- **Case-insensitive scheme matching**: `Bearer`, `bearer`, `BEARER` all work
- **Flexible whitespace handling**: Supports spaces and tabs after scheme
- **Proper error handling**: Validates header format and content
- **Security-conscious**: Prevents common parsing vulnerabilities

```go
// All of these work correctly:
extractors.FromAuthHeader("Bearer")  // Standard case
extractors.FromAuthHeader("bearer")  // Lowercase
extractors.FromAuthHeader("BEARER")  // Uppercase
extractors.FromAuthHeader("")        // No scheme, returns header (or ErrNotFound if empty)
```

## Troubleshooting

### Extraction Fails

**Problem**: Middleware returns "value not found" or authentication fails

**Solutions**:

1. Check if the expected header/cookie/query parameter is present
2. Verify the parameter name matches exactly (case-sensitive)
3. Ensure the request uses the correct HTTP method (GET vs POST)
4. Check if middleware is configured with the right extractor

**Debug Example**:

```go
// Add simple debug logging (avoid logging secrets in production)
app.Use(func(c fiber.Ctx) error {
    hdr := c.Get("X-API-Key")
    cookie := c.Cookies("session_id")
    if hdr != "" || cookie != "" {
        log.Printf("debug: X-API-Key present=%t, session_id present=%t", hdr != "", cookie != "")
    }
    return c.Next()
})
```

### Wrong Source Used

**Problem**: Values extracted from unexpected sources

**Solutions**:

1. Check middleware configuration order
2. Verify chain order (first successful extraction wins)
3. Use more specific extractors when needed

### Security Warnings

**Problem**: Getting security warnings in logs

**Solutions**:

1. Switch to more secure sources (headers/cookies)
2. Use HTTPS to encrypt traffic
3. Review if sensitive data should be in that source

## Advanced Usage

### Custom Extraction Logic

Extractors support custom extractors for complex scenarios:

```go
// Extract from custom logic (rarely needed)
customExtractor := extractors.FromCustom("my-source", func(c fiber.Ctx) (string, error) {
    // Complex extraction logic
    if value := c.Locals("computed_token"); value != nil {
        return value.(string), nil
    }
    return "", extractors.ErrNotFound
})
```

:::warning
**Custom extractors break source awareness.** When you use `FromCustom`, middleware cannot determine where the value came from, which means:

- **No automatic security warnings** for potentially insecure sources
- **No source-based logging** or monitoring capabilities
- **Developer responsibility** for ensuring the extraction is secure and appropriate

**Only use `FromCustom` when:**

- Standard extractors don't meet your needs
- You've carefully evaluated the security implications
- You're confident in the security of your custom extraction logic
- You understand that middleware cannot provide source-aware security guidance

**Note:** If you pass `nil` as the function parameter, `FromCustom` will return an extractor that always fails with `ErrNotFound`.
:::

### Multiple Middleware Coordination

When using multiple middleware that extract values, ensure they don't conflict:

```go
// Good: Different sources for different purposes
app.Use(keyauth.New(keyauth.Config{
    Extractor: extractors.FromHeader("X-API-Key"),
}))
app.Use(session.New(session.Config{
    Extractor: extractors.FromCookie("session_id"),
}))

// Avoid: Same source for different middleware
app.Use(keyauth.New(keyauth.Config{
    Extractor: extractors.FromCookie("token"), // API auth
}))
app.Use(session.New(session.Config{
    Extractor: extractors.FromCookie("token"), // Session - CONFLICT!
}))
```
