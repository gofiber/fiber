---
id: monitor
---

# Monitor

Monitor middleware for [Fiber](https://github.com/gofiber/fiber) that reports server metrics, inspired by [express-status-monitor](https://github.com/RafalWilinski/express-status-monitor)

:::caution

Monitor is still in beta, API might change in the future!

:::

![](https://i.imgur.com/nHAtBpJ.gif)

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

After you initiate your Fiber app, you can use the following possibilities:
```go
// Initialize default config (Assign the middleware to /metrics)
app.Get("/metrics", monitor.New())

// Or extend your config for customization
// Assign the middleware to /metrics
// and change the Title to `MyService Metrics Page`
app.Get("/metrics", monitor.New(monitor.Config{Title: "MyService Metrics Page"}))
```
You can also access the API endpoint with
`curl -X GET -H "Accept: application/json" http://localhost:3000/metrics` which returns:
```json
{"pid":{ "cpu":0.4568381746582226, "ram":20516864,   "conns":3 },
 "os": { "cpu":8.759124087593099,  "ram":3997155328, "conns":44,
    "total_ram":8245489664, "load_avg":0.51 }}
```

## Config

| Property   | Type                    | Description                                                         | Default                                                                     |
|:-----------|:------------------------|:--------------------------------------------------------------------|:----------------------------------------------------------------------------|
| Title      | `string`                | Metrics page title                                                  | "Fiber Monitor"                                                             |
| Refresh    | `time.Duration`         | Refresh period                                                      | 3 seconds                                                                   |
| APIOnly    | `bool`                  | Whether the service should expose only the monitoring API           | false                                                                       |
| Next       | `func(*fiber.Ctx) bool` | Next defines a function to skip this middleware when returned true. | `nil`                                                                       |
| CustomHead | `string`                | Custom HTML Code to Head Section(Before End)                        | empty                                                                       |
| FontURL    | `string`                | FontURL for specify font resource path or URL                       | "https://fonts.googleapis.com/css2?family=Roboto:wght@400;900&display=swap" |
| ChartJsURL | `string`                | ChartJsURL for specify ChartJS library path or URL                  | "https://cdn.jsdelivr.net/npm/chart.js@2.9/dist/Chart.bundle.min.js"        |

## Default Config

```go
var ConfigDefault = Config{
	Title:      defaultTitle,
	Refresh:    defaultRefresh,
	FontURL:    defaultFontURL,
	ChartJsURL: defaultChartJSURL,
	CustomHead: defaultCustomHead,
	APIOnly:    false,
	Next:       nil,
	index: newIndex(viewBag{
		defaultTitle,
		defaultRefresh,
		defaultFontURL,
		defaultChartJSURL,
		defaultCustomHead,
	}),
}
```
