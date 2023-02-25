---
id: envvar
title: EnvVar
---

EnvVar middleware for [Fiber](https://github.com/gofiber/fiber) that can be used to expose environment variables with various options.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

First import the middleware from Fiber,

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/envvar"
)
```

Then create a Fiber app with `app := fiber.New()`.

**Note**: You need to provide a path to use envvar middleware.

### Default Config

```go
app.Use("/expose/envvars", envvar.New())
```

### Custom Config

```go
app.Use("/expose/envvars", envvar.New(envvar.Config{
    ExportVars:  map[string]string{"testKey": "", "testDefaultKey": "testDefaultVal"},
    ExcludeVars: map[string]string{"excludeKey": ""},
}))
```

### Response

Http response contract:
```
{
  "vars": {
    "someEnvVariable": "someValue",
    "anotherEnvVariable": "anotherValue",
  }
}

```

## Config

```go
// Config defines the config for middleware.
type Config struct {
    // ExportVars specifies the environment variables that should export
    ExportVars map[string]string
    // ExcludeVars specifies the environment variables that should not export
    ExcludeVars map[string]string
}

```

## Default Config

```go
Config{}
```
