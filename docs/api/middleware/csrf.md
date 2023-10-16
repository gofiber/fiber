---
id: csrf
---

# CSRF

CSRF middleware for [Fiber](https://github.com/gofiber/fiber) that provides [Cross-site request forgery](https://en.wikipedia.org/wiki/Cross-site_request_forgery) protection by passing a csrf token via cookies. This cookie value will be used to compare against the client csrf token on requests, other than those defined as "safe" by [RFC9110#section-9.2.1](https://datatracker.ietf.org/doc/html/rfc9110.html#section-9.2.1) \(GET, HEAD, OPTIONS, or TRACE\). When the csrf token is invalid, this middleware will return the `fiber.ErrForbidden` error. 

CSRF Tokens are generated on GET requests. You can retrieve the CSRF token with `c.Locals(contextKey)`, where `contextKey` is the string you set in the config (see Custom Config below).

When no `csrf_` cookie is set, or the token has expired, a new token will be generated and `csrf_` cookie set.

:::note
This middleware uses our [Storage](https://github.com/gofiber/storage) package to support various databases through a single interface. The default configuration for this middleware saves data to memory, see the examples below for other databases.
:::

## Security Considerations

This middleware is designed to protect against CSRF attacks. It does not protect against other attack vectors, such as XSS, and should be used in combination with other security measures.

:::warning
Never use 'safe' methods to mutate data. For example, never use a GET request to delete a resource. This middleware will not protect against CSRF attacks on 'safe' methods.
:::

### The Double Submit Cookie Pattern (Default)

In the default configuration, the middleware will generate and store tokens using the `fiber.Storage` interface. These tokens are not associated with a user session, and, therefore, a Double Submit Cookie pattern is used to validate the token. This means that the token is stored in a cookie and also sent as a header on requests. The middleware will compare the cookie value with the header value to validate the token. This is a secure method of validating the token, as cookies are not accessible to JavaScript and, therefore, cannot be read by an attacker.

:::warning
When using this method, it is important that you set the `CookieSameSite` option to `Lax` or `Strict` and that the Extractor is not `CsrfFromCookie`, and KeyLookup is not `cookie:<name>`.
:::

### The Synchronizer Token Pattern (Session)

When using this middleware with a user session, the middleware can be configured to store the token in the session. This method is recommended when using a user session as it is generally more secure than the Double Submit Cookie Pattern.

:::warning
When using this method, pre-sessions are required and will be created if a session is not already present. This means that the middleware will create a session for every safe request, even if the request does not require a session. Therefore it is required that the existence of a session is not used to indicate that a user is logged in or authenticated, and that a session value is used to indicate this instead.
:::

### Defense In Depth

When using this middleware, it is recommended that you serve your pages over HTTPS, that the `CookieSecure` option is set to `true`, and that the `CookieSameSite` option is set to `Lax` or `Strict`. This will ensure that the cookie is only sent over HTTPS and that it is not sent on requests from external sites.

:::note
Cookie prefixes __Host- and __Secure- can be used to further secure the cookie. However, these prefixes are not supported by all browsers and there are some other limitations. See [MDN#Set-Cookie#cookie_prefixes](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#cookie_prefixes) for more information.

To use these prefixes, set the `CookieName` option to `__Host-csrf_` or `__Secure-csrf_`.
:::

### Referer Checking

For HTTPS requests, this middleware performs strict referer checking. This means that even if a subdomain can set or modify cookies on your domain, it can’t force a user to post to your application since that request won’t come from your own exact domain.

:::warning
Referer checking is required for https requests protected by CSRF. All modern browsers will automatically include the Referer header in requests, including those made with the JS Fetch API. However, if you are using this middleware with a custom client you must ensure that the client sends a valid Referer header.
:::

### Token Lifecycle

Tokens are valid until they expire, or until they are deleted. By default, tokens are valid for 1 hour and each subsequent request will extend the expiration by 1 hour. This means that if a user makes a request every hour, the token will never expire. If a user makes a request after the token has expired, then a new token will be generated and the `csrf_` cookie will be set again. This means that the token will only expire if the user does not make a request for the duration of the expiration time.

#### Token Reuse

By default tokens may be used multiple times. This means that the token will not be deleted after it has been used. If you would like to delete the token after it has been used, then you can set the `SingleUseToken` option to `true`. This will delete the token after it has been used, and a new token will be generated on the next request.

:::note
Using `SingleUseToken` comes with usability tradeoffs, and therefore is not enabled by default. It can interfere with the user experience if the user has multiple tabs open, or if the user uses the back button.
:::

#### Deleting Tokens

When the authorization status changes, the CSRF token should be deleted and a new one generated. This can be done by calling `handler.DeleteToken(c)`. This will remove the token found in the request context from the storage and set the `csrf_` cookie to an empty value. The next 'safe' request will generate a new token and set the cookie again.

```go
if handler, ok := app.AcquireCtx(ctx).Locals(ConfigDefault.HandlerContextKey).(*CSRFHandler); ok {
	if err := handler.DeleteToken(app.AcquireCtx(ctx)); err != nil {
		// handle error
	}
}
```

:::note
If you are using this middleware with the fiber session middleware, then you can simply call `session.Destroy()`, `session.Regenerate()`, or `session.Reset()` to delete session and the token stored therein.
:::

### BREACH

It is important to note that the token is sent as a header on every request, and if you include the token in a page that is vulnerable to [BREACH](https://en.wikipedia.org/wiki/BREACH), then an attacker may be able to extract the token. To mitigate this, you should take steps such as ensuring that your pages are served over HTTPS, that HTTP compression is disabled, and rate limiting requests.

## Signatures

```go
func New(config ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/csrf"
)
```

After you initiate your Fiber app, you can use the following possibilities:

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

:::note
KeyLookup will be ignored if Extractor is explicitly set.
:::

### Use with fiber/middleware/session (recommended)

It's recommended to use this middleware with [fiber/middleware/session](https://docs.gofiber.io/api/middleware/session) to store the CSRF token in the session. This is generally more secure than the default configuration.

## Config

### Config

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

## Default Config

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

## Recommended Config (with session)

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

### Custom Storage/Database

You can use any storage from our [storage](https://github.com/gofiber/storage/) package.

```go
storage := sqlite3.New() // From github.com/gofiber/storage/sqlite3
app.Use(csrf.New(csrf.Config{
	Storage: storage,
}))
```
