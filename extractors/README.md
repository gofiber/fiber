# Extractors Package

Package providing shared value extraction utilities for Fiber middleware packages.

## Architecture

### Core Types

- `Extractor`: Core extraction function with metadata
- `Source`: Enumeration of extraction sources (Header, AuthHeader, Query, Form, Param, Cookie, Custom)
- `ErrNotFound`: Standardized error for missing values

### Extractor Structure

```go
type Extractor struct {
    Extract    func(fiber.Ctx) (string, error)  // Extraction function
    Key        string                           // Parameter/header name
    Source     Source                           // Source type for inspection
    AuthScheme string                           // Auth scheme (FromAuthHeader)
    Chain      []Extractor                      // Chained extractors
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

The `Source` field enables security-aware extraction by identifying the origin of extracted values:

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

The `Source` field enables security-aware extraction by identifying the origin of extracted values. However, when using `FromCustom`, middleware cannot determine the source of the extracted value, which can compromise security:

- **CSRF Protection**: The double-submit-cookie pattern requires tokens to come from cookies. Custom extractors may read from insecure sources without middleware being able to detect or prevent this
- **Authentication**: Security middleware may not be able to enforce source-specific security policies
- **Audit Trails**: Source information is lost, making security analysis more difficult

Documentation and examples should clearly warn about these risks when using custom extractors.

## Testing

Run the comprehensive test suite:

```bash
go test -v ./extractors
```

Tests cover:

- Individual extractor functionality across all source types
- Error handling and edge cases (whitespace, empty values, malformed headers)
- Chained extractor behavior and error propagation
- Custom extractor support including nil function handling
- RFC 7235 compliance for Authorization header parsing
- Metadata validation and source introspection
- Chain ordering and fallback logic (17 comprehensive test functions)

## Maintenance Notes

- **Single Source of Truth**: All extraction logic lives here
- **Direct Usage**: Middleware imports and uses extractors directly
- **Security Consistency**: Security warnings and source awareness must be kept in sync across all extractors
- **Breaking Changes**: Require coordinated updates across dependent packages
- **Performance**: Shared functions reduce overhead across middleware

## Future Extensions

Potential enhancements:

- Additional extraction sources (body fields, custom parsers)
- Configurable options (case sensitivity, trimming, validation)
- Performance optimizations for high-throughput scenarios
- Enhanced security features (value validation, rate limiting)
