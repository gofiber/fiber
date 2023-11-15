---
id: ctx
title: 🧠 Ctx
description: >-
  The Ctx struct represents the Context which hold the HTTP request and
  response. It has methods for the request query string, parameters, body, HTTP
  headers, and so on.
sidebar_position: 3
---

## Accepts

Checks, if the specified **extensions** or **content** **types** are acceptable.

:::info
Based on the request’s [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header.
:::

```go title="Signature"
func (c *Ctx) Accepts(offers ...string)          string
func (c *Ctx) AcceptsCharsets(offers ...string)  string
func (c *Ctx) AcceptsEncodings(offers ...string) string
func (c *Ctx) AcceptsLanguages(offers ...string) string
```

```go title="Example"
// Accept: text/html, application/json; q=0.8, text/plain; q=0.5; charset="utf-8"

app.Get("/", func(c *fiber.Ctx) error {
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

app.Get("/", func(c *fiber.Ctx) error {
  c.Accepts("text/plain", "application/json") // "application/json", due to specificity
  c.Accepts("application/json", "text/html") // "text/html", due to first match
  c.Accepts("image/png")        // "", due to */* without q factor 0 is Not Acceptable
  // ...
})
```

Media-Type parameters are supported.

```go title="Example 3"
// Accept: text/plain, application/json; version=1; foo=bar

app.Get("/", func(c *fiber.Ctx) error {
  // Extra parameters in the accept are ignored
  c.Accepts("text/plain;format=flowed") // "text/plain;format=flowed"
  
  // An offer must contain all parameters present in the Accept type
  c.Accepts("application/json") // ""

  // Parameter order and capitalization does not matter. Quotes on values are stripped.
  c.Accepts(`application/json;foo="bar";VERSION=1`) // "application/json;foo="bar";VERSION=1"
})
```

```go title="Example 4"
// Accept: text/plain;format=flowed;q=0.9, text/plain
// i.e., "I prefer text/plain;format=flowed less than other forms of text/plain"
app.Get("/", func(c *fiber.Ctx) error {
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

app.Get("/", func(c *fiber.Ctx) error {
  c.AcceptsCharsets("utf-16", "iso-8859-1")
  // "iso-8859-1"

  c.AcceptsEncodings("compress", "br")
  // "compress"

  c.AcceptsLanguages("pt", "nl", "ru")
  // "nl"
  // ...
})
```

## AllParams

Params is used to get all route parameters.
Using Params method to get params.

```go title="Signature"
func (c *Ctx) AllParams() map[string]string
```

```go title="Example"
// GET http://example.com/user/fenny
app.Get("/user/:name", func(c *fiber.Ctx) error {
  c.AllParams() // "{"name": "fenny"}"

  // ...
})

// GET http://example.com/user/fenny/123
app.Get("/user/*", func(c *fiber.Ctx) error {
  c.AllParams()  // "{"*1": "fenny/123"}"

  // ...
})
```

## App

Returns the [\*App](ctx.md) reference so you could easily access all application settings.

```go title="Signature"
func (c *Ctx) App() *App
```

```go title="Example"
app.Get("/stack", func(c *fiber.Ctx) error {
  return c.JSON(c.App().Stack())
})
```

## Append

Appends the specified **value** to the HTTP response header field.

:::caution
If the header is **not** already set, it creates the header with the specified value.
:::

```go title="Signature"
func (c *Ctx) Append(field string, values ...string)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Append("Link", "http://google.com", "http://localhost")
  // => Link: http://localhost, http://google.com

  c.Append("Link", "Test")
  // => Link: http://localhost, http://google.com, Test

  // ...
})
```

## Attachment

Sets the HTTP response [Content-Disposition](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition) header field to `attachment`.

```go title="Signature"
func (c *Ctx) Attachment(filename ...string)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Attachment()
  // => Content-Disposition: attachment

  c.Attachment("./upload/images/logo.png")
  // => Content-Disposition: attachment; filename="logo.png"
  // => Content-Type: image/png

  // ...
})
```

## BaseURL

Returns the base URL \(**protocol** + **host**\) as a `string`.

```go title="Signature"
func (c *Ctx) BaseURL() string
```

```go title="Example"
// GET https://example.com/page#chapter-1

app.Get("/", func(c *fiber.Ctx) error {
  c.BaseURL() // https://example.com
  // ...
})
```

## Bind

Add vars to default view var map binding to template engine.
Variables are read by the Render method and may be overwritten.

```go title="Signature"
func (c *Ctx) Bind(vars Map) error
```

```go title="Example"
app.Use(func(c *fiber.Ctx) error {
  c.Bind(fiber.Map{
    "Title": "Hello, World!",
  })
})

app.Get("/", func(c *fiber.Ctx) error {
  return c.Render("xxx.tmpl", fiber.Map{}) // Render will use Title variable
})
```

## BodyRaw

Returns the raw request **body**.

```go title="Signature"
func (c *Ctx) BodyRaw() []byte
```

```go title="Example"
// curl -X POST http://localhost:8080 -d user=john

app.Post("/", func(c *fiber.Ctx) error {
  // Get raw body from POST request:
  return c.Send(c.BodyRaw()) // []byte("user=john")
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## Body

As per the header `Content-Encoding`, this method will try to perform a file decompression from the **body** bytes. In case no `Content-Encoding` header is sent, it will perform as [BodyRaw](#bodyraw).

```go title="Signature"
func (c *Ctx) Body() []byte
```

```go title="Example"
// echo 'user=john' | gzip | curl -v -i --data-binary @- -H "Content-Encoding: gzip" http://localhost:8080

app.Post("/", func(c *fiber.Ctx) error {
  // Decompress body from POST request based on the Content-Encoding and return the raw content:
  return c.Send(c.Body()) // []byte("user=john")
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## BodyParser

Binds the request body to a struct.

It is important to specify the correct struct tag based on the content type to be parsed. For example, if you want to parse a JSON body with a field called Pass, you would use a struct field of `json:"pass"`.

| content-type                        | struct tag |
| ----------------------------------- | ---------- |
| `application/x-www-form-urlencoded` | form       |
| `multipart/form-data`               | form       |
| `application/json`                  | json       |
| `application/xml`                   | xml        |
| `text/xml`                          | xml        |

```go title="Signature"
func (c *Ctx) BodyParser(out interface{}) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name string `json:"name" xml:"name" form:"name"`
    Pass string `json:"pass" xml:"pass" form:"pass"`
}

app.Post("/", func(c *fiber.Ctx) error {
        p := new(Person)

        if err := c.BodyParser(p); err != nil {
            return err
        }

        log.Println(p.Name) // john
        log.Println(p.Pass) // doe

        // ...
})

// Run tests with the following curl commands

// curl -X POST -H "Content-Type: application/json" --data "{\"name\":\"john\",\"pass\":\"doe\"}" localhost:3000

// curl -X POST -H "Content-Type: application/xml" --data "<login><name>john</name><pass>doe</pass></login>" localhost:3000

// curl -X POST -H "Content-Type: application/x-www-form-urlencoded" --data "name=john&pass=doe" localhost:3000

// curl -X POST -F name=john -F pass=doe http://localhost:3000

// curl -X POST "http://localhost:3000/?name=john&pass=doe"
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## ClearCookie

Expire a client cookie \(_or all cookies if left empty\)_

```go title="Signature"
func (c *Ctx) ClearCookie(key ...string)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
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
Web browsers and other compliant clients will only clear the cookie if the given options are identical to those when creating the cookie, excluding expires and maxAge. ClearCookie will not set these values for you - a technique similar to the one shown below should be used to ensure your cookie is deleted.
:::

```go title="Example"
app.Get("/set", func(c *fiber.Ctx) error {
    c.Cookie(&fiber.Cookie{
        Name:     "token",
        Value:    "randomvalue",
        Expires:  time.Now().Add(24 * time.Hour),
        HTTPOnly: true,
        SameSite: "lax",
    })

    // ...
})

app.Get("/delete", func(c *fiber.Ctx) error {
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

ClientHelloInfo contains information from a ClientHello message in order to guide application logic in the GetCertificate and GetConfigForClient callbacks.
You can refer to the [ClientHelloInfo](https://golang.org/pkg/crypto/tls/#ClientHelloInfo) struct documentation for more information on the returned struct.

```go title="Signature"
func (c *Ctx) ClientHelloInfo() *tls.ClientHelloInfo
```

```go title="Example"
// GET http://example.com/hello
app.Get("/hello", func(c *fiber.Ctx) error {
  chi := c.ClientHelloInfo()
  // ...
})
```

## Context

Returns [\*fasthttp.RequestCtx](https://godoc.org/github.com/valyala/fasthttp#RequestCtx) that is compatible with the context.Context interface that requires a deadline, a cancellation signal, and other values across API boundaries.

```go title="Signature"
func (c *Ctx) Context() *fasthttp.RequestCtx
```

:::info
Please read the [Fasthttp Documentation](https://pkg.go.dev/github.com/valyala/fasthttp?tab=doc) for more information.
:::

## Cookie

Set cookie

```go title="Signature"
func (c *Ctx) Cookie(cookie *Cookie)
```

```go
type Cookie struct {
    Name        string    `json:"name"`
    Value       string    `json:"value"`
    Path        string    `json:"path"`
    Domain      string    `json:"domain"`
    MaxAge      int       `json:"max_age"`
    Expires     time.Time `json:"expires"`
    Secure      bool      `json:"secure"`
    HTTPOnly    bool      `json:"http_only"`
    SameSite    string    `json:"same_site"`
    SessionOnly bool      `json:"session_only"`
}
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
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

## CookieParser

This method is similar to [BodyParser](ctx.md#bodyparser), but for cookie parameters.
It is important to use the struct tag "cookie". For example, if you want to parse a cookie with a field called Age, you would use a struct field of `cookie:"age"`.

```go title="Signature"
func (c *Ctx) CookieParser(out interface{}) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name     string  `cookie:"name"`
    Age      int     `cookie:"age"`
    Job      bool    `cookie:"job"`
}

app.Get("/", func(c *fiber.Ctx) error {
        p := new(Person)

        if err := c.CookieParser(p); err != nil {
            return err
        }

        log.Println(p.Name)     // Joseph
        log.Println(p.Age)      // 23
        log.Println(p.Job)      // true
})
// Run tests with the following curl command
// curl.exe --cookie "name=Joseph; age=23; job=true" http://localhost:8000/
```

## Cookies

Get cookie value by key, you could pass an optional default value that will be returned if the cookie key does not exist.

```go title="Signature"
func (c *Ctx) Cookies(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  // Get cookie by key:
  c.Cookies("name")         // "john"
  c.Cookies("empty", "doe") // "doe"
  // ...
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## Download

Transfers the file from path as an `attachment`.

Typically, browsers will prompt the user to download. By default, the [Content-Disposition](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition) header `filename=` parameter is the file path \(_this typically appears in the browser dialog_\).

Override this default with the **filename** parameter.

```go title="Signature"
func (c *Ctx) Download(file string, filename ...string) error
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  return c.Download("./files/report-12345.pdf");
  // => Download report-12345.pdf

  return c.Download("./files/report-12345.pdf", "report.pdf");
  // => Download report.pdf
})
```

## Format

Performs content-negotiation on the [Accept](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept) HTTP header. It uses [Accepts](ctx.md#accepts) to select a proper format.

:::info
If the header is **not** specified or there is **no** proper format, **text/plain** is used.
:::

```go title="Signature"
func (c *Ctx) Format(body interface{}) error
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  // Accept: text/plain
  c.Format("Hello, World!")
  // => Hello, World!

  // Accept: text/html
  c.Format("Hello, World!")
  // => <p>Hello, World!</p>

  // Accept: application/json
  c.Format("Hello, World!")
  // => "Hello, World!"
  // ..
})
```

## FormFile

MultipartForm files can be retrieved by name, the **first** file from the given key is returned.

```go title="Signature"
func (c *Ctx) FormFile(key string) (*multipart.FileHeader, error)
```

```go title="Example"
app.Post("/", func(c *fiber.Ctx) error {
  // Get first file from form field "document":
  file, err := c.FormFile("document")

  // Save file to root directory:
  return c.SaveFile(file, fmt.Sprintf("./%s", file.Filename))
})
```

## FormValue

Any form values can be retrieved by name, the **first** value from the given key is returned.

```go title="Signature"
func (c *Ctx) FormValue(key string, defaultValue ...string) string
```

```go title="Example"
app.Post("/", func(c *fiber.Ctx) error {
  // Get first value from form field "name":
  c.FormValue("name")
  // => "john" or "" if not exist

  // ..
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## Fresh

When the response is still **fresh** in the client's cache **true** is returned, otherwise **false** is returned to indicate that the client cache is now stale and the full response should be sent.

When a client sends the Cache-Control: no-cache request header to indicate an end-to-end reload request, `Fresh` will return false to make handling these requests transparent.

Read more on [https://expressjs.com/en/4x/api.html\#req.fresh](https://expressjs.com/en/4x/api.html#req.fresh)

```go title="Signature"
func (c *Ctx) Fresh() bool
```

## Get

Returns the HTTP request header specified by the field.

:::tip
The match is **case-insensitive**.
:::

```go title="Signature"
func (c *Ctx) Get(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Get("Content-Type")       // "text/plain"
  c.Get("CoNtEnT-TypE")       // "text/plain"
  c.Get("something", "john")  // "john"
  // ..
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## GetReqHeaders

Returns the HTTP request headers as a map. Since a header can be set multiple times in a single request, the values of the map are slices of strings containing all the different values of the header.

```go title="Signature"
func (c *Ctx) GetReqHeaders() map[string][]string
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## GetRespHeader

Returns the HTTP response header specified by the field.

:::tip
The match is **case-insensitive**.
:::

```go title="Signature"
func (c *Ctx) GetRespHeader(key string, defaultValue ...string) string
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.GetRespHeader("X-Request-Id")       // "8d7ad5e3-aaf3-450b-a241-2beb887efd54"
  c.GetRespHeader("Content-Type")       // "text/plain"
  c.GetRespHeader("something", "john")  // "john"
  // ..
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## GetRespHeaders

Returns the HTTP response headers as a map. Since a header can be set multiple times in a single request, the values of the map are slices of strings containing all the different values of the header.

```go title="Signature"
func (c *Ctx) GetRespHeaders() map[string][]string
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## GetRouteURL

Generates URLs to named routes, with parameters. URLs are relative, for example: "/user/1831"

```go title="Signature"
func (c *Ctx) GetRouteURL(routeName string, params Map) (string, error)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
    return c.SendString("Home page")
}).Name("home")

app.Get("/user/:id", func(c *fiber.Ctx) error {
    return c.SendString(c.Params("id"))
}).Name("user.show")

app.Get("/test", func(c *fiber.Ctx) error {
    location, _ := c.GetRouteURL("user.show", fiber.Map{"id": 1})
    return c.SendString(location)
})

// /test returns "/user/1"
```

## Hostname

Returns the hostname derived from the [Host](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Host) HTTP header.

```go title="Signature"
func (c *Ctx) Hostname() string
```

```go title="Example"
// GET http://google.com/search

app.Get("/", func(c *fiber.Ctx) error {
  c.Hostname() // "google.com"

  // ...
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## IP

Returns the remote IP address of the request.

```go title="Signature"
func (c *Ctx) IP() string
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.IP() // "127.0.0.1"

  // ...
})
```

When registering the proxy request header in the fiber app, the ip address of the header is returned [(Fiber configuration)](fiber.md#config)

```go
app := fiber.New(fiber.Config{
  ProxyHeader: fiber.HeaderXForwardedFor,
})
```

## IPs

Returns an array of IP addresses specified in the [X-Forwarded-For](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For) request header.

```go title="Signature"
func (c *Ctx) IPs() []string
```

```go title="Example"
// X-Forwarded-For: proxy1, 127.0.0.1, proxy3

app.Get("/", func(c *fiber.Ctx) error {
  c.IPs() // ["proxy1", "127.0.0.1", "proxy3"]

  // ...
})
```

:::caution
Improper use of the X-Forwarded-For header can be a security risk. For details, see the [Security and privacy concerns](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For#security_and_privacy_concerns) section.
:::

## Is

Returns the matching **content type**, if the incoming request’s [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) HTTP header field matches the [MIME type](https://developer.mozilla.org/ru/docs/Web/HTTP/Basics_of_HTTP/MIME_types) specified by the type parameter.

:::info
If the request has **no** body, it returns **false**.
:::

```go title="Signature"
func (c *Ctx) Is(extension string) bool
```

```go title="Example"
// Content-Type: text/html; charset=utf-8

app.Get("/", func(c *fiber.Ctx) error {
  c.Is("html")  // true
  c.Is(".html") // true
  c.Is("json")  // false

  // ...
})
```

## IsFromLocal

Returns true if request came from localhost

```go title="Signature"
func (c *Ctx) IsFromLocal() bool {
```

```go title="Example"

app.Get("/", func(c *fiber.Ctx) error {
  // If request came from localhost, return true else return false
  c.IsFromLocal()

  // ...
})
```

## JSON

Converts any **interface** or **string** to JSON using the [encoding/json](https://pkg.go.dev/encoding/json) package.

:::info
JSON also sets the content header to the `ctype` parameter. If no `ctype` is passed in, the header is set to `application/json`.
:::

```go title="Signature"
func (c *Ctx) JSON(data interface{}, ctype ...string) error
```

```go title="Example"
type SomeStruct struct {
  Name string
  Age  uint8
}

app.Get("/json", func(c *fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.JSON(data)
  // => Content-Type: application/json
  // => "{"Name": "Grame", "Age": 20}"

  return c.JSON(fiber.Map{
    "name": "Grame",
    "age": 20,
  })
  // => Content-Type: application/json
  // => "{"name": "Grame", "age": 20}"

  return c.JSON(fiber.Map{
    "type": "https://example.com/probs/out-of-credit",
    "title": "You do not have enough credit.",
    "status": 403,
    "detail": "Your current balance is 30, but that costs 50.",
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

Sends a JSON response with JSONP support. This method is identical to [JSON](ctx.md#json), except that it opts-in to JSONP callback support. By default, the callback name is simply callback.

Override this by passing a **named string** in the method.

```go title="Signature"
func (c *Ctx) JSONP(data interface{}, callback ...string) error
```

```go title="Example"
type SomeStruct struct {
  name string
  age  uint8
}

app.Get("/", func(c *fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    name: "Grame",
    age:  20,
  }

  return c.JSONP(data)
  // => callback({"name": "Grame", "age": 20})

  return c.JSONP(data, "customFunc")
  // => customFunc({"name": "Grame", "age": 20})
})
```

## Links

Joins the links followed by the property to populate the response’s [Link](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Link) HTTP header field.

```go title="Signature"
func (c *Ctx) Links(link ...string)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
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

A method that stores variables scoped to the request and, therefore, are available only to the routes that match the request.

:::tip
This is useful if you want to pass some **specific** data to the next middleware.
:::

```go title="Signature"
func (c *Ctx) Locals(key interface{}, value ...interface{}) interface{}
```

```go title="Example"
app.Use(func(c *fiber.Ctx) error {
  c.Locals("user", "admin")
  return c.Next()
})

app.Get("/admin", func(c *fiber.Ctx) error {
  if c.Locals("user") == "admin" {
    return c.Status(fiber.StatusOK).SendString("Welcome, admin!")
  }
  return c.SendStatus(fiber.StatusForbidden)

})
```

## Location

Sets the response [Location](https://developer.mozilla.org/ru/docs/Web/HTTP/Headers/Location) HTTP header to the specified path parameter.

```go title="Signature"
func (c *Ctx) Location(path string)
```

```go title="Example"
app.Post("/", func(c *fiber.Ctx) error {
  c.Location("http://example.com")

  c.Location("/foo/bar")

  return nil
})
```

## Method

Returns a string corresponding to the HTTP method of the request: `GET`, `POST`, `PUT`, and so on.  
Optionally, you could override the method by passing a string.

```go title="Signature"
func (c *Ctx) Method(override ...string) string
```

```go title="Example"
app.Post("/", func(c *fiber.Ctx) error {
  c.Method() // "POST"

  c.Method("GET")
  c.Method() // GET

  // ...
})
```

## MultipartForm

To access multipart form entries, you can parse the binary with `MultipartForm()`. This returns a `map[string][]string`, so given a key, the value will be a string slice.

```go title="Signature"
func (c *Ctx) MultipartForm() (*multipart.Form, error)
```

```go title="Example"
app.Post("/", func(c *fiber.Ctx) error {
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

  return err
})
```

## Next

When **Next** is called, it executes the next method in the stack that matches the current route. You can pass an error struct within the method that will end the chaining and call the [error handler](https://docs.gofiber.io/guide/error-handling).

```go title="Signature"
func (c *Ctx) Next() error
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  fmt.Println("1st route!")
  return c.Next()
})

app.Get("*", func(c *fiber.Ctx) error {
  fmt.Println("2nd route!")
  return c.Next()
})

app.Get("/", func(c *fiber.Ctx) error {
  fmt.Println("3rd route!")
  return c.SendString("Hello, World!")
})
```

## OriginalURL

Returns the original request URL.

```go title="Signature"
func (c *Ctx) OriginalURL() string
```

```go title="Example"
// GET http://example.com/search?q=something

app.Get("/", func(c *fiber.Ctx) error {
  c.OriginalURL() // "/search?q=something"

  // ...
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## Params

Method can be used to get the route parameters, you could pass an optional default value that will be returned if the param key does not exist.

:::info
Defaults to empty string \(`""`\), if the param **doesn't** exist.
:::

```go title="Signature"
func (c *Ctx) Params(key string, defaultValue ...string) string
```

```go title="Example"
// GET http://example.com/user/fenny
app.Get("/user/:name", func(c *fiber.Ctx) error {
  c.Params("name") // "fenny"

  // ...
})

// GET http://example.com/user/fenny/123
app.Get("/user/*", func(c *fiber.Ctx) error {
  c.Params("*")  // "fenny/123"
  c.Params("*1") // "fenny/123"

  // ...
})
```

Unnamed route parameters\(\*, +\) can be fetched by the **character** and the **counter** in the route.

```go title="Example"
// ROUTE: /v1/*/shop/*
// GET:   /v1/brand/4/shop/blue/xs
c.Params("*1")  // "brand/4"
c.Params("*2")  // "blue/xs"
```

For reasons of **downward compatibility**, the first parameter segment for the parameter character can also be accessed without the counter.

```go title="Example"
app.Get("/v1/*/shop/*", func(c *fiber.Ctx) error {
  c.Params("*") // outputs the values of the first wildcard segment
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## ParamsInt

Method can be used to get an integer from the route parameters.
Please note if that parameter is not in the request, zero
will be returned. If the parameter is NOT a number, zero and an error
will be returned

:::info
Defaults to the integer zero \(`0`\), if the param **doesn't** exist.
:::

```go title="Signature"
func (c *Ctx) ParamsInt(key string) (int, error)
```

```go title="Example"
// GET http://example.com/user/123
app.Get("/user/:id", func(c *fiber.Ctx) error {
  id, err := c.ParamsInt("id") // int 123 and no error

  // ...
})

```

This method is equivalent of using `atoi` with ctx.Params

## ParamsParser

This method is similar to BodyParser, but for path parameters. It is important to use the struct tag "params". For example, if you want to parse a path parameter with a field called Pass, you would use a struct field of params:"pass"

```go title="Signature"
func (c *Ctx) ParamsParser(out interface{}) error
```

```go title="Example"
// GET http://example.com/user/111
app.Get("/user/:id", func(c *fiber.Ctx) error {
  param := struct {ID uint `params:"id"`}{}

  c.ParamsParser(&param) // "{"id": 111}"

  // ...
})

```

## Path

Contains the path part of the request URL. Optionally, you could override the path by passing a string. For internal redirects, you might want to call [RestartRouting](ctx.md#restartrouting) instead of [Next](ctx.md#next).

```go title="Signature"
func (c *Ctx) Path(override ...string) string
```

```go title="Example"
// GET http://example.com/users?sort=desc

app.Get("/users", func(c *fiber.Ctx) error {
  c.Path() // "/users"

  c.Path("/john")
  c.Path() // "/john"

  // ...
})
```

## Protocol

Contains the request protocol string: `http` or `https` for **TLS** requests.

```go title="Signature"
func (c *Ctx) Protocol() string
```

```go title="Example"
// GET http://example.com

app.Get("/", func(c *fiber.Ctx) error {
  c.Protocol() // "http"

  // ...
})
```

## Queries

Queries is a function that returns an object containing a property for each query string parameter in the route.

```go title="Signature"
func (c *Ctx) Queries() map[string]string
```

```go title="Example"
// GET http://example.com/?name=alex&want_pizza=false&id=

app.Get("/", func(c *fiber.Ctx) error {
	m := c.Queries()
	m["name"] // "alex"
	m["want_pizza"] // "false"
	m["id"] // ""
	// ...
})
```

```go title="Example"
// GET http://example.com/?field1=value1&field1=value2&field2=value3

app.Get("/", func (c *fiber.Ctx) error {
	m := c.Queries()
	m["field1"] // "value2"
	m["field2"] // value3
})
```

```go title="Example"
// GET http://example.com/?list_a=1&list_a=2&list_a=3&list_b[]=1&list_b[]=2&list_b[]=3&list_c=1,2,3

app.Get("/", func(c *fiber.Ctx) error {
	m := c.Queries()
	m["list_a"] // "3"
	m["list_b[]"] // "3"
	m["list_c"] // "1,2,3"
})
```

```go title="Example"
// GET /api/posts?filters.author.name=John&filters.category.name=Technology

app.Get("/", func(c *fiber.Ctx) error {
	m := c.Queries()
	m["filters.author.name"] // John
	m["filters.category.name"] // Technology
})
```

```go title="Example"
// GET /api/posts?tags=apple,orange,banana&filters[tags]=apple,orange,banana&filters[category][name]=fruits&filters.tags=apple,orange,banana&filters.category.name=fruits

app.Get("/", func(c *fiber.Ctx) error {
	m := c.Queries()
	m["tags"] // apple,orange,banana
	m["filters[tags]"] // apple,orange,banana
	m["filters[category][name]"] // fruits
	m["filters.tags"] // apple,orange,banana
	m["filters.category.name"] // fruits
})
```

## Query

This property is an object containing a property for each query string parameter in the route, you could pass an optional default value that will be returned if the query key does not exist.

:::info
If there is **no** query string, it returns an **empty string**.
:::

```go title="Signature"
func (c *Ctx) Query(key string, defaultValue ...string) string
```

```go title="Example"
// GET http://example.com/?order=desc&brand=nike

app.Get("/", func(c *fiber.Ctx) error {
  c.Query("order")         // "desc"
  c.Query("brand")         // "nike"
  c.Query("empty", "nike") // "nike"

  // ...
})
```

> _Returned value is only valid within the handler. Do not store any references.  
> Make copies or use the_ [_**`Immutable`**_](ctx.md) _setting instead._ [_Read more..._](../#zero-allocation)

## QueryBool

This property is an object containing a property for each query boolean parameter in the route, you could pass an optional default value that will be returned if the query key does not exist.

:::caution
Please note if that parameter is not in the request, false will be returned.
If the parameter is not a boolean, it is still tried to be converted and usually returned as false.
:::

```go title="Signature"
func (c *Ctx) QueryBool(key string, defaultValue ...bool) bool
```

```go title="Example"
// GET http://example.com/?name=alex&want_pizza=false&id=

app.Get("/", func(c *fiber.Ctx) error {
    c.QueryBool("want_pizza")           // false
	c.QueryBool("want_pizza", true) // false
    c.QueryBool("name")                 // false
    c.QueryBool("name", true)           // true
    c.QueryBool("id")                   // false
    c.QueryBool("id", true)             // true

  // ...
})
```

## QueryFloat

This property is an object containing a property for each query float64 parameter in the route, you could pass an optional default value that will be returned if the query key does not exist.

:::caution
Please note if that parameter is not in the request, zero will be returned.
If the parameter is not a number, it is still tried to be converted and usually returned as 1.
:::

:::info
Defaults to the float64 zero \(`0`\), if the param **doesn't** exist.
:::

```go title="Signature"
func (c *Ctx) QueryFloat(key string, defaultValue ...float64) float64
```

```go title="Example"
// GET http://example.com/?name=alex&amount=32.23&id=

app.Get("/", func(c *fiber.Ctx) error {
    c.QueryFloat("amount")      // 32.23
    c.QueryFloat("amount", 3)   // 32.23
    c.QueryFloat("name", 1)     // 1
    c.QueryFloat("name")        // 0
    c.QueryFloat("id", 3)       // 3

  // ...
})
```

## QueryInt

This property is an object containing a property for each query integer parameter in the route, you could pass an optional default value that will be returned if the query key does not exist.

:::caution
Please note if that parameter is not in the request, zero will be returned.
If the parameter is not a number, it is still tried to be converted and usually returned as 1.
:::

:::info
Defaults to the integer zero \(`0`\), if the param **doesn't** exist.
:::

```go title="Signature"
func (c *Ctx) QueryInt(key string, defaultValue ...int) int
```

```go title="Example"
// GET http://example.com/?name=alex&wanna_cake=2&id=

app.Get("/", func(c *fiber.Ctx) error {
    c.QueryInt("wanna_cake", 1) // 2
    c.QueryInt("name", 1)       // 1
    c.QueryInt("id", 1)         // 1
    c.QueryInt("id")            // 0

  // ...
})
```

## QueryParser

This method is similar to [BodyParser](ctx.md#bodyparser), but for query parameters.
It is important to use the struct tag "query". For example, if you want to parse a query parameter with a field called Pass, you would use a struct field of `query:"pass"`.

```go title="Signature"
func (c *Ctx) QueryParser(out interface{}) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name     string     `query:"name"`
    Pass     string     `query:"pass"`
    Products []string   `query:"products"`
}

app.Get("/", func(c *fiber.Ctx) error {
        p := new(Person)

        if err := c.QueryParser(p); err != nil {
            return err
        }

        log.Println(p.Name)     // john
        log.Println(p.Pass)     // doe
        log.Println(p.Products) // [shoe, hat]

        // ...
})
// Run tests with the following curl command

// curl "http://localhost:3000/?name=john&pass=doe&products=shoe,hat"
```

## Range

A struct containing the type and a slice of ranges will be returned.

```go title="Signature"
func (c *Ctx) Range(size int) (Range, error)
```

```go title="Example"
// Range: bytes=500-700, 700-900
app.Get("/", func(c *fiber.Ctx) error {
  b := c.Range(1000)
  if b.Type == "bytes" {
      for r := range r.Ranges {
      fmt.Println(r)
      // [500, 700]
    }
  }
})
```

## Redirect

Redirects to the URL derived from the specified path, with specified status, a positive integer that corresponds to an HTTP status code.

:::info
If **not** specified, status defaults to **302 Found**.
:::

```go title="Signature"
func (c *Ctx) Redirect(location string, status ...int) error
```

```go title="Example"
app.Get("/coffee", func(c *fiber.Ctx) error {
  return c.Redirect("/teapot")
})

app.Get("/teapot", func(c *fiber.Ctx) error {
  return c.Status(fiber.StatusTeapot).Send("🍵 short and stout 🍵")
})
```

```go title="More examples"
app.Get("/", func(c *fiber.Ctx) error {
  return c.Redirect("/foo/bar")
  return c.Redirect("../login")
  return c.Redirect("http://example.com")
  return c.Redirect("http://example.com", 301)
})
```

## RedirectToRoute

Redirects to the specific route along with the parameters and with specified status, a positive integer that corresponds to an HTTP status code.

:::info
If **not** specified, status defaults to **302 Found**.
:::

:::info
If you want to send queries to route, you must add **"queries"** key typed as **map[string]string** to params.
:::

```go title="Signature"
func (c *Ctx) RedirectToRoute(routeName string, params fiber.Map, status ...int) error
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  // /user/fiber
  return c.RedirectToRoute("user", fiber.Map{
    "name": "fiber"
  })
})

app.Get("/with-queries", func(c *fiber.Ctx) error {
  // /user/fiber?data[0][name]=john&data[0][age]=10&test=doe
  return c.RedirectToRoute("user", fiber.Map{
    "name": "fiber",
    "queries": map[string]string{"data[0][name]": "john", "data[0][age]": "10", "test": "doe"},
  })
})

app.Get("/user/:name", func(c *fiber.Ctx) error {
  return c.SendString(c.Params("name"))
}).Name("user")
```

## RedirectBack

Redirects back to refer URL. It redirects to fallback URL if refer header doesn't exists, with specified status, a positive integer that corresponds to an HTTP status code.

:::info
If **not** specified, status defaults to **302 Found**.
:::

```go title="Signature"
func (c *Ctx) RedirectBack(fallback string, status ...int) error
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  return c.SendString("Home page")
})
app.Get("/test", func(c *fiber.Ctx) error {
  c.Set("Content-Type", "text/html")
  return c.SendString(`<a href="/back">Back</a>`)
})

app.Get("/back", func(c *fiber.Ctx) error {
  return c.RedirectBack("/")
})
```

## Render

Renders a view with data and sends a `text/html` response. By default `Render` uses the default [**Go Template engine**](https://pkg.go.dev/html/template/). If you want to use another View engine, please take a look at our [**Template middleware**](https://docs.gofiber.io/template).

```go title="Signature"
func (c *Ctx) Render(name string, bind interface{}, layouts ...string) error
```

## Request

Request return the [\*fasthttp.Request](https://godoc.org/github.com/valyala/fasthttp#Request) pointer

```go title="Signature"
func (c *Ctx) Request() *fasthttp.Request
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Request().Header.Method()
  // => []byte("GET")
})
```

## ReqHeaderParser

This method is similar to [BodyParser](ctx.md#bodyparser), but for request headers.
It is important to use the struct tag "reqHeader". For example, if you want to parse a request header with a field called Pass, you would use a struct field of `reqHeader:"pass"`.

```go title="Signature"
func (c *Ctx) ReqHeaderParser(out interface{}) error
```

```go title="Example"
// Field names should start with an uppercase letter
type Person struct {
    Name     string     `reqHeader:"name"`
    Pass     string     `reqHeader:"pass"`
    Products []string   `reqHeader:"products"`
}

