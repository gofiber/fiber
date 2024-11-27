---
id: ctx
title: 🧠 Ctx
description: >-
  The Ctx interface represents the Context which holds the HTTP request and
  response. It has methods for the request query string, parameters, body, HTTP
  headers, and so on.
sidebar_position: 3
---

## Accepts

Checks if the specified **extensions** or **content** **types** are acceptable.

:::info
Based on the request’s [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header.
:::

```go title="Signature"
func (c fiber.Ctx) Accepts(offers ...string) string
func (c fiber.Ctx) AcceptsCharsets(offers ...string) string
func (c fiber.Ctx) AcceptsEncodings(offers ...string) string
func (c fiber.Ctx) AcceptsLanguages(offers ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Accepts("html")             // "html"
  c.Accepts("text/html")        // "text/html"
  c.Accepts("json", "text")     // "json"
  c.Accepts("application/json") // "application/json"
  c.Accepts("text/plain", "application/json") // "application/json", due to quality
  c.Accepts("image/png")        // ""
  c.Accepts("png")              // ""
  // ...
})
```

```go title="Example 2"
// Accept: text/html, text/*, application/json, */*; q=0

app.Get("/", func(c fiber.Ctx) error {
  c.Accepts("text/plain", "application/json") // "application/json", due to specificity
  c.Accepts("application/json", "text/html") // "text/html", due to first match
  c.Accepts("image/png")                      // "", due to */* with q=0 is Not Acceptable
  // ...
})
```

Media-Type parameters are supported.

```go title="Example 3"
// Accept: text/plain, application/json; version=1; foo=bar

app.Get("/", func(c fiber.Ctx) error {
  // Extra parameters in the accept are ignored
  c.Accepts("text/plain;format=flowed") // "text/plain;format=flowed"
  
  // An offer must contain all parameters present in the Accept type
  c.Accepts("application/json") // ""

  // Parameter order and capitalization do not matter. Quotes on values are stripped.
  c.Accepts(`application/json;foo="bar";VERSION=1`) // "application/json;foo="bar";VERSION=1"
})
```

```go title="Example 4"
// Accept: text/plain;format=flowed;q=0.9, text/plain
// i.e., "I prefer text/plain;format=flowed less than other forms of text/plain"

app.Get("/", func(c fiber.Ctx) error {
  // Beware: the order in which offers are listed matters.
  // Although the client specified they prefer not to receive format=flowed,
  // the text/plain Accept matches with "text/plain;format=flowed" first, so it is returned.
  c.Accepts("text/plain;format=flowed", "text/plain") // "text/plain;format=flowed"

  // Here, things behave as expected:
  c.Accepts("text/plain", "text/plain;format=flowed") // "text/plain"
})
```

Fiber provides similar functions for the other accept headers.

```go
// Accept-Charset: utf-8, iso-8859-1;q=0.2
// Accept-Encoding: gzip, compress;q=0.2
// Accept-Language: en;q=0.8, nl, ru

app.Get("/", func(c fiber.Ctx) error {
  c.AcceptsCharsets("utf-16", "iso-8859-1")
  // "iso-8859-1"

  c.AcceptsEncodings("compress", "br")
  // "compress"

  c.AcceptsLanguages("pt", "nl", "ru")
  // "nl"
  // ...
})
```

## App

Returns the [\*App](app.md) reference so you can easily access all application settings.

```go title="Signature"
func (c fiber.Ctx) App() *App
```

```go title="Example"
app.Get("/stack", func(c fiber.Ctx) error {
  return c.JSON(c.App().Stack())
})
```

## Append

Appends the specified **value** to the HTTP response header field.

:::caution
If the header is **not** already set, it creates the header with the specified value.
:::

```go title="Signature"
func (c fiber.Ctx) Append(field string, values ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Append("Link", "http://google.com", "http://localhost")
  // => Link: http://google.com, http://localhost

  c.Append("Link", "Test")
  // => Link: http://google.com, http://localhost, Test

  // ...
})
```

## Attachment

Sets the HTTP response [Content-Disposition](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition) header field to `attachment`.

```go title="Signature"
func (c fiber.Ctx) Attachment(filename ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Attachment()
  // => Content-Disposition: attachment

  c.Attachment("./upload/images/logo.png")
  // => Content-Disposition: attachment; filename="logo.png"
  // => Content-Type: image/png

  // ...
})
```

## AutoFormat

Performs content-negotiation on the [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header. It uses [Accepts](ctx.md#accepts) to select a proper format.
The supported content types are `text/html`, `text/plain`, `application/json`, and `application/xml`.
For more flexible content negotiation, use [Format](ctx.md#format).

:::info
If the header is **not** specified or there is **no** proper format, **text/plain** is used.
:::

```go title="Signature"
func (c fiber.Ctx) AutoFormat(body any) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Accept: text/plain
  c.AutoFormat("Hello, World!")
  // => Hello, World!

  // Accept: text/html
  c.AutoFormat("Hello, World!")
  // => <p>Hello, World!</p>

  type User struct {
    Name string
  }
  user := User{"John Doe"}

  // Accept: application/json
  c.AutoFormat(user)
  // => {"Name":"John Doe"}

  // Accept: application/xml
  c.AutoFormat(user)
  // => <User><Name>John Doe</Name></User>
  // ..
})
```

## BaseURL

Returns the base URL (**protocol** + **host**) as a `string`.

```go title="Signature"
func (c fiber.Ctx) BaseURL() string
```

```go title="Example"
// GET https://example.com/page#chapter-1

app.Get("/", func(c fiber.Ctx) error {
  c.BaseURL() // "https://example.com"
  // ...
})
```

## Bind

Bind is a method that supports bindings for the request/response body, query parameters, URL parameters, cookies, and much more.
It returns a pointer to the [Bind](./bind.md) struct which contains all the methods to bind the request/response data.

For detailed information, check the [Bind](./bind.md) documentation.

```go title="Signature"
func (c fiber.Ctx) Bind() *Bind
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  user := new(User)
  // Bind the request body to a struct:
  return c.Bind().Body(user)
})
```

## Body

As per the header `Content-Encoding`, this method will try to perform a file decompression from the **body** bytes. In case no `Content-Encoding` header is sent, it will perform as [BodyRaw](#bodyraw).

```go title="Signature"
func (c fiber.Ctx) Body() []byte
```

```go title="Example"
// echo 'user=john' | gzip | curl -v -i --data-binary @- -H "Content-Encoding: gzip" http://localhost:8080

app.Post("/", func(c fiber.Ctx) error {
  // Decompress body from POST request based on the Content-Encoding and return the raw content:
  return c.Send(c.Body()) // []byte("user=john")
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## BodyRaw

Returns the raw request **body**.

```go title="Signature"
func (c fiber.Ctx) BodyRaw() []byte
```

```go title="Example"
// curl -X POST http://localhost:8080 -d user=john

app.Post("/", func(c fiber.Ctx) error {
  // Get raw body from POST request:
  return c.Send(c.BodyRaw()) // []byte("user=john")
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## ClearCookie

Expires a client cookie (or all cookies if left empty).

```go title="Signature"
func (c fiber.Ctx) ClearCookie(key ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Clears all cookies:
  c.ClearCookie()

  // Expire specific cookie by name:
  c.ClearCookie("user")

  // Expire multiple cookies by names:
  c.ClearCookie("token", "session", "track_id", "version")
  // ...
})
```

:::caution
Web browsers and other compliant clients will only clear the cookie if the given options are identical to those when creating the cookie, excluding `Expires` and `MaxAge`. `ClearCookie` will not set these values for you - a technique similar to the one shown below should be used to ensure your cookie is deleted.
:::

```go title="Example"
app.Get("/set", func(c fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        Value:    "randomvalue",
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: true,
        SameSite: "lax",
    })

    // ...
})

app.Get("/delete", func(c fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        // Set expiry date to the past
        Expires:  time.Now().Add(-(time.Hour * 2)),
        HTTPOnly: true,
        SameSite: "lax",
    })

    // ...
})
```

## ClientHelloInfo

`ClientHelloInfo` contains information from a ClientHello message in order to guide application logic in the `GetCertificate` and `GetConfigForClient` callbacks.
You can refer to the [ClientHelloInfo](https://golang.org/pkg/crypto/tls/#ClientHelloInfo) struct documentation for more information on the returned struct.

```go title="Signature"
func (c fiber.Ctx) ClientHelloInfo() *tls.ClientHelloInfo
```

```go title="Example"
// GET http://example.com/hello
app.Get("/hello", func(c fiber.Ctx) error {
  chi := c.ClientHelloInfo()
  // ...
})
```

## Context

`Context` returns a context implementation that was set by the user earlier or returns a non-nil, empty context if it was not set earlier.

```go title="Signature"
func (c fiber.Ctx) Context() context.Context
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  ctx := c.Context()
  // ctx is context implementation set by user

  // ...
})
```

## Cookie

Sets a cookie.

```go title="Signature"
func (c fiber.Ctx) Cookie(cookie *Cookie)
```

```go
type Cookie struct {
    Name        string    `json:"name"`         // The name of the cookie
    Value       string    `json:"value"`        // The value of the cookie
    Path        string    `json:"path"`         // Specifies a URL path which is allowed to receive the cookie
    Domain      string    `json:"domain"`       // Specifies the domain which is allowed to receive the cookie
    MaxAge      int       `json:"max_age"`      // The maximum age (in seconds) of the cookie
    Expires     time.Time `json:"expires"`      // The expiration date of the cookie
    Secure      bool      `json:"secure"`       // Indicates that the cookie should only be transmitted over a secure HTTPS connection
    HTTPOnly    bool      `json:"http_only"`    // Indicates that the cookie is accessible only through the HTTP protocol
    SameSite    string    `json:"same_site"`    // Controls whether or not a cookie is sent with cross-site requests
    Partitioned bool      `json:"partitioned"`  // Indicates if the cookie is stored in a partitioned cookie jar
    SessionOnly bool      `json:"session_only"` // Indicates if the cookie is a session-only cookie
}
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Create cookie
  cookie := new(fiber.Cookie)
  cookie.Name = "john"
  cookie.Value = "doe"
  cookie.Expires = time.Now().Add(24 * time.Hour)

  // Set cookie
  c.Cookie(cookie)
  // ...
})
```

:::info
Partitioned cookies allow partitioning the cookie jar by top-level site, enhancing user privacy by preventing cookies from being shared across different sites. This feature is particularly useful in scenarios where a user interacts with embedded third-party services that should not have access to the main site's cookies. You can check out [CHIPS](https://developers.google.com/privacy-sandbox/3pcd/chips) for more information.
:::

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Create a new partitioned cookie
  cookie := new(fiber.Cookie)
  cookie.Name = "user_session"
  cookie.Value = "abc123"
  cookie.Partitioned = true  // This cookie will be stored in a separate jar when it's embedded into another website

  // Set the cookie in the response
  c.Cookie(cookie)
  return c.SendString("Partitioned cookie set")
})
```

## Cookies

Gets a cookie value by key. You can pass an optional default value that will be returned if the cookie key does not exist.

```go title="Signature"
func (c fiber.Ctx) Cookies(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // Get cookie by key:
  c.Cookies("name")         // "john"
  c.Cookies("empty", "doe") // "doe"
  // ...
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## Download

Transfers the file from the given path as an `attachment`.

Typically, browsers will prompt the user to download. By default, the [Content-Disposition](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition) header `filename=` parameter is the file path (_this typically appears in the browser dialog_).
Override this default with the **filename** parameter.

```go title="Signature"
func (c fiber.Ctx) Download(file string, filename ...string) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.Download("./files/report-12345.pdf")
  // => Download report-12345.pdf

  return c.Download("./files/report-12345.pdf", "report.pdf")
  // => Download report.pdf
})
```

## Format

Performs content-negotiation on the [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header. It uses [Accepts](ctx.md#accepts) to select a proper format from the supplied offers. A default handler can be provided by setting the `MediaType` to `"default"`. If no offers match and no default is provided, a 406 (Not Acceptable) response is sent. The Content-Type is automatically set when a handler is selected.

:::info
If the Accept header is **not** specified, the first handler will be used.
:::

```go title="Signature"
func (c fiber.Ctx) Format(handlers ...ResFmt) error
```

```go title="Example"
// Accept: application/json => {"command":"eat","subject":"fruit"}
// Accept: text/plain => Eat Fruit!
// Accept: application/xml => Not Acceptable
app.Get("/no-default", func(c fiber.Ctx) error {
  return c.Format(
    fiber.ResFmt{"application/json", func(c fiber.Ctx) error {
      return c.JSON(fiber.Map{
        "command": "eat",
        "subject": "fruit",
      })
    }},
    fiber.ResFmt{"text/plain", func(c fiber.Ctx) error {
      return c.SendString("Eat Fruit!")
    }},
  )
})

// Accept: application/json => {"command":"eat","subject":"fruit"}
// Accept: text/plain => Eat Fruit!
// Accept: application/xml => Eat Fruit!
app.Get("/default", func(c fiber.Ctx) error {
  textHandler := func(c fiber.Ctx) error {
    return c.SendString("Eat Fruit!")
  }

  handlers := []fiber.ResFmt{
    {"application/json", func(c fiber.Ctx) error {
      return c.JSON(fiber.Map{
        "command": "eat",
        "subject": "fruit",
      })
    }},
    {"text/plain", textHandler},
    {"default", textHandler},
  }

  return c.Format(handlers...)
})
```

## FormFile

MultipartForm files can be retrieved by name, the **first** file from the given key is returned.

```go title="Signature"
func (c fiber.Ctx) FormFile(key string) (*multipart.FileHeader, error)
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Get first file from form field "document":
  file, err := c.FormFile("document")

  // Save file to root directory:
  return c.SaveFile(file, fmt.Sprintf("./%s", file.Filename))
})
```

## FormValue

Form values can be retrieved by name, the **first** value for the given key is returned.

```go title="Signature"
func (c fiber.Ctx) FormValue(key string, defaultValue ...string) string
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Get first value from form field "name":
  c.FormValue("name")
  // => "john" or "" if not exist

  // ..
})
```

:::info

Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)

:::

## Fresh

When the response is still **fresh** in the client's cache **true** is returned, otherwise **false** is returned to indicate that the client cache is now stale and the full response should be sent.

When a client sends the Cache-Control: no-cache request header to indicate an end-to-end reload request, `Fresh` will return false to make handling these requests transparent.

Read more on [https://expressjs.com/en/4x/api.html\#req.fresh](https://expressjs.com/en/4x/api.html#req.fresh)

```go title="Signature"
func (c fiber.Ctx) Fresh() bool
```

## Get

Returns the HTTP request header specified by the field.

:::tip
The match is **case-insensitive**.
:::

```go title="Signature"
func (c fiber.Ctx) Get(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Get("Content-Type")       // "text/plain"
  c.Get("CoNtEnT-TypE")       // "text/plain"
  c.Get("something", "john")  // "john"
  // ..
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## GetReqHeaders

Returns the HTTP request headers as a map. Since a header can be set multiple times in a single request, the values of the map are slices of strings containing all the different values of the header.

```go title="Signature"
func (c fiber.Ctx) GetReqHeaders() map[string][]string
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## GetRespHeader

Returns the HTTP response header specified by the field.

:::tip
The match is **case-insensitive**.
:::

```go title="Signature"
func (c fiber.Ctx) GetRespHeader(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.GetRespHeader("X-Request-Id")       // "8d7ad5e3-aaf3-450b-a241-2beb887efd54"
  c.GetRespHeader("Content-Type")       // "text/plain"
  c.GetRespHeader("something", "john")  // "john"
  // ..
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## GetRespHeaders

Returns the HTTP response headers as a map. Since a header can be set multiple times in a single request, the values of the map are slices of strings containing all the different values of the header.

```go title="Signature"
func (c fiber.Ctx) GetRespHeaders() map[string][]string
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## GetRouteURL

Generates URLs to named routes, with parameters. URLs are relative, for example: "/user/1831"

```go title="Signature"
func (c fiber.Ctx) GetRouteURL(routeName string, params Map) (string, error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
    return c.SendString("Home page")
}).Name("home")

app.Get("/user/:id", func(c fiber.Ctx) error {
    return c.SendString(c.Params("id"))
}).Name("user.show")

app.Get("/test", func(c fiber.Ctx) error {
    location, _ := c.GetRouteURL("user.show", fiber.Map{"id": 1})
    return c.SendString(location)
})

// /test returns "/user/1"
```

## Host

Returns the host derived from the [Host](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Host) HTTP header.

In a network context, [`Host`](#host) refers to the combination of a hostname and potentially a port number used for connecting, while [`Hostname`](#hostname) refers specifically to the name assigned to a device on a network, excluding any port information.

```go title="Signature"
func (c fiber.Ctx) Host() string
```

```go title="Example"
// GET http://google.com:8080/search

app.Get("/", func(c fiber.Ctx) error {
  c.Host()      // "google.com:8080"
  c.Hostname()  // "google.com"

  // ...
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## Hostname

Returns the hostname derived from the [Host](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Host) HTTP header.

```go title="Signature"
func (c fiber.Ctx) Hostname() string
```

```go title="Example"
// GET http://google.com/search

app.Get("/", func(c fiber.Ctx) error {
  c.Hostname() // "google.com"

  // ...
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## IP

Returns the remote IP address of the request.

```go title="Signature"
func (c fiber.Ctx) IP() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.IP() // "127.0.0.1"

  // ...
})
```

When registering the proxy request header in the Fiber app, the IP address of the header is returned [(Fiber configuration)](fiber.md#proxyheader)

```go
app := fiber.New(fiber.Config{
  ProxyHeader: fiber.HeaderXForwardedFor,
})
```

## IPs

Returns an array of IP addresses specified in the [X-Forwarded-For](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For) request header.

```go title="Signature"
func (c fiber.Ctx) IPs() []string
```

```go title="Example"
// X-Forwarded-For: proxy1, 127.0.0.1, proxy3

app.Get("/", func(c fiber.Ctx) error {
  c.IPs() // ["proxy1", "127.0.0.1", "proxy3"]

  // ...
})
```

:::caution
Improper use of the X-Forwarded-For header can be a security risk. For details, see the [Security and privacy concerns](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For#security_and_privacy_concerns) section.
:::

## Is

Returns the matching **content type**, if the incoming request’s [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) HTTP header field matches the [MIME type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types) specified by the type parameter.

:::info
If the request has **no** body, it returns **false**.
:::

```go title="Signature"
func (c fiber.Ctx) Is(extension string) bool
```

```go title="Example"
// Content-Type: text/html; charset=utf-8

app.Get("/", func(c fiber.Ctx) error {
  c.Is("html")  // true
  c.Is(".html") // true
  c.Is("json")  // false

  // ...
})
```

## IsFromLocal

Returns `true` if the request came from localhost.

```go title="Signature"
func (c fiber.Ctx) IsFromLocal() bool
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  // If request came from localhost, return true; else return false
  c.IsFromLocal()

  // ...
})
```

## IsProxyTrusted

Checks the trustworthiness of the remote IP.
If [`TrustProxy`](fiber.md#trustproxy) is `false`, it returns `true`.
`IsProxyTrusted` can check the remote IP by proxy ranges and IP map.

```go title="Signature"
func (c fiber.Ctx) IsProxyTrusted() bool
```

```go title="Example"
app := fiber.New(fiber.Config{
  // TrustProxy enables the trusted proxy check
  TrustProxy: true,
  // TrustProxyConfig allows for configuring trusted proxies.
  // Proxies is a list of trusted proxy IP ranges/addresses
  TrustProxyConfig: fiber.TrustProxyConfig{
    Proxies: []string{"0.8.0.0", "1.1.1.1/30"}, // IP address or IP address range
  },
})

app.Get("/", func(c fiber.Ctx) error {
  // If request came from trusted proxy, return true; else return false
  c.IsProxyTrusted()

  // ...
})
```

## JSON

Converts any **interface** or **string** to JSON using the [encoding/json](https://pkg.go.dev/encoding/json) package.

:::info
JSON also sets the content header to the `ctype` parameter. If no `ctype` is passed in, the header is set to `application/json`.
:::

```go title="Signature"
func (c fiber.Ctx) JSON(data any, ctype ...string) error
```

```go title="Example"
type SomeStruct struct {
  Name string
  Age  uint8
}

app.Get("/json", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.JSON(data)
  // => Content-Type: application/json
  // => {"Name": "Grame", "Age": 20}

  return c.JSON(fiber.Map{
    "name": "Grame",
    "age":  20,
  })
  // => Content-Type: application/json
  // => {"name": "Grame", "age": 20}

  return c.JSON(fiber.Map{
    "type":     "https://example.com/probs/out-of-credit",
    "title":    "You do not have enough credit.",
    "status":   403,
    "detail":   "Your current balance is 30, but that costs 50.",
    "instance": "/account/12345/msgs/abc",
  }, "application/problem+json")
  // => Content-Type: application/problem+json
  // => "{
  // =>     "type": "https://example.com/probs/out-of-credit",
  // =>     "title": "You do not have enough credit.",
  // =>     "status": 403,
  // =>     "detail": "Your current balance is 30, but that costs 50.",
  // =>     "instance": "/account/12345/msgs/abc",
  // => }"
})
```

## JSONP

Sends a JSON response with JSONP support. This method is identical to [JSON](ctx.md#json), except that it opts-in to JSONP callback support. By default, the callback name is simply `callback`.

Override this by passing a **named string** in the method.

```go title="Signature"
func (c fiber.Ctx) JSONP(data any, callback ...string) error
```

```go title="Example"
type SomeStruct struct {
  Name string
  Age  uint8
}

app.Get("/", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.JSONP(data)
  // => callback({"Name": "Grame", "Age": 20})

  return c.JSONP(data, "customFunc")
  // => customFunc({"Name": "Grame", "Age": 20})
})
```

## Links

Joins the links followed by the property to populate the response’s [Link](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Link) HTTP header field.

```go title="Signature"
func (c fiber.Ctx) Links(link ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Links(
    "http://api.example.com/users?page=2", "next",
    "http://api.example.com/users?page=5", "last",
  )
  // Link: <http://api.example.com/users?page=2>; rel="next",
  //       <http://api.example.com/users?page=5>; rel="last"

  // ...
})
```

## Locals

A method that stores variables scoped to the request and, therefore, are available only to the routes that match the request. The stored variables are removed after the request is handled. If any of the stored data implements the `io.Closer` interface, its `Close` method will be called before it's removed.

:::tip
This is useful if you want to pass some **specific** data to the next middleware. Remember to perform type assertions when retrieving the data to ensure it is of the expected type. You can also use a non-exported type as a key to avoid collisions.
:::

```go title="Signature"
func (c fiber.Ctx) Locals(key any, value ...any) any
```

```go title="Example"

// keyType is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type keyType int

// userKey is the key for user.User values in Contexts. It is
// unexported; clients use user.NewContext and user.FromContext
// instead of using this key directly.
var userKey keyType

app.Use(func(c fiber.Ctx) error {
  c.Locals(userKey, "admin") // Stores the string "admin" under a non-exported type key
  return c.Next()
})

app.Get("/admin", func(c fiber.Ctx) error {
  user, ok := c.Locals(userKey).(string) // Retrieves the data stored under the key and performs a type assertion
  if ok && user == "admin" {
    return c.Status(fiber.StatusOK).SendString("Welcome, admin!")
  }
  return c.SendStatus(fiber.StatusForbidden)
})
```

An alternative version of the `Locals` method that takes advantage of Go's generics feature is also available. This version allows for the manipulation and retrieval of local values within a request's context with a more specific data type.

```go title="Signature"
func Locals[V any](c fiber.Ctx, key any, value ...V) V
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  fiber.Locals[string](c, "john", "doe")
  fiber.Locals[int](c, "age", 18)
  fiber.Locals[bool](c, "isHuman", true)
  return c.Next()
})

app.Get("/test", func(c fiber.Ctx) error {
  fiber.Locals[string](c, "john")    // "doe"
  fiber.Locals[int](c, "age")        // 18
  fiber.Locals[bool](c, "isHuman")   // true
  return nil
})
````

Make sure to understand and correctly implement the `Locals` method in both its standard and generic form for better control over route-specific data within your application.

## Location

Sets the response [Location](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Location) HTTP header to the specified path parameter.

```go title="Signature"
func (c fiber.Ctx) Location(path string)
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  c.Location("http://example.com")

  c.Location("/foo/bar")

  return nil
})
```

## Method

Returns a string corresponding to the HTTP method of the request: `GET`, `POST`, `PUT`, and so on.
Optionally, you can override the method by passing a string.

```go title="Signature"
func (c fiber.Ctx) Method(override ...string) string
```

```go title="Example"
app.Post("/override", func(c fiber.Ctx) error {
  c.Method()          // "POST"

  c.Method("GET")
  c.Method()          // "GET"

  // ...
})
```

## MultipartForm

To access multipart form entries, you can parse the binary with `MultipartForm()`. This returns a `*multipart.Form`, allowing you to access form values and files.

```go title="Signature"
func (c fiber.Ctx) MultipartForm() (*multipart.Form, error)
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Parse the multipart form:
  if form, err := c.MultipartForm(); err == nil {
    // => *multipart.Form

    if token := form.Value["token"]; len(token) > 0 {
      // Get key value:
      fmt.Println(token[0])
    }

    // Get all files from "documents" key:
    files := form.File["documents"]
    // => []*multipart.FileHeader

    // Loop through files:
    for _, file := range files {
      fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
      // => "tutorial.pdf" 360641 "application/pdf"

      // Save the files to disk:
      if err := c.SaveFile(file, fmt.Sprintf("./%s", file.Filename)); err != nil {
        return err
      }
    }
  }

  return nil
})
```

## Next

When **Next** is called, it executes the next method in the stack that matches the current route. You can pass an error struct within the method that will end the chaining and call the [error handler](https://docs.gofiber.io/guide/error-handling).

```go title="Signature"
func (c fiber.Ctx) Next() error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  fmt.Println("1st route!")
  return c.Next()
})

app.Get("*", func(c fiber.Ctx) error {
  fmt.Println("2nd route!")
  return c.Next()
})

app.Get("/", func(c fiber.Ctx) error {
  fmt.Println("3rd route!")
  return c.SendString("Hello, World!")
})
```

## OriginalURL

Returns the original request URL.

```go title="Signature"
func (c fiber.Ctx) OriginalURL() string
```

```go title="Example"
// GET http://example.com/search?q=something

app.Get("/", func(c fiber.Ctx) error {
  c.OriginalURL() // "/search?q=something"

  // ...
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

## Params

This method can be used to get the route parameters. You can pass an optional default value that will be returned if the param key does not exist.

:::info
Defaults to an empty string \(`""`\) if the param **doesn't** exist.
:::

```go title="Signature"
func (c fiber.Ctx) Params(key string, defaultValue ...string) string
```

```go title="Example"
// GET http://example.com/user/fenny
app.Get("/user/:name", func(c fiber.Ctx) error {
  c.Params("name") // "fenny"

  // ...
})

// GET http://example.com/user/fenny/123
app.Get("/user/*", func(c fiber.Ctx) error {
  c.Params("*")  // "fenny/123"
  c.Params("*1") // "fenny/123"

  // ...
})
```

Unnamed route parameters \(\*, +\) can be fetched by the **character** and the **counter** in the route.

```go title="Example"
// ROUTE: /v1/*/shop/*
// GET:   /v1/brand/4/shop/blue/xs
c.Params("*1")  // "brand/4"
c.Params("*2")  // "blue/xs"
```

For reasons of **downward compatibility**, the first parameter segment for the parameter character can also be accessed without the counter.

```go title="Example"
app.Get("/v1/*/shop/*", func(c fiber.Ctx) error {
  c.Params("*") // outputs the value of the first wildcard segment
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

In certain scenarios, it can be useful to have an alternative approach to handle different types of parameters, not
just strings. This can be achieved using a generic `Params` function known as `Params[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V`.
This function is capable of parsing a route parameter and returning a value of a type that is assumed and specified by `V GenericType`.

```go title="Signature"
func Params[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V
```

```go title="Example"
// GET http://example.com/user/114
app.Get("/user/:id", func(c fiber.Ctx) error{
  fiber.Params[string](c, "id") // returns "114" as string.
  fiber.Params[int](c, "id")    // returns 114 as integer
  fiber.Params[string](c, "number") // returns "" (default string type)
  fiber.Params[int](c, "number")    // returns 0 (default integer value type)
})
```

The generic `Params` function supports returning the following data types based on `V GenericType`:

- Integer: `int`, `int8`, `int16`, `int32`, `int64`
- Unsigned integer: `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- Floating-point numbers: `float32`, `float64`
- Boolean: `bool`
- String: `string`
- Byte array: `[]byte`

## Path

Contains the path part of the request URL. Optionally, you can override the path by passing a string. For internal redirects, you might want to call [RestartRouting](ctx.md#restartrouting) instead of [Next](ctx.md#next).

```go title="Signature"
func (c fiber.Ctx) Path(override ...string) string
```

```go title="Example"
// GET http://example.com/users?sort=desc

app.Get("/users", func(c fiber.Ctx) error {
  c.Path()       // "/users"

  c.Path("/john")
  c.Path()       // "/john"

  // ...
})
```

## Port

Returns the remote port of the request.

```go title="Signature"
func (c fiber.Ctx) Port() string
```

```go title="Example"
// GET http://example.com:8080

app.Get("/", func(c fiber.Ctx) error {
  c.Port() // "8080"

  // ...
})
```

## Protocol

Contains the request protocol string: `http` or `https` for **TLS** requests.

```go title="Signature"
func (c fiber.Ctx) Protocol() string
```

```go title="Example"
// GET http://example.com

app.Get("/", func(c fiber.Ctx) error {
  c.Protocol() // "http"

  // ...
})
```

## Queries

`Queries` is a function that returns an object containing a property for each query string parameter in the route.

```go title="Signature"
func (c fiber.Ctx) Queries() map[string]string
```

```go title="Example"
// GET http://example.com/?name=alex&want_pizza=false&id=

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["name"]        // "alex"
    m["want_pizza"]  // "false"
    m["id"]          // ""
    // ...
})
```

```go title="Example"
// GET http://example.com/?field1=value1&field1=value2&field2=value3

app.Get("/", func (c fiber.Ctx) error {
    m := c.Queries()
    m["field1"] // "value2"
    m["field2"] // "value3"
})
```

```go title="Example"
// GET http://example.com/?list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["list_a"] // "3"
    m["list_b[]"] // "3"
    m["list_c"] // "1,2,3"
})
```

```go title="Example"
// GET /api/posts?filters.author.name=John&filters.category.name=Technology

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["filters.author.name"] // John
    m["filters.category.name"] // Technology
})
```

```go title="Example"
// GET /api/posts?tags=apple,orange,banana&filters[tags]=apple,orange,banana&filters[category][name]=fruits&filters.tags=apple,orange,banana&filters.category.name=fruits

app.Get("/", func(c fiber.Ctx) error {
    m := c.Queries()
    m["tags"] // apple,orange,banana
    m["filters[tags]"] // apple,orange,banana
    m["filters[category][name]"] // fruits
    m["filters.tags"] // apple,orange,banana
    m["filters.category.name"] // fruits
})
```

## Query

This method returns a string corresponding to a query string parameter by name. You can pass an optional default value that will be returned if the query key does not exist.

:::info
If there is **no** query string, it returns an **empty string**.
:::

```go title="Signature"
func (c fiber.Ctx) Query(key string, defaultValue ...string) string
```

```go title="Example"
// GET http://example.com/?order=desc&brand=nike

app.Get("/", func(c fiber.Ctx) error {
  c.Query("order")         // "desc"
  c.Query("brand")         // "nike"
  c.Query("empty", "nike") // "nike"

  // ...
})
```

:::info
Returned value is only valid within the handler. Do not store any references.  
Make copies or use the [**`Immutable`**](./ctx.md) setting instead. [Read more...](../#zero-allocation)
:::

In certain scenarios, it can be useful to have an alternative approach to handle different types of query parameters, not
just strings. This can be achieved using a generic `Query` function known as `Query[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V`.
This function is capable of parsing a query string and returning a value of a type that is assumed and specified by `V GenericType`.

Here is the signature for the generic `Query` function:

```go title="Signature"
func Query[V GenericType](c fiber.Ctx, key string, defaultValue ...V) V
```

```go title="Example"
// GET http://example.com/?page=1&brand=nike&new=true

app.Get("/", func(c fiber.Ctx) error {
  fiber.Query[int](c, "page")     // 1
  fiber.Query[string](c, "brand") // "nike"
  fiber.Query[bool](c, "new")     // true

  // ...
})
```

In this case, `Query[V GenericType](c Ctx, key string, defaultValue ...V) V` can retrieve `page` as an integer, `brand` as a string, and `new` as a boolean. The function uses the appropriate parsing function for each specified type to ensure the correct type is returned. This simplifies the retrieval process of different types of query parameters, making your controller actions cleaner.
The generic `Query` function supports returning the following data types based on `V GenericType`:

- Integer: `int`, `int8`, `int16`, `int32`, `int64`
- Unsigned integer: `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- Floating-point numbers: `float32`, `float64`
- Boolean: `bool`
- String: `string`
- Byte array: `[]byte`

## Range

Returns a struct containing the type and a slice of ranges.

```go title="Signature"
func (c fiber.Ctx) Range(size int) (Range, error)
```

```go title="Example"
// Range: bytes=500-700, 700-900
app.Get("/", func(c fiber.Ctx) error {
  r := c.Range(1000)
  if r.Type == "bytes" {
      for _, rng := range r.Ranges {
      fmt.Println(rng)
      // [500, 700]
    }
  }
})
```

## Redirect

Returns the Redirect reference.

For detailed information, check the [Redirect](./redirect.md) documentation.

```go title="Signature"
func (c fiber.Ctx) Redirect() *Redirect
```

```go title="Example"
app.Get("/coffee", func(c fiber.Ctx) error {
    return c.Redirect().To("/teapot")
})

app.Get("/teapot", func(c fiber.Ctx) error {
    return c.Status(fiber.StatusTeapot).Send("🍵 short and stout 🍵")
})
```

## Render

Renders a view with data and sends a `text/html` response. By default, `Render` uses the default [**Go Template engine**](https://pkg.go.dev/html/template/). If you want to use another view engine, please take a look at our [**Template middleware**](https://docs.gofiber.io/template).

```go title="Signature"
func (c fiber.Ctx) Render(name string, bind Map, layouts ...string) error
```

## Request

Returns the [*fasthttp.Request](https://pkg.go.dev/github.com/valyala/fasthttp#Request) pointer.

```go title="Signature"
func (c fiber.Ctx) Request() *fasthttp.Request
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Request().Header.Method()
  // => []byte("GET")
})
```

## RequestCtx

Returns [\*fasthttp.RequestCtx](https://pkg.go.dev/github.com/valyala/fasthttp#RequestCtx) that is compatible with the `context.Context` interface that requires a deadline, a cancellation signal, and other values across API boundaries.

```go title="Signature"
func (c fiber.Ctx) RequestCtx() *fasthttp.RequestCtx
```

:::info
Please read the [Fasthttp Documentation](https://pkg.go.dev/github.com/valyala/fasthttp?tab=doc) for more information.
:::

## Response

Returns the [\*fasthttp.Response](https://pkg.go.dev/github.com/valyala/fasthttp#Response) pointer.

```go title="Signature"
func (c fiber.Ctx) Response() *fasthttp.Response
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Response().BodyWriter().Write([]byte("Hello, World!"))
  // => "Hello, World!"
  return nil
})
```

## Reset

Resets the context fields by the given request when using server handlers.

```go title="Signature"
func (c fiber.Ctx) Reset(fctx *fasthttp.RequestCtx)
```

It is used outside of the Fiber Handlers to reset the context for the next request.

## RestartRouting

Instead of executing the next method when calling [Next](ctx.md#next), **RestartRouting** restarts execution from the first method that matches the current route. This may be helpful after overriding the path, i.e., an internal redirect. Note that handlers might be executed again, which could result in an infinite loop.

```go title="Signature"
func (c fiber.Ctx) RestartRouting() error
```

```go title="Example"
app.Get("/new", func(c fiber.Ctx) error {
  return c.SendString("From /new")
})

app.Get("/old", func(c fiber.Ctx) error {
  c.Path("/new")
  return c.RestartRouting()
})
```

## Route

Returns the matched [Route](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc#Route) struct.

```go title="Signature"
func (c fiber.Ctx) Route() *Route
```

```go title="Example"
// http://localhost:8080/hello

app.Get("/hello/:name", func(c fiber.Ctx) error {
  r := c.Route()
  fmt.Println(r.Method, r.Path, r.Params, r.Handlers)
  // GET /hello/:name handler [name]

  // ...
})
```

:::caution
Do not rely on `c.Route()` in middlewares **before** calling `c.Next()` - `c.Route()` returns the **last executed route**.
:::

```go title="Example"
func MyMiddleware() fiber.Handler {
  return func(c fiber.Ctx) error {
    beforeNext := c.Route().Path // Will be '/'
    err := c.Next()
    afterNext := c.Route().Path // Will be '/hello/:name'
    return err
  }
}
```

## SaveFile

Method is used to save **any** multipart file to disk.

```go title="Signature"
func (c fiber.Ctx) SaveFile(fh *multipart.FileHeader, path string) error
```

```go title="Example"
app.Post("/", func(c fiber.Ctx) error {
  // Parse the multipart form:
  if form, err := c.MultipartForm(); err == nil {
    // => *multipart.Form

    // Get all files from "documents" key:
    files := form.File["documents"]
    // => []*multipart.FileHeader

    // Loop through files:
    for _, file := range files {
      fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
      // => "tutorial.pdf" 360641 "application/pdf"

      // Save the files to disk:
      if err := c.SaveFile(file, fmt.Sprintf("./%s", file.Filename)); err != nil {
        return err
      }
    }
    return err
  }
})
```

## SaveFileToStorage

Method is used to save **any** multipart file to an external storage system.

```go title="Signature"
func (c fiber.Ctx) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error
```

```go title="Example"
storage := memory.New()

app.Post("/", func(c fiber.Ctx) error {
  // Parse the multipart form:
  if form, err := c.MultipartForm(); err == nil {
    // => *multipart.Form

    // Get all files from "documents" key:
    files := form.File["documents"]
    // => []*multipart.FileHeader

    // Loop through files:
    for _, file := range files {
      fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
      // => "tutorial.pdf" 360641 "application/pdf"

      // Save the files to storage:
      if err := c.SaveFileToStorage(file, fmt.Sprintf("./%s", file.Filename), storage); err != nil {
        return err
      }
    }
    return err
  }
})
```

## Schema

Contains the request protocol string: `http` or `https` for TLS requests.

:::info
Please use [`Config.TrustProxy`](fiber.md#trustproxy) to prevent header spoofing if your app is behind a proxy.
:::

```go title="Signature"
func (c fiber.Ctx) Schema() string
```

```go title="Example"
// GET http://example.com

app.Get("/", func(c fiber.Ctx) error {
  c.Schema() // "http"

  // ...
})
```

## Secure

A boolean property that is `true` if a **TLS** connection is established.

```go title="Signature"
func (c fiber.Ctx) Secure() bool
```

```go title="Example"
// Secure() method is equivalent to:
c.Protocol() == "https"
```

## Send

Sets the HTTP response body.

```go title="Signature"
func (c fiber.Ctx) Send(body []byte) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.Send([]byte("Hello, World!")) // => "Hello, World!"
})
```

Fiber also provides `SendString` and `SendStream` methods for raw inputs.

:::tip
Use this if you **don't need** type assertion, recommended for **faster** performance.
:::

```go title="Signature"
func (c fiber.Ctx) SendString(body string) error
func (c fiber.Ctx) SendStream(stream io.Reader, size ...int) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.SendString("Hello, World!")
  // => "Hello, World!"

  return c.SendStream(bytes.NewReader([]byte("Hello, World!")))
  // => "Hello, World!"
})
```

## SendFile

Transfers the file from the given path. Sets the [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) response HTTP header field based on the **file** extension or format.

```go title="Config" title="Config"
// SendFile defines configuration options when to transfer file with SendFile.
type SendFile struct {
  // FS is the file system to serve the static files from.
  // You can use interfaces compatible with fs.FS like embed.FS, os.DirFS etc.
  //
  // Optional. Default: nil
  FS fs.FS

  // When set to true, the server tries minimizing CPU usage by caching compressed files.
  // This works differently than the github.com/gofiber/compression middleware.
  // You have to set Content-Encoding header to compress the file.
  // Available compression methods are gzip, br, and zstd.
  //
  // Optional. Default: false
  Compress bool `json:"compress"`

  // When set to true, enables byte range requests.
  //
  // Optional. Default: false
  ByteRange bool `json:"byte_range"`

  // When set to true, enables direct download.
  //
  // Optional. Default: false
  Download bool `json:"download"`

  // Expiration duration for inactive file handlers.
  // Use a negative time.Duration to disable it.
  //
  // Optional. Default: 10 * time.Second
  CacheDuration time.Duration `json:"cache_duration"`

  // The value for the Cache-Control HTTP-header
  // that is set on the file response. MaxAge is defined in seconds.
  //
  // Optional. Default: 0
  MaxAge int `json:"max_age"`
}
```

```go title="Signature" title="Signature"
func (c fiber.Ctx) SendFile(file string, config ...SendFile) error
```

```go title="Example"
app.Get("/not-found", func(c fiber.Ctx) error {
  return c.SendFile("./public/404.html")

  // Disable compression
  return c.SendFile("./static/index.html", fiber.SendFile{
    Compress: false,
  })
})
```

:::info
If the file contains a URL-specific character, you have to escape it before passing the file path into the `SendFile` function.
:::

```go title="Example"
app.Get("/file-with-url-chars", func(c fiber.Ctx) error {
  return c.SendFile(url.PathEscape("hash_sign_#.txt"))
})
```

:::info
You can set the `CacheDuration` config property to `-1` to disable caching.
:::

```go title="Example"
app.Get("/file", func(c fiber.Ctx) error {
  return c.SendFile("style.css", fiber.SendFile{
    CacheDuration: -1,
  })
})
```

:::info
You can use multiple `SendFile` calls with different configurations in a single route. Fiber creates different filesystem handlers per config.
:::

```go title="Example"
app.Get("/file", func(c fiber.Ctx) error {
  switch c.Query("config") {
    case "filesystem":
      return c.SendFile("style.css", fiber.SendFile{
        FS: os.DirFS(".")
      })
    case "filesystem-compress":
      return c.SendFile("style.css", fiber.SendFile{
        FS: os.DirFS("."),
        Compress: true,
      })
    case "compress":
      return c.SendFile("style.css", fiber.SendFile{
        Compress: true,
      })
    default:
      return c.SendFile("style.css")
  }

  return nil
})
```

:::info
For sending multiple files from an embedded file system, [this functionality](../middleware/static.md#serving-files-using-embedfs) can be used.
:::

## SendStatus

Sets the status code and the correct status message in the body if the response body is **empty**.

:::tip
You can find all used status codes and messages [here](https://github.com/gofiber/fiber/blob/dffab20bcdf4f3597d2c74633a7705a517d2c8c2/utils.go#L183-L244).
:::

```go title="Signature"
func (c fiber.Ctx) SendStatus(status int) error
```

```go title="Example"
app.Get("/not-found", func(c fiber.Ctx) error {
  return c.SendStatus(415)
  // => 415 "Unsupported Media Type"

  c.SendString("Hello, World!")
  return c.SendStatus(415)
  // => 415 "Hello, World!"
})
```

## SendStream

Sets the response body to a stream of data and adds an optional body size.

```go title="Signature"
func (c fiber.Ctx) SendStream(stream io.Reader, size ...int) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.SendStream(bytes.NewReader([]byte("Hello, World!")))
  // => "Hello, World!"
})
```

## SendString

Sets the response body to a string.

```go title="Signature"
func (c fiber.Ctx) SendString(body string) error
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.SendString("Hello, World!")
  // => "Hello, World!"
})
```

## SendStreamWriter

Sets the response body stream writer.

:::note
The argument `streamWriter` represents a function that populates
the response body using a buffered stream writer.
:::

```go title="Signature"
func (c Ctx) SendStreamWriter(streamWriter func(*bufio.Writer)) error
```

```go title="Example"
app.Get("/", func (c fiber.Ctx) error {
  return c.SendStreamWriter(func(w *bufio.Writer) {
    fmt.Fprintf(w, "Hello, World!\n")
  })
  // => "Hello, World!"
})
```

:::info
To send data before `streamWriter` returns, you can call `w.Flush()`
on the provided writer. Otherwise, the buffered stream flushes after
`streamWriter` returns.
:::

:::note
`w.Flush()` will return an error if the client disconnects before `streamWriter` finishes writing a response.
:::

```go title="Example"
app.Get("/wait", func(c fiber.Ctx) error {
  return c.SendStreamWriter(func(w *bufio.Writer) {
    // Begin Work
    fmt.Fprintf(w, "Please wait for 10 seconds\n")
    if err := w.Flush(); err != nil {
      log.Print("Client disconnected!")
      return
    }

    // Send progress over time
    time.Sleep(time.Second)
    for i := 0; i < 9; i++ {
      fmt.Fprintf(w, "Still waiting...\n")
      if err := w.Flush(); err != nil {
        // If client disconnected, cancel work and finish
        log.Print("Client disconnected!")
        return
      }
      time.Sleep(time.Second)
    }

    // Finish
    fmt.Fprintf(w, "Done!\n")
  })
})
```

## Set

Sets the response’s HTTP header field to the specified `key`, `value`.

```go title="Signature"
func (c fiber.Ctx) Set(key string, val string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Set("Content-Type", "text/plain")
  // => "Content-Type: text/plain"

  // ...
})
```

## SetContext

Sets the user-specified implementation for the `context.Context` interface.

```go title="Signature"
func (c fiber.Ctx) SetContext(ctx context.Context)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  ctx := context.Background()
  c.SetContext(ctx)
  // Here ctx could be any context implementation

  // ...
})
```

## Stale

[https://expressjs.com/en/4x/api.html#req.stale](https://expressjs.com/en/4x/api.html#req.stale)

```go title="Signature"
func (c fiber.Ctx) Stale() bool
```

## Status

Sets the HTTP status for the response.

:::info
This method is **chainable**.
:::

```go title="Signature"
func (c fiber.Ctx) Status(status int) fiber.Ctx
```

```go title="Example"
app.Get("/fiber", func(c fiber.Ctx) error {
  c.Status(fiber.StatusOK)
  return nil
})

app.Get("/hello", func(c fiber.Ctx) error {
  return c.Status(fiber.StatusBadRequest).SendString("Bad Request")
})

app.Get("/world", func(c fiber.Ctx) error {
  return c.Status(fiber.StatusNotFound).SendFile("./public/gopher.png")
})
```

## String

Returns a unique string representation of the context.

```go title="Signature"
func (c fiber.Ctx) String() string
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.String() // => "#0000000100000001 - 127.0.0.1:3000 <-> 127.0.0.1:61516 - GET http://localhost:3000/"

  // ...
})
```

## Subdomains

Returns a slice of subdomains in the domain name of the request.

The application property `subdomain offset`, which defaults to `2`, is used for determining the beginning of the subdomain segments.

```go title="Signature"
func (c fiber.Ctx) Subdomains(offset ...int) []string
```

```go title="Example"
// Host: "tobi.ferrets.example.com"

