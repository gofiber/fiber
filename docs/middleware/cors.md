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
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/cors"
)
```

After you initiate your Fiber app, you can use the following possibilities:

### Basic usage

To use the default configuration, simply use `cors.New()`. This will allow wildcard origins '*', all methods, no credentials, and no headers or exposed headers.

```go
app.Use(cors.New())
```

### Custom configuration (specific origins, headers, etc.)

```go
// Initialize default config
app.Use(cors.New())

// Or extend your config for customization
app.Use(cors.New(cors.Config{
    AllowOrigins: []string{"https://gofiber.io", "https://gofiber.net"},
    AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
}))
```

### Dynamic origin validation

You can use `AllowOriginsFunc` to programmatically determine whether to allow a request based on its origin. This is useful when you need to validate origins against a database or other dynamic sources. The function should return `true` if the origin is allowed, and `false` otherwise.

Be sure to review the [security considerations](#security-considerations) when using `AllowOriginsFunc`.

:::caution
Never allow `AllowOriginsFunc` to return `true` for all origins. This is particularly crucial when `AllowCredentials` is set to `true`. Doing so can bypass the restriction of using a wildcard origin with credentials, exposing your application to serious security threats.

If you need to allow wildcard origins, use `AllowOrigins` with a wildcard `"*"` instead of `AllowOriginsFunc`.
:::

```go
// dbCheckOrigin checks if the origin is in the list of allowed origins in the database.
func dbCheckOrigin(db *sql.DB, origin string) bool {
    // Placeholder query - adjust according to your database schema and query needs
    query := "SELECT COUNT(*) FROM allowed_origins WHERE origin = $1"
    
    var count int
    err := db.QueryRow(query, origin).Scan(&count)
    if err != nil {
      // Handle error (e.g., log it); for simplicity, we return false here
      return false
    }
    
    return count > 0
}

// ...

