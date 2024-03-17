---
id: cors
---

# CORS

CORS middleware for [Fiber](https://github.com/gofiber/fiber) that can be used to enable [Cross-Origin Resource Sharing](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) with various options.

The middleware conforms to the `access-control-allow-origin` specification by parsing `AllowOrigins`. First, the middleware checks if there is a matching allowed origin for the requesting 'origin' header. If there is a match, it returns exactly one matching domain from the list of allowed origins.

For more control, `AllowOriginsFunc` can be used to programatically determine if an origin is allowed. If no match was found in `AllowOrigins` and if `AllowOriginsFunc` returns true then the 'access-control-allow-origin' response header is set to the 'origin' request header.

When defining your Origins make sure they are properly formatted. The middleware validates and normalizes the provided origins, ensuring they're in the correct format by checking for valid schemes (http or https), and removing any trailing slashes.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/middleware/cors"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Initialize default config
app.Use(cors.New())

// Or extend your config for customization
app.Use(cors.New(cors.Config{
    AllowOrigins: "https://gofiber.io, https://gofiber.net",
    AllowHeaders:  "Origin, Content-Type, Accept",
}))
```

Using the `AllowOriginsFunc` function. In this example any origin will be allowed via CORS.

For example, if a browser running on `http://localhost:3000` sends a request, this will be accepted and the `access-control-allow-origin` response header will be set to `http://localhost:3000`.

**Note: Using this feature is discouraged in production and it's best practice to explicitly set CORS origins via `AllowOrigins`.**

```go
app.Use(cors.New())

app.Use(cors.New(cors.Config{
    AllowOriginsFunc: func(origin string) bool {
        return os.Getenv("ENVIRONMENT") == "development"
    },
}))
```

**Note: The following configuration is considered insecure and will result in a panic.**

```go
app.Use(cors.New(cors.Config{
    AllowOrigins: "*",
    AllowCredentials: true,
}))
```

## Config

| Property         | Type                       | Description                                                                                                                                                                                                                                                                                                                                                        | Default                            |
|:-----------------|:---------------------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:-----------------------------------|
| Next             | `func(fiber.Ctx) bool`    | Next defines a function to skip this middleware when returned true.                                                                                                                                                                                                                                                                                                | `nil`                              |
| AllowOriginsFunc | `func(origin string) bool` | AllowOriginsFunc defines a function that will set the 'access-control-allow-origin' response header to the 'origin' request header when returned true. This allows for dynamic evaluation of allowed origins. Note if AllowCredentials is true, wildcard origins will be not have the 'access-control-allow-credentials' header set to 'true'.                     | `nil`                              |
| AllowOrigins     | `string`                   | AllowOrigin defines a comma separated list of origins that may access the resource.                                                                                                                                                                                                                                                                                | `"*"`                              |
| AllowMethods     | `string`                   | AllowMethods defines a list of methods allowed when accessing the resource. This is used in response to a preflight request.                                                                                                                                                                                                                                       | `"GET,POST,HEAD,PUT,DELETE,PATCH"` |
| AllowHeaders     | `string`                   | AllowHeaders defines a list of request headers that can be used when making the actual request. This is in response to a preflight request.                                                                                                                                                                                                                        | `""`                               |
| AllowCredentials | `bool`                     | AllowCredentials indicates whether or not the response to the request can be exposed when the credentials flag is true. When used as part of a response to a preflight request, this indicates whether or not the actual request can be made using credentials. Note: If true, AllowOrigins cannot be set to a wildcard ("*") to prevent security vulnerabilities. | `false`                            |
| ExposeHeaders    | `string`                   | ExposeHeaders defines a whitelist headers that clients are allowed to access.                                                                                                                                                                                                                                                                                      | `""`                               |
| MaxAge           | `int`                      | MaxAge indicates how long (in seconds) the results of a preflight request can be cached. If you pass MaxAge 0, Access-Control-Max-Age header will not be added and browser will use 5 seconds by default. To disable caching completely, pass MaxAge value negative. It will set the Access-Control-Max-Age header 0.                                              | `0`                                |

| `AllowPrivateNetwork`  | `bool`  | Indicates whether the `Access-Control-Allow-Private-Network` response header should be set to `true`, allowing requests from private networks. This aligns with modern security practices for web applications interacting with private networks. |


## Default Config

```go
var ConfigDefault = Config{
	Next:         nil,
	AllowOriginsFunc: nil,
	AllowOrigins: "*",
	AllowMethods: strings.Join([]string{
		fiber.MethodGet,
		fiber.MethodPost,
		fiber.MethodHead,
		fiber.MethodPut,
		fiber.MethodDelete,
		fiber.MethodPatch,
	}, ","),
	AllowHeaders:     "",
	AllowCredentials: false,
	ExposeHeaders:    "",
	MaxAge:           0,
	AllowPrivateNetwork: false,
}
```
