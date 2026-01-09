---
id: envvar
---

# EnvVar

EnvVar middleware for [Fiber](https://github.com/gofiber/fiber) exposes environment variables with configurable options.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/envvar"
)
```

Once your Fiber app is initialized, configure the middleware as shown:

```go
// Initialize default config (exports no variables)
app.Use("/expose/envvars", envvar.New())

// Or extend your config for customization
app.Use("/expose/envvars", envvar.New(
    envvar.Config{
        ExportVars: map[string]string{"testKey": "", "testDefaultKey": "testDefaultVal"},
    }),
)
```

:::note
Mount the middleware on a path; it cannot be used without one.
:::

## Response

Sample response:

```json
{
  "vars": {
    "someEnvVariable": "someValue",
    "anotherEnvVariable": "anotherValue"
  }
}

```

## Config

| Property    | Type                | Description                                                                  | Default |
|:------------|:--------------------|:-----------------------------------------------------------------------------|:--------|
| ExportVars  | `map[string]string` | ExportVars lists the environment variables to expose. | `nil` |

## Default Config

```go
Config{}
// Exports no environment variables
```
