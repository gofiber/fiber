---
id: openapi
---

# OpenAPI

OpenAPI middleware for [Fiber](https://github.com/gofiber/fiber) that generates an OpenAPI specification based on the routes registered in your application.

This middleware supports both OpenAPI 3.0.0 and 3.1.0 specifications.

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
// The middleware also serves a Swagger UI page at GET /swagger by default.
//
// To avoid surprises, register the middleware *after* all routes have been
// added and before you start serving traffic.
app.Use(openapi.New())

// Or extend your config for customization
app.Use(openapi.New(openapi.Config{
    Title:          "My API",
    Version:        "1.0.0",
    ServerURL:      "https://example.com",
    OpenAPIVersion: "3.1.0", // or "3.0.0"
    // Components holds reusable schema definitions that $ref targets resolve to.
    Components: map[string]any{
        "schemas": map[string]any{
            "User": map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "name": map[string]any{"type": "string"},
                    "email": map[string]any{"type": "string"},
                },
            },
        },
    },
}))

// Customize the generated Swagger UI page and CDN asset URLs.
app.Use(openapi.New(openapi.Config{
    Path:             "/spec.json",
    UIPath:           "/docs",
    SwaggerCSSURL:    "https://cdn.example.com/swagger-ui.css",
    SwaggerBundleURL: "https://cdn.example.com/swagger-ui-bundle.js",
    SwaggerOptions: map[string]any{
        "docExpansion": "list",
        "deepLinking":  true,
    },
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

// If not specified, generated operations default to a summary of "METHOD path",
// an empty description, no tags, not deprecated, and a "text/plain" request
// and response media type. Consumes, Produces, and RequestBody will panic if
// provided an invalid or empty media type.
```

If no responses are declared, the middleware adds a sensible default: `200 OK` for most methods and `204 No Content` for `DELETE` and `HEAD`. When any responses are provided via the route helpers, no automatic default is added.

`CONNECT` routes are ignored because the OpenAPI specification does not define a `connect` operation.

## Config

| Property       | Type                    | Description                                                     | Default            |
|:---------------|:------------------------|:----------------------------------------------------------------|:------------------:|
| Next           | `func(fiber.Ctx) bool`  | Next defines a function to skip this middleware when returned true. | `nil` |
| Title          | `string`                | Title is the title for the generated OpenAPI specification.     | `"Fiber API"`     |
| Version        | `string`                | Version is the version for the generated OpenAPI specification. | `"1.0.0"`         |
| Description    | `string`                | Description is the description for the generated specification. | `""`             |
| ServerURL      | `string`                | ServerURL is the server URL used in the generated specification.| `""`             |
| Path           | `string`                | Path is the route where the specification will be served.       | `"/openapi.json"` |
| UIPath         | `string`                | Path is the route where the Swagger UI page will be served.     | `"/swagger"` |
| SwaggerCSSURL  | `string`                | Stylesheet URL used by the generated Swagger UI page.           | `"https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css"` |
| SwaggerBundleURL | `string`              | Script URL used by the generated Swagger UI page.               | `"https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js"` |
| SwaggerOptions | `map[string]any`        | Additional options merged into the generated `SwaggerUIBundle` call. | `nil` |
| OpenAPIVersion | `string`                | OpenAPI specification version to generate (`"3.0.0"` or `"3.1.0"`) | `"3.1.0"`     |
| Components     | `map[string]any`        | Reusable OpenAPI component definitions (schemas, responses, etc.) emitted under `"components"`. | `nil` |

When the middleware is attached to a group or mounted under a prefixed `Use`, the configured `Path` is resolved relative to that
prefix. For example, `app.Group("/v1").Use(openapi.New())` serves the specification at `/v1/openapi.json`, while a global
`app.Use(openapi.New())` only intercepts `/openapi.json` and will not affect other endpoints ending in `openapi.json`.
The same prefix resolution applies to `UIPath`, so `app.Group("/v1").Use(openapi.New())` also serves the Swagger UI page at
`/v1/swagger` by default.

## Default Config

```go
var ConfigDefault = Config{
    Next:           nil,
    Title:          "Fiber API",
    Version:        "1.0.0",
    Description:    "",
    ServerURL:      "",
    Path:           "/openapi.json",
    UIPath:         "/swagger",
    SwaggerCSSURL:  "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css",
    SwaggerBundleURL: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js",
    SwaggerOptions: nil,
    OpenAPIVersion: "3.1.0",
    Components:     nil,
}
```

Schema references (`SchemaRef`) are emitted as `$ref` entries in the generated JSON and can point to components such as `#/components/schemas/User`. To make these references resolve correctly, provide the corresponding definitions via the `Components` config field. `Example` and `Examples` follow the OpenAPI specification's mutual exclusivity rule: when both are provided, `Examples` takes precedence and `Example` is omitted.
