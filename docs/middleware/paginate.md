---
id: paginate
---

# Paginate

Pagination middleware for [Fiber](https://github.com/gofiber/fiber) that extracts pagination parameters from query strings and stores them in the request context. Supports page-based, offset-based, and cursor-based pagination with multi-field sorting.

## Signatures

```go
func New(config ...Config) fiber.Handler
func FromContext(ctx any) (*PageInfo, bool)
func NewPageInfo(page, limit, offset int, sort []SortField) *PageInfo
func SortOrderFromString(s string) SortOrder
```

`FromContext` accepts `fiber.CustomCtx`, `fiber.Ctx`, `*fasthttp.RequestCtx`, or `context.Context`. It returns `nil, false` when the middleware did not run for the request, for example because `Next` skipped it.

`NewPageInfo` builds a `PageInfo` from the given values without running the middleware, which helps when you create fixtures for tests or paginate outside a request. It keeps its arguments as given and does not clamp them, so `NewPageInfo(-1, 10000, -5, nil)` holds all three invalid values. `SortOrderFromString` converts `"asc"` or `"desc"` (case-insensitive) into a `SortOrder`; any other value gives `ASC`.

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/paginate"
)
```

Once your Fiber app is initialized, choose one of the following approaches:

### Basic Usage

```go
app.Use(paginate.New())

app.Get("/users", func(c fiber.Ctx) error {
    pageInfo, ok := paginate.FromContext(c)
    if !ok {
        return fiber.ErrBadRequest
    }

    // Use pageInfo.Page, pageInfo.Limit, pageInfo.Start()
    // GET /users?page=2&limit=20 → Page: 2, Limit: 20, Start(): 20
    return c.JSON(pageInfo)
})
```

### Sorting

```go
app.Use(paginate.New(paginate.Config{
    SortKey:      "sort",
    DefaultSort:  "id",
    AllowedSorts: []string{"id", "name", "created_at"},
}))

// GET /users?sort=name,-created_at
// → Sort: [{Field: "name", Order: "asc"}, {Field: "created_at", Order: "desc"}]
```

### Cursor Pagination

```go
app.Use(paginate.New())

app.Get("/feed", func(c fiber.Ctx) error {
    pageInfo, ok := paginate.FromContext(c)
    if !ok {
        return fiber.ErrBadRequest
    }

    if pageInfo.Cursor != "" {
        // Decode the cursor to get keyset values
        values := pageInfo.CursorValues()
        // Use values["id"], values["created_at"], etc. for WHERE clause
    }

    // results is a slice of items from your database query
    // After fetching results, set the next cursor for the client
    if len(results) > 0 {
        lastItem := results[len(results)-1]
        if err := pageInfo.SetNextCursor(map[string]any{
            "id":         lastItem.ID,
            "created_at": lastItem.CreatedAt,
        }); err != nil {
            return err
        }
    }

    return c.JSON(fiber.Map{
        "data":        results,
        "has_more":    pageInfo.HasMore,
        "next_cursor": pageInfo.NextCursor,
    })
})

