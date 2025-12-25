---
id: rest
title: 🖥️ REST
description: >-
  HTTP client for Fiber.
sidebar_position: 1
toc_max_heading_level: 5
---

The Fiber Client is a high-performance HTTP client built on FastHTTP. It handles both internal service calls and external requests with minimal overhead.

## Features

- **Lightweight and fast**: built on FastHTTP for minimal overhead.
- **Flexible configuration**: set global defaults like timeouts or headers and override them per request.
- **Connection pooling**: reuses persistent connections instead of opening new ones.
- **Timeouts and retries**: supports per-request deadlines and retry policies for transient errors.

## Usage

Create a client with any required configuration, then send requests:

```go
package main

import (
    "fmt"
    "time"

    "github.com/gofiber/fiber/v3/client"
)

func main() {
    cc := client.New()
    cc.SetTimeout(10 * time.Second)

    // Send a GET request
    resp, err := cc.Get("https://httpbin.org/get")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Status: %d\n", resp.StatusCode())
    fmt.Printf("Body: %s\n", string(resp.Body()))
}
```

See [examples](examples.md) for more detailed usage.

```go
type Client struct {
    mu sync.RWMutex

    fasthttp *fasthttp.Client

    baseURL   string
    userAgent string
    referer   string
    header    *Header
    params    *QueryParam
    cookies   *Cookie
    path      *PathParam

    debug bool

    timeout time.Duration

    // user-defined request hooks
    userRequestHooks []RequestHook

    // client package-defined request hooks
    builtinRequestHooks []RequestHook

    // user-defined response hooks
    userResponseHooks []ResponseHook

    // client package-defined response hooks
    builtinResponseHooks []ResponseHook

    jsonMarshal   utils.JSONMarshal
    jsonUnmarshal utils.JSONUnmarshal
    xmlMarshal    utils.XMLMarshal
    xmlUnmarshal  utils.XMLUnmarshal
    cborMarshal   utils.CBORMarshal
    cborUnmarshal utils.CBORUnmarshal

    cookieJar *CookieJar

    // proxy
    proxyURL string

    // retry
    retryConfig *RetryConfig

    // logger
    logger log.CommonLogger
}
```

### New

**New** creates and returns a new Client object.

```go title="Signature"
func New() *Client
```

### NewWithClient

**NewWithClient** creates and returns a new Client object from an existing `fasthttp.Client`.

```go title="Signature"
func NewWithClient(c *fasthttp.Client) *Client
```

## REST Methods

These helpers mirror axios-style method names and send HTTP requests using the configured client:

### Get

Sends a GET request.

```go title="Signature"
func (c *Client) Get(url string, cfg ...Config) (*Response, error)
```

### Post

Sends a POST request.

```go title="Signature"
func (c *Client) Post(url string, cfg ...Config) (*Response, error)
```

### Put

Sends a PUT request.

```go title="Signature"
func (c *Client) Put(url string, cfg ...Config) (*Response, error)
```

### Patch

Sends a PATCH request.

```go title="Signature"
func (c *Client) Patch(url string, cfg ...Config) (*Response, error)
```

### Delete

Sends a DELETE request.

```go title="Signature"
func (c *Client) Delete(url string, cfg ...Config) (*Response, error)
```

### Head

Sends a HEAD request.

```go title="Signature"
func (c *Client) Head(url string, cfg ...Config) (*Response, error)
```

### Options

Sends an OPTIONS request.

```go title="Signature"
func (c *Client) Options(url string, cfg ...Config) (*Response, error)
```

### Custom

Sends a request with any HTTP method.

```go title="Signature"
func (c *Client) Custom(url, method string, cfg ...Config) (*Response, error)
```

## Request Configuration

The `Config` type holds per-request parameters. JSON is used to serialize the body by default. If multiple body sources are set, precedence is:

1. Body
2. FormData
3. File

```go
type Config struct {
    Ctx context.Context

    UserAgent string
    Referer   string
    Header    map[string]string
    Param     map[string]string
    Cookie    map[string]string
    PathParam map[string]string

    Timeout      time.Duration
    MaxRedirects int

    Body     any
    FormData map[string]string
    File     []*File
}
```

