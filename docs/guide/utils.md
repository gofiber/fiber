---
id: utils
title: 🧰 Utils
sidebar_position: 8
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

// GET http://example.com/search?id=65f6f54221fb90e6a6b76db7
app.Get("/search", func(c fiber.Ctx) error {
    fiber.Convert(c.Query("id"), mongo.ParseObjectID)           // objectid(65f6f54221fb90e6a6b76db7), nil
    fiber.Convert(c.Query("id"), uuid.Parse)                    // uuid.Nil, error(cannot parse given uuid)
    fiber.Convert(c.Query("id"), uuid.Parse, mongo.NewObjectID) // new object id generated and return nil as error.
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
// Set a value
func Locals[V any](c Ctx, key any, value ...V) V
// Get a value
func Locals[V any](c Ctx, key any) V
```

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
and strict routing.

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
