---
id: utils
title: 🧰 Utils
sidebar_position: 8
toc_max_heading_level: 4
---

## Generics

### Convert

[//]: # (TODO: put it in a different section for generics)
[//]: # (TODO: add Locals, Params, Query, GetReqHeader method)

Converts a string value to a specified type, handling errors and optional default values.
This function simplifies the conversion process by encapsulating error handling and the management of default values, making your code cleaner and more consistent.

```go title="Signature"
func Convert[T any](value string, convertor func(string) (T, error), defaultValue ...T) (*T, error)
```

```go title="Example"
// GET http://example.com/id/bb70ab33-d455-4a03-8d78-d3c1dacae9ff
app.Get("/id/:id", func(c fiber.Ctx) error {
  fiber.Convert(c.Params("id"), uuid.Parse) // UUID(bb70ab33-d455-4a03-8d78-d3c1dacae9ff), nil


// GET http://example.com/search?id=65f6f54221fb90e6a6b76db7
app.Get("/search", func(c fiber.Ctx) error) {
  fiber.Convert(c.Query("id"), mongo.ParseObjectID) // objectid(65f6f54221fb90e6a6b76db7), nil
  fiber.Convert(c.Query("id"), uuid.Parse) // uuid.Nil, error(cannot parse given uuid)
  fiber.Convert(c.Query("id"), uuid.Parse, mongo.NewObjectID) // new object id generated and return nil as error.
}

  // ...
})
```