## R

**R** gets a `Request` object from the pool. Call `ReleaseRequest` when finished.

```go title="Signature"
func (c *Client) R() *Request
```

## Hooks

Hooks allow you to add custom logic before a request is sent or after a response is received.

### RequestHook

**RequestHook** returns user-defined request hooks.

```go title="Signature"
func (c *Client) RequestHook() []RequestHook
```

### ResponseHook

**ResponseHook** returns user-defined response hooks.

```go title="Signature"
func (c *Client) ResponseHook() []ResponseHook
```

### AddRequestHook

Adds one or more user-defined request hooks.

```go title="Signature"
func (c *Client) AddRequestHook(h ...RequestHook) *Client
```

### AddResponseHook

Adds one or more user-defined response hooks.

```go title="Signature"
func (c *Client) AddResponseHook(h ...ResponseHook) *Client
```

## JSON

### JSONMarshal

Returns the JSON marshaler function used by the client.

```go title="Signature"
func (c *Client) JSONMarshal() utils.JSONMarshal
```

### JSONUnmarshal

Returns the JSON unmarshaler function used by the client.

```go title="Signature"
func (c *Client) JSONUnmarshal() utils.JSONUnmarshal
```

### SetJSONMarshal

Sets a custom JSON marshaler.

```go title="Signature"
func (c *Client) SetJSONMarshal(f utils.JSONMarshal) *Client
```

### SetJSONUnmarshal

Sets a custom JSON unmarshaler.

```go title="Signature"
func (c *Client) SetJSONUnmarshal(f utils.JSONUnmarshal) *Client
```

## XML

### XMLMarshal

Returns the XML marshaler function used by the client.

```go title="Signature"
func (c *Client) XMLMarshal() utils.XMLMarshal
```

### XMLUnmarshal

Returns the XML unmarshaler function used by the client.

```go title="Signature"
func (c *Client) XMLUnmarshal() utils.XMLUnmarshal
```

### SetXMLMarshal

Sets a custom XML marshaler.

```go title="Signature"
func (c *Client) SetXMLMarshal(f utils.XMLMarshal) *Client
```

### SetXMLUnmarshal

Sets a custom XML unmarshaler.

```go title="Signature"
func (c *Client) SetXMLUnmarshal(f utils.XMLUnmarshal) *Client
```

## CBOR

### CBORMarshal

Returns the CBOR marshaler function used by the client.

```go title="Signature"
func (c *Client) CBORMarshal() utils.CBORMarshal
```

### CBORUnmarshal

Returns the CBOR unmarshaler function used by the client.

```go title="Signature"
func (c *Client) CBORUnmarshal() utils.CBORUnmarshal
```

### SetCBORMarshal

Sets a custom CBOR marshaler.

```go title="Signature"
func (c *Client) SetCBORMarshal(f utils.CBORMarshal) *Client
```

### SetCBORUnmarshal

Sets a custom CBOR unmarshaler.

```go title="Signature"
func (c *Client) SetCBORUnmarshal(f utils.CBORUnmarshal) *Client
```

## TLS

### TLSConfig

Returns the client's TLS configuration. If none is set, it initializes a new
configuration with `MinVersion` defaulting to TLS 1.2.

```go title="Signature"
func (c *Client) TLSConfig() *tls.Config
```

### SetTLSConfig

Sets the TLS configuration for the client.

```go title="Signature"
func (c *Client) SetTLSConfig(config *tls.Config) *Client
```

### SetCertificates

Adds client certificates to the TLS configuration.

```go title="Signature"
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client
```

### SetRootCertificate

Adds one or more root certificates to the client's trust store.

```go title="Signature"
func (c *Client) SetRootCertificate(path string) *Client
```

### SetRootCertificateFromString

Adds one or more root certificates from a string.

```go title="Signature"
func (c *Client) SetRootCertificateFromString(pem string) *Client
```

## SetProxyURL

Sets a proxy URL for the client. All subsequent requests will use this proxy.

