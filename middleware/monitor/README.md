# Monitor
Monitor middleware for [Fiber](https://github.com/gofiber/fiber) that reports server metrics, inspired by [express-status-monitor](https://github.com/RafalWilinski/express-status-monitor)

![](https://i.imgur.com/4NfRCDm.gif)

### Signatures
```go
func New() fiber.Handler
```

### Examples
Import the middleware package and assign it to a route.
```go
package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
)

func main() {
	app := fiber.New()
	
	app.Get("/dashboard", monitor.New())
	
	log.Fatal(app.Listen(":3000"))
}
```
