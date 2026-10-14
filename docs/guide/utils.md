---
id: utils
title: 🧰 Utils
sidebar_position: 9
toc_max_heading_level: 4
---

## Generics

### Convert

Converts a string to a specific type while handling errors and optional defaults.
It wraps conversion and fallback logic to keep your code clean and consistent.

```go title="Signature"
func Convert[T any](value string, converter func(string) (T, error), defaultValue ...T) (T, error)
```

```go title="Example"
// GET http://example.com/id/bb70ab33-d455-4a03-8d78-d3c1dacae9ff
app.Get("/id/:id", func(c fiber.Ctx) error {
    fiber.Convert(c.Params("id"), uuid.Parse)                   // UUID(bb70ab33-d455-4a03-8d78-d3c1dacae9ff), nil
})

// GET http://example.com/search?page=3&id=not-a-uuid
app.Get("/search", func(c fiber.Ctx) error {
    fiber.Convert(c.Query("page"), strconv.Atoi)          // 3, nil
    fiber.Convert(c.Query("id"), uuid.Parse)              // uuid.Nil, error(failed to convert: ...)
    fiber.Convert(c.Query("id"), uuid.Parse, uuid.Nil)    // uuid.Nil, nil: the default replaces the error
    return nil
})

// ...
```

### GetReqHeader

Retrieves an HTTP request header as a specific type using generics.

```go title="Signature"
func GetReqHeader[V GenericType](c Ctx, key string, defaultValue ...V) V
```

```go title="Example"
app.Get("/search", func(c fiber.Ctx) error {
    // curl -X GET http://example.com/search -H "X-Request-ID: 12345" -H "X-Request-Name: John"
    fiber.GetReqHeader[int](c, "X-Request-ID")               // => returns 12345 as integer.
    fiber.GetReqHeader[string](c, "X-Request-Name")          // => returns "John" as string.
    fiber.GetReqHeader[string](c, "unknownParam", "default") // => returns "default" as string.
    // ...
})
```

### Locals

Reads or writes local values in the request context using generics.

```go title="Signature"
func Locals[V any](c Ctx, key any, value ...V) V
```

Pass a value to store it for the rest of the request, or omit it to read the stored value back. A missing key, or a value of another type, gives the zero value of `V`.

```go title="Example"
app.Use("/user/:user/:id", func(c fiber.Ctx) error {
    // set local values
    fiber.Locals[string](c, "user", "john")
    fiber.Locals[int](c, "id", 25)
    // ...
    
    return c.Next()
})


app.Get("/user/*", func(c fiber.Ctx) error {
    // get local values
    name := fiber.Locals[string](c, "user") // john
    age := fiber.Locals[int](c, "id")       // 25
    // ...
})
```

### Params

Retrieves route parameters as a specific type.

```go title="Signature"
func Params[V GenericType](c Ctx, key string, defaultValue ...V) V
```

```go title="Example"
app.Get("/user/:user/:id", func(c fiber.Ctx) error {
    // http://example.com/user/john/25
    fiber.Params[int](c, "id")               // => returns 25 as integer.
    fiber.Params[int](c, "unknownParam", 99) // => returns the default 99 as integer.
    // ...
    return c.SendString("Hello, " + fiber.Params[string](c, "user"))
})
```

### Query

Retrieves query parameters as a specific type.

```go title="Signature"
func Query[V GenericType](c Ctx, key string, defaultValue ...V) V
```

```go title="Example"
app.Get("/search", func(c fiber.Ctx) error {
    // http://example.com/search?name=john&age=25
    fiber.Query[string](c, "name")                    // => returns "john"
    fiber.Query[int](c, "age")                        // => returns 25 as integer.
    fiber.Query[string](c, "unknownParam", "default") // => returns "default" as string.
    // ...
})
```

### RoutePatternMatch

Checks whether a given path matches a Fiber route pattern. Useful for testing
patterns without registering them. Patterns may contain parameters, wildcards
and optional segments. An optional `Config` allows control over case sensitivity
and strict routing; only `CaseSensitive`, `StrictRouting`, `UnescapePath` and
`RegexHandler` are read from it. It knows no app, so a pattern that names a
custom constraint is matched without that constraint.

The path is normalized the way the router normalizes a request path before
matching, so the answer agrees with what the app would actually do. In
particular, with the default `StrictRouting: false` a trailing slash on the
path is ignored, so `RoutePatternMatch("/a/", "/a")` reports `true`. Set
`StrictRouting: true` if you need the two forms to be distinguished.

It takes a path, not a request target. The empty string is read as `/`, and any
other string without a leading slash matches no pattern, so
`RoutePatternMatch("*", "/*")` reports `false`. The router instead roots a
request target that has no leading slash before matching, so it routes
`OPTIONS *` as `/*`.

```go title="Signature"
func RoutePatternMatch(path, pattern string, cfg ...Config) bool
```

```go title="Example"
fiber.RoutePatternMatch("/user/john", "/user/:name") // true

fiber.RoutePatternMatch(
    "/User/john",
    "/user/:name",
    fiber.Config{CaseSensitive: true},
) // false
```

## HTTP Methods

### IsMethodSafe

Reports whether an HTTP method is safe in the sense of [RFC 9110 Section 9.2.1](https://www.rfc-editor.org/rfc/rfc9110#section-9.2.1) (`GET`, `HEAD`, `OPTIONS` and `TRACE`) and [RFC 10008](https://www.rfc-editor.org/rfc/rfc10008.html) (`QUERY`). Methods are compared exactly, so pass them in upper case, as `c.Method()` returns them.

```go title="Signature"
func IsMethodSafe(m string) bool
```

### IsMethodIdempotent

Reports whether an HTTP method is idempotent in the sense of [RFC 9110 Section 9.2.2](https://www.rfc-editor.org/rfc/rfc9110#section-9.2.2): every safe method, plus `PUT` and `DELETE`.

```go title="Signature"
func IsMethodIdempotent(m string) bool
```

```go title="Example"
fiber.IsMethodSafe(fiber.MethodGet)       // true
fiber.IsMethodSafe(fiber.MethodPost)      // false
fiber.IsMethodIdempotent(fiber.MethodPut) // true
fiber.IsMethodIdempotent("put")           // false: names are compared exactly

// Log only the requests that may change state
app.Use(func(c fiber.Ctx) error {
    if !fiber.IsMethodSafe(c.Method()) {
        log.Printf("%s %s", c.Method(), c.Path())
    }
    return c.Next()
})
```
