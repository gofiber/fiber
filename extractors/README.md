# Extractors Package

Package providing shared value extraction utilities for Fiber middleware packages.

## Architecture

### Core Types

- `Extractor`: Core extraction function with metadata
- `Source`: Enumeration of extraction sources (Header, Query, Form, Param, Cookie, Custom)
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

- `FromAuthHeader(authScheme string)`: Extract from Authorization header
- `FromCookie(key string)`: Extract from HTTP cookies
- `FromParam(param string)`: Extract from URL path parameters
- `FromForm(param string)`: Extract from form data
- `FromHeader(header string)`: Extract from custom HTTP headers
- `FromQuery(param string)`: Extract from URL query parameters
- `Chain(extractors ...Extractor)`: Chain multiple extractors with fallback

### Source Inspection

The `Source` field enables security-aware extraction:

```go
switch extractor.Source {
case SourceQuery:
    // Query parameters - potential security risk
case SourceCookie:
    // Cookies - generally secure
case SourceHeader:
    // Headers - secure
}
```

### Chain Behavior

The `Chain` function implements fallback logic:

- Returns first successful extraction (non-empty value, no error)
- If all fail, returns last error or `ErrNotFound`
- Preserves metadata from first extractor
- Stores defensive copy for introspection

## Testing

Run the comprehensive test suite:

```bash
go test -v ./extractors
```

Tests cover:

- Individual extractor functionality
- Error handling and edge cases
- Chained extractor behavior
- Security warning propagation
- Custom extractor support
- Error propagation in chains
- Metadata and introspection

## Maintenance Notes

- **Single Source of Truth**: All extraction logic lives here
- **Direct Usage**: Middleware imports and uses extractors directly
- **Security Consistency**: Security warnings must be kept in sync
- **Breaking Changes**: Require coordinated updates across dependent packages
- **Performance**: Shared functions reduce overhead across middleware

## Future Extensions

Potential enhancements:

- Additional extraction sources (body fields, custom parsers)
- Configurable options (case sensitivity, trimming, validation)
- Performance optimizations for high-throughput scenarios
- Enhanced security features (value validation, rate limiting)
