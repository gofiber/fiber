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
// The middleware inspects the app's routes and generates the OpenAPI spec on
// the first matching request (for example, GET /openapi.json). The spec is
// cached, but the cache is automatically invalidated whenever the number of
// registered routes changes, so routes added after the first request are
// reflected without a process restart.
// The middleware also serves a Swagger UI page at GET /swagger by default.
app.Use(openapi.New())

// Or extend your config for customization
app.Use(openapi.New(openapi.Config{
    Title:          "My API",
    Version:        "1.0.0",
    OpenAPIVersion: "3.1.0", // or "3.0.0"
    Description:    "Example API",
    TermsOfService: "https://example.com/terms",
    Contact:        &openapi.Contact{Name: "API Team", Email: "api@example.com"},
    License:        &openapi.License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
    // Servers takes precedence over ServerURL and supports multiple entries.
    Servers: []openapi.Server{
        {URL: "https://prod.example.com", Description: "Production"},
        {URL: "https://staging.example.com", Description: "Staging"},
    },
    // Top-level tag definitions and external documentation.
    Tags:         []openapi.Tag{{Name: "users", Description: "User operations"}},
    ExternalDocs: &openapi.ExternalDocs{Description: "Docs", URL: "https://docs.example.com"},
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

// Document authentication with security schemes.
//
// SecuritySchemes are emitted under components.securitySchemes; Security sets the
// document-level (default) requirement applied to every operation.
app.Use(openapi.New(openapi.Config{
    SecuritySchemes: map[string]any{
        "bearerAuth": map[string]any{
            "type":         "http",
            "scheme":       "bearer",
            "bearerFormat": "JWT",
        },
    },
    Security: []map[string][]string{
        {"bearerAuth": {}},
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
    // Per-operation security. Multiple requirements are combined with OR;
    // pass an empty requirement (map[string][]string{}) to document "no auth".
    Security(map[string][]string{"bearerAuth": {}}).
    Produces(fiber.MIMEApplicationJSON)

// If not specified, generated operations default to a summary of "METHOD path",
// an empty description, no tags, not deprecated, and a "text/plain" request
// and response media type. Consumes, Produces, and RequestBody will panic if
// provided an invalid or empty media type.
```

If no responses are declared, the middleware adds a sensible default: `200 OK` for most methods and `204 No Content` for `DELETE` and `HEAD`. When any responses are provided via the route helpers, no automatic default is added.

Each operation gets a unique `operationId`. Routes documented with `Name` use that name; routes without one get an id generated from the method and path (for example `GET /users/{id}` → `getUsersId`). If two operations would share an id, a numeric suffix (`_2`, `_3`, …) is appended so the generated document stays valid.

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
| SwaggerStandalonePresetURL | `string`    | Standalone preset script URL; when set the UI uses `StandaloneLayout` (top bar with the Authorize button). | `"https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-standalone-preset.js"` |
| SwaggerOptions | `map[string]any`        | Additional options merged into the generated `SwaggerUIBundle` call. | `nil` |
| OpenAPIVersion | `string`                | OpenAPI specification version to generate (`"3.0.0"` or `"3.1.0"`) | `"3.1.0"`     |
| Components     | `map[string]any`        | Reusable OpenAPI component definitions (schemas, responses, etc.) emitted under `"components"`. | `nil` |
| SecuritySchemes | `map[string]any`       | Reusable security scheme definitions, emitted under `"components.securitySchemes"`. | `nil` |
| Security       | `[]map[string][]string` | Document-level (default) security requirements; each map is a requirement (OR semantics across entries). | `nil` |
| Contact        | `*Contact`              | Contact information for the API (`info.contact`).               | `nil` |
| License        | `*License`              | License information for the API (`info.license`).               | `nil` |
| TermsOfService | `string`                | Terms of Service URL (`info.termsOfService`).                   | `""` |
| Servers        | `[]Server`              | Servers hosting the API; takes precedence over `ServerURL`.     | `nil` |
| Tags           | `[]Tag`                 | Top-level tag definitions (with descriptions).                  | `nil` |
| ExternalDocs   | `*ExternalDocs`         | External documentation reference (`externalDocs`).             | `nil` |

When the middleware is attached to a group or mounted under a prefixed `Use`, the configured `Path` is resolved relative to that
prefix. For example, `app.Group("/v1").Use(openapi.New())` serves the specification at `/v1/openapi.json`, while a global
`app.Use(openapi.New())` only intercepts `/openapi.json` and will not affect other endpoints ending in `openapi.json`.
The same prefix resolution applies to `UIPath`, so `app.Group("/v1").Use(openapi.New())` also serves the Swagger UI page at
`/v1/swagger` by default.

## Default Config

```go
var ConfigDefault = Config{
    Next:                       nil,
    Title:                      "Fiber API",
    Version:                    "1.0.0",
    Description:                "",
    ServerURL:                  "",
    Path:                       "/openapi.json",
    UIPath:                     "/swagger",
    SwaggerCSSURL:              "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui.css",
    SwaggerBundleURL:           "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-bundle.js",
    SwaggerStandalonePresetURL: "https://unpkg.com/swagger-ui-dist@5.32.6/swagger-ui-standalone-preset.js",
    SwaggerOptions:             nil,
    OpenAPIVersion:             "3.1.0",
    Components:                 nil,
}
```

:::note Offline / self-hosted Swagger UI
By default the Swagger UI page loads its assets from the `unpkg.com` CDN, which
requires outbound internet access from the browser. For offline, air-gapped, or
strict-CSP deployments, host the `swagger-ui` assets yourself and point
`SwaggerCSSURL`, `SwaggerBundleURL`, and `SwaggerStandalonePresetURL` at your own
URLs.
:::

Schema references (`SchemaRef`) are emitted as `$ref` entries in the generated JSON and can point to components such as `#/components/schemas/User`. To make these references resolve correctly, provide the corresponding definitions via the `Components` config field. `Example` and `Examples` follow the OpenAPI specification's mutual exclusivity rule: when both are provided, `Examples` takes precedence and `Example` is omitted.

## Automatic Schema Inference

The `SchemaOf` helper generates an OpenAPI JSON Schema from a Go struct using reflection:

```go
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email" openapi:"format:email,description:User email address"`
}

// Generate schema automatically from struct
app.Post("/users", createUser).
    RequestBodyWithExample("Create user", true, openapi.SchemaOf(User{}), "", nil, nil, fiber.MIMEApplicationJSON).
    ResponseWithExample(201, "Created", openapi.SchemaOf(User{}), "", nil, nil, fiber.MIMEApplicationJSON)

// Or use it in Components for $ref reuse
app.Use(openapi.New(openapi.Config{
    Components: map[string]any{
        "schemas": map[string]any{
            "User": openapi.SchemaOf(User{}),
        },
    },
}))
```

### Supported types

| Go type | OpenAPI type |
|:--------|:-------------|
| `string` | `string` |
| `bool` | `boolean` |
| `int`, `int8`–`int64`, `uint`–`uint64` | `integer` |
| `float32`, `float64` | `number` |
| `time.Time` | `string` (format: `date-time`) |
| `[]byte` | `string` (format: `byte`, base64) |
| `[]T` / `[N]T` | `array` (items: schema of `T`) |
| `map[string]T` | `object` (additionalProperties: schema of `T`) |
| struct | `object` (properties from fields) |
| `*T` | schema of `T` (field not included in `required`) |
| `any` / `interface{}` | `{}` (accepts any value) |

Embedded structs and embedded pointers to structs are flattened into the parent
object (matching `encoding/json`). Self-referential or mutually recursive structs
are handled safely by emitting a bare `{"type": "object"}` where the cycle
repeats. Fields whose type has no JSON representation (channels, functions, etc.)
are skipped.

### Struct field tags

- **`json:"name"`** — sets the property name; `json:"-"` skips the field
- **`json:",omitempty"`** — makes the field optional (not in `required`)
- **`openapi:"description:text"`** — sets the property description
- **`openapi:"example:value"`** — sets the property example (auto-converted to the correct type)
- **`openapi:"format:fmt"`** — sets the format (e.g., `email`, `uuid`, `date-time`)
- **`openapi:"enum:a|b|c"`** — sets allowed enum values (pipe-separated)

Multiple `openapi` directives can be combined with commas:

```go
type Product struct {
    Status string `json:"status" openapi:"enum:active|inactive,description:Product status"`
}
```

A directive value may itself contain commas and colons (for example a
description); the only limitation is that a value cannot contain a comma
immediately followed by another directive key such as `,description:`.
