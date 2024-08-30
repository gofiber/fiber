---
id: rest
title: 🖥️ REST
description: >-
  HTTP client for Gofiber.
sidebar_position: 1
toc_max_heading_level: 5
---

The Fiber Client for Fiber v3 is a powerful HTTP client optimized for high performance and ease of use in server-side applications. Built on top of the robust FastHTTP library, it inherits FastHTTP's high-speed HTTP protocol implementation. The client is designed to make HTTP requests both internally within services or externally to other web services.

## Features

- **Lightweight & Fast**: Leveraging the minimalistic design of FastHTTP, the Fiber Client is lightweight and extremely fast.
- **Flexible Configuration**: Configure client-level settings such as timeouts, headers, and more, which apply to all requests. Specific requests can further override or merge these settings.
- **Connection Pooling**: Manages a pool of persistent connections that reduce the overhead of repeatedly establishing connections.
- **Timeouts & Retries**: Supports setting request timeouts and retry mechanisms to handle transient failures.

## Usage

To use the Fiber Client, instantiate it with the desired configuration. Here's a simple example:

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

    // Get request
    resp, err := cc.Get("https://httpbin.org/get")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Status: %d\n", resp.StatusCode())
    fmt.Printf("Body: %s\n", string(resp.Body()))
}
```

You can check out [examples](examples.md) for more examples!

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

    // user defined request hooks
    userRequestHooks []RequestHook

    // client package defined request hooks
    builtinRequestHooks []RequestHook

    // user defined response hooks
    userResponseHooks []ResponseHook

    // client package defined response hooks
    builtinResponseHooks []ResponseHook

    jsonMarshal   utils.JSONMarshal
    jsonUnmarshal utils.JSONUnmarshal
    xmlMarshal    utils.XMLMarshal
    xmlUnmarshal  utils.XMLUnmarshal

    cookieJar *CookieJar

    // proxy
    proxyURL string

    // retry
    retryConfig *RetryConfig

    // logger
    logger log.CommonLogger
}
```

 New

New creates and returns a new Client object.

```go title="Signature"
func New() *Client
```

## REST Methods

### Get

Get provides an API like axios which sends a get request.

```go title="Signature"
func (c *Client) Get(url string, cfg ...Config) (*Response, error)
```

### Post

Post provides an API like axios which send post request.

```go title="Signature"
func (c *Client) Post(url string, cfg ...Config) (*Response, error)
```

### Put

Put provides an API like axios which send put request.

```go title="Signature"
func (c *Client) Put(url string, cfg ...Config) (*Response, error)
```

### Patch

Patch provides an API like axios which send patch request.

```go title="Signature"
func (c *Client) Patch(url string, cfg ...Config) (*Response, error)
```

### Delete

Delete provides an API like axios which send delete request.

```go title="Signature"
func (c *Client) Delete(url string, cfg ...Config) (*Response, error)
```

### Head

Head provides an API like axios which send head request.

```go title="Signature"
func (c *Client) Head(url string, cfg ...Config) (*Response, error)
```

### Options

Options provides an API like axios which send options request.

```go title="Signature"
func (c *Client) Options(url string, cfg ...Config) (*Response, error)
```

### Custom

Custom provides an API like axios which send custom request.

```go title="Signature"
func (c *Client) Custom(url, method string, cfg ...Config) (*Response, error)
```

## Request Configuration

Config for easy to set the request parameters, it should be noted that when setting the request body will use JSON as the default serialization mechanism, while the priority of Body is higher than FormData, and the priority of FormData is higher than File.

It can be used to configure request data while sending requests using Get, Post, etc.

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

### R

R raise a request from the client.
It acquires a request from the pool. You have to release it using `ReleaseRequest()` when it's no longer needed.

```go title="Signature"
func (c *Client) R() *Request
```

### Hooks

#### RequestHook

RequestHook Request returns user-defined request hooks.

```go title="Signature"
func (c *Client) RequestHook() []RequestHook
```

#### ResponseHook

ResponseHook return user-define response hooks.

```go title="Signature"
func (c *Client) ResponseHook() []ResponseHook
```

#### AddRequestHook

AddRequestHook Add user-defined request hooks.

```go title="Signature"
func (c *Client) AddRequestHook(h ...RequestHook) *Client
```

#### AddResponseHook

AddResponseHook Add user-defined response hooks.

```go title="Signature"
func (c *Client) AddResponseHook(h ...ResponseHook) *Client
```

### JSON

#### JSONMarshal

JSONMarshal returns json marshal function in Core.

```go title="Signature"
func (c *Client) JSONMarshal() utils.JSONMarshal
```

#### JSONUnmarshal

JSONUnmarshal returns json unmarshal function in Core.

```go title="Signature"
func (c *Client) JSONUnmarshal() utils.JSONUnmarshal
```

#### SetJSONMarshal

SetJSONMarshal sets the JSON encoder.