app.Use(cors.New(cors.Config{
    AllowOriginsFunc: func(origin string) bool {
      return dbCheckOrigin(db, origin)
    },
}))
```

### Prohibited usage

The following example is prohibited because it can expose your application to security risks. It sets `AllowOrigins` to `"*"` (a wildcard) and `AllowCredentials` to `true`.

```go
app.Use(cors.New(cors.Config{
    AllowOrigins: []string{"*"},
    AllowCredentials: true,
}))
```

This will result in the following panic:

```text
panic: [CORS] Configuration error: When 'AllowCredentials' is set to true, 'AllowOrigins' cannot contain a wildcard origin '*'. Please specify allowed origins explicitly or adjust 'AllowCredentials' setting.
```

## Config

| Property             | Type                        | Description                                                                                                                                                                                                                                                                                                                                                          | Default                                 |
|:---------------------|:----------------------------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:----------------------------------------|
| AllowCredentials     | `bool`                      | AllowCredentials indicates whether or not the response to the request can be exposed when the credentials flag is true. When used as part of a response to a preflight request, this indicates whether or not the actual request can be made using credentials. Note: If true, AllowOrigins cannot be set to a wildcard (`"*"`) to prevent security vulnerabilities. | `false`                                 |
| AllowHeaders         | `[]string`                  | AllowHeaders defines a list of request headers that can be used when making the actual request. This is in response to a preflight request.                                                                                                                                                                                                                          | `[]`                                    |
| AllowMethods         | `[]string`                  | AllowMethods defines a list of methods allowed when accessing the resource. This is used in response to a preflight request.                                                                                                                                                                                                                                         | `"GET, POST, HEAD, PUT, DELETE, PATCH"` |
| AllowOrigins         | `[]string`                  | AllowOrigins defines a list of origins that may access the resource. This supports subdomain matching, so you can use a value like "https://*.example.com" to allow any subdomain of example.com to submit requests. If the special wildcard `"*"` is present in the list, all origins will be allowed.                                                              | `["*"]`                                 |
| AllowOriginsFunc     | `func(origin string) bool`  | `AllowOriginsFunc` is a function that dynamically determines whether to allow a request based on its origin. If this function returns `true`, the 'Access-Control-Allow-Origin' response header will be set to the request's 'origin' header. This function is only used if the request's origin doesn't match any origin in `AllowOrigins`.                         | `nil`                                   |
| AllowPrivateNetwork  | `bool`                      | Indicates whether the `Access-Control-Allow-Private-Network` response header should be set to `true`, allowing requests from private networks. This aligns with modern security practices for web applications interacting with private networks.                                                                                                                    | `false`                                 |
| ExposeHeaders        | `string`                    | ExposeHeaders defines an allowlist of headers that clients are allowed to access.                                                                                                                                                                                                                                                                                    | `[]`                                    |
| MaxAge               | `int`                       | MaxAge indicates how long (in seconds) the results of a preflight request can be cached. If you pass MaxAge 0, the Access-Control-Max-Age header will not be added and the browser will use 5 seconds by default. To disable caching completely, pass MaxAge value negative. It will set the Access-Control-Max-Age header to 0.                                     | `0`                                     |
| Next                 | `func(fiber.Ctx) bool`      | Next defines a function to skip this middleware when returned true.                                                                                                                                                                                                                                                                                                  | `nil`                                   |

:::note
If AllowOrigins is a zero value `[]string{}`, and AllowOriginsFunc is provided, the middleware will not default to allowing all origins with the wildcard value "*". Instead, it will rely on the AllowOriginsFunc to dynamically determine whether to allow a request based on its origin. This provides more flexibility and control over which origins are allowed.
:::

## Default Config

```go
var ConfigDefault = Config{
    Next:             nil,
    AllowOriginsFunc: nil,
    AllowOrigins:     []string{"*"},
    AllowMethods: []string{
        fiber.MethodGet,
        fiber.MethodPost,
        fiber.MethodHead,
        fiber.MethodPut,
        fiber.MethodDelete,
        fiber.MethodPatch,
    },
    AllowHeaders:        []string{},
    AllowCredentials:    false,
    ExposeHeaders:       []string{},
    MaxAge:              0,
    AllowPrivateNetwork: false,
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

## How It Works

The CORS middleware works by adding the necessary CORS headers to responses from your Fiber application. These headers tell browsers what origins, methods, and headers are allowed for cross-origin requests.

When a request comes in, the middleware first checks if it's a preflight request, which is a CORS mechanism to determine whether the actual request is safe to send. Preflight requests are HTTP OPTIONS requests with specific CORS headers. If it's a preflight request, the middleware responds with the appropriate CORS headers and ends the request.

:::note
Preflight requests are typically sent by browsers before making actual cross-origin requests, especially for methods other than GET or POST, or when custom headers are used.

A preflight request is an HTTP OPTIONS request that includes the `Origin`, `Access-Control-Request-Method`, and optionally `Access-Control-Request-Headers` headers. The browser sends this request to check if the server allows the actual request method and headers.
:::

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

The `ExposeHeaders` option defines an allowlist of headers that clients are allowed to access. If `ExposeHeaders` is set to `"X-Custom-Header"`, the middleware adds the header `Access-Control-Expose-Headers: X-Custom-Header` to the response.

The `MaxAge` option indicates how long the results of a preflight request can be cached. If `MaxAge` is set to `3600`, the middleware adds the header `Access-Control-Max-Age: 3600` to the response.

The `Vary` header is used in this middleware to inform the client that the server's response to a request. For or both preflight and actual requests, the Vary header is set to `Access-Control-Request-Method` and `Access-Control-Request-Headers`. For preflight requests, the Vary header is also set to `Origin`. The `Vary` header is important for caching. It helps caches (like a web browser's cache or a CDN) determine when a cached response can be used in response to a future request, and when the server needs to be queried for a new response.

## Infrastructure Considerations

When deploying Fiber applications behind infrastructure components like CDNs, API gateways, load balancers, or reverse proxies, you have two main options for handling CORS:

### Option 1: Use Infrastructure-Level CORS (Recommended)

**For most production deployments, it's often preferable to handle CORS at the infrastructure level** rather than in your Fiber application. This approach offers several advantages:

