---
id: cors
---

# CORS

CORS (Cross-Origin Resource Sharing) is a middleware for [Fiber](https://github.com/gofiber/fiber) that allows servers to specify who can access its resources and how. It's not a security feature, but a way to relax the security model of web browsers for cross-origin requests. You can learn more about CORS on [Mozilla Developer Network](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS).

This middleware works by adding CORS headers to responses from your Fiber application. These headers specify which origins, methods, and headers are allowed for cross-origin requests. It also handles preflight requests, which are a CORS mechanism to check if the actual request is safe to send.

The middleware uses the `AllowOrigins` option to control which origins can make cross-origin requests. It supports single origin, multiple origins, subdomain matching, and wildcard origin. It also allows programmatic origin validation with the `AllowOriginsFunc` option.

To ensure that the provided `AllowOrigins` origins are correctly formatted, this middleware validates and normalizes them. It checks for valid schemes, i.e., HTTP or HTTPS, and it will automatically remove trailing slashes. If the provided origin is invalid, the middleware will panic.

When configuring CORS, it's important to avoid [common pitfalls](#common-pitfalls) like using a wildcard origin with credentials, being overly permissive with origins, and inadequate validation with `AllowOriginsFunc`. Misconfiguration can expose your application to various security risks.

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
| ExposeHeaders    | `string`                   | ExposeHeaders defines whitelist headers that clients are allowed to access.                                                                                                                                                                                                                                                                                        | `""`                               |
| MaxAge           | `int`                      | MaxAge indicates how long (in seconds) the results of a preflight request can be cached. If you pass MaxAge 0, the Access-Control-Max-Age header will not be added and the browser will use 5 seconds by default. To disable caching completely, pass MaxAge value negative. It will set the Access-Control-Max-Age header to 0.                                   | `0`                                |

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

- **Programmatic origin validation:**: The middleware also handles the `AllowOriginsFunc` option, which allows you to programmatically determine if an origin is allowed. If `AllowOriginsFunc` returns `true` for an origin, the middleware sets the `Access-Control-Allow-Origin` header to that origin.

The `AllowMethods` option controls which HTTP methods are allowed. For example, if `AllowMethods` is set to `"GET, POST"`, the middleware adds the header `Access-Control-Allow-Methods: GET, POST` to the response.

The `AllowHeaders` option specifies which headers are allowed in the actual request. The middleware sets the Access-Control-Allow-Headers response header to the value of `AllowHeaders`. This informs the client which headers it can use in the actual request.

The `AllowCredentials` option indicates whether the response to the request can be exposed when the credentials flag is true. If `AllowCredentials` is set to `true`, the middleware adds the header `Access-Control-Allow-Credentials: true` to the response. To prevent security vulnerabilities, `AllowCredentials` cannot be set to `true` if `AllowOrigins` is set to a wildcard (`*`). 

The `ExposeHeaders` option defines a whitelist of headers that clients are allowed to access. If `ExposeHeaders` is set to `"X-Custom-Header"`, the middleware adds the header `Access-Control-Expose-Headers: X-Custom-Header` to the response.

The `MaxAge` option indicates how long the results of a preflight request can be cached. If `MaxAge` is set to `3600`, the middleware adds the header `Access-Control-Max-Age: 3600` to the response.

The `Vary` header is used in this middleware to inform the client that the server's response to a request. For or both preflight and actual requests, the Vary header is set to `Access-Control-Request-Method` and `Access-Control-Request-Headers`. For preflight requests, the Vary header is also set to `Origin`. The `Vary` header is important for caching. It helps caches (like a web browser's cache or a CDN) determine when a cached response can be used in response to a future request, and when the server needs to be queried for a new response.

## Security Considerations

When configuring CORS, misconfiguration can potentially expose your application to various security risks. Here are some secure configurations and common pitfalls to avoid:

### Secure Configurations

- **Specify Allowed Origins**: Instead of using a wildcard (`*`), specify the exact domains allowed to make requests. For example, `AllowOrigins: "https://www.example.com, https://api.example.com"` ensures only these domains can make cross-origin requests to your application.

- **Use Credentials Carefully**: If your application needs to support credentials in cross-origin requests, ensure `AllowCredentials` is set to `true` and specify exact origins in `AllowOrigins`. Do not use a wildcard origin in this case.

- **Limit Exposed Headers**: Only whitelist headers that are necessary for the client-side application by setting `ExposeHeaders` appropriately. This minimizes the risk of exposing sensitive information.

### Common Pitfalls

- **Wildcard Origin with Credentials**: Setting `AllowOrigins` to `*` (a wildcard) and `AllowCredentials` to `true` is a common misconfiguration. This combination is prohibited because it can expose your application to security risks.

- **Overly Permissive Origins**: Specifying too many origins or using overly broad patterns (e.g., `https://*.example.com`) can inadvertently allow malicious sites to interact with your application. Be as specific as possible with allowed origins.

- **Inadequate `AllowOriginsFunc` Validation**: When using `AllowOriginsFunc` for dynamic origin validation, ensure the function includes robust checks to prevent unauthorized origins from being accepted. Overly permissive validation can lead to security vulnerabilities.

:::caution
Never allow `AllowOriginsFunc` to return `true` for all origins. This is particularly crucial when `AllowCredentials` is set to `true`. Doing so can bypass the restriction of using a wildcard origin with credentials, exposing your application to serious security threats.

If you need to allow wildcard origins, use `AllowOrigins` with a wildcard '*' instead of `AllowOriginsFunc`.
:::

Remember, the key to secure CORS configuration is specificity and caution. By carefully selecting which origins, methods, and headers are allowed, you can help protect your application from cross-origin attacks.