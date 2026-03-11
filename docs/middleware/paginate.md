---
id: paginate
---

# Paginate

Pagination middleware for [Fiber](https://github.com/gofiber/fiber) that extracts pagination parameters from query strings and stores them in the request context. Supports page-based, offset-based, and cursor-based pagination with multi-field sorting.

## Signatures

```go
func New(config ...Config) fiber.Handler
func FromContext(ctx any) (*PageInfo, bool)
```

`FromContext` accepts `fiber.CustomCtx`, `fiber.Ctx`, `*fasthttp.RequestCtx`, or `context.Context`.

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
| DefaultLimit | `int`                    | Default items per page.                                            | `10`       |
| SortKey      | `string`                 | Query string key for sort.                                         | `""`       |
| DefaultSort  | `string`                 | Default sort field.                                                | `"id"`     |
| AllowedSorts | `[]string`               | Whitelist of allowed sort fields. If nil, all fields are allowed.  | `nil`      |
| OffsetKey    | `string`                 | Query string key for offset.                                       | `"offset"` |
| CursorKey    | `string`                 | Query string key for cursor-based pagination.                      | `"cursor"` |
| CursorParam  | `string`                 | Optional alias for the cursor query key.                           | `""`       |
| MaxLimit     | `int`                    | Maximum items per page.                                            | `100`      |

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

The `PageInfo` struct is stored in the request context and provides:

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
| `CursorValues()`                                | Decodes cursor token into key-value map                        |
| `SetNextCursor(values)`                         | Encodes values into cursor token, sets HasMore; returns error  |

## Safety

- Limit is capped at `MaxLimit` (default: 100, configurable) to prevent excessive memory usage
- Page values below 1 reset to 1
- Negative offsets reset to 0
- Sort fields are validated against `AllowedSorts`
- Cursor tokens exceeding 2048 characters are rejected with `400 Bad Request`
- Invalid cursor tokens return `400 Bad Request` via Fiber's error handler
- URL helpers preserve existing query parameters when building pagination links