app.Get("/", func(c *fiber.Ctx) error {
        p := new(Person)

        if err := c.ReqHeaderParser(p); err != nil {
            return err
        }

        log.Println(p.Name)     // john
        log.Println(p.Pass)     // doe
        log.Println(p.Products) // [shoe, hat]

        // ...
})
// Run tests with the following curl command

// curl "http://localhost:3000/" -H "name: john" -H "pass: doe" -H "products: shoe,hat"
```

## Response

Response return the [\*fasthttp.Response](https://godoc.org/github.com/valyala/fasthttp#Response) pointer

```go title="Signature"
func (c *Ctx) Response() *fasthttp.Response
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Response().BodyWriter().Write([]byte("Hello, World!"))
  // => "Hello, World!"
  return nil
})
```

## RestartRouting

Instead of executing the next method when calling [Next](ctx.md#next), **RestartRouting** restarts execution from the first method that matches the current route. This may be helpful after overriding the path, i. e. an internal redirect. Note that handlers might be executed again which could result in an infinite loop.

```go title="Signature"
func (c *Ctx) RestartRouting() error
```

```go title="Example"
app.Get("/new", func(c *fiber.Ctx) error {
  return c.SendString("From /new")
})

app.Get("/old", func(c *fiber.Ctx) error {
  c.Path("/new")
  return c.RestartRouting()
})
```

## Route

Returns the matched [Route](https://pkg.go.dev/github.com/gofiber/fiber?tab=doc#Route) struct.

```go title="Signature"
func (c *Ctx) Route() *Route
```

```go title="Example"
// http://localhost:8080/hello


