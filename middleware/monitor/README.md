# Monitor
Monitor middleware for [Fiber](https://github.com/gofiber/fiber) that reports server metrics. Inspired by [express-status-monitor](https://github.com/RafalWilinski/express-status-monitor)

![](https://i.imgur.com/4NfRCDm.gif)


### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)

### Signatures
```go
func New() fiber.Handler
```

### Examples
Import the middleware package that is part of the Fiber web framework
```go
import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
)
```

Assign a path to display the monitor page
```go
// Default middleware
app.Get("/monitor", monitor.New())
```