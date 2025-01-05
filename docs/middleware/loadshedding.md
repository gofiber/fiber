---
id: loadshedding
---

# Load Shedding

The **Load Shedding** middleware for [Fiber](https://github.com/gofiber/fiber) helps maintain server stability by applying request-processing timeouts. It prevents resource exhaustion by gracefully rejecting requests that exceed a specified time limit. This is particularly beneficial in high-traffic scenarios, where preventing overload is crucial to sustaining service availability and performance.

## Features

- **Request Timeout Enforcement**: Automatically terminates any request that exceeds the configured processing time.
- **Customizable Response**: Enables you to define a specialized response for timed-out requests.
- **Exclusion Logic**: Lets you skip load-shedding for specific requests, such as health checks or other critical endpoints.
- **Enhanced Stability**: Helps avoid server crashes or sluggish performance under heavy load by shedding excess requests.

## Use Cases

- **High-Traffic Scenarios**: Safeguard critical resources by rejecting overly long or resource-intensive requests.
- **Health Check Protection**: Exclude monitoring endpoints (e.g., `/health`) to ensure uninterrupted external checks.
- **Dynamic Load Management**: Utilize exclusion logic to adjust load-shedding behavior for specific routes or request types.

---

## Signature

```go
func New(timeout time.Duration, loadSheddingHandler fiber.Handler, exclude func(fiber.Ctx) bool) fiber.Handler
```

## Config

| Property              | Type               | Description                                                                      | Default   |
|-----------------------|--------------------|----------------------------------------------------------------------------------|-----------|
| `timeout`            | `time.Duration`    | The maximum allowed processing time for a request.                               | Required  |
| `loadSheddingHandler`| `fiber.Handler`    | The handler invoked for requests that exceed the `timeout`.                      | Required  |
| `exclude`            | `func(fiber.Ctx) bool` | Optional function to exclude certain requests from load-shedding logic.       | `nil`     |

## Example Usage

Import the middleware and configure it within your Fiber application:

```go
import (
    "time"
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/loadshedding"
)

func main() {
    app := fiber.New()

    // Basic usage with a 5-second timeout
    app.Use(loadshedding.New(
        5*time.Second,
        func(c fiber.Ctx) error {
            return c.Status(fiber.StatusServiceUnavailable).SendString("Service unavailable due to high load")
        },
        nil,
    ))

    // Advanced usage with an exclusion function for specific endpoints
    app.Use(loadshedding.New(
        3*time.Second,
        func(c fiber.Ctx) error {
            return c.Status(fiber.StatusServiceUnavailable).SendString("Request timed out")
        },
        func(c fiber.Ctx) bool {
            // Exclude /health from load-shedding
            return c.Path() == "/health"
        },
    ))

    app.Get("/", func(c fiber.Ctx) error {
        // Simulating a long-running request
        time.Sleep(4 * time.Second)
        return c.SendString("Hello, world!")
    })

    app.Listen(":3000")
}
```