```go title="Signature"
func (c *Client) SetProxyURL(proxyURL string) error
```

## StreamResponseBody

Returns whether response body streaming is enabled. When enabled, the response body is not fully loaded into memory and can be read as a stream using `BodyStream()`. This is useful for handling large responses or server-sent events.

```go title="Signature"
func (c *Client) StreamResponseBody() bool
```

## SetStreamResponseBody

Sets whether the response body should be streamed directly to the caller instead of being fully buffered in memory. This is useful for handling large responses or streaming responses.

```go title="Signature"
func (c *Client) SetStreamResponseBody(stream bool) *Client
```

## RetryConfig

Returns the retry configuration of the client.

```go title="Signature"
func (c *Client) RetryConfig() *RetryConfig
```

## SetRetryConfig

Sets the retry configuration for the client.

```go title="Signature"
func (c *Client) SetRetryConfig(config *RetryConfig) *Client
```

## BaseURL

### BaseURL

**BaseURL** returns the base URL currently set in the client.

```go title="Signature"
func (c *Client) BaseURL() string
```

### SetBaseURL

Sets a base URL prefix for all requests made by the client.

```go title="Signature"
func (c *Client) SetBaseURL(url string) *Client
```

**Example:**

```go title="Example"
cc := client.New()
cc.SetBaseURL("https://httpbin.org/")

resp, err := cc.Get("/get")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

<details>
<summary>Click here to see the result</summary>

```json
{
  "args": {},
  ...
}
```

</details>

## Headers

### Header

Retrieves all values of a header key at the client level. The returned values apply to all requests.

```go title="Signature"
func (c *Client) Header(key string) []string
```

### AddHeader

Adds a single header to all requests initiated by this client.

```go title="Signature"
func (c *Client) AddHeader(key, val string) *Client
```

### SetHeader

Sets a single header, overriding any existing headers with the same key.

```go title="Signature"
func (c *Client) SetHeader(key, val string) *Client
```

### AddHeaders

Adds multiple headers at once, all applying to all future requests from this client.

```go title="Signature"
func (c *Client) AddHeaders(h map[string][]string) *Client
```

### SetHeaders

Sets multiple headers at once, overriding previously set headers.

```go title="Signature"
func (c *Client) SetHeaders(h map[string]string) *Client
```

## Query Parameters

### Param

Returns the values for a given query parameter key.

```go title="Signature"
func (c *Client) Param(key string) []string
```

### AddParam

Adds a single query parameter for all requests.

```go title="Signature"
func (c *Client) AddParam(key, val string) *Client
```

### SetParam

Sets a single query parameter, overriding previously set values.

```go title="Signature"
func (c *Client) SetParam(key, val string) *Client
```

### AddParams

Adds multiple query parameters from a map of string slices.

```go title="Signature"
func (c *Client) AddParams(m map[string][]string) *Client
```

### SetParams

Sets multiple query parameters from a map, overriding previously set values.

```go title="Signature"
func (c *Client) SetParams(m map[string]string) *Client
```

### SetParamsWithStruct

Sets multiple query parameters from a struct. Nested structs are not currently supported.

```go title="Signature"
func (c *Client) SetParamsWithStruct(v any) *Client
```

### DelParams

Deletes one or more query parameters.

```go title="Signature"
func (c *Client) DelParams(key ...string) *Client
```

## UserAgent & Referer

### SetUserAgent

Sets the user agent header for all requests.

```go title="Signature"
func (c *Client) SetUserAgent(ua string) *Client
```

### SetReferer

Sets the referer header for all requests.

```go title="Signature"
func (c *Client) SetReferer(r string) *Client
```

## Path Parameters

### PathParam

Returns the value of a named path parameter, if set.

```go title="Signature"
func (c *Client) PathParam(key string) string
```

### SetPathParam

Sets a single path parameter.

```go title="Signature"
func (c *Client) SetPathParam(key, val string) *Client
```

### SetPathParams

Sets multiple path parameters at once.

```go title="Signature"
func (c *Client) SetPathParams(m map[string]string) *Client
```

### SetPathParamsWithStruct

Sets multiple path parameters from a struct.

```go title="Signature"
func (c *Client) SetPathParamsWithStruct(v any) *Client
```

### DelPathParams

Deletes one or more path parameters.

```go title="Signature"
func (c *Client) DelPathParams(key ...string) *Client
```

## Cookies

### Cookie

Returns the value of a named cookie if set at the client level.

```go title="Signature"
func (c *Client) Cookie(key string) string
```

### SetCookie

Sets a single cookie for all requests.

```go title="Signature"
func (c *Client) SetCookie(key, val string) *Client
```

**Example:**

```go title="Example"
cc := client.New()
cc.SetCookie("john", "doe")

