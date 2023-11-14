---
id: loadshed
---

# LoadShed

The LoadShed middleware for [Fiber](https://github.com/gofiber/fiber) is designed to help manage server load by shedding requests based on certain load criteria.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

To use the LoadShed middleware in your Fiber application, import it and apply it to your Fiber app. Here's an example:

```go
package main

import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/loadshed"
)

func main() {
  app := fiber.New()

  // Configure and use LoadShed middleware
  app.Use(loadshed.New(loadshed.Config{
    Criteria: &loadshed.CPULoadCriteria{
      LowerThreshold: 0.75, // Set your own lower threshold
      UpperThreshold: 0.90, // Set your own upper threshold
      Interval:       10 * time.Second,
      Getter:         &loadshed.DefaultCPUPercentGetter{},
    },
  }))

  app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("Welcome!")
  })

  app.Listen(":3000")
}
```

## Config

The LoadShed middleware in Fiber offers various configuration options to tailor the load shedding behavior according to the needs of your application.

| Property | Type                    | Description                                          | Default                 |
| :------- | :---------------------- | :--------------------------------------------------- | :---------------------- |
| Next     | `func(*fiber.Ctx) bool` | Function to skip this middleware when returned true. | `nil`                   |
| Criteria | `LoadCriteria`          | Interface for defining load shedding criteria.       | `&CPULoadCriteria{...}` |

## LoadCriteria

LoadCriteria is an interface in the LoadShed middleware that defines the criteria for determining when to shed load in the system. Different implementations of this interface can use various metrics and algorithms to decide when and how to shed incoming requests to maintain system performance.

### CPULoadCriteria

`CPULoadCriteria` is an implementation of the `LoadCriteria` interface, using CPU load as the metric for determining whether to shed requests.

#### Properties

| Property       | Type               | Description                                                                                                                           |
| :------------- | :----------------- | :------------------------------------------------------------------------------------------------------------------------------------ |
| LowerThreshold | `float64`          | The lower CPU usage threshold as a fraction (0.0 to 1.0). Requests are considered for shedding when CPU usage exceeds this threshold. |
| UpperThreshold | `float64`          | The upper CPU usage threshold as a fraction (0.0 to 1.0). All requests are shed when CPU usage exceeds this threshold.                |
| Interval       | `time.Duration`    | The time interval over which the CPU usage is averaged for decision making.                                                           |
| Getter         | `CPUPercentGetter` | Interface to retrieve CPU usage percentages.                                                                                          |

#### How It Works

`CPULoadCriteria` determines the load on the system based on CPU usage and decides whether to shed incoming requests. It operates on the following principles:

- **CPU Usage Measurement**: It measures the CPU usage over a specified interval.
- **Thresholds**: Utilizes `LowerThreshold` and `UpperThreshold` values to decide when to start shedding requests.
- **Proportional Rejection Probability**:
  - **Below `LowerThreshold`**: No requests are rejected, as the system is considered under acceptable load.
  - **Between `LowerThreshold` and `UpperThreshold`**: The probability of rejecting a request increases as the CPU usage approaches the `UpperThreshold`. This is calculated using the formula:
    ```plaintext
    rejectionProbability := (cpuUsage - LowerThreshold*100) / (UpperThreshold - LowerThreshold)
    ```
  - **Above `UpperThreshold`**: All requests are rejected to prevent system overload.

This mechanism ensures that the system can adaptively manage its load, maintaining stability and performance under varying traffic conditions.

## Default Config

This is the default configuration for `LoadCriteria` in the LoadShed middleware.

```go
var ConfigDefault = Config{
  Next: nil,
  Criteria: &CPULoadCriteria{
    LowerThreshold: 0.90,  // 90% CPU usage as the start point for considering shedding
    UpperThreshold: 0.95,  // 95% CPU usage as the point where all requests are shed
    Interval:       10 * time.Second,  // CPU usage is averaged over 10 seconds
    Getter:         &DefaultCPUPercentGetter{},  // Default method for getting CPU usage
  },
}
```
