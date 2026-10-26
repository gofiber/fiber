---
id: rest
title: 🖥️ REST
description: >-
  HTTP client for Fiber.
sidebar_position: 1
toc_max_heading_level: 5
---

import MethodIndex from '@site/src/components/method-index';

The Fiber Client is a high-performance HTTP client built on FastHTTP. It handles both internal service calls and external requests with minimal overhead.

Use the index to jump straight to any client method; filter by name or by category:

<MethodIndex itemNoun="methods" placeholder="Filter methods, e.g. timeout" />

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

:::note
A client can send requests from many goroutines at once after its configuration is complete. Changing the configuration while requests are in flight needs your own synchronization.
:::

```go
type Client struct {
    logger    log.CommonLogger
    transport httpClientTransport

    header  *Header
    params  *QueryParam
    cookies *Cookie
    path    *PathParam

    jsonMarshal   utils.JSONMarshal
    jsonUnmarshal utils.JSONUnmarshal
    xmlMarshal    utils.XMLMarshal
    xmlUnmarshal  utils.XMLUnmarshal
    cborMarshal   utils.CBORMarshal
    cborUnmarshal utils.CBORUnmarshal

    cookieJar            *CookieJar
    retryConfig          *RetryConfig
    baseURL              string
    userAgent            string
    referer              string
    userRequestHooks     []RequestHook
    builtinRequestHooks  []RequestHook
    finalRequestHooks    []RequestHook
    userResponseHooks    []ResponseHook
    builtinResponseHooks []ResponseHook

    timeout                   time.Duration
    mu                        sync.RWMutex
    isDebug                   bool
    isPathNormalizingDisabled bool
}
```

### New

**New** creates and returns a new Client object.

```go title="Signature"
func New() *Client
```

### NewWithClient

**NewWithClient** creates and returns a new Client object from an existing `fasthttp.Client`. It panics if `c` is `nil`.

