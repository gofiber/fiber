# Swagger
Swagger middleware for [Fiber](https://github.com/gofiber/fiber). The middleware handles Swagger UI. 

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)


### Signatures
```go
func New(config ...Config) fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/swagger"
)
```

Then create a Fiber app with app := fiber.New().

After you initiate your Fiber app, you can use the following possibilities:

### Default Config

```go
app.Use(swagger.New(cfg))
```

### Custom Config

```go
cfg := Config{
    BasePath: "/", //swagger ui base path
    FilePath: "./docs/swagger.json"
}

app.Use(swagger.New(cfg))
```