app.Get("/", func(c fiber.Ctx) error {
  c.Subdomains()    // ["ferrets", "tobi"]
  c.Subdomains(1)   // ["tobi"]

  // ...
})
```

## Type

Sets the [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) HTTP header to the MIME type listed [here](https://github.com/nginx/nginx/blob/master/conf/mime.types) specified by the file **extension**.

:::info
This method is **chainable**.
:::

```go title="Signature"
func (c fiber.Ctx) Type(ext string, charset ...string) fiber.Ctx
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Type(".html") // => "text/html"
  c.Type("html")  // => "text/html"
  c.Type("png")   // => "image/png"

  c.Type("json", "utf-8")  // => "application/json; charset=utf-8"

  // ...
})
```

## Vary

Adds the given header field to the [Vary](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Vary) response header. This will append the header if not already listed; otherwise, it leaves it listed in the current location.

:::info
Multiple fields are **allowed**.
:::

```go title="Signature"
func (c fiber.Ctx) Vary(fields ...string)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Vary("Origin")     // => Vary: Origin
  c.Vary("User-Agent") // => Vary: Origin, User-Agent

  // No duplicates
  c.Vary("Origin") // => Vary: Origin, User-Agent

  c.Vary("Accept-Encoding", "Accept")
  // => Vary: Origin, User-Agent, Accept-Encoding, Accept

  // ...
})
```

## ViewBind

Adds variables to the default view variable map binding to the template engine.
Variables are read by the `Render` method and may be overwritten.

```go title="Signature"
func (c fiber.Ctx) ViewBind(vars Map) error
```

```go title="Example"
app.Use(func(c fiber.Ctx) error {
  c.ViewBind(fiber.Map{
    "Title": "Hello, World!",
  })
  return c.Next()
})