Fiber cannot tell whether that client has served a host already. Unless it was created with `DisablePathNormalizing: true`, [SetDisablePathNormalizing](#setdisablepathnormalizing) changes what is sent to hosts it has not served yet, but the check that keeps a path parameter value inside its segment stays on.

```go title="Signature"
func NewWithClient(c *fasthttp.Client) *Client
```

### NewWithHostClient

**NewWithHostClient** creates and returns a new Client object from an existing `fasthttp.HostClient`. A `HostClient` connects to its own `Addr` whatever host the request URL names; that host only becomes the `Host` header. It panics if `c` is `nil`.

```go title="Signature"
func NewWithHostClient(c *fasthttp.HostClient) *Client
```

### NewWithLBClient

**NewWithLBClient** creates and returns a new Client object from an existing `fasthttp.LBClient`. TLS, dialer, proxy and response streaming settings are applied to every host client behind the load balancer. Requests go through `LBClient.Do`, so they fail with `fasthttp.ErrTimeout` after `LBClient.Timeout`, which is one second when left unset, even if the client timeout is longer. It panics if `c` is `nil`.

```go title="Signature"
func NewWithLBClient(c *fasthttp.LBClient) *Client
```

See [Reusing fasthttp transports](./examples.md#reusing-fasthttp-transports) for examples, and [Underlying Transport](#underlying-transport) for the methods that give access to the wrapped client.

## REST Methods

These helpers mirror axios-style method names and send HTTP requests using the configured client. Each helper acquires its own `Request`, which `Response.Close` releases together with the response. When a helper returns an error there is no `Response` to close: the helper has already released its `Request`.

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

### Query

Sends a QUERY request.

```go title="Signature"
func (c *Client) Query(url string, cfg ...Config) (*Response, error)
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

    Timeout                  time.Duration
    MaxRedirects             int
    RedirectSensitiveHeaders []string
    DisablePathNormalizing   bool

    Body     any
    FormData map[string]string
    File     []*File
}
```

The REST helpers accept an optional `Config` and use only the first one passed. A `string` or `[]byte` `Body` is sent as is; any other value is encoded as JSON.

`MaxRedirects` is honored for `GET`, `HEAD` and `QUERY` requests only; other methods receive the redirect response as-is. The default is `0`, which follows no redirects. See [SetMaxRedirects](./request.md#setmaxredirects) for how redirects are followed.

`RedirectSensitiveHeaders` sets [SetRedirectSensitiveHeaders](./request.md#setredirectsensitiveheaders) for the request; an empty list leaves the request's own names as they are.

`DisablePathNormalizing: true` turns path normalizing off for the request, like [SetDisablePathNormalizing](./request.md#setdisablepathnormalizing) on the request, which also says when the path is really sent as written; `false` leaves the client's setting in place.

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

### FinalRequestHook

Returns final request hooks. These run after Fiber has serialized the URL,
headers, and body, immediately before the transport sends the request.

```go title="Signature"
func (c *Client) FinalRequestHook() []RequestHook
```

### AddFinalRequestHook

Adds final request hooks. Use these when request signing or authentication must
inspect the serialized request body together with the merged headers. See
[Final Request Hooks](./hooks.md#final-request-hooks) for what is already set at
that point and what fasthttp adds afterward.

```go title="Signature"
func (c *Client) AddFinalRequestHook(h ...RequestHook) *Client
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

:::note
Set the TLS configuration before the first request. fasthttp copies it for a host the first time the client connects there, so a later `SetTLSConfig` or `SetCertificates` call does not reach a host the client already uses. On a client built with `New` or `NewWithClient` the copy lives as long as fasthttp keeps its entry for that host. The entry goes at the first pass of fasthttp's cleaner after the host has no open connection and no request waiting for one, and the passes come every second, or as far apart as `MaxIdleConnDuration`, up to 10 seconds, when that is longer. A failed first handshake leaves no open connection either, so a retry within that second still runs with the old settings. A client built with `NewWithHostClient` or `NewWithLBClient` keeps the copy for as long as the host client lives.

Certificates that [SetRootCertificate](#setrootcertificate) or [SetRootCertificateFromString](#setrootcertificatefromstring) add to a `RootCAs` pool that already existed when the host was first used are the exception: the copy shares the pool, so they reach the host at once. The first call on a client without a pool creates the pool, and a copy made before that does not see it.
:::

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

Adds one or more root certificates from a PEM file to the `RootCAs` pool of the client's TLS configuration. A new client has no pool (`RootCAs` is `nil`, which means the system roots), so the first call creates one that holds only the given certificates: they replace the system roots instead of being added to them, and later calls add to that pool. If the configuration already has a `RootCAs` pool, for example one built from `x509.SystemCertPool()` and set with [SetTLSConfig](#settlsconfig), the certificates are appended to that pool, and they reach a host the client already uses; the call that creates the pool does not, see the note under [TLS](#tls). To keep trusting the system roots, build the pool that way, as in the [TLS example](./examples.md#tls). It panics if the file cannot be read or holds no valid PEM certificate.

```go title="Signature"
func (c *Client) SetRootCertificate(path string) *Client
```

### SetRootCertificateFromString

Adds one or more root certificates from a PEM string to the `RootCAs` pool of the client's TLS configuration, with the same rules as [SetRootCertificate](#setrootcertificate): the first call on a client without a pool replaces the system roots. It panics if the string holds no valid PEM certificate.

```go title="Signature"
func (c *Client) SetRootCertificateFromString(pem string) *Client
```

## SetProxyURL

Sets a proxy URL for the client. All subsequent requests will use this proxy. It returns an error when the URL has a host and its scheme is not `http`, `socks5` or `socks5h`, for example `ftp://proxy:1080`. Other malformed URLs are not rejected here. If `net/url` cannot parse the URL, even with `http://` put in front of it, for example because of a trailing newline, a leading space, `%zz` in the port, or a non-numeric port on an address without a scheme such as `127.0.0.1:abc`, the URL is dropped without an error and requests go to their origin directly, without a proxy. An empty string is dropped the same way, and a dropped URL also replaces a proxy set earlier. A URL that parses, or that parses once `http://` is put in front of it, but names no usable address makes each request fail when it dials. That covers `http://127.0.0.1:abc` with its scheme, `http://` or `ftp://` with nothing after it, and a trailing `/` or a path. The proxy is installed as the client's dial function, so a later [SetDial](#setdial) replaces it and the other way around. Call it before the first request: on a client built with `New` or `NewWithClient`, a host the client already uses keeps its previous dial function until fasthttp drops its entry for that host, see the note under [TLS](#tls) for when. A client built with `NewWithHostClient` or `NewWithLBClient` uses the new function for every new connection.

```go title="Signature"
func (c *Client) SetProxyURL(proxyURL string) error
```

## Response Streaming

### StreamResponseBody

Returns whether response body streaming is enabled. When enabled, the response body is not fully loaded into memory and can be read as a stream using `Response.BodyStream()`. This is useful for handling large responses or server-sent events (SSE).

```go title="Signature"
func (c *Client) StreamResponseBody() bool
```

### SetStreamResponseBody

Enables or disables response body streaming. When enabled, responses can be consumed incrementally without loading the entire body into memory.

```go title="Signature"
func (c *Client) SetStreamResponseBody(enable bool) *Client
```

**Example:**

```go title="Example"
cc := client.New()
cc.SetStreamResponseBody(true)

resp, err := cc.Get("https://example.com/large-file")
if err != nil {
    panic(err)
}
defer resp.Close()

// Check if response is streaming
if resp.IsStreaming() {
    // Read body incrementally
    reader := resp.BodyStream()
    buf := make([]byte, 4096)
    for {
        n, err := reader.Read(buf)
        if n > 0 {
            // Process chunk...
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            panic(err)
        }
    }
} else {
    // Regular body access
    body := resp.Body()
    fmt.Println(string(body))
}
```

**Server-Sent Events Example:**

```go title="SSE Example"
cc := client.New()
cc.SetStreamResponseBody(true)

resp, err := cc.Get("https://example.com/events")
if err != nil {
    panic(err)
}
defer resp.Close()

reader := bufio.NewReader(resp.BodyStream())
for {
    line, err := reader.ReadString('\n')
    if err == io.EOF {
        break
    }
    if err != nil {
        panic(err)
    }
    fmt.Print(line) // Process SSE event
}
```

## RetryConfig

Returns a copy of the retry configuration of the client, or `nil` when none is set.

```go title="Signature"
func (c *Client) RetryConfig() *RetryConfig
```

## SetRetryConfig

Sets the retry configuration for the client. `RetryConfig` is an alias of `retry.Config` from the [Retry addon](../addon/retry.md), which describes its fields and defaults. Without a retry configuration, which is the default, Fiber does not retry failed requests. With one, a request that fails to send is retried with exponential backoff; an HTTP error status such as `500` is a response rather than a failure, so it is not retried. A request that follows redirects starts each retry at its original URL, see [SetMaxRedirects](./request.md#setmaxredirects).

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

Sets a base URL prefix for all requests made by the client. A request URL that already carries an `http://` or `https://` scheme, in any letter case, is used as-is; any other URL is appended to the base URL. If the result still has no `http://` or `https://` scheme, the request fails with `ErrURLFormat`.

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

Client-level headers are applied to every request when it is sent. A header set on the request itself with the same key replaces the client's values rather than being added to them, and each send starts from those keys cleared, so re-sending a request does not accumulate values.

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

Query parameters set on the client are added to the ones on each request and to those already in the URL. A request does not replace a client parameter of the same name, so the server receives every value.

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

Sets multiple query parameters from a struct. Each field is named by its `param` struct tag, or by the field name when it has none. Nested structs are not currently supported. See [SetValWithStruct](./request.md#setvalwithstruct) for the supported field types.

```go title="Signature"
func (c *Client) SetParamsWithStruct(v any) *Client
```

### DelParams

Deletes one or more query parameters.

```go title="Signature"
func (c *Client) DelParams(key ...string) *Client
```

## UserAgent & Referer

Requests are sent with `User-Agent: fiber` unless a user agent is set on the client or on the request, and the request's value wins. They are sent without a `Referer` header unless a referer is set the same way. Use these methods, or `Config.UserAgent` and `Config.Referer`, for these two headers: a `User-Agent` or `Referer` set through `SetHeader`, `AddHeader` or `Config.Header` is replaced when the request is sent, and a `Referer` is removed when no referer is set.

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

Path parameters fill the `:name` placeholders in a request URL, including one in the base URL, as in `/users/:id`. A path parameter set on a request overrides a client-level one with the same name. See [SetPathParam](./request.md#setpathparam) for how placeholders are matched and which values are accepted.

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

Sets multiple path parameters from a struct. Each field is named by its `path` struct tag, or by the field name when it has none.

```go title="Signature"
func (c *Client) SetPathParamsWithStruct(v any) *Client
```

### DelPathParams

Deletes one or more path parameters.

```go title="Signature"
func (c *Client) DelPathParams(key ...string) *Client
```

## Path Normalizing

### DisablePathNormalizing

Reports whether path normalizing was disabled with [SetDisablePathNormalizing](#setdisablepathnormalizing). It does not read the setting of a fasthttp client that was created with `DisablePathNormalizing: true`.

```go title="Signature"
func (c *Client) DisablePathNormalizing() bool
```

### SetDisablePathNormalizing

Turns path normalizing off or on for every request of the client. It is on by default: the request path is normalized before it is sent, which decodes percent-escapes, collapses `//` and resolves `.` and `..` segments, so `/a//b/./c%2Fd` goes out as `/a/b/c/d`. With normalizing off, the path is sent as written. fasthttp normalizes the path when it writes the request, according to the client that sends it, so the setting is applied to the `DisablePathNormalizing` field of the wrapped fasthttp client, and of every host client behind a load balancer: a client created with [NewWithClient](#newwithclient), [NewWithHostClient](#newwithhostclient) or [NewWithLBClient](#newwithlbclient) changes the fasthttp client it wraps, for everyone who shares it. Call it before the first request, because a `fasthttp.Client` copies the setting into each host client when it first creates it, and a host it already serves keeps normalizing until fasthttp drops its entry, see the note under [TLS](#tls) for when. Fiber keeps rejecting path parameter values that would leave their segment unless it knows that the path is sent as built. It knows for a client from `New`, until a request has gone out through it while it normalized; for a client from `NewWithHostClient`, which reads the field on every request; for a client from `NewWithLBClient` whose host clients all have the field on; and for a `fasthttp.Client` given to `NewWithClient` that was created with `DisablePathNormalizing: true`. A `fasthttp.Client` given to `NewWithClient` without it may have served hosts already, so Fiber cannot tell and keeps the check on, although hosts it has not served yet get the setting. A balancer that holds a client that is neither a `*fasthttp.HostClient` nor a wrapper exposing a nested balancer through `LBClient()` cannot be told either and counts as normalizing. A request skips Fiber's normalizing when either the client or the request disables it; see [SetDisablePathNormalizing](./request.md#setdisablepathnormalizing) for the request-level setting.

```go title="Signature"
func (c *Client) SetDisablePathNormalizing(disable bool) *Client
```

## Redirects

### RedirectSensitiveHeaders

Returns a copy of the header names set with [SetRedirectSensitiveHeaders](#setredirectsensitiveheaders). The names a request adds with [SetRedirectSensitiveHeaders](./request.md#setredirectsensitiveheaders) are not included.

```go title="Signature"
func (c *Client) RedirectSensitiveHeaders() []string
```

### SetRedirectSensitiveHeaders

Names request headers that must not follow a redirect to another host, such as a credential of your own like `X-Api-Key`, for every request of the client. A redirect to a host that is neither the original hostname (on any port) nor one of its subdomains drops them with the built-in sensitive headers; every other header is forwarded. Names match case-insensitively, and surrounding whitespace and empty entries are ignored. A request can add names of its own with [SetRedirectSensitiveHeaders](./request.md#setredirectsensitiveheaders). The list applies to [DoRedirects](#doredirects) too. It is empty by default, calling the method with no names clears it, and `Reset` clears it. It matters only while redirects are followed, see [SetMaxRedirects](./request.md#setmaxredirects).

The boundary is the hostname, as in `net/http`: a redirect of the same host to another port or scheme keeps the headers. [proxy.DoRedirects](../middleware/proxy.md#headers-on-a-cross-origin-redirect) is stricter and drops them whenever the scheme, host or port changes.

```go title="Signature"
func (c *Client) SetRedirectSensitiveHeaders(names ...string) *Client
```

```go title="Example"
cli := client.New().
    SetHeader("X-Api-Key", os.Getenv("UPSTREAM_API_KEY")).
    SetRedirectSensitiveHeaders("X-Api-Key")

resp, err := cli.Get("https://api.example.com/items", client.Config{MaxRedirects: 3})
```

## Cookies

A cookie set on a request replaces a client cookie with the same name, and a client cookie replaces one from the [cookie jar](#cookie-jar).

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

Sets multiple cookies from a struct. Each field is named by its `cookie` struct tag, or by the field name when it has none.

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

Sets a default timeout for all requests, which can be overridden per request. A new client has no timeout. When the timeout elapses, or the request's context is canceled, the request fails with `ErrTimeoutOrCancel`.

```go title="Signature"
func (c *Client) SetTimeout(t time.Duration) *Client
```

## Debugging

### Debug

Enables debug-level logging output. With debug enabled, the raw request and response are logged through the client's logger after each response, as they are: the `Authorization` and `Cookie` headers and the bodies are in the log, so keep debug logging off wherever people who should not see them can read it. When either body is a stream, only its headers are logged so the stream stays readable for the caller. A new client logs through Fiber's default logger, the one the `log` package writes to. Nothing is logged when the logger was set to `nil`. Use [SetLogger](#setlogger) to choose the logger.

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

Assigns a cookie jar to the client to store and manage cookies across requests. A cookie received with a `Max-Age` attribute expires that many seconds after it was received, taking precedence over `Expires`; a `Max-Age` of zero or less expires it immediately (RFC 6265). A `Max-Age` that is not an integer is ignored on its own, as is any other attribute the parser cannot read, such as an `Expires` in an unrecognized format (RFC 6265 Section 5.2): the cookie keeps its name, its value and every other attribute, `Path`, `Domain` and `Secure` included. When the attribute repeats, the last integer value counts. A `Set-Cookie` whose name and value cannot be parsed is dropped. Cookies for the same host and path are stored under the origin that answered, so a redirect cannot plant cookies for another host. See [Cookie jar](./examples.md#cookie-jar) for how the jar scopes, orders and limits cookies.

```go title="Signature"
func (c *Client) SetCookieJar(cookieJar *CookieJar) *Client
```

A `CookieJar` is safe for concurrent use, except for `Release`, which must not run at the same time as any other method.

### AcquireCookieJar

**AcquireCookieJar** returns an empty `CookieJar` from the pool.

```go title="Signature"
func AcquireCookieJar() *CookieJar
```

### ReleaseCookieJar

**ReleaseCookieJar** empties the jar and returns it to the pool. Do not use the jar afterward.

```go title="Signature"
func ReleaseCookieJar(c *CookieJar)
```

### CookieJar.Get

**Get** returns the cookies that apply to a URI: the host and path match, `Secure` cookies are returned only for an `https` URI, and expired cookies are dropped. It returns `nil` when no cookie applies or the URI is `nil`. The jar keeps its own copies, so the returned cookies may be released with `fasthttp.ReleaseCookie`.

```go title="Signature"
func (cj *CookieJar) Get(uri *fasthttp.URI) []*fasthttp.Cookie
```

### CookieJar.Set

**Set** stores copies of the cookies for the host of a URI, ignoring the port. A cookie without a usable `Path` is scoped to the default path of the URI (RFC 6265), so cookies set against `/a/b` apply to `/a` and below; set `Path` on the cookie to widen it. A cookie whose `Domain` the host may not set, such as an unrelated domain or a public suffix, is skipped.

```go title="Signature"
func (cj *CookieJar) Set(uri *fasthttp.URI, cookies ...*fasthttp.Cookie)
```

### CookieJar.SetByHost

**SetByHost** stores copies of the cookies for a host. There is no request path to derive a default from, so a cookie without a usable `Path` is scoped to `/`.

```go title="Signature"
func (cj *CookieJar) SetByHost(host []byte, cookies ...*fasthttp.Cookie)
```

### CookieJar.SetKeyValue

**SetKeyValue** stores a cookie with the given key and value for a host, scoped to `/`.

```go title="Signature"
func (cj *CookieJar) SetKeyValue(host, key, value string)
```

### CookieJar.SetKeyValueBytes

**SetKeyValueBytes** is like `SetKeyValue` but takes the key and value as byte slices.

```go title="Signature"
func (cj *CookieJar) SetKeyValueBytes(host string, key, value []byte)
```

### CookieJar.Release

**Release** removes every stored cookie. The jar must not be used afterward, and `ReleaseCookieJar` calls it for you.

```go title="Signature"
func (cj *CookieJar) Release()
```

## Dial & Logger

### SetDial

Sets a custom dial function. Call it before the first request: [SetProxyURL](#setproxyurl) says when a new one reaches a host the client already uses.

```go title="Signature"
func (c *Client) SetDial(dial fasthttp.DialFunc) *Client
```

### SetLogger

Sets the logger instance used by the client. A new client starts with Fiber's default logger. `nil` turns client logging off; [SetRootCertificate](#setrootcertificate) and [SetRootCertificateFromString](#setrootcertificatefromstring) still panic on invalid input, only without a log line.

```go title="Signature"
func (c *Client) SetLogger(logger log.CommonLogger) *Client
```

### Logger

Returns the current logger instance.

```go title="Signature"
func (c *Client) Logger() log.CommonLogger
```

## Underlying Transport

A client wraps one fasthttp client: the `fasthttp.Client` that `New` creates, or the one passed to `NewWithClient`, `NewWithHostClient` or `NewWithLBClient`. These methods give access to it.

### FasthttpClient

Returns the underlying `*fasthttp.Client`, or `nil` if the client was created from a `HostClient` or an `LBClient`.

```go title="Signature"
func (c *Client) FasthttpClient() *fasthttp.Client
```

### HostClient

Returns the underlying `*fasthttp.HostClient`, or `nil` if the client was not created with `NewWithHostClient`.

```go title="Signature"
func (c *Client) HostClient() *fasthttp.HostClient
```

### LBClient

Returns the underlying `*fasthttp.LBClient`, or `nil` if the client was not created with `NewWithLBClient`.

```go title="Signature"
func (c *Client) LBClient() *fasthttp.LBClient
```

### Do

Sends a raw `fasthttp` request through the underlying client, as the `Do` method of the wrapped `fasthttp.Client`, `HostClient` or `LBClient` does. It does not follow redirects.

`Do`, `DoTimeout`, `DoDeadline` and `DoRedirects` work directly on `fasthttp` requests and responses and bypass Fiber's request pipeline: the base URL, default headers, query and path parameters, cookies, cookie jar, hooks, client timeout and retry configuration are not applied. Settings of the transport itself, such as the TLS configuration, the dial function or proxy, and response streaming, are.

```go title="Signature"
func (c *Client) Do(req *fasthttp.Request, resp *fasthttp.Response) error
```

### DoTimeout

Like `Do`, but fails with `fasthttp.ErrTimeout` if no response arrives within `timeout`.

```go title="Signature"
func (c *Client) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error
```

### DoDeadline

Like `Do`, but fails with `fasthttp.ErrTimeout` if no response arrives before `deadline`.

```go title="Signature"
func (c *Client) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error
```

### DoRedirects

Like `Do`, but follows up to `maxRedirects` redirects, on every kind of client, with the rules described under [SetMaxRedirects](./request.md#setmaxredirects). A chain of more than `maxRedirects` redirects fails with `fasthttp.ErrTooManyRedirects`, so `0` fails on the first redirect, and a negative value allows 16. Unlike the `Request` methods, it follows redirects for any request method. It drops the headers named with [SetRedirectSensitiveHeaders](#setredirectsensitiveheaders) on the client as well; there is no request-level list, because it takes a raw `fasthttp` request.

```go title="Signature"
func (c *Client) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, maxRedirects int) error
```

### CloseIdleConnections

Closes the idle keep-alive connections of the underlying client, or of every host client behind a load balancer. Connections that are in use are not interrupted.

```go title="Signature"
func (c *Client) CloseIdleConnections()
```

## Reset

### Reset

Resets the client's configuration: it clears the base URL, timeout, user agent,
referer, retry configuration, debug flag, path-normalizing setting, the
redirect-sensitive header names, headers, query and path parameters and
cookies, and it empties and detaches the cookie
jar. It also replaces the transport with a new default `fasthttp.Client`, which
drops a `HostClient` or `LBClient` the client wrapped together with any custom
dial function, proxy, TLS configuration and response streaming setting. Request
and response hooks, the JSON, XML and CBOR marshal and unmarshal functions and
the logger are kept.

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

### Query

Query is a convenience method that sends a QUERY request using the `defaultClient`.

```go title="Signature"
func Query(url string, cfg ...Config) (*Response, error)
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
Replacing the default client is concurrency-safe, but mutating the same `Client` instance still requires external synchronization.
:::

```go title="Signature"
func Replace(c *Client) func()
```