app.Get("/hello/:name", func(c *fiber.Ctx) error {
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
  return func(c *fiber.Ctx) error {
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
func (c *Ctx) SaveFile(fh *multipart.FileHeader, path string) error
```

```go title="Example"
app.Post("/", func(c *fiber.Ctx) error {
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
func (c *Ctx) SaveFileToStorage(fileheader *multipart.FileHeader, path string, storage Storage) error
```

```go title="Example"
storage := memory.New()

app.Post("/", func(c *fiber.Ctx) error {
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

## Secure

A boolean property that is `true` , if a **TLS** connection is established.

```go title="Signature"
func (c *Ctx) Secure() bool
```

```go title="Example"
// Secure() method is equivalent to:
c.Protocol() == "https"
```

## Send

Sets the HTTP response body.

```go title="Signature"
func (c *Ctx) Send(body []byte) error
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  return c.Send([]byte("Hello, World!")) // => "Hello, World!"
})
```

Fiber also provides `SendString` and `SendStream` methods for raw inputs.

:::tip
Use this if you **don't need** type assertion, recommended for **faster** performance.
:::

```go title="Signature"
func (c *Ctx) SendString(body string) error
func (c *Ctx) SendStream(stream io.Reader, size ...int) error
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  return c.SendString("Hello, World!")
  // => "Hello, World!"

  return c.SendStream(bytes.NewReader([]byte("Hello, World!")))
  // => "Hello, World!"
})
```

## SendFile

Transfers the file from the given path. Sets the [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) response HTTP header field based on the **filenames** extension.

:::caution
Method doesn´t use **gzipping** by default, set it to **true** to enable.
:::

```go title="Signature" title="Signature"
func (c *Ctx) SendFile(file string, compress ...bool) error
```

```go title="Example"
app.Get("/not-found", func(c *fiber.Ctx) error {
  return c.SendFile("./public/404.html");

  // Disable compression
  return c.SendFile("./static/index.html", false);
})
```

:::info
If the file contains an url specific character you have to escape it before passing the file path into the `sendFile` function.
:::

```go title="Example"
app.Get("/file-with-url-chars", func(c *fiber.Ctx) error {
  return c.SendFile(url.PathEscape("hash_sign_#.txt"))
})
```

:::info
For sending files from embedded file system [this functionality](./middleware/filesystem.md#sendfile) can be used
:::

## SendStatus

Sets the status code and the correct status message in the body, if the response body is **empty**.

:::tip
You can find all used status codes and messages [here](https://github.com/gofiber/fiber/blob/dffab20bcdf4f3597d2c74633a7705a517d2c8c2/utils.go#L183-L244).
:::

```go title="Signature"
func (c *Ctx) SendStatus(status int) error
```

```go title="Example"
app.Get("/not-found", func(c *fiber.Ctx) error {
  return c.SendStatus(415)
  // => 415 "Unsupported Media Type"

  c.SendString("Hello, World!")
  return c.SendStatus(415)
  // => 415 "Hello, World!"
})
```

## Set

Sets the response’s HTTP header field to the specified `key`, `value`.

```go title="Signature"
func (c *Ctx) Set(key string, val string)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Set("Content-Type", "text/plain")
  // => "Content-type: text/plain"

  // ...
})
```

## SetParserDecoder

Allow you to config BodyParser/QueryParser decoder, base on schema's options, providing possibility to add custom type for parsing.

```go title="Signature"
func SetParserDecoder(parserConfig fiber.ParserConfig{
  IgnoreUnknownKeys bool,
  ParserType        []fiber.ParserType{
      Customtype interface{},
      Converter  func(string) reflect.Value,
  },
  ZeroEmpty         bool,
  SetAliasTag       string,
})
```

```go title="Example"

type CustomTime time.Time

// String() returns the time in string
func (ct *CustomTime) String() string {
    t := time.Time(*ct).String()
    return t
}

// Register the converter for CustomTime type format as 2006-01-02
var timeConverter = func(value string) reflect.Value {
  fmt.Println("timeConverter", value)
  if v, err := time.Parse("2006-01-02", value); err == nil {
    return reflect.ValueOf(v)
  }
  return reflect.Value{}
}

customTime := fiber.ParserType{
  Customtype: CustomTime{},
  Converter:  timeConverter,
}

// Add setting to the Decoder
fiber.SetParserDecoder(fiber.ParserConfig{
  IgnoreUnknownKeys: true,
  ParserType:        []fiber.ParserType{customTime},
  ZeroEmpty:         true,
})

// Example to use CustomType, you pause custom time format not in RFC3339
type Demo struct {
    Date  CustomTime `form:"date" query:"date"`
    Title string     `form:"title" query:"title"`
    Body  string     `form:"body" query:"body"`
}

app.Post("/body", func(c *fiber.Ctx) error {
    var d Demo
    c.BodyParser(&d)
    fmt.Println("d.Date", d.Date.String())
    return c.JSON(d)
})

app.Get("/query", func(c *fiber.Ctx) error {
    var d Demo
    c.QueryParser(&d)
    fmt.Println("d.Date", d.Date.String())
    return c.JSON(d)
})

// curl -X POST -F title=title -F body=body -F date=2021-10-20 http://localhost:3000/body

// curl -X GET "http://localhost:3000/query?title=title&body=body&date=2021-10-20"

```

## SetUserContext

Sets the user specified implementation for context interface.

```go title="Signature"
func (c *Ctx) SetUserContext(ctx context.Context)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  ctx := context.Background()
  c.SetUserContext(ctx)
  // Here ctx could be any context implementation

  // ...
})
```

## Stale

[https://expressjs.com/en/4x/api.html\#req.stale](https://expressjs.com/en/4x/api.html#req.stale)

```go title="Signature"
func (c *Ctx) Stale() bool
```

## Status

Sets the HTTP status for the response.

:::info
Method is a **chainable**.
:::

```go title="Signature"
func (c *Ctx) Status(status int) *Ctx
```

```go title="Example"
app.Get("/fiber", func(c *fiber.Ctx) error {
  c.Status(fiber.StatusOK)
  return nil
}

app.Get("/hello", func(c *fiber.Ctx) error {
  return c.Status(fiber.StatusBadRequest).SendString("Bad Request")
}

app.Get("/world", func(c *fiber.Ctx) error {
  return c.Status(fiber.StatusNotFound).SendFile("./public/gopher.png")
})
```

## Subdomains

Returns a string slice of subdomains in the domain name of the request.

The application property subdomain offset, which defaults to `2`, is used for determining the beginning of the subdomain segments.

```go title="Signature"
func (c *Ctx) Subdomains(offset ...int) []string
```

```go title="Example"
// Host: "tobi.ferrets.example.com"

app.Get("/", func(c *fiber.Ctx) error {
  c.Subdomains()  // ["ferrets", "tobi"]
  c.Subdomains(1) // ["tobi"]

  // ...
})
```

## Type

Sets the [Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type) HTTP header to the MIME type listed [here](https://github.com/nginx/nginx/blob/master/conf/mime.types) specified by the file **extension**.

```go title="Signature"
func (c *Ctx) Type(ext string, charset ...string) *Ctx
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Type(".html") // => "text/html"
  c.Type("html")  // => "text/html"
  c.Type("png")   // => "image/png"

  c.Type("json", "utf-8")  // => "application/json; charset=utf-8"

  // ...
})
```

## UserContext

UserContext returns a context implementation that was set by user earlier
or returns a non-nil, empty context, if it was not set earlier.

```go title="Signature"
func (c *Ctx) UserContext() context.Context
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  ctx := c.UserContext()
  // ctx is context implementation set by user

  // ...
})
```

## Vary

Adds the given header field to the [Vary](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Vary) response header. This will append the header, if not already listed, otherwise leaves it listed in the current location.

:::info
Multiple fields are **allowed**.
:::

```go title="Signature"
func (c *Ctx) Vary(fields ...string)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Vary("Origin")     // => Vary: Origin
  c.Vary("User-Agent") // => Vary: Origin, User-Agent

  // No duplicates
  c.Vary("Origin") // => Vary: Origin, User-Agent

  c.Vary("Accept-Encoding", "Accept")
  // => Vary: Origin, User-Agent, Accept-Encoding, Accept

  // ...
})
```

## Write

Write adopts the Writer interface

```go title="Signature"
func (c *Ctx) Write(p []byte) (n int, err error)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.Write([]byte("Hello, World!")) // => "Hello, World!"

  fmt.Fprintf(c, "%s\n", "Hello, World!") // "Hello, World!Hello, World!"
})
```

## Writef

Writef adopts the string with variables

```go title="Signature"
func (c *Ctx) Writef(f string, a ...interface{}) (n int, err error)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  world := "World!"
  c.Writef("Hello, %s", world) // => "Hello, World!"

  fmt.Fprintf(c, "%s\n", "Hello, World!") // "Hello, World!Hello, World!"
})
```

## WriteString

WriteString adopts the string

```go title="Signature"
func (c *Ctx) WriteString(s string) (n int, err error)
```

```go title="Example"
app.Get("/", func(c *fiber.Ctx) error {
  c.WriteString("Hello, World!") // => "Hello, World!"

  fmt.Fprintf(c, "%s\n", "Hello, World!") // "Hello, World!Hello, World!"
})
```

## XHR

A Boolean property, that is `true`, if the request’s [X-Requested-With](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers) header field is [XMLHttpRequest](https://developer.mozilla.org/en-US/docs/Web/API/XMLHttpRequest), indicating that the request was issued by a client library \(such as [jQuery](https://api.jquery.com/jQuery.ajax/)\).

```go title="Signature"
func (c *Ctx) XHR() bool
```

```go title="Example"
// X-Requested-With: XMLHttpRequest

app.Get("/", func(c *fiber.Ctx) error {
  c.XHR() // true

  // ...
})
```

## XML

Converts any **interface** or **string** to XML using the standard `encoding/xml` package.

:::info
XML also sets the content header to **application/xml**.
:::

```go title="Signature"
func (c *Ctx) XML(data interface{}) error
```

```go title="Example"
type SomeStruct struct {
  XMLName xml.Name `xml:"Fiber"`
  Name    string   `xml:"Name"`
  Age     uint8    `xml:"Age"`
}

app.Get("/", func(c *fiber.Ctx) error {
  // Create data struct:
  data := SomeStruct{
    Name: "Grame",
    Age:  20,
  }

  return c.XML(data)
  // <Fiber>
  //     <Name>Grame</Name>
  //    <Age>20</Age>
  // </Fiber>
})
```
