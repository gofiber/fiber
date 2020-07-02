# Recover
Recover middleware recovers from panics anywhere in the stack chain and handles the control to the centralized [ErrorHandler](https://docs.gofiber.io/error-handling).

### Example
Import the middleware package that is part of the Fiber web framework
```go
import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/middleware"
)
```

After you initiate your Fiber app, you can use the following possibilities:
```go
func main() {
  app := fiber.New()
    
  // Default recover
  app.Use(middleware.Recover())

  // ...
}
```

### Signatures
```go
func Recover() fiber.Handler {}
```