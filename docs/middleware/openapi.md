---
id: openapi
---

# OpenAPI

OpenAPI middleware for [Fiber](https://github.com/gofiber/fiber) that generates an OpenAPI specification based on the routes registered in your application.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/openapi"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config. Register the middleware *after* all routes
// so that the spec includes every handler.
app.Use(openapi.New())

// Or extend your config for customization
app.Use(openapi.New(openapi.Config{
    Title:   "My API",
    Version: "1.0.0",
    ServerURL: "https://example.com",
}))

// Customize metadata for specific operations
app.Use(openapi.New(openapi.Config{
    Operations: map[string]openapi.Operation{
        "GET /users": {
            Summary:     "List users",
            Description: "Returns all users",
            Produces:    fiber.MIMEApplicationJSON,
        },
    },
}))

// Routes may optionally document themselves using Summary, Description, Produces and Consumes
app.Get("/users", listUsers).
    Summary("List users").
    Description("List all users").
    Produces(fiber.MIMEApplicationJSON)

// If not specified, routes default to an empty summary and description and a
// "text/plain" request and response media type.
```

Each documented route automatically includes a `200` response with the description `OK` to satisfy the minimum OpenAPI requirements.

`CONNECT` routes are ignored because the OpenAPI specification does not define a `connect` operation.

## Config

| Property    | Type                    | Description                                                     | Default            |
|:------------|:------------------------|:----------------------------------------------------------------|:------------------:|
| Next        | `func(fiber.Ctx) bool`  | Next defines a function to skip this middleware when returned true. | `nil` |
| Title       | `string`                | Title is the title for the generated OpenAPI specification.     | `"Fiber API"`     |
| Version     | `string`                | Version is the version for the generated OpenAPI specification. | `"1.0.0"`         |
| Description | `string`                | Description is the description for the generated specification. | `""`             |
| ServerURL   | `string`                | ServerURL is the server URL used in the generated specification.| `""`             |
| Path        | `string`                | Path is the route where the specification will be served.       | `"/openapi.json"` |
| Operations  | `map[string]Operation`  | Per-route metadata keyed by `METHOD /path`.                     | `nil`             |

## Default Config

```go
var ConfigDefault = Config{
    Next:        nil,
    Title:       "Fiber API",
    Version:     "1.0.0",
    Description: "",
    ServerURL:   "",
    Path:        "/openapi.json",
    Operations:         nil,
}
```

### Operation

```go
type Operation struct {
    Id          string
    Summary     string
    Description string
    Tags        []string
    Deprecated  bool
    Consumes    string
    Produces    string
}
```

