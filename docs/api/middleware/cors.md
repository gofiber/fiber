---
id: cors
---

# CORS

CORS middleware for [Fiber](https://github.com/gofiber/fiber) that can be used to enable [Cross-Origin Resource Sharing](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) with various options. CORS is not a security feature, it's a feature that relaxes the security model of web browsers, and it's only intended to be used by servers that need to allow cross-origin requests.

The middleware conforms to the `Access-Control-Allow-Origin` specification by parsing `AllowOrigins`. First, the middleware checks if there is a matching allowed origin for the requesting 'origin' header. If there is a match, the `Access-Control-Allow-Origin` response header will be set to the 'origin' request header. If there is no match, the `Access-Control-Allow-Origin` response header will not be set.

To ensure that the provided origins are correctly formatted, this middleware validates and normalizes them. It checks for valid schemes, i.e., HTTP or HTTPS, and it will automatically remove trailing slashes. If the provided origin is invalid, the middleware will panic.

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
    AllowHeaders: "Origin, Content-Type, Accept",
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
| AllowOriginsFunc | `func(origin string) bool` | `AllowOriginsFunc` is a function that dynamically determines whether to allow a request based on its origin. If this function returns `true`, the 'Access-Control-Allow-Origin' response header will be set to the request's 'origin' header. This function is only used if the request's origin doesn't match any origin in `AllowOrigins`.                       | `nil`                              |
| AllowOrigins     | `string`                   | AllowOrigins defines a comma separated list of origins that may access the resource. This supports subdomain matching, so you can use a value like "https://*.example.com" to allow any subdomain of example.com to submit requests.                                                                                                                               | `"*"`                              |
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

## Subdomain Matching  

The `AllowOrigins` configuration supports matching subdomains at any level. This means you can use a value like `"https://*.example.com"` to allow any subdomain of `example.com` to submit requests, including multiple subdomain levels such as `"https://sub.sub.example.com"`.

### Example  

If you want to allow CORS requests from any subdomain of `example.com`, including nested subdomains, you can configure the `AllowOrigins` like so:  

```go
app.Use(cors.New(cors.Config{
    AllowOrigins: "https://*.example.com",
}))
```

# How It Works

The CORS middleware works by adding the necessary CORS headers to responses from your Fiber application. These headers tell browsers what origins, methods, and headers are allowed for cross-origin requests.

When a request comes in, the middleware first checks if it's a preflight request, which is a CORS mechanism to determine whether the actual request is safe to send. Preflight requests are HTTP OPTIONS requests with specific CORS headers. If it's a preflight request, the middleware responds with the appropriate CORS headers and ends the request.

If it's not a preflight request, the middleware adds the CORS headers to the response and passes the request to the next handler. The actual CORS headers added depend on the configuration of the middleware.

The `AllowOrigins` option controls which origins can make cross-origin requests. The middleware handles different `AllowOrigins` configurations as follows:

- **Single origin:** If `AllowOrigins` is set to a single origin like `"http://www.example.com"`, and that origin matches the origin of the incoming request, the middleware adds the header `Access-Control-Allow-Origin: http://www.example.com` to the response.

- **Multiple origins:** If `AllowOrigins` is set to multiple origins like `"https://example.com, https://www.example.com"`, the middleware picks the origin that matches the origin of the incoming request.

- **Subdomain matching:** If `AllowOrigins` includes `"https://*.example.com"`, a subdomain like `https://sub.example.com` will be matched and `"https://sub.example.com"` will be the header. This will also match `https://sub.sub.example.com` and so on, but not `https://example.com`.

- **Wildcard origin:** If `AllowOrigins` is set to `"*"`, the middleware uses that and adds the header `Access-Control-Allow-Origin: *` to the response.

In all cases above, except the **Wildcard origin**, the middleware will either add the `Access-Control-Allow-Origin` header to the response matching the origin of the incoming request, or it will not add the header at all if the origin is not allowed.

The `AllowMethods` option controls which HTTP methods are allowed. For example, if `AllowMethods` is set to `"GET, POST"`, the middleware adds the header `Access-Control-Allow-Methods: GET, POST` to the response.

- **Programmatic origin validation:**: The middleware also handles the `AllowOriginsFunc` option, which allows you to programmatically determine if an origin is allowed. If `AllowOriginsFunc` returns `true` for an origin, the middleware sets the `Access-Control-Allow-Origin` header to that origin.

This way, the CORS middleware allows you to control how your Fiber application responds to cross-origin requests.

## Security Considerations

When configuring CORS, misconfiguration can potentially expose your application to various security risks. Here are some secure configurations and common pitfalls to avoid:

### Secure Configurations

- **Specify Allowed Origins**: Instead of using a wildcard (`*`), specify the exact domains allowed to make requests. For example, `AllowOrigins: "https://www.example.com, https://api.example.com"` ensures only these domains can make cross-origin requests to your application.

- **Use Credentials Carefully**: If your application needs to support credentials in cross-origin requests, ensure `AllowCredentials` is set to `true` and specify exact origins in `AllowOrigins`. Do not use a wildcard origin in this case.

- **Limit Exposed Headers**: Only whitelist headers that are necessary for the client-side application by setting `ExposeHeaders` appropriately. This minimizes the risk of exposing sensitive information.

### Common Pitfalls

- **Wildcard Origin with Credentials**: Setting `AllowOrigins` to `*` (a wildcard) and `AllowCredentials` to `true` is a common misconfiguration. This combination is prohibited because it can expose your application to security risks.

- **Overly Permissive Origins**: Specifying too many origins or using overly broad patterns (e.g., `https://*.example.com`) can inadvertently allow malicious sites to interact with your application. Be as specific as possible with allowed origins.

- **Neglecting `AllowOriginsFunc` Validation**: When using `AllowOriginsFunc` for dynamic origin validation, ensure the function includes robust checks to prevent unauthorized origins from being accepted. Simply returning `true` for all origins can bypass security protections.

:::caution
Do not use `AllowOriginsFunc` to return `true` for all origins. This is particularly crucial when `AllowCredentials` is set to `true`. Doing so can bypass the restriction of using a wildcard origin with credentials, exposing your application to serious security threats.
:::

Remember, the key to secure CORS configuration is specificity and caution. By carefully selecting which origins, methods, and headers are allowed, you can help protect your application from cross-origin attacks.