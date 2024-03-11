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
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/cors"
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
| Next             | `func(*fiber.Ctx) bool`    | Next defines a function to skip this middleware when returned true.                                                                                                                                                                                                                                                                                                | `nil`                              |
| AllowOriginsFunc | `func(origin string) bool` | AllowOriginsFunc defines a function that will set the 'access-control-allow-origin' response header to the 'origin' request header when returned true. This allows for dynamic evaluation of allowed origins. Note if AllowCredentials is true, wildcard origins will be not have the 'access-control-allow-credentials' header set to 'true'.                     | `nil`                              |
| AllowOrigins     | `string`                   | AllowOrigin defines a comma separated list of origins that may access the resource. This supports subdomain matching, so you can use a value like "https://.example.com" to allow any subdomain of example.com to submit requests.                                                                                                                                 | `"*"`                              |
| AllowMethods     | `string`                   | AllowMethods defines a list of methods allowed when accessing the resource. This is used in response to a preflight request.                                                                                                                                                                                                                                       | `"GET,POST,HEAD,PUT,DELETE,PATCH"` |
| AllowHeaders     | `string`                   | AllowHeaders defines a list of request headers that can be used when making the actual request. This is in response to a preflight request.                                                                                                                                                                                                                        | `""`                               |
| AllowCredentials | `bool`                     | AllowCredentials indicates whether or not the response to the request can be exposed when the credentials flag is true. When used as part of a response to a preflight request, this indicates whether or not the actual request can be made using credentials. Note: If true, AllowOrigins cannot be set to a wildcard ("*") to prevent security vulnerabilities. | `false`                            |
| ExposeHeaders    | `string`                   | ExposeHeaders defines a whitelist headers that clients are allowed to access.                                                                                                                                                                                                                                                                                      | `""`                               |
| MaxAge           | `int`                      | MaxAge indicates how long (in seconds) the results of a preflight request can be cached. If you pass MaxAge 0, Access-Control-Max-Age header will not be added and browser will use 5 seconds by default. To disable caching completely, pass MaxAge value negative. It will set the Access-Control-Max-Age header 0.                                              | `0`                                |

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
}
```

# How It Works

The CORS middleware works by adding the necessary CORS headers to responses from your Fiber application. These headers tell browsers what origins, methods, and headers are allowed for cross-origin requests.

When a request comes in, the middleware first checks if it's a preflight request, which is a CORS mechanism to determine whether the actual request is safe to send. Preflight requests are HTTP OPTIONS requests with specific CORS headers. If it's a preflight request, the middleware responds with the appropriate CORS headers and ends the request.

If it's not a preflight request, the middleware adds the CORS headers to the response and passes the request to the next handler. The actual CORS headers added depend on the configuration of the middleware.

The `AllowOrigins` option controls which origins can make cross-origin requests. The middleware handles different `AllowOrigins` configurations as follows:

- **Single origin:** If `AllowOrigins` is set to a single origin like `"http://www.example.com"`, and that origin matches the origin of the incoming request, the middleware adds the header `Access-Control-Allow-Origin: http://www.example.com` to the response.

- **Multiple origins:** If `AllowOrigins` is set to multiple origins like `"https://example.com, https://www.example.com"`, the middleware picks the origin that matches the origin of the incoming request.

- **Subdomain wildcard:** If `AllowOrigins` contains `"https://.example.com"`, a subdomain like `www` would be matched and `"https://www.example.com"` would be the header.

- **Wildcard origin:** If `AllowOrigins` is set to `"*"`, the middleware uses that and adds the header `Access-Control-Allow-Origin: *` to the response.

In all cases above, except the **Wildcard origin**, the middleware will either add the `Access-Control-Allow-Origin` header to the response matching the origin of the incoming request, or it will not add the header at all if the origin is not allowed.

The `AllowMethods` option controls which HTTP methods are allowed. For example, if `AllowMethods` is set to `"GET, POST"`, the middleware adds the header `Access-Control-Allow-Methods: GET, POST` to the response.

The middleware also handles the `AllowOriginsFunc` option, which allows you to programmatically determine if an origin is allowed. If `AllowOriginsFunc` returns `true` for an origin, the middleware sets the `Access-Control-Allow-Origin` header to that origin.

This way, the CORS middleware allows you to control how your Fiber application responds to cross-origin requests.

## Security Considerations

When configuring CORS, misconfiguration can potentially expose your application to various security risks.

- **Allowing all origins:** Setting `Access-Control-Allow-Origin` to `*` (a wildcard) allows any domain to make cross-origin requests. This can expose your application to cross-site request forgery (CSRF) attacks. It's generally safer to specify the exact domains allowed to make requests.

- **Allowing credentials:** The `Access-Control-Allow-Credentials` header indicates whether the browser should include credentials with cross-origin requests. If this is set to `true`, it can expose your application to attacks if combined with a wildcard `Access-Control-Allow-Origin`. We specifically prohibit this action in our CORS middleware, in line with the Fetch specification.

- **Exposing headers:** The `Access-Control-Expose-Headers` header lets the server whitelist headers that browsers are allowed to access. Be careful not to expose sensitive headers.

:::note
In our CORS middleware, we specifically prevent `Access-Control-Allow-Credentials` from being `true` when `Access-Control-Allow-Origin` is set to the wildcard (`*`). This prevents potential security risks associated with allowing credentials to be shared with all origins. 

When using `AllowOrigins`, a configuration check will cause a panic if `Access-Control-Allow-Credentials` is `true` and `Access-Control-Allow-Origin` is set to the wildcard.
:::

:::caution
Be extra careful when using `AllowOriginsFunc`. Make sure to properly validate the origin to prevent potential security risks.

When using `AllowOriginsFunc`, the `Access-Control-Allow-Origin` header will always be set to the origin header if the func returns `true`, which can bypass such protections if you return `true` in all situations. 
:::