- **Better Performance**: CORS headers are added at the edge, closer to the client
- **Reduced Server Load**: Preflight requests are handled without reaching your application
- **Centralized Configuration**: Manage CORS policies alongside other infrastructure settings
- **Built-in Caching**: Infrastructure providers optimize CORS response caching

**Common infrastructure CORS solutions:**
- **CDNs**: CloudFront, CloudFlare, Azure CDN - handle CORS at edge locations
- **API Gateways**: AWS API Gateway, Google Cloud API Gateway - centralized CORS management
- **Load Balancers**: Application Load Balancers with CORS rules
- **Reverse Proxies**: Nginx, Apache with CORS modules

If using infrastructure-level CORS, **disable Fiber's CORS middleware** to avoid conflicts:

```go
// Don't use both - choose one approach
// app.Use(cors.New()) // Remove this line when using infrastructure CORS
```

### Option 2: Application-Level CORS (Fiber Middleware)

Use Fiber's CORS middleware when you need:
- **Dynamic origin validation** based on application logic
- **Fine-grained control** over CORS policies per route
- **Integration with application state** (database-driven origins, etc.)
- **Development environments** where infrastructure CORS isn't available

If choosing this approach, ensure that **all CORS headers reach your Fiber application unchanged**.

### Required Headers for CORS Preflight Requests

For CORS preflight requests to work correctly, these headers **must not be stripped or modified by caching layers**:

- `Origin` - Required to identify the requesting origin
- `Access-Control-Request-Method` - Required to identify the HTTP method for the actual request
- `Access-Control-Request-Headers` - Optional, contains custom headers the actual request will use
- `Access-Control-Request-Private-Network` - Optional, for private network access requests

:::warning Critical Preflight Requirement
If the `Access-Control-Request-Method` header is missing from an OPTIONS request, Fiber will not recognize them as CORS preflight requests. Instead, they'll be treated as regular OPTIONS requests, which typically return `405 Method Not Allowed` since most applications don't define explicit OPTIONS handlers.
:::

### CORS Response Headers (Set by Fiber)

The middleware sets these response headers based on your configuration:

**For all CORS requests:**
- `Access-Control-Allow-Origin` - Set to the allowed origin or "*"
- `Access-Control-Allow-Credentials` - Set to "true" when `AllowCredentials: true`
- `Access-Control-Expose-Headers` - Lists headers the client can access
- `Vary` - Set to "Origin" (unless wildcard origins are used)

**For preflight responses only:**
- `Access-Control-Allow-Methods` - Lists allowed HTTP methods
- `Access-Control-Allow-Headers` - Lists allowed request headers (or echoes the request)
- `Access-Control-Max-Age` - Cache duration for preflight results (if MaxAge > 0)
- `Access-Control-Allow-Private-Network` - Set to "true" when private network access is allowed
- `Vary` - Set to "Access-Control-Request-Method, Access-Control-Request-Headers, Origin"

### Common Infrastructure Issues

**CDNs (CloudFront, CloudFlare, etc.)**: 
- Configure cache policies to forward all CORS headers
- Ensure OPTIONS requests are not cached inappropriately or cache them correctly with proper Vary headers
- Don't strip or modify CORS request headers

**API Gateways**: 
- Choose either gateway-level CORS OR application-level CORS, not both
- If using gateway CORS, disable Fiber's CORS middleware
- If forwarding to Fiber, ensure all headers pass through unchanged

**Load Balancers/Reverse Proxies**: 
- Preserve all HTTP headers, especially CORS-related ones
- Don't modify or strip `Origin`, `Access-Control-Request-*` headers

**WAFs/Security Services**: 
- Whitelist CORS headers in security rules
- Ensure OPTIONS requests with CORS headers aren't blocked

### Debugging CORS Issues

Add this middleware **before** your CORS configuration to debug what headers Fiber receives:

```go
app.Use(func(c *fiber.Ctx) error {
    if c.Method() == "OPTIONS" {
        fmt.Printf("OPTIONS %s\n", c.Path())
        fmt.Printf("  Origin: %s\n", c.Get("Origin"))
        fmt.Printf("  Access-Control-Request-Method: %s\n", c.Get("Access-Control-Request-Method"))
        fmt.Printf("  Access-Control-Request-Headers: %s\n", c.Get("Access-Control-Request-Headers"))
    }
    return c.Next()
})

app.Use(cors.New(cors.Config{
    AllowOrigins: []string{"https://yourdomain.com"},
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
}))
```