```go title="Signature"
func (c *Client) SetJSONMarshal(f utils.JSONMarshal) *Client
```

#### SetJSONUnmarshal

Set the JSON decoder.

```go title="Signature"
func (c *Client) SetJSONUnmarshal(f utils.JSONUnmarshal) *Client
```

### XML

#### XMLMarshal

XMLMarshal returns xml marshal function in Core.

```go title="Signature"
func (c *Client) XMLMarshal() utils.XMLMarshal
```

#### XMLUnmarshal

XMLUnmarshal returns xml unmarshal function in Core.

```go title="Signature"
func (c *Client) XMLUnmarshal() utils.XMLUnmarshal
```

#### SetXMLMarshal

SetXMLMarshal sets the XML encoder.

```go title="Signature"
func (c *Client) SetXMLMarshal(f utils.XMLMarshal) *Client
```

#### SetXMLUnmarshal

SetXMLUnmarshal sets the XML decoder.

```go title="Signature"
func (c *Client) SetXMLUnmarshal(f utils.XMLUnmarshal) *Client
```

### TLS

#### TLSConfig

TLSConfig returns tlsConfig in client.
If the client doesn't have a tlsConfig, this function will initialize it.

```go title="Signature"
func (c *Client) TLSConfig() *tls.Config
```

#### SetTLSConfig

SetTLSConfig sets tlsConfig in client.

```go title="Signature"
func (c *Client) SetTLSConfig(config *tls.Config) *Client
```

#### SetCertificates

SetCertificates method sets client certificates into client.

```go title="Signature"
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client
```

#### SetRootCertificate

SetRootCertificate adds one or more root certificates into client.

```go title="Signature"
func (c *Client) SetRootCertificate(path string) *Client
```

#### SetRootCertificateFromString

SetRootCertificateFromString method adds one or more root certificates into the client.

```go title="Signature"
func (c *Client) SetRootCertificateFromString(pem string) *Client
```

### SetProxyURL

SetProxyURL sets proxy url in client. It will apply via core to hostclient.

```go title="Signature"
func (c *Client) SetProxyURL(proxyURL string) error
```

### RetryConfig

RetryConfig returns retry config in client.

```go title="Signature"
func (c *Client) RetryConfig() *RetryConfig
```

### SetRetryConfig

SetRetryConfig sets retry config in client, which is impl by addon/retry package.

```go title="Signature"
func (c *Client) SetRetryConfig(config *RetryConfig) *Client
```

### BaseURL

BaseURL returns baseurl in Client instance.

```go title="Signature"
func (c *Client) BaseURL() string
```

### SetBaseURL

SetBaseURL Set baseUrl which is prefix of real url.

```go title="Signature"
func (c *Client) SetBaseURL(url string) *Client
```

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

### Header

Header method returns header value via key, this method will visit all field in the header

```go title="Signature"
func (c *Client) Header(key string) []string
```

#### AddHeader

AddHeader method adds a single header field and its value in the client instance.
These headers will be applied to all requests raised from this client instance.
Also, it can be overridden at request level header options.

```go title="Signature"
func (c *Client) AddHeader(key, val string) *Client
```

#### SetHeader

SetHeader method sets a single header field and its value in the client instance.
These headers will be applied to all requests raised from this client instance.
Also, it can be overridden at request level header options.

```go title="Signature"
func (c *Client) SetHeader(key, val string) *Client
```

#### AddHeaders

AddHeaders method adds multiple headers field and its values at one go in the client instance.
These headers will be applied to all requests raised from this client instance.
Also it can be overridden at request level headers options.

```go title="Signature"
func (c *Client) AddHeaders(h map[string][]string) *Client
```

#### SetHeaders

SetHeaders method sets multiple headers field and its values at one go in the client instance.
These headers will be applied to all requests raised from this client instance.
Also it can be overridden at request level headers options.

```go title="Signature"
func (c *Client) SetHeaders(h map[string]string) *Client
```

### Param

Param method returns params value via key, this method will visit all field in the query param.

```go title="Signature"
func (c *Client) Param(key string) []string
```

#### AddParam

AddParam method adds a single query param field and its value in the client instance.
These params will be applied to all requests raised from this client instance.
Also, it can be overridden at request level param options.

```go title="Signature"
func (c *Client) AddParam(key, val string) *Client
```

#### SetParam

SetParam method sets a single query param field and its value in the client instance.
These params will be applied to all requests raised from this client instance.
Also, it can be overridden at request level param options.

```go title="Signature"
func (c *Client) SetParam(key, val string) *Client
```

#### AddParams

AddParams method adds multiple query params field and its values at one go in the client instance.
These params will be applied to all requests raised from this client instance.
Also it can be overridden at request level params options.

```go title="Signature"
func (c *Client) AddParams(m map[string][]string) *Client
```

#### SetParams

SetParams method sets multiple params field and its values at one go in the client instance.
These params will be applied to all requests raised from this client instance.
Also it can be overridden at request level params options.