resp, err := cc.Get("https://httpbin.org/cookies")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

<details>
<summary>Click here to see the result</summary>

```json
{
  "cookies": {
    "john": "doe"
  }
}
```

</details>

### SetCookies

Sets multiple cookies at once.

```go title="Signature"
func (c *Client) SetCookies(m map[string]string) *Client
```

### SetCookiesWithStruct

Sets multiple cookies from a struct.

```go title="Signature"
func (c *Client) SetCookiesWithStruct(v any) *Client
```

### DelCookies

Deletes one or more cookies.

```go title="Signature"
func (c *Client) DelCookies(key ...string) *Client
```

## Timeout

### SetTimeout

Sets a default timeout for all requests, which can be overridden per request.

```go title="Signature"
func (c *Client) SetTimeout(t time.Duration) *Client
```

## Debugging

### Debug

Enables debug-level logging output.

```go title="Signature"
func (c *Client) Debug() *Client
```

### DisableDebug

Disables debug-level logging output.

```go title="Signature"
func (c *Client) DisableDebug() *Client
```

## Cookie Jar

### SetCookieJar

Assigns a cookie jar to the client to store and manage cookies across requests.

```go title="Signature"
func (c *Client) SetCookieJar(cookieJar *CookieJar) *Client
```

## Dial & Logger

### SetDial

Sets a custom dial function.

```go title="Signature"
func (c *Client) SetDial(dial fasthttp.DialFunc) *Client
```

### SetLogger

Sets the logger instance used by the client.

```go title="Signature"
func (c *Client) SetLogger(logger log.CommonLogger) *Client
```

### Logger

Returns the current logger instance.

```go title="Signature"
func (c *Client) Logger() log.CommonLogger
```

## Reset

### Reset

Clears and resets the client to its default state and reinstates the default
`fasthttp.Client` transport.

```go title="Signature"
func (c *Client) Reset()
```

## Default Client

Fiber provides a default client (created with `New()`). You can configure it or replace it as needed.

### C

**C** returns the default client.

```go title="Signature"
func C() *Client
```

### Get

Get is a convenience method that sends a GET request using the `defaultClient`.

```go title="Signature"
func Get(url string, cfg ...Config) (*Response, error)
```

### Post

Post is a convenience method that sends a POST request using the `defaultClient`.

```go title="Signature"
func Post(url string, cfg ...Config) (*Response, error)
```

### Put

Put is a convenience method that sends a PUT request using the `defaultClient`.

```go title="Signature"
func Put(url string, cfg ...Config) (*Response, error)
```

### Patch

Patch is a convenience method that sends a PATCH request using the `defaultClient`.

```go title="Signature"
func Patch(url string, cfg ...Config) (*Response, error)
```

### Delete

Delete is a convenience method that sends a DELETE request using the `defaultClient`.

```go title="Signature"
func Delete(url string, cfg ...Config) (*Response, error)
```

### Head

Head sends a HEAD request using the `defaultClient`, a convenience method.

```go title="Signature"
func Head(url string, cfg ...Config) (*Response, error)
```

### Options

Options is a convenience method that sends an OPTIONS request using the `defaultClient`.

```go title="Signature"
func Options(url string, cfg ...Config) (*Response, error)
```

### Replace

**Replace** replaces the default client with a new one. It returns a function that can restore the old client.

:::caution
Do not modify the default client concurrently.
:::

```go title="Signature"
func Replace(c *Client) func()
```
