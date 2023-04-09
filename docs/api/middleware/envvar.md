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

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/envvar"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
app.Use("/expose/envvars", envvar.New())

// Or extend your config for customization
app.Use("/expose/envvars", envvar.New(
	envvar.Config{
		ExportVars:  map[string]string{"testKey": "", "testDefaultKey": "testDefaultVal"},
		ExcludeVars: map[string]string{"excludeKey": ""},
	}),
)
```

:::note
You will need to provide a path to use the envvar middleware.
:::

## Response

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
