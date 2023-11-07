---
id: csrf
---

# CSRF

The CSRF middleware for [Fiber](https://github.com/gofiber/fiber) provides protection against [Cross-Site Request Forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) (CSRF) attacks using tokens. These tokens verify requests made using methods other than those defined as "safe" by [RFC9110#section-9.2.1](https://datatracker.ietf.org/doc/html/rfc9110.html#section-9.2.1) (Safe-Methods: GET, HEAD, OPTIONS, and TRACE). If a potential attack is detected this middleware will, by default, return a 403 Forbidden error.

This middleware can be used with or without a user session and offers two token validation patterns. In addition, it implements strict referer checking for HTTPS requests, ensuring the security of your application. For HTTPS requests, even if a subdomain can set or modify cookies on your domain, it can't force a user to post to your application since that request won't come from your own exact domain.

## Token Generation

CSRF tokens are generated on 'safe' requests and when the existing token has expired or hasn't been set yet. If `SingleUseToken` is `true`, a new token is generated after each use. Retrieve the CSRF token using `c.Locals(contextKey)`, where `contextKey` is defined in the configuration.

## Security Considerations

This middleware is designed to protect against CSRF attacks but does not protect against other attack vectors, such as XSS. It should be used in combination with other security measures.

:::danger
Never use 'safe' methods to mutate data, for example, never use a GET request to modify a resource. This middleware will not protect against CSRF attacks on 'safe' methods.
:::

### Token Validation Patterns

#### Double Submit Cookie Pattern (Default)

In the default configuration, the middleware generates and stores tokens using the `fiber.Storage` interface. These tokens are not associated with a user session, and a Double Submit Cookie pattern is used to validate the token. The token is stored in a cookie and sent as a header on requests. The middleware compares the cookie value with the header value to validate the token. This is a secure pattern that does not require a user session.

When using this pattern, it's important to delete the token when the authorization status changes, see: [Token Lifecycle](#token-lifecycle) for more information.

:::caution
When using this method, it's important to set the `CookieSameSite` option to `Lax` or `Strict` and ensure that the Extractor is not `CsrfFromCookie`, and KeyLookup is not `cookie:<name>`.
:::

:::note
When using this pattern, this middleware uses our [Storage](https://github.com/gofiber/storage) package to support various databases through a single interface. The default configuration for Storage saves data to memory. See [Custom Storage/Database](#custom-storagedatabase) for customizing the storage.
:::

#### Synchronizer Token Pattern (Session)

When using this middleware with a user session, the middleware can be configured to store the token in the session. This method is recommended when using a user session, as it is generally more secure than the Double Submit Cookie Pattern.

When using this pattern it's important to regenerate the session when the authorization status changes, this will also delete the token. See: [Token Lifecycle](#token-lifecycle) for more information.

:::caution
When using this method, pre-sessions are required and will be created if a session is not already present. This means the middleware will create a session for every safe request, even if the request does not require a session. Therefore, the existence of a session should not be used to indicate that a user is logged in or authenticated; a session value should be used for this purpose.
:::

### Defense In Depth

When using this middleware, it's recommended to serve your pages over HTTPS, set the `CookieSecure` option to `true`, and set the `CookieSameSite` option to `Lax` or `Strict`. This ensures that the cookie is only sent over HTTPS and not on requests from external sites.

:::note
Cookie prefixes __Host- and __Secure- can be used to further secure the cookie. However, these prefixes are not supported by all browsers and there are some other limitations. See [MDN#Set-Cookie#cookie_prefixes](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#cookie_prefixes) for more information.

To use these prefixes, set the `CookieName` option to `__Host-csrf_` or `__Secure-csrf_`.
:::

### Referer Checking

For HTTPS requests, this middleware performs strict referer checking. Even if a subdomain can set or modify cookies on your domain, it can't force a user to post to your application since that request won't come from your own exact domain.

:::caution
Referer checking is required for https requests protected by CSRF. All modern browsers will automatically include the Referer header in requests, including those made with the JS Fetch API. However, if you are using this middleware with a custom client you must ensure that the client sends a valid Referer header.
:::


### Token Lifecycle

Tokens are valid until they expire or until they are deleted. By default, tokens are valid for 1 hour, and each subsequent request extends the expiration by 1 hour. The token only expires if the user doesn't make a request for the duration of the expiration time.

#### Token Reuse

By default, tokens may be used multiple times. If you want to delete the token after it has been used, you can set the `SingleUseToken` option to `true`. This will delete the token after it has been used, and a new token will be generated on the next request.

:::info
Using `SingleUseToken` comes with usability trade-offs and is not enabled by default. It can interfere with the user experience if the user has multiple tabs open or uses the back button.
:::

#### Deleting Tokens

When the authorization status changes, the CSRF token MUST be deleted, and a new one generated. This can be done by calling `handler.DeleteToken(c)`.

```go
if handler, ok := app.AcquireCtx(ctx).Locals(ConfigDefault.HandlerContextKey).(*CSRFHandler); ok {
    if err := handler.DeleteToken(app.AcquireCtx(ctx)); err != nil {
        // handle error
    }
}
```

:::tip
If you are using this middleware with the fiber session middleware, then you can simply call `session.Destroy()`, `session.Regenerate()`, or `session.Reset()` to delete session and the token stored therein.
:::

### BREACH

It's important to note that the token is sent as a header on every request. If you include the token in a page that is vulnerable to [BREACH](https://en.wikipedia.org/wiki/BREACH), an attacker may be able to extract the token. To mitigate this, ensure your pages are served over HTTPS, disable HTTP compression, and implement rate limiting for requests.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework:

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/csrf"
)
```

After initializing your Fiber app, you can use the following code to initialize the middleware:

```go
// Initialize default config
app.Use(csrf.New())

// Or extend your config for customization
app.Use(csrf.New(csrf.Config{
    KeyLookup:      "header:X-Csrf-Token",
    CookieName:     "csrf_",
	CookieSameSite: "Lax",
    Expiration:     1 * time.Hour,
    KeyGenerator:   utils.UUIDv4,
    Extractor:      func(c *fiber.Ctx) (string, error) { ... },
}))
```

:::info
KeyLookup will be ignored if Extractor is explicitly set.
:::

## Config

| Property          | Type                               | Description                                                                                                                                                                                                                                                                                  | Default                      |
|:------------------|:-----------------------------------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:-----------------------------|
| Next              | `func(*fiber.Ctx) bool`            | Next defines a function to skip this middleware when returned true.                                                                                                                                                                                                                          | `nil`                        |
| KeyLookup         | `string`                           | KeyLookup is a string in the form of "`<source>:<key>`" that is used to create an Extractor that extracts the token from the request. Possible values: "`header:<name>`", "`query:<name>`", "`param:<name>`", "`form:<name>`", "`cookie:<name>`". Ignored if an Extractor is explicitly set. | "header:X-CSRF-Token"        |
| CookieName        | `string`                           | Name of the csrf cookie. This cookie will store the csrf key.                                                                                                                                                                                                                                | "csrf_"                      |
| CookieDomain      | `string`                           | Domain of the CSRF cookie.                                                                                                                                                                                                                                                                   | ""                           |
| CookiePath        | `string`                           | Path of the CSRF cookie.                                                                                                                                                                                                                                                                     | ""                           |
| CookieSecure      | `bool`                             | Indicates if the CSRF cookie is secure.                                                                                                                                                                                                                                                      | false                        |
| CookieHTTPOnly    | `bool`                             | Indicates if the CSRF cookie is HTTP-only.                                                                                                                                                                                                                                                   | false                        |
| CookieSameSite    | `string`                           | Value of SameSite cookie.                                                                                                                                                                                                                                                                    | "Lax"                        |
| CookieSessionOnly | `bool`                             | Decides whether the cookie should last for only the browser session. Ignores Expiration if set to true.                                                                                                                                                                                      | false                        |
| Expiration        | `time.Duration`                    | Expiration is the duration before the CSRF token will expire.                                                                                                                                                                                                                                | 1 * time.Hour                |
| SingleUseToken    | `bool`                             | SingleUseToken indicates if the CSRF token be destroyed and a new one generated on each use. (See TokenLifecycle)                                                                                                                                                                            | false                        |
| Storage           | `fiber.Storage`                    | Store is used to store the state of the middleware.                                                                                                                                                                                                                                          | `nil`                        |
| Session           | `*session.Store`                   | Session is used to store the state of the middleware. Overrides Storage if set.                                                                                                                                                                                                              | `nil`                        |
| SessionKey        | `string`                           | SessionKey is the key used to store the token in the session.                                                                                                                                                                                                                                | "fiber.csrf.token"           |
| ContextKey        | `string`                           | Context key to store the generated CSRF token into the context. If left empty, the token will not be stored in the context.                                                                                                                                                                  | ""                           |
| KeyGenerator      | `func() string`                    | KeyGenerator creates a new CSRF token.                                                                                                                                                                                                                                                       | utils.UUID                   |
| CookieExpires     | `time.Duration` (Deprecated)       | Deprecated: Please use Expiration.                                                                                                                                                                                                                                                           | 0                            |
| Cookie            | `*fiber.Cookie` (Deprecated)       | Deprecated: Please use Cookie* related fields.                                                                                                                                                                                                                                               | `nil`                        |
| TokenLookup       | `string` (Deprecated)              | Deprecated: Please use KeyLookup.                                                                                                                                                                                                                                                            | ""                           |
| ErrorHandler      | `fiber.ErrorHandler`               | ErrorHandler is executed when an error is returned from fiber.Handler.                                                                                                                                                                                                                       | DefaultErrorHandler          |
| Extractor         | `func(*fiber.Ctx) (string, error)` | Extractor returns the CSRF token. If set, this will be used in place of an Extractor based on KeyLookup.                                                                                                                                                                                     | Extractor based on KeyLookup |
| HandlerContextKey | `string`                           | HandlerContextKey is used to store the CSRF Handler into context.                                                                                                                                                                                                                            | "fiber.csrf.handler"         |

### Default Config

```go
var ConfigDefault = Config{
	KeyLookup:         "header:" + HeaderName,
	CookieName:        "csrf_",
	CookieSameSite:    "Lax",
	Expiration:        1 * time.Hour,
	KeyGenerator:      utils.UUIDv4,
	ErrorHandler:      defaultErrorHandler,
	Extractor:         CsrfFromHeader(HeaderName),
	SessionKey:        "fiber.csrf.token",
	HandlerContextKey: "fiber.csrf.handler",
}
```

### Recommended Config (with session)

It's recommended to use this middleware with [fiber/middleware/session](https://docs.gofiber.io/api/middleware/session) to store the CSRF token in the session. This is generally more secure than the default configuration.

```go
var ConfigDefault = Config{
	KeyLookup:         "header:" + HeaderName,
	CookieName:        "csrf_",
	CookieSameSite:    "Lax",
	CookieSessionOnly: true,
	CookieHTTPOnly:    true,
	Expiration:        1 * time.Hour,
	KeyGenerator:      utils.UUIDv4,
	ErrorHandler:      defaultErrorHandler,
	Extractor:         CsrfFromHeader(HeaderName),
	Session:           session.Store,
	SessionKey:        "fiber.csrf.token",
	HandlerContextKey: "fiber.csrf.handler",
}
```

## Constants

```go
const (
    HeaderName = "X-Csrf-Token"
)
```

## Sentinel Errors

The CSRF middleware utilizes a set of sentinel errors to handle various scenarios and communicate errors effectively. These can be used within a [custom error handler](#custom-error-handler) to handle errors returned by the middleware.

### Errors Returned to Error Handler

- `ErrTokenNotFound`: Indicates that the CSRF token was not found.
- `ErrTokenInvalid`: Indicates that the CSRF token is invalid.
- `ErrNoReferer`: Indicates that the referer was not supplied.
- `ErrBadReferer`: Indicates that the referer is invalid.

If you are using the default error handler, it will return a 403 Forbidden error for any of these errors without providing any additional information to the client.

## Custom Error Handler

You can use a custom error handler to handle errors returned by the CSRF middleware. The error handler is executed when an error is returned from the middleware. The error handler is passed the error returned from the middleware and the fiber.Ctx.

Example, returning a JSON response for API requests and rendering an error page for other requests:

```go
app.Use(csrf.New(csrf.Config{
	ErrorHandler: func(c *fiber.Ctx, err error) error {
		accepts := c.Accepts("html", "json")
		path := c.Path()
		if accepts == "json" || strings.HasPrefix(path, "/api/") {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden",
			})
		}
		return c.Status(fiber.StatusForbidden).Render("error", fiber.Map{
			"Title": "Forbidden",
			"Status": fiber.StatusForbidden,
		}, "layouts/main")
	},
}))
```

## Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3
app.Use(csrf.New(csrf.Config{
	Storage: storage,
}))
```