// First request:  GET /feed?limit=20
// Next request:   GET /feed?cursor=<token>&limit=20
```

A request with a non-empty cursor is handled as cursor pagination: the page and offset parameters are ignored, so `Page` and `Offset` are `0` and `Start()` returns `0`. An empty cursor falls back to page-based pagination.

### Custom Configuration

```go
app.Use(paginate.New(paginate.Config{
    PageKey:      "p",
    LimitKey:     "size",
    DefaultPage:  1,
    DefaultLimit: 25,
    SortKey:      "order_by",
    DefaultSort:  "created_at",
    AllowedSorts: []string{"created_at", "name", "email"},
    CursorKey:    "after",
    CursorParam:  "starting_after",
}))
```

## Config

| Property     | Type                     | Description                                                        | Default    |
|:-------------|:-------------------------|:-------------------------------------------------------------------|:-----------|
| Next         | `func(fiber.Ctx) bool`   | Next defines a function to skip this middleware when returned true. | `nil`      |
| PageKey      | `string`                 | Query string key for page number.                                  | `"page"`   |
| DefaultPage  | `int`                    | Default page number.                                               | `1`        |
| LimitKey     | `string`                 | Query string key for limit.                                        | `"limit"`  |
| DefaultLimit | `int`                    | Default items per page. It is capped at `MaxLimit`, and a limit below 1 in the request also falls back to it. | `10`       |
| SortKey      | `string`                 | Query string key for sort. While it is empty, the query string cannot change the sort order. | `""`       |
| DefaultSort  | `string`                 | Default sort field, applied in ascending order when the request names no allowed sort field. | `"id"`     |
| AllowedSorts | `[]string`               | Whitelist of allowed sort fields. If nil or empty, request sort fields are ignored and the default sort is used.  | `nil`      |
| OffsetKey    | `string`                 | Query string key for offset.                                       | `"offset"` |
| CursorKey    | `string`                 | Query string key for cursor-based pagination.                      | `"cursor"` |
| CursorParam  | `string`                 | Optional alias for the cursor query key.                           | `""`       |
| MaxLimit     | `int`                    | Maximum items per page. A value below 1 selects the default.       | `100` (`paginate.DefaultMaxLimit`) |

## Default Config

```go
var ConfigDefault = Config{
    Next:         nil,
    PageKey:      "page",
    DefaultPage:  1,
    LimitKey:     "limit",
    DefaultLimit: 10,
    MaxLimit:     100,
    DefaultSort:  "id",
    OffsetKey:    "offset",
    CursorKey:    "cursor",
}
```

## PageInfo

The `PageInfo` struct is stored in the request context. Its fields, with the JSON keys they serialize to, are listed below. The bounds in the descriptions hold for the value the middleware stores, not for one built with `NewPageInfo`:

| Field        | JSON key      | Description                                                                                      |
|:-------------|:--------------|:-------------------------------------------------------------------------------------------------|
| `Page`       | `page`        | Current page, never below 1. It is `0` on a cursor request.                                      |
| `Limit`      | `limit`       | Items per page, never above `MaxLimit`.                                                          |
| `Offset`     | `offset`      | Offset taken from `OffsetKey`, never negative. When above 0, `Start()` returns it.               |
| `Sort`       | `sort`        | Parsed sort fields as `[]SortField`. It holds the default sort, ascending, when the request names no allowed field. |
| `Cursor`     | `cursor`      | Cursor token of the request. Omitted from the JSON when empty.                                   |
| `NextCursor` | `next_cursor` | Token for the next page, set by `SetNextCursor`. Omitted from the JSON when empty.               |
| `HasMore`    | `has_more`    | Set to `true` by `SetNextCursor`. Omitted from the JSON when `false`.                            |

Each `SortField` has a `Field` name and an `Order`, serialized as `field` and `order`. `Order` is a `SortOrder` string, either `paginate.ASC` (`"asc"`) or `paginate.DESC` (`"desc"`).

`PageInfo` provides these methods:

| Method                                          | Description                                                    |
|:------------------------------------------------|:---------------------------------------------------------------|
| `Start() int`                                   | Returns calculated start index (from page/limit or offset)     |
| `SortBy(field, order)`                          | Adds a sort field (chainable)                                  |
| `NextPageURL(baseURL)`                          | Generates next page URL with default keys                      |
| `NextPageURLWithKeys(baseURL, pageKey, limitKey)` | Generates next page URL with custom query keys               |
| `PreviousPageURL(baseURL)`                      | Generates previous page URL (empty on page 1)                  |
| `PreviousPageURLWithKeys(baseURL, pageKey, limitKey)` | Generates previous page URL with custom query keys       |
| `NextCursorURL(baseURL)`                        | Generates next cursor URL (empty if no more)                   |
| `NextCursorURLWithKeys(baseURL, cursorKey, limitKey)` | Generates next cursor URL with custom query keys         |
| `CursorValues()`                                | Decodes cursor token into key-value map (`nil` for an empty or invalid token); numbers decode as `float64` with the default JSON decoder |
| `SetNextCursor(values)`                         | Encodes values into cursor token, sets `NextCursor` and `HasMore`; returns an error wrapping `paginate.ErrCursorEncode` |

## Safety

- Limit is capped at `MaxLimit` (default: 100, configurable) to prevent excessive memory usage
- Page values below 1 reset to 1
- Negative offsets reset to 0
- Sort fields are validated against `AllowedSorts`
- Cursor tokens exceeding 2048 characters are rejected with `400 Bad Request`
- `SetNextCursor` returns an error if the encoded token would exceed 2048 characters, preventing the server from issuing cursors it would later reject
- Invalid cursor tokens return `400 Bad Request` via Fiber's error handler
- If `DefaultSort` is not included in `AllowedSorts`, it falls back to the first allowed sort field
- URL helpers preserve existing query parameters when building pagination links
