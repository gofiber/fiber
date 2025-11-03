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

// Routes may optionally document themselves using Summary, Description,
// RequestBody, Parameter, Response, Tags, Deprecated, Produces and Consumes.
app.Post("/users", createUser).
    Summary("Create user").
    Description("Creates a new user").
    RequestBody("User payload", true, fiber.MIMEApplicationJSON).
    Parameter("trace-id", "header", true, nil, "Tracing identifier").
    Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON).
    Tags("users", "admin").
    Produces(fiber.MIMEApplicationJSON)

// If not specified, routes default to an empty summary and description, no tags,
// not deprecated, and a "text/plain" request and response media type.
// Consumes and Produces will panic if provided an invalid media type.
```

Each documented route automatically includes a `200` response with the description `OK` to satisfy the minimum OpenAPI requirements. Additional responses can be declared via the `Response` helper or the middleware configuration.

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

When the middleware is attached to a group or mounted under a prefixed `Use`, the configured `Path` is resolved relative to that
prefix. For example, `app.Group("/v1").Use(openapi.New())` serves the specification at `/v1/openapi.json`, while a global `app.U
se(openapi.New())` only intercepts `/openapi.json` and will not affect other endpoints ending in `openapi.json`.

## Default Config

```go
var ConfigDefault = Config{
    Next:        nil,
    Operations:  nil,
    Title:       "Fiber API",
    Version:     "1.0.0",
    Description: "",
    ServerURL:   "",
    Path:        "/openapi.json",
}
```

### Operation

```go
type Operation struct {
    RequestBody *RequestBody
    Responses   map[string]Response
    Parameters  []Parameter
    Tags        []string

    ID          string
    Summary     string
    Description string
    Consumes    string
    Produces    string
    Deprecated  bool
}

type Parameter struct {
    Schema      map[string]any
    Name        string
    In          string
    Description string
    Required    bool
}

type Media struct {
    Schema map[string]any
}

type Response struct {
    Content     map[string]Media
    Description string
}

type RequestBody struct {
    Content     map[string]Media
    Description string
    Required    bool
}
```

Refer to the type definitions above when customizing OpenAPI operations in your configuration.