Test CORS preflight directly with curl:

```bash
# Test preflight request
curl -X OPTIONS https://your-app.com/api/test \
  -H "Origin: https://yourdomain.com" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -v

# Test simple CORS request
curl -X GET https://your-app.com/api/test \
  -H "Origin: https://yourdomain.com" \
  -v
```

### Caching Considerations

The middleware sets appropriate `Vary` headers to ensure proper caching:

- **Non-wildcard origins**: `Vary: Origin` is set to cache responses per origin
- **Preflight requests**: `Vary: Access-Control-Request-Method, Access-Control-Request-Headers, Origin`
- **OPTIONS without preflight headers**: `Vary: Origin` to avoid cache poisoning

Ensure your infrastructure respects these `Vary` headers for correct caching behavior.

### Choosing the Right Approach

| Scenario | Recommended Approach |
|----------|---------------------|
| Production with CDN/API Gateway | Infrastructure-level CORS |
| Dynamic origin validation needed | Application-level CORS |
| Microservices with different CORS policies | Application-level CORS |
| Simple static origins | Infrastructure-level CORS |
| Development/testing | Application-level CORS |
| High traffic applications | Infrastructure-level CORS |

:::tip Infrastructure CORS Configuration
Most cloud providers offer comprehensive CORS documentation:
- [AWS CloudFront CORS](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/header-caching.html#header-caching-web-cors)
- [Google Cloud CORS](https://cloud.google.com/storage/docs/cross-origin)
- [Azure CDN CORS](https://docs.microsoft.com/en-us/azure/cdn/cdn-cors)
- [CloudFlare CORS](https://developers.cloudflare.com/fundamentals/get-started/reference/http-request-headers/#cf-connecting-ip)

Configure CORS at the infrastructure level when possible for optimal performance and reduced complexity.
:::

## Security Considerations

When configuring CORS, misconfiguration can potentially expose your application to various security risks. Here are some secure configurations and common pitfalls to avoid:

### Secure Configurations

- **Specify Allowed Origins**: Instead of using a wildcard (`"*"`), specify the exact domains allowed to make requests. For example, `AllowOrigins: "https://www.example.com, https://api.example.com"` ensures only these domains can make cross-origin requests to your application.

- **Use Credentials Carefully**: If your application needs to support credentials in cross-origin requests, ensure `AllowCredentials` is set to `true` and specify exact origins in `AllowOrigins`. Do not use a wildcard origin in this case.

- **Limit Exposed Headers**: Only allowlist headers that are necessary for the client-side application by setting `ExposeHeaders` appropriately. This minimizes the risk of exposing sensitive information.

### Common Pitfalls

- **Wildcard Origin with Credentials**: Setting `AllowOrigins` to `"*"` (a wildcard) and `AllowCredentials` to `true` is a common misconfiguration. This combination is prohibited because it can expose your application to security risks.

- **Overly Permissive Origins**: Specifying too many origins or using overly broad patterns (e.g., `https://*.example.com`) can inadvertently allow malicious sites to interact with your application. Be as specific as possible with allowed origins.

- **Inadequate `AllowOriginsFunc` Validation**: When using `AllowOriginsFunc` for dynamic origin validation, ensure the function includes robust checks to prevent unauthorized origins from being accepted. Overly permissive validation can lead to security vulnerabilities. Never allow `AllowOriginsFunc` to return `true` for all origins. This is particularly crucial when `AllowCredentials` is set to `true`. Doing so can bypass the restriction of using a wildcard origin with credentials, exposing your application to serious security threats. If you need to allow wildcard origins, use `AllowOrigins` with a wildcard `"*"` instead of `AllowOriginsFunc`.

Remember, the key to secure CORS configuration is specificity and caution. By carefully selecting which origins, methods, and headers are allowed, you can help protect your application from cross-origin attacks.
