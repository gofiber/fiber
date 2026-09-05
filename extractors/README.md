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
  Source     Source      // Declared/static source metadata (first child for a chain)
}
```

Source-aware extraction does **not** add fields to `Extractor`, so existing unkeyed
literals and keyed constructors keep working. Use `ExtractWithSource` for the
winning source.

### Available Functions

- `FromAuthHeader(authScheme string)`: Extract from Authorization header with optional scheme
- `FromCookie(key string)`: Extract from HTTP cookies
- `FromParam(param string)`: Extract from URL path parameters
- `FromForm(param string)`: Extract from form data
- `FromHeader(header string)`: Extract from custom HTTP headers
- `FromQuery(param string)`: Extract from URL query parameters
- `FromCustom(key string, fn func(fiber.Ctx) (string, error))`: Define custom extraction logic with metadata
- `Chain(extractors ...Extractor)`: Chain multiple extractors with fallback
- `ExtractWithSource(e Extractor, c fiber.Ctx) (string, Source, error)`: Extract value and the source that supplied it
- `Extractor.Contains(pred func(Extractor) bool)`: Check whether this extractor, or any nested chained extractor, matches a predicate

### Source Inspection

`Extractor.Source` is **declared/static metadata**, not an authoritative proof of origin by itself:

- For a single built-in extractor it matches that constructor's source.
- For a `Chain`, static `Source` is always the **first** child's source.
- `SourceHeader` is the zero value of `Source`. A legacy `Extract`-only extractor that omits `Source` therefore reports `SourceHeader` through `ExtractWithSource`.
- On failure (`err != nil`), `ExtractWithSource` may still return static or last-child source metadata even though no value was supplied. Treat runtime source as meaningful **only when `err == nil`**.

Prefer `ExtractWithSource` when security or audit decisions depend on which source actually produced the value, and verify the returned source before acting on it:

```go
tokenExtractor := extractors.Chain(
    extractors.FromHeader("X-API-Key"),
    extractors.FromQuery("api_key"),
)
token, src, err := extractors.ExtractWithSource(tokenExtractor, c)
if err != nil {
    return err
}
switch src {
case extractors.SourceAuthHeader:
    // Authorization header - commonly used for authentication tokens
case extractors.SourceHeader:
    // Custom HTTP headers - application-specific data
case extractors.SourceCookie:
    // HTTP cookies - client-side stored data
case extractors.SourceQuery:
    // URL query parameters - visible in URLs and logs (security consideration)
case extractors.SourceForm:
    // Form data - POST body data
case extractors.SourceParam:
    // URL path parameters - route-based data
case extractors.SourceCustom:
    // Custom extraction logic
}
```

`ExtractWithSource` always calls `Extract` when set (leaves and chains), so decorating `Extract` for validation or normalization is visible to source-aware callers. Built-in `Chain.Extract` records the winning child's `Source` during that pass; `ExtractWithSource` consumes the capture (no second child walk). If `Extract` succeeds without a capture (custom full replacement, or leaf), the declared static `Source` is returned — `Chain` children are not re-executed to guess provenance. There is no `ExtractWithSource` struct field.

### Chain Behavior

The `Chain` function implements fallback logic:

- Returns first successful extraction (non-empty value, no error)
- If all extractors fail, returns the last error encountered or `ErrNotFound`
- **Skips extractors with `nil` Extract** (zero-value children)
- Detects recursive chain re-entry and returns `ErrChainCycle` (shared guard across Extract and ExtractWithSource)
- Preserves `Source` and `Key` from the first extractor for static introspection (not `AuthScheme`)
- Exposes a **separate defensive copy** via the `Chain` field for introspection; mutating it does not change which children `Extract` runs
- On success, `ExtractWithSource` reports the **winning child's** `Source`
- On failure, the returned source is fallback metadata only — do not treat it as the origin of an extracted value

### Chain Introspection

Use `Contains` to inspect a single extractor or extractor tree with a predicate.

```go
chain := Chain(
    FromHeader("X-CSRF-Token"),
    FromCookie("CSRF"),
)

hasCSRFCookie := chain.Contains(func(e Extractor) bool {
    return e.Source == SourceCookie && e.Key == "CSRF"
})
```

## Security Considerations

### Source Awareness and Custom Extractors

As described in the [Source Inspection](#source-inspection) section, the `Source` field enables middleware to enforce security policies based on data source:

- **CSRF Protection**: The double-submit-cookie pattern requires tokens to be submitted in both a cookie AND a form field/header. Source awareness allows CSRF middleware to verify that tokens come from both expected sources, and not for example only from cookies
- **Authentication**: Security middleware can enforce source-specific policies (e.g., auth tokens from headers, not query parameters)
- **Audit Trails**: Source information enables security analysis and compliance reporting

However, when using `FromCustom`, middleware cannot determine the source of the extracted value, which can limit the ability of a middleware to provide warnings about potential security risks. Documentation and examples should clearly warn about these risks when using custom extractors.
