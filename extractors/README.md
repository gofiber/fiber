# Extractors Package

Package providing shared value extraction utilities for Fiber middleware packages.

## Audience

**This README is targeted at middleware developers and contributors.** If you are a Fiber framework user looking to use extractors in your application, please refer to the [Extractors Guide](https://docs.gofiber.io/guide/extractors) instead.

## Architecture

### Core Types

- `Extractor`: Core extraction function with metadata
- `Source`: Enumeration of extraction sources (Header, AuthHeader, Query, Form, Param, Cookie, Custom)
- `ErrNotFound`: Standardized error for missing values

### Extractor Structure

```go
type Extractor struct {
	Extract    func(fiber.Ctx) (string, error)
	Key        string      // The parameter/header name used for extraction
	AuthScheme string      // The auth scheme used, e.g., "Bearer"
	Chain      []Extractor // For chained extractors, stores all extractors in the chain
	Source     Source      // The type of source being extracted from
}
```

### Available Functions

- `FromAuthHeader(authScheme string)`: Extract from Authorization header with optional scheme
- `FromCookie(key string)`: Extract from HTTP cookies
- `FromParam(param string)`: Extract from URL path parameters
- `FromForm(param string)`: Extract from form data
- `FromHeader(header string)`: Extract from custom HTTP headers
- `FromQuery(param string)`: Extract from URL query parameters
- `FromCustom(key string, fn func(fiber.Ctx) (string, error))`: Define custom extraction logic with metadata
- `Chain(extractors ...Extractor)`: Chain multiple extractors with fallback

### Source Inspection

The `Source` field provides **security-aware extraction** by explicitly identifying the origin of extracted values. This enables middleware to enforce security policies based on data source:

```go
switch extractor.Source {
case SourceAuthHeader:
    // Authorization header - commonly used for authentication tokens
case SourceHeader:
    // Custom HTTP headers - application-specific data
case SourceCookie:
    // HTTP cookies - client-side stored data
case SourceQuery:
    // URL query parameters - visible in URLs and logs (security consideration)
case SourceForm:
    // Form data - POST body data
case SourceParam:
    // URL path parameters - route-based data
case SourceCustom:
    // Custom extraction logic
}
```

### Chain Behavior

The `Chain` function implements fallback logic:

- Returns first successful extraction (non-empty value, no error)
- If all extractors fail, returns the last error encountered or `ErrNotFound`
- **Skips extractors with `nil` Extract functions** (graceful error handling)
- Preserves metadata from first extractor for introspection
- Stores defensive copy for runtime inspection via the `Chain` field

## Security Considerations

### Source Awareness and Custom Extractors

As described in the [Source Inspection](#source-inspection) section, the `Source` field enables middleware to enforce security policies based on data source:

- **CSRF Protection**: The double-submit-cookie pattern requires tokens to come from cookies. Source awareness allows CSRF middleware to verify tokens originate from the expected secure source
- **Authentication**: Security middleware can enforce source-specific policies (e.g., auth tokens from headers, not query parameters)
- **Audit Trails**: Source information enables security analysis and compliance reporting

However, when using `FromCustom`, middleware cannot determine the source of the extracted value, which can compromise security:

- **CSRF Protection**: The double-submit-cookie pattern requires tokens to come from cookies. Custom extractors may read from insecure sources without middleware being able to detect or prevent this
- **Authentication**: Security middleware may not be able to enforce source-specific security policies
- **Audit Trails**: Source information is lost, making security analysis more difficult

Documentation and examples should clearly warn about these risks when using custom extractors.