app.Get("/", func(c fiber.Ctx) error {
  return c.Render("xxx.tmpl", fiber.Map{}) // Render will use the Title variable
})
```

## Write

Adopts the `Writer` interface.

```go title="Signature"
func (c fiber.Ctx) Write(p []byte) (n int, err error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  c.Write([]byte("Hello, World!")) // => "Hello, World!"

  fmt.Fprintf(c, "%s\n", "Hello, World!") // => "Hello, World!"
})
```

## Writef

Writes a formatted string using a format specifier.

```go title="Signature"
func (c fiber.Ctx) Writef(format string, a ...any) (n int, err error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  world := "World!"
  c.Writef("Hello, %s", world) // => "Hello, World!"

  fmt.Fprintf(c, "%s\n", "Hello, World!") // => "Hello, World!"
})
```

## WriteString

Writes a string to the response body.

```go title="Signature"
func (c fiber.Ctx) WriteString(s string) (n int, err error)
```

```go title="Example"
app.Get("/", func(c fiber.Ctx) error {
  return c.WriteString("Hello, World!")
  // => "Hello, World!"
})
```

## XHR

A boolean property that is `true` if the request’s [X-Requested-With](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers) header field is [XMLHttpRequest](https://developer.mozilla.org/en-US/docs/Web/API/XMLHttpRequest), indicating that the request was issued by a client library (such as [jQuery](https://api.jquery.com/jQuery.ajax/)).

```go title="Signature"
func (c fiber.Ctx) XHR() bool
```

```go title="Example"
// X-Requested-With: XMLHttpRequest

app.Get("/", func(c fiber.Ctx) error {
  c.XHR() // true

  // ...
})
```

## XML

Converts any **interface** or **string** to XML using the standard `encoding/xml` package.

:::info
XML also sets the content header to `application/xml`.
:::

```go title="Signature"
func (c fiber.Ctx) XML(data any) error
```

```go title="Example"
type SomeStruct struct {
  XMLName xml.Name `xml:"Fiber"`
  Name    string   `xml:"Name"`
  Age     uint8    `xml:"Age"`
}

app.Get("/", func(c fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.XML(data)
  // <Fiber>
  //     <Name>Grame</Name>
  //     <Age>20</Age>
  // </Fiber>
})
```