```go title="Signature"
func (c *Client) SetParams(m map[string]string) *Client
```

#### SetParamsWithStruct

SetParamsWithStruct method sets multiple params field and its values at one go in the client instance.
These params will be applied to all requests raised from this client instance.
Also it can be overridden at request level params options.

```go title="Signature"
func (c *Client) SetParamsWithStruct(v any) *Client
```

#### DelParams

DelParams method deletes single or multiple params field and its values in client.

```go title="Signature"
func (c *Client) DelParams(key ...string) *Client
```

### SetUserAgent

SetUserAgent method sets the userAgent field and its value in the client instance.
This ua will be applied to all requests raised from this client instance.
Also it can be overridden at request level ua options.

```go title="Signature"
func (c *Client) SetUserAgent(ua string) *Client
```

### SetReferer

SetReferer method sets referer field and its value in the client instance.
This referer will be applied to all requests raised from this client instance.
Also it can be overridden at request level referer options.

```go title="Signature"
func (c *Client) SetReferer(r string) *Client
```

### PathParam

PathParam returns the path param be set in request instance.
If the path param doesn't exist, return empty string.

```go title="Signature"
func (c *Client) PathParam(key string) string
```

#### SetPathParam

SetPathParam method sets a single path param field and its value in the client instance.
These path params will be applied to all requests raised from this client instance.
Also it can be overridden at request level path params options.

```go title="Signature"
func (c *Client) SetPathParam(key, val string) *Client
```

#### SetPathParams

SetPathParams method sets multiple path params field and its values at one go in the client instance.
These path params will be applied to all requests raised from this client instance.
Also it can be overridden at request level path params options.

```go title="Signature"
func (c *Client) SetPathParams(m map[string]string) *Client
```

#### SetPathParamsWithStruct

SetPathParamsWithStruct method sets multiple path params field and its values at one go in the client instance.
These path params will be applied to all requests raised from this client instance.
Also it can be overridden at request level path params options.

```go title="Signature"
func (c *Client) SetPathParamsWithStruct(v any) *Client
```

#### DelPathParams

DelPathParams method deletes single or multiple path params field and its values in client.

```go title="Signature"
func (c *Client) DelPathParams(key ...string) *Client
```

### Cookie

Cookie returns the cookie be set in request instance.
If cookie doesn't exist, return empty string.

```go title="Signature"
func (c *Client) Cookie(key string) string
```

#### SetCookie

SetCookie method sets a single cookie field and its value in the client instance.
These cookies will be applied to all requests raised from this client instance.
Also it can be overridden at request level cookie options.

```go title="Signature"
func (c *Client) SetCookie(key, val string) *Client
```

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

#### SetCookies

SetCookies method sets multiple cookies field and its values at one go in the client instance.
These cookies will be applied to all requests raised from this client instance.
Also it can be overridden at request level cookie options.

```go title="Signature"
func (c *Client) SetCookies(m map[string]string) *Client
```

#### SetCookiesWithStruct

SetCookiesWithStruct method sets multiple cookies field and its values at one go in the client instance.
These cookies will be applied to all requests raised from this client instance.
Also it can be overridden at request level cookies options.

```go title="Signature"
func (c *Client) SetCookiesWithStruct(v any) *Client
```

#### DelCookies

DelCookies method deletes single or multiple cookies field and its values in client.

```go title="Signature"
func (c *Client) DelCookies(key ...string) *Client
```

### SetTimeout

SetTimeout method sets timeout val in client instance.
This value will be applied to all requests raised from this client instance.
Also, it can be overridden at request level timeout options.

```go title="Signature"
func (c *Client) SetTimeout(t time.Duration) *Client
```

### Debug

Debug enable log debug level output.

```go title="Signature"
func (c *Client) Debug() *Client
```

#### DisableDebug

DisableDebug disables log debug level output.

```go title="Signature"
func (c *Client) DisableDebug() *Client
```

### SetCookieJar

SetCookieJar sets cookie jar in client instance.

```go title="Signature"
func (c *Client) SetCookieJar(cookieJar *CookieJar) *Client
```

### SetDial

SetDial sets dial function in client.

```go title="Signature"
func (c *Client) SetDial(dial fasthttp.DialFunc) *Client
```

### SetLogger

SetLogger sets logger instance in client.

```go title="Signature"
func (c *Client) SetLogger(logger log.CommonLogger) *Client
```

### Logger

Logger returns logger instance of client.

```go title="Signature"
func (c *Client) Logger() log.CommonLogger
```

### Reset

Reset clears the Client object

```go title="Signature"
func (c *Client) Reset()
```

## Default Client

Default client is default client object of Gofiber and created using `New()`.
You can configurate it as you wish or replace it with another clients.

### C

C gets default client.

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

Replace the defaultClient, the returned function can undo.

:::caution
The default client should not be changed concurrently.
:::

```go title="Signature"
func Replace(c *Client) func()
```
