---
id: loadshedding
---

# Load Shedding

Load Shedding middleware for [Fiber](https://github.com/gofiber/fiber) is designed to enhance server stability by
enforcing timeouts on request processing. It helps manage server load effectively by gracefully handling requests that
exceed a specified timeout duration. This is especially useful in high-traffic scenarios, where preventing server
overload is critical to maintaining service availability and performance.

## Features

- **Request Timeout Enforcement**: Ensures that no request exceeds the specified processing time.
- **Customizable Response**: Allows you to define a specific response for timed-out requests.
- **Exclusion Criteria**: Provides flexibility to exclude specific requests from load-shedding logic.
- **Improved Stability**: Helps prevent server crashes under heavy load by shedding excess requests.

## Use Cases

- **High-Traffic Scenarios**: Protect critical resources by shedding long-running or resource-intensive requests.
- **Health Check Protection**: Exclude endpoints like `/health` to ensure critical monitoring remains unaffected.
- **Dynamic Load Management**: Use exclusion logic to prioritize or deprioritize requests dynamically.

---

## Signature

```go
func New(timeout time.Duration, loadSheddingHandler fiber.Handler, exclude func (fiber.Ctx) bool) fiber.Handler
```

## Config

| Property                          | Type                                | Description                                                                                    | Default  |
|:----------------------|:------------------------------------|:-----------------------------------------------------------------------------------------------|:---------|
| `timeout`             | `time.Duration`                     | Maximum allowed processing time for a request.                                                 | Required |
| `loadSheddingHandler` | `fiber.Handler`                     | Handler invoked for requests that exceed the timeout.                                          | Required |
| `exclude`             | `func(fiber.Ctx) bool`              | Optional function to exclude specific requests from load-shedding logic.                       | `nil`    |

## Example Usage

Import the middleware package and integrate it into your Fiber application:

```go
import (
    "time"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/loadshedding"
)

func main() {
    app := fiber.New()

    // Basic usage with a 5-second timeout
    app.Use(loadshedding.New(5*time.Second, func(c fiber.Ctx) error {
        return c.Status(fiber.StatusServiceUnavailable).SendString("Service unavailable due to high load")
    }, nil))

    // Advanced usage with exclusion logic for specific endpoints
    app.Use(loadshedding.New(3*time.Second, func(c fiber.Ctx) error {
        return c.Status(fiber.StatusServiceUnavailable).SendString("Request timed out")
    }, func(c fiber.Ctx) bool {
        return c.Path() == "/health"
    }))

    app.Get("/", func(c fiber.Ctx) error {
        time.Sleep(4 * time.Second) // Simulating a long-running request
        return c.SendString("Hello, world!")
    })

    app.Listen(":3000")
}
```
