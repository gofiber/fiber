---
id: extractors
title: 🔍 Extractors
description: Shared value extraction utilities for Use In Middleware Packages
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

## Architecture

### Shared Extractor Types

- `Extractor`: Core extraction function with metadata (source type, key, auth scheme)
- `Source`: Enumeration of extraction sources (Header, AuthHeader, Query, Form, Param, Cookie, Custom)
- `ErrNotFound`: Standardized error for missing values

### Available Extractors

- `FromAuthHeader(authScheme string)`: Extract from Authorization header with optional scheme
- `FromCookie(key string)`: Extract from HTTP cookies
- `FromParam(param string)`: Extract from URL path parameters
- `FromForm(param string)`: Extract from form data
- `FromHeader(header string)`: Extract from custom HTTP headers
- `FromQuery(param string)`: Extract from URL query parameters
- `Chain(extractors ...Extractor)`: Chain multiple extractors with fallback logic

### Extractor Structure

Each `Extractor` contains:

- `Extract`: Function that performs the actual extraction from a Fiber context
- `Key`: The parameter/header name used for extraction
- `Source`: The type of source being extracted from (can be inspected for security restrictions)
- `AuthScheme`: The auth scheme used (for `FromAuthHeader`)
- `Chain`: For chained extractors, stores all extractors in the chain in a defensive copy for introspection

### Chain Behavior

The `Chain` function creates extractors that try multiple sources in order:

- Returns the first successful extraction (non-empty value with no error)
- If all extractors fail, returns the last error encountered or `ErrNotFound`
- Preserves the source and key from the first extractor for metadata
- Stores a defensive copy of all chained extractors for introspection via the `Chain` field

## Usage Examples

### Basic Usage

```go
import "github.com/gofiber/fiber/v3/extractors"

// Extract API key from header
apiKeyExtractor := extractors.FromHeader("X-API-Key")

// Extract session ID from cookie
sessionExtractor := extractors.FromCookie("session_id")

// Extract user ID from query parameter
userIdExtractor := extractors.FromQuery("user_id")

// Extract from Authorization header with Bearer scheme
bearerExtractor := extractors.FromAuthHeader("Bearer")

// Extract from Authorization header without scheme stripping
rawAuthExtractor := extractors.FromAuthHeader("")

// Chain multiple extractors with fallback
tokenExtractor := extractors.Chain(
    extractors.FromAuthHeader("Bearer"),
    extractors.FromCookie("auth_token"),
    extractors.FromQuery("token"),
)
```

## Security Considerations

Several extractors include security warnings:

- **Query Parameters**: Can leak values through logs, referrer headers, and browser history
- **Form Data**: Can leak values through logs and referrer headers (for GET submissions)
- **URL Parameters**: Can leak values through logs and browser history

These warnings ensure developers are aware of the security implications when using these extractors.

<parameter name="filePath">/Users/sixcolors/Documents/GitHub/fiber/docs/guide/extractors.md
