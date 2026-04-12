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
// Initialize default config.
//
// The middleware inspects the app's routes and generates the OpenAPI spec
// the first time a matching request (for example, GET /openapi.json) is served.
// That spec is then cached for the lifetime of the process, so any routes
// registered after the first OpenAPI request will not appear in the spec.
//
// To avoid surprises, register the middleware *after* all routes have been
// added and before you start serving traffic.
app.Use(openapi.New())

// Or extend your config for customization
app.Use(openapi.New(openapi.Config{
    Title:   "My API",
    Version: "1.0.0",
    ServerURL: "https://example.com",
}))

// Routes may optionally document themselves using Summary, Description,
// RequestBody, Parameter, Response, Tags, Deprecated, Produces and Consumes.
app.Post("/users", createUser).
    Summary("Create user").
    Description("Creates a new user").
    RequestBody("User payload", true, fiber.MIMEApplicationJSON).
    // Use *WithExample helpers to attach schemas and examples (including $ref).
    RequestBodyWithExample(
        "User payload", true,
        nil, "#/components/schemas/User",
        map[string]any{"name": "alice"},
        map[string]any{"sample": map[string]any{"name": "bob"}},
        fiber.MIMEApplicationJSON,
    ).
    Parameter("trace-id", "header", true, nil, "Tracing identifier").
    ParameterWithExample(
        "trace-id", "header", true, nil, "",
        "Tracing identifier", "abc-123", map[string]any{"sample": "xyz-789"},
    ).
    Response(fiber.StatusCreated, "Created", fiber.MIMEApplicationJSON).
    ResponseWithExample(
        fiber.StatusCreated, "Created",
        nil, "#/components/schemas/UserResponse",
        map[string]any{"id": 1},
        map[string]any{"sample": map[string]any{"id": 2}},
        fiber.MIMEApplicationJSON,
    ).
    Tags("users", "admin").
    Produces(fiber.MIMEApplicationJSON)

// If not specified, routes default to an empty summary and description, no tags,
// not deprecated, and a "text/plain" request and response media type.
// Consumes and Produces will panic if provided an invalid media type.
```

If no responses are declared, the middleware adds a sensible default: `200 OK` for most methods and `204 No Content` for `DELETE` and `HEAD`. When any responses are provided (either via route helpers or middleware configuration), no automatic default is added.

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

When the middleware is attached to a group or mounted under a prefixed `Use`, the configured `Path` is resolved relative to that
prefix. For example, `app.Group("/v1").Use(openapi.New())` serves the specification at `/v1/openapi.json`, while a global
`app.Use(openapi.New())` only intercepts `/openapi.json` and will not affect other endpoints ending in `openapi.json`.

## Default Config

```go
var ConfigDefault = Config{
    Next:        nil,
    Title:       "Fiber API",
    Version:     "1.0.0",
    Description: "",
    ServerURL:   "",
    Path:        "/openapi.json",
}
```

Schema references (`SchemaRef`) are emitted as `$ref` entries in the generated JSON and can point to components such as `#/components/schemas/User`. `Example` and `Examples` are forwarded verbatim into operation parameters, request bodies, and responses so that client generators can surface realistic payloads.
