---
id: request
title: 📤 Request
description: >-
  Request methods of Gofiber HTTP client.
sidebar_position: 2
---

import MethodIndex from '@site/src/components/method-index';

The `Request` struct in Fiber's HTTP client represents an HTTP request. It encapsulates the data required to send a request, including:

- **URL**: The endpoint to which the request is sent.
- **Method**: The HTTP method (GET, POST, PUT, DELETE, etc.).
- **Headers**: Key-value pairs that provide additional information about the request or guide how the response should be processed.
- **Body**: The data sent with the request, commonly used with methods like POST and PUT.
- **Query Parameters**: Parameters appended to the URL to pass additional data or modify the request's behavior.

This structure is designed to be both flexible and efficient, allowing you to easily build and modify HTTP requests as needed.

```go
type Request struct {
    ctx context.Context

    body    any
    header  Header
    params  QueryParam
    cookies Cookie
    path    PathParam

    client *Client

    formData FormData

    RawRequest *fasthttp.Request
    url        string
    method     string
    userAgent  string
    boundary   string
    referer    string
    files      []*File

    timeout      time.Duration
    maxRedirects int

    bodyType bodyType

    isPathNormalizingDisabled bool
    clientOwned               bool
}
```

Use the index to jump straight to any request method; filter by name:

<MethodIndex itemNoun="methods" placeholder="Filter methods, e.g. header" flat />

## REST Methods

### Get

**Get** sends a GET request to the specified URL. It sets the URL and HTTP method, then dispatches the request to the server.

```go title="Signature"
func (r *Request) Get(url string) (*Response, error)
```

### Post

**Post** sends a POST request. It sets the URL and method to POST, then sends the request.

```go title="Signature"
func (r *Request) Post(url string) (*Response, error)
```

### Put

**Put** sends a PUT request. It sets the URL and method to PUT, then sends the request.

```go title="Signature"
func (r *Request) Put(url string) (*Response, error)
```

### Patch

**Patch** sends a PATCH request. It sets the URL and method to PATCH, then sends the request.

```go title="Signature"
func (r *Request) Patch(url string) (*Response, error)
```

### Delete

**Delete** sends a DELETE request. It sets the URL and method to DELETE, then sends the request.

```go title="Signature"
func (r *Request) Delete(url string) (*Response, error)
```

### Head

**Head** sends a HEAD request. It sets the URL and method to HEAD, then sends the request.

```go title="Signature"
func (r *Request) Head(url string) (*Response, error)
```

### Options

**Options** sends an OPTIONS request. It sets the URL and method to OPTIONS, then sends the request.

```go title="Signature"
func (r *Request) Options(url string) (*Response, error)
```

### Query

**Query** sends a QUERY request. It sets the URL and method to QUERY, then sends the request.

```go title="Signature"
func (r *Request) Query(url string) (*Response, error)
```

### Custom

**Custom** sends a request using a custom HTTP method. For example, you can use this to send a TRACE or CONNECT request.

```go title="Signature"
func (r *Request) Custom(url, method string) (*Response, error)
```

## AcquireRequest

**AcquireRequest** returns a new pooled `Request`. Call `ReleaseRequest` when you're finished to return it to the pool and limit allocations. A request acquired this way stays yours: `Response.Close` releases only the response for it. Requests that the client's REST helpers (`Client.Get`, `Client.Post`, ...) create on your behalf are released by `Response.Close`.

```go title="Signature"
func AcquireRequest() *Request
```

## ReleaseRequest

**ReleaseRequest** returns the `Request` to the pool. Do not use it after releasing; doing so may cause data races.

```go title="Signature"
func ReleaseRequest(req *Request)
```

## Method

**Method** returns the current HTTP method set for the request.

```go title="Signature"
func (r *Request) Method() string
```

## SetMethod

**SetMethod** sets the HTTP method for the `Request` object. Typically, you should use the specialized request methods (`Get`, `Post`, etc.) instead of calling `SetMethod` directly.

```go title="Signature"
func (r *Request) SetMethod(method string) *Request
```

## URL

**URL** returns the current URL set in the `Request`.

```go title="Signature"
func (r *Request) URL() string
```

## SetURL

**SetURL** sets the URL for the `Request` object. When the request is sent, the URL must start with `http://` or `https://`, after the client's base URL is prepended to one that does not; otherwise the request fails with `ErrURLFormat`.

```go title="Signature"
func (r *Request) SetURL(url string) *Request
```

## Client

**Client** retrieves the `Client` instance associated with the `Request`.

```go title="Signature"
func (r *Request) Client() *Client
```

## SetClient

**SetClient** assigns a `Client` to the `Request`. If the provided client is `nil`, it will panic with `ErrClientNil`.

```go title="Signature"
func (r *Request) SetClient(c *Client) *Request
```

## Context

**Context** returns the `context.Context` of the request, or `context.Background()` if none is set.

```go title="Signature"
func (r *Request) Context() context.Context
```

## SetContext

**SetContext** sets the `context.Context` for the request, allowing you to cancel or time out the request. See the [Go blog](https://blog.golang.org/context) and [context](https://pkg.go.dev/context) docs for more details.

```go title="Signature"
func (r *Request) SetContext(ctx context.Context) *Request
```

## Header

**Header** returns all values for the specified header key. It searches all header fields stored in the request.

```go title="Signature"
func (r *Request) Header(key string) []string
```

:::info
When the request is sent, the client's headers are applied first and the request's own headers with the same key replace them rather than being appended. Each send clears those keys before applying them again, so sending the same request twice does not accumulate values.
:::

### Headers

**Headers** returns an iterator over all headers in the request. Use `maps.Collect()` to transform them into a map if needed. The returned values are valid only until the request is released. Make copies as required.

```go title="Signature"
func (r *Request) Headers() iter.Seq2[string, []string]
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()

req.AddHeader("Golang", "Fiber")
req.AddHeader("Test", "123456")
req.AddHeader("Test", "654321")

for k, v := range req.Headers() {
  fmt.Printf("Header Key: %s, Header Value: %v\n", k, v)
}
```

```sh
Header Key: Golang, Header Value: [Fiber]
Header Key: Test, Header Value: [123456 654321]
```

</details>

<details>
<summary>Example with maps.Collect()</summary>

```go title="Example with maps.Collect()"
req := client.AcquireRequest()

req.AddHeader("Golang", "Fiber")
req.AddHeader("Test", "123456")
req.AddHeader("Test", "654321")

headers := maps.Collect(req.Headers()) // Collect all headers into a map
for k, v := range headers {
  fmt.Printf("Header Key: %s, Header Value: %v\n", k, v)
}
```

```sh
Header Key: Golang, Header Value: [Fiber]
Header Key: Test, Header Value: [123456 654321]
```

</details>

### AddHeader

**AddHeader** adds a single header field and its value to the request.

```go title="Signature"
func (r *Request) AddHeader(key, val string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.AddHeader("Golang", "Fiber")
req.AddHeader("Test", "123456")
req.AddHeader("Test", "654321")

resp, err := req.Get("https://httpbin.org/headers")
if err != nil {
    panic(err)
}

fmt.Println(resp.String())
```

```json
{
  "headers": {
    "Golang": "Fiber", 
    "Host": "httpbin.org", 
    "Test": "123456,654321", 
    "User-Agent": "fiber", 
    "X-Amzn-Trace-Id": "Root=1-664105d2-033cf7173457adb56d9e7193"
  }
}
```

</details>

### SetHeader

**SetHeader** sets a single header field and its value, overriding any previously set header with the same key.

```go title="Signature"
func (r *Request) SetHeader(key, val string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetHeader("Test", "123456")
req.SetHeader("Test", "654321")

resp, err := req.Get("https://httpbin.org/headers")
if err != nil {
    panic(err)
}

fmt.Println(resp.String())
```

```json
{
  "headers": {
    "Golang": "Fiber", 
    "Host": "httpbin.org", 
    "Test": "654321", 
    "User-Agent": "fiber", 
    "X-Amzn-Trace-Id": "Root=1-664105e5-5d676ba348450cdb62847f04"
  }
}
```

</details>

### AddHeaders

**AddHeaders** adds multiple headers at once from a map of string slices.

```go title="Signature"
func (r *Request) AddHeaders(h map[string][]string) *Request
```

### SetHeaders

**SetHeaders** sets multiple headers at once from a map of strings, overriding any previously set headers.

```go title="Signature"
func (r *Request) SetHeaders(h map[string]string) *Request
```

## Param

**Param** returns all values associated with a given query parameter key.

```go title="Signature"
func (r *Request) Param(key string) []string
```

### Params

**Params** returns an iterator over all query parameters. Use `maps.Collect()` if you need them in a map. The returned values are valid only until the request is released.

```go title="Signature"
func (r *Request) Params() iter.Seq2[string, []string]
```

### AddParam

**AddParam** adds a single query parameter key-value pair.

```go title="Signature"
func (r *Request) AddParam(key, val string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.AddParam("name", "john")
req.AddParam("hobbies", "football")
req.AddParam("hobbies", "basketball")

resp, err := req.Get("https://httpbin.org/response-headers")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "Content-Length": "145", 
  "Content-Type": "application/json", 
  "hobbies": [
    "football", 
    "basketball"
  ], 
  "name": "joe"
}
```

</details>

### SetParam

**SetParam** sets a single query parameter key-value pair, overriding any previously set values for that key.

```go title="Signature"
func (r *Request) SetParam(key, val string) *Request
```

### AddParams

**AddParams** adds multiple query parameters from a map of string slices.

```go title="Signature"
func (r *Request) AddParams(m map[string][]string) *Request
```

### SetParams

**SetParams** sets multiple query parameters from a map of strings, overriding previously set values.

```go title="Signature"
func (r *Request) SetParams(m map[string]string) *Request
```

### SetParamsWithStruct

**SetParamsWithStruct** sets multiple query parameters from a struct. Each exported field becomes a parameter named by its `param` struct tag, or by the field name when it has no tag. Nested structs are not supported; see [SetValWithStruct](#setvalwithstruct) for the field types that are used.

```go title="Signature"
func (r *Request) SetParamsWithStruct(v any) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetParamsWithStruct(struct {
    Name    string   `param:"name"`
    Hobbies []string `param:"hobbies"`
}{
    Name: "John Doe",
    Hobbies: []string{
        "Football",
        "Basketball",
    },
})

resp, err := req.Get("https://httpbin.org/response-headers")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "Content-Length": "147", 
  "Content-Type": "application/json", 
  "hobbies": [
    "Football", 
    "Basketball"
  ], 
  "name": "John Doe"
}
```

</details>

### DelParams

**DelParams** removes one or more query parameters by their keys.

```go title="Signature"
func (r *Request) DelParams(key ...string) *Request
```

## UserAgent

**UserAgent** returns the user agent currently set in the request.

```go title="Signature"
func (r *Request) UserAgent() string
```

## SetUserAgent

**SetUserAgent** sets the user agent header for the request, overriding the one set at the client level if any. Without either, `User-Agent: fiber` is sent. A `User-Agent` set with `SetHeader` or `AddHeader` is replaced when the request is sent, so use this method for it.

```go title="Signature"
func (r *Request) SetUserAgent(ua string) *Request
```

## Boundary

**Boundary** returns the multipart boundary used by the request.

```go title="Signature"
func (r *Request) Boundary() string
```

## SetBoundary

**SetBoundary** sets the multipart boundary for file uploads. Without one, a boundary made of `FiberFormBoundary` and 16 random characters is generated when a request with files is sent.

```go title="Signature"
func (r *Request) SetBoundary(b string) *Request
```

## Referer

**Referer** returns the Referer header value currently set in the request.

```go title="Signature"
func (r *Request) Referer() string
```

## SetReferer

**SetReferer** sets the Referer header for the request, overriding the one set at the client level if any. When neither is set, no `Referer` header is sent. A `Referer` set with `SetHeader` or `AddHeader` is replaced when the request is sent, or removed when no referer is set, so use this method for it.

```go title="Signature"
func (r *Request) SetReferer(referer string) *Request
```

## Cookie

**Cookie** returns the value of the specified cookie. If the cookie does not exist, it returns an empty string.

```go title="Signature"
func (r *Request) Cookie(key string) string
```

### Cookies

**Cookies** returns an iterator over all cookies set in the request. Use `maps.Collect()` to gather them into a map.

```go title="Signature"
func (r *Request) Cookies() iter.Seq2[string, string]
```

### SetCookie

**SetCookie** sets a single cookie key-value pair, overriding any previously set cookie with the same key.

```go title="Signature"
func (r *Request) SetCookie(key, val string) *Request
```

### SetCookies

**SetCookies** sets multiple cookies from a map, overriding previously set values.

```go title="Signature"
func (r *Request) SetCookies(m map[string]string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetCookies(map[string]string{
    "cookie1": "value1",
    "cookie2": "value2",
})

resp, err := req.Get("https://httpbin.org/cookies")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "cookies": {
    "cookie1": "value1",
    "cookie2": "value2"
  }
}
```

</details>

### SetCookiesWithStruct

**SetCookiesWithStruct** sets multiple cookies from a struct. Each field is named by its `cookie` struct tag, or by the field name when it has no tag.

```go title="Signature"
func (r *Request) SetCookiesWithStruct(v any) *Request
```

### DelCookies

**DelCookies** removes one or more cookies by their keys.

```go title="Signature"
func (r *Request) DelCookies(key ...string) *Request
```

## PathParam

**PathParam** returns the value of a named path parameter. If not found, returns an empty string.

```go title="Signature"
func (r *Request) PathParam(key string) string
```

### PathParams

**PathParams** returns an iterator over all path parameters in the request. Use `maps.Collect()` to convert them into a map.

```go title="Signature"
func (r *Request) PathParams() iter.Seq2[string, string]
```

### SetPathParam

**SetPathParam** sets a single path parameter key-value pair, overriding previously set values.

Placeholders are matched as `:name`, and where several registered names fit, the longest one wins, so a name holding a `-`, `.` or `:` resolves as itself. A name otherwise ends at a path-segment boundary (`/`, `-`, `.`, `:`, `\`, `?` or `#`), so `:id` does not also match the start of `:idx`. Values are percent-encoded with path-segment rules, and each placeholder is resolved once — a substituted value is never scanned for further placeholders. A placeholder with no value set is sent as written.

A value stays inside the one path segment its placeholder occupies. Path normalizing percent-decodes the path before it collapses `.` and `..`, so escaping a separator is not enough on its own: a value containing `/` or `\`, a value that is exactly `.` or `..`, or an empty value — which leaves a `//` that collapses and drops the segment — fails the request with `ErrPathParamInPath` rather than changing the path the template set. A `%2F` you write yourself is not a separator — it is escaped to `%252F` and decodes back to the literal text `%2F`.

The host can be templated too (`https://:tenant.example.com/api`). A placeholder in the authority whose name is only digits is treated as the port and is left unchanged, and the colons inside an IPv6 literal such as `http://[2001:db8::1]/api` are part of the address rather than placeholders. A value substituted into the authority is **not** escaped — a `%XX` below `0x80` inside a host makes the URL parser discard the rest of the URL — so it must already be a valid host: unreserved characters, `$`, `&`, `+`, `=` and valid UTF-8 for an IDN label. Malformed UTF-8 is rejected too — the bytes reach the `Host` header verbatim, and an overlong sequence such as `%C0%AF` is a `/` to a lenient decoder. Anything else, including a `:` or `@` that would add a port or a userinfo section and send the request to another host, fails the request with `ErrPathParamInHost`. A port can still be templated by giving the placeholder a name that is not all digits, as in `https://example.com::port/api`.

`SetDisablePathNormalizing(true)` on the client or request turns that check off for `/` and `\`, but only for a client that Fiber knows sends the path as built: one whose wrapped fasthttp client has path normalizing turned off and has not served a host while it was on. [SetDisablePathNormalizing](./rest.md#setdisablepathnormalizing) on the client does that for a client from `New`, `NewWithHostClient` or `NewWithLBClient`. A `fasthttp.Client` given to [NewWithClient](./rest.md#newwithclient) may have served hosts already and Fiber cannot tell, so it counts only when it was created with `DisablePathNormalizing: true`. Nothing decodes the path afterwards then, a `/` in a value is emitted as `%2F` and the value is sent as one segment. Whether it stays one segment is up to the server — one that unescapes paths, including a Fiber server configured with `UnescapePath`, decodes it again while routing. On any other client the check stays on, whatever the client or the request asks for, so a value such as `x/../../admin` is rejected rather than changing the path that is sent. The values the setting never lets through are `.` and `..`, which would go out as dot segments that the receiving server resolves, and an empty value, which leaves a `//` that a server merging slashes drops: no escape keeps them in place. A `?` or `#` in a value is always contained; normalizing does not decode those.

```go title="Signature"
func (r *Request) SetPathParam(key, val string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetPathParam("base64", "R29maWJlcg==")

resp, err := req.Get("https://httpbin.org/base64/:base64")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```plaintext
Gofiber
```

</details>

### SetPathParams

**SetPathParams** sets multiple path parameters at once, overriding previously set values.

```go title="Signature"
func (r *Request) SetPathParams(m map[string]string) *Request
```

### SetPathParamsWithStruct

**SetPathParamsWithStruct** sets multiple path parameters from a struct. Each field is named by its `path` struct tag, or by the field name when it has no tag.

```go title="Signature"
func (r *Request) SetPathParamsWithStruct(v any) *Request
```

### DelPathParams

**DelPathParams** deletes one or more path parameters by their keys.

```go title="Signature"
func (r *Request) DelPathParams(key ...string) *Request
```

### ResetPathParams

**ResetPathParams** deletes all path parameters.

```go title="Signature"
func (r *Request) ResetPathParams() *Request
```

## SetJSON

**SetJSON** sets the request body to a JSON-encoded payload. When the request is sent, the `Content-Type` and `Accept` headers are set to `application/json`, replacing any values set on the client or the request.

```go title="Signature"
func (r *Request) SetJSON(v any) *Request
```

## SetXML

**SetXML** sets the request body to an XML-encoded payload. When the request is sent, the `Content-Type` header is set to `application/xml`.

```go title="Signature"
func (r *Request) SetXML(v any) *Request
```

## SetCBOR

**SetCBOR** sets the request body to a CBOR-encoded payload. It automatically sets the `Content-Type` to `application/cbor`.

```go title="Signature"
func (r *Request) SetCBOR(v any) *Request
```

## SetRawBody

**SetRawBody** sets the request body to raw bytes.

```go title="Signature"
func (r *Request) SetRawBody(v []byte) *Request
```

## FormData

**FormData** returns all values associated with the given form data field.

```go title="Signature"
func (r *Request) FormData(key string) []string
```

### AllFormData

**AllFormData** returns an iterator over all form data fields. Use `maps.Collect()` if needed.

```go title="Signature"
func (r *Request) AllFormData() iter.Seq2[string, []string]
```

### AddFormData

**AddFormData** adds a single form data key-value pair.

```go title="Signature"
func (r *Request) AddFormData(key, val string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.AddFormData("points", "80")
req.AddFormData("points", "90")
req.AddFormData("points", "100")

resp, err := req.Post("https://httpbin.org/post")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "args": {}, 
  "data": "", 
  "files": {}, 
  "form": {
    "points": [
      "80", 
      "90", 
      "100"
    ]
  }, 
  // ...
}
```

</details>

### SetFormData

**SetFormData** sets a single form data field, overriding any previously set values.

```go title="Signature"
func (r *Request) SetFormData(key, val string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetFormData("name", "john")
req.SetFormData("email", "john@doe.com")

resp, err := req.Post("https://httpbin.org/post")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "args": {}, 
  "data": "", 
  "files": {}, 
  "form": {
    "email": "john@doe.com", 
    "name": "john"
  }, 
  // ...
}
```

</details>

### AddFormDataWithMap

**AddFormDataWithMap** adds multiple form data fields and values from a map of string slices.

```go title="Signature"
func (r *Request) AddFormDataWithMap(m map[string][]string) *Request
```

### SetFormDataWithMap

**SetFormDataWithMap** sets multiple form data fields from a map of strings.

```go title="Signature"
func (r *Request) SetFormDataWithMap(m map[string]string) *Request
```

### SetFormDataWithStruct

**SetFormDataWithStruct** sets multiple form data fields from a struct. Each field is named by its `form` struct tag, or by the field name when it has no tag.

```go title="Signature"
func (r *Request) SetFormDataWithStruct(v any) *Request
```

### DelFormData

**DelFormData** deletes one or more form data fields by their keys.

```go title="Signature"
func (r *Request) DelFormData(key ...string) *Request
```

## File

**File** returns a file from the request by its name. A file added without a name is matched by the base name of its path.

```go title="Signature"
func (r *Request) File(name string) *File
```

### Files

**Files** returns all files in the request as a slice. The returned slice is valid only until the request is released.

```go title="Signature"
func (r *Request) Files() []*File
```

### FileByPath

**FileByPath** returns a file from the request by its file path.

```go title="Signature"
func (r *Request) FileByPath(path string) *File
```

### AddFile

**AddFile** adds a single file to the request from a file path.

```go title="Signature"
func (r *Request) AddFile(path string) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.AddFile("test.txt")

resp, err := req.Post("https://httpbin.org/post")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "args": {}, 
  "data": "", 
  "files": {
    "file1": "This is an empty file!\n"
  }, 
  "form": {}, 
  // ...
}
```

</details>

### AddFileWithReader

**AddFileWithReader** adds a single file to the request from an `io.ReadCloser`.

```go title="Signature"
func (r *Request) AddFileWithReader(name string, reader io.ReadCloser) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

buf := bytes.NewBuffer([]byte("Hello, World!"))
req.AddFileWithReader("test.txt", io.NopCloser(buf))

resp, err := req.Post("https://httpbin.org/post")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "args": {}, 
  "data": "", 
  "files": {
    "file1": "Hello, World!"
  }, 
  "form": {}, 
  // ...
}
```

</details>

### AddFiles

**AddFiles** adds multiple files to the request at once.

```go title="Signature"
func (r *Request) AddFiles(files ...*File) *Request
```

## Timeout

**Timeout** returns the timeout duration set in the request.

```go title="Signature"
func (r *Request) Timeout() time.Duration
```

## SetTimeout

**SetTimeout** sets a timeout for the request, overriding any timeout set at the client level. When the timeout elapses, or the request's context is canceled, the request fails with `ErrTimeoutOrCancel`.

```go title="Signature"
func (r *Request) SetTimeout(t time.Duration) *Request
```

<details>
<summary>Example 1</summary>

```go title="Example 1"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetTimeout(5 * time.Second)

resp, err := req.Get("https://httpbin.org/delay/4")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```json
{
  "args": {}, 
  "data": "", 
  "files": {}, 
  "form": {}, 
  // ...
}
```

</details>

<details>
<summary>Example 2</summary>

```go title="Example 2"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetTimeout(5 * time.Second)

resp, err := req.Get("https://httpbin.org/delay/6")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

```shell
panic: timeout or cancel

goroutine 1 [running]:
main.main()
        main.go:18 +0xeb
exit status 2
```

</details>

## MaxRedirects

**MaxRedirects** returns the maximum number of redirects allowed for the request. The default is `0`.

```go title="Signature"
func (r *Request) MaxRedirects() int
```

## SetMaxRedirects

**SetMaxRedirects** sets the maximum number of redirects to follow for the request. The default is `0`, which follows none: the redirect response is returned as-is. The client has no redirect limit of its own, so set one on each request or with `Config.MaxRedirects`.

```go title="Signature"
func (r *Request) SetMaxRedirects(count int) *Request
```

:::info
Redirects are followed only for `GET`, `HEAD` and `QUERY` requests. Other methods receive the redirect response as-is, whatever the limit.
:::

While redirects are followed:

- After a `301`, `302` or `303` the request is repeated as a `GET` without its body; a `HEAD` stays a `HEAD`. A `307` or `308` keeps the method and the body.
- When the target is on another host, the `Authorization`, `Proxy-Authorization`, `Proxy-Authenticate`, `WWW-Authenticate`, `Cookie` and `Cookie2` headers are dropped. They are removed whatever case they are stored under. The names set with [SetRedirectSensitiveHeaders](#setredirectsensitiveheaders) on the request or the client are dropped with them. The same host on any port, and its subdomains, keep them.
- A redirect from `https` to `http` fails with `ErrRedirectDowngrade`. Leave the limit at `0` to take such a hop yourself.
- A redirect without a `Location` fails with `fasthttp.ErrMissingLocation`. A `Location` that cannot be parsed or names a `scheme://` other than `http` or `https` fails with `fasthttp.ErrorInvalidURI` without being requested, as does a request URL that cannot be parsed. A chain longer than the limit fails with `fasthttp.ErrTooManyRedirects`.
- With a [retry configuration](./rest.md#setretryconfig), a chain that fails, on one of the errors above or on a hop that cannot be sent, is retried from the request as it was sent: the original URL, method, body and headers. A request with a streamed body cannot be copied, so its retry starts from the hop the failed attempt reached. A chain that succeeds is not repeated.

## RedirectSensitiveHeaders

**RedirectSensitiveHeaders** returns a copy of the header names set on the request with [SetRedirectSensitiveHeaders](#setredirectsensitiveheaders). The client's own list is not included.

```go title="Signature"
func (r *Request) RedirectSensitiveHeaders() []string
```

## SetRedirectSensitiveHeaders

**SetRedirectSensitiveHeaders** names request headers that must not follow a redirect to another host, such as a credential of your own like `X-Api-Key`. When the target is on a host that is neither the original hostname (on any port) nor one of its subdomains, they are dropped with `Authorization`, `Proxy-Authorization`, `Proxy-Authenticate`, `WWW-Authenticate`, `Cookie` and `Cookie2`; every other header is forwarded, as `net/http` does. Names match case-insensitively, and surrounding whitespace and empty entries are ignored. The names are added to the client's own list ([SetRedirectSensitiveHeaders](./rest.md#setredirectsensitiveheaders)). The default is none, and it matters only while redirects are followed, see [SetMaxRedirects](#setmaxredirects). Calling it with no names clears the request's list.

```go title="Signature"
func (r *Request) SetRedirectSensitiveHeaders(names ...string) *Request
```

```go title="Example"
resp, err := client.New().Get("https://api.example.com/items", client.Config{
    Header:                   map[string]string{"X-Api-Key": os.Getenv("UPSTREAM_API_KEY")},
    MaxRedirects:             3,
    RedirectSensitiveHeaders: []string{"X-Api-Key"},
})
```

## DisablePathNormalizing

**DisablePathNormalizing** reports whether path normalizing is disabled for this request. It does not reflect the client's setting; Fiber builds the URL without normalizing it when either one disables it, see [SetDisablePathNormalizing](#setdisablepathnormalizing) for what reaches the wire.

```go title="Signature"
func (r *Request) DisablePathNormalizing() bool
```

## SetDisablePathNormalizing

**SetDisablePathNormalizing** turns path normalizing off or on for the request. It is on by default: the request path is normalized before it is sent, which decodes percent-escapes, collapses `//` and resolves `.` and `..` segments, so `/a//b/./c%2Fd` goes out as `/a/b/c/d`. With normalizing off, Fiber builds the URL without normalizing it. Whether the path is normalized again when the request is written is up to the wrapped fasthttp client, which a single request cannot change: the path goes out as written only when the client has normalizing turned off, with [SetDisablePathNormalizing](./rest.md#setdisablepathnormalizing) on the client or with a fasthttp client created with `DisablePathNormalizing: true` (see [NewWithClient](./rest.md#newwithclient)). On any other client this setting changes neither what is sent nor the path parameter check. Path parameter values are checked differently while normalizing is off, as described under [SetPathParam](#setpathparam).

```go title="Signature"
func (r *Request) SetDisablePathNormalizing(disable bool) *Request
```

## Send

**Send** executes the HTTP request and returns a `Response`. A request without a client, such as one from `AcquireRequest`, is sent with the default client returned by `client.C()`.

```go title="Signature"
func (r *Request) Send() (*Response, error)
```

## Reset

**Reset** clears the `Request` object, making it ready for reuse. This is used by `ReleaseRequest`.

```go title="Signature"
func (r *Request) Reset()
```

## Header

**Header** is a wrapper around `fasthttp.RequestHeader`, storing headers for both the client and request.

```go
type Header struct {
    *fasthttp.RequestHeader
}
```

### PeekMultiple

**PeekMultiple** returns multiple values associated with the same header key.

```go title="Signature"
func (h *Header) PeekMultiple(key string) []string
```

### AddHeaders

**AddHeaders** adds multiple headers from a map of string slices.

```go title="Signature"
func (h *Header) AddHeaders(r map[string][]string)
```

### SetHeaders

**SetHeaders** sets multiple headers from a map of strings, overriding previously set headers.

```go title="Signature"
func (h *Header) SetHeaders(r map[string]string)
```

## QueryParam

**QueryParam** is a wrapper around `fasthttp.Args`, storing query parameters.

```go
type QueryParam struct {
    *fasthttp.Args
}
```

### Keys

**Keys** returns all keys in the query parameters.

```go title="Signature"
func (p *QueryParam) Keys() []string
```

### AddParams

**AddParams** adds multiple query parameters from a map of string slices.

```go title="Signature"
func (p *QueryParam) AddParams(r map[string][]string)
```

### SetParams

**SetParams** sets multiple query parameters from a map of strings, overriding previously set values.

```go title="Signature"
func (p *QueryParam) SetParams(r map[string]string)
```

### SetParamsWithStruct

**SetParamsWithStruct** sets multiple query parameters from a struct, naming each field by its `param` struct tag. Nested structs are not supported.

```go title="Signature"
func (p *QueryParam) SetParamsWithStruct(v any)
```

## Cookie

**Cookie** is a map that stores cookies.

```go
type Cookie map[string]string
```

### Add

**Add** adds a cookie key-value pair.

```go title="Signature"
func (c Cookie) Add(key, val string)
```

### Del

**Del** removes a cookie by its key.

```go title="Signature"
func (c Cookie) Del(key string)
```

### SetCookie

**SetCookie** sets a single cookie key-value pair, overriding previously set values.

```go title="Signature"
func (c Cookie) SetCookie(key, val string)
```

### SetCookies

**SetCookies** sets multiple cookies from a map of strings.

```go title="Signature"
func (c Cookie) SetCookies(m map[string]string)
```

### SetCookiesWithStruct

**SetCookiesWithStruct** sets multiple cookies from a struct, naming each field by its `cookie` struct tag.

```go title="Signature"
func (c Cookie) SetCookiesWithStruct(v any)
```

### DelCookies

**DelCookies** deletes one or more cookies by their keys.

```go title="Signature"
func (c Cookie) DelCookies(key ...string)
```

### All

**All** returns an iterator over all cookies. The key and value returned
should not be retained after the loop ends.

```go title="Signature"
func (c Cookie) All() iter.Seq2[string, string]
```

### Reset

**Reset** clears all cookies.

```go title="Signature"
func (c Cookie) Reset()
```

## PathParam

**PathParam** is a map that stores path parameters.

```go
type PathParam map[string]string
```

### Add

**Add** adds a path parameter key-value pair.

```go title="Signature"
func (p PathParam) Add(key, val string)
```

### Del

**Del** removes a path parameter by its key.

```go title="Signature"
func (p PathParam) Del(key string)
```

### SetParam

**SetParam** sets a single path parameter key-value pair, overriding previously set values.

```go title="Signature"
func (p PathParam) SetParam(key, val string)
```

### SetParams

**SetParams** sets multiple path parameters from a map of strings.

```go title="Signature"
func (p PathParam) SetParams(m map[string]string)
```

### SetParamsWithStruct

**SetParamsWithStruct** sets multiple path parameters from a struct, naming each field by its `path` struct tag.

```go title="Signature"
func (p PathParam) SetParamsWithStruct(v any)
```

### DelParams

**DelParams** deletes one or more path parameters by their keys.

```go title="Signature"
func (p PathParam) DelParams(key ...string)
```

### All

**All** returns an iterator over all path parameters. The key and value returned
should not be retained after the loop ends.

```go title="Signature"
func (p PathParam) All() iter.Seq2[string, string]
```

### Reset

**Reset** clears all path parameters.

```go title="Signature"
func (p PathParam) Reset()
```

## FormData

**FormData** is a wrapper around `fasthttp.Args`, used to handle URL-encoded and form-data (multipart) request bodies.

```go
type FormData struct {
    *fasthttp.Args
}
```

### Keys

**Keys** returns all form data keys.

```go title="Signature"
func (f *FormData) Keys() []string
```

### Add

**Add** adds a single form field key-value pair.

```go title="Signature"
func (f *FormData) Add(key, val string)
```

### Set

**Set** sets a single form field key-value pair, overriding any previously set values.

```go title="Signature"
func (f *FormData) Set(key, val string)
```

### AddWithMap

**AddWithMap** adds multiple form fields from a map of string slices.

```go title="Signature"
func (f *FormData) AddWithMap(m map[string][]string)
```

### SetWithMap

**SetWithMap** sets multiple form fields from a map of strings.

```go title="Signature"
func (f *FormData) SetWithMap(m map[string]string)
```

### SetWithStruct

**SetWithStruct** sets multiple form fields from a struct, naming each field by its `form` struct tag.

```go title="Signature"
func (f *FormData) SetWithStruct(v any)
```

### DelData

**DelData** deletes one or more form fields by their keys.

```go title="Signature"
func (f *FormData) DelData(key ...string)
```

### Reset

**Reset** clears all form data fields.

```go title="Signature"
func (f *FormData) Reset()
```

## File

**File** represents a file to be uploaded. It can be specified by name, path, or an `io.ReadCloser`.

```go
type File struct {
    name      string
    fieldName string
    path      string
    reader    io.ReadCloser
}
```

When the request is sent, a file with a path but no name takes the base name of the path as its file name, and a file without a field name is sent in the field `file1`, `file2`, and so on, by its position. A file with neither a name nor a path makes the request fail with `ErrFileNoName`.

### AcquireFile

**AcquireFile** returns a `File` from the pool and applies any provided `SetFileFunc` functions to it. Release it with `ReleaseFile` when done. A file that you add to a request is released together with the request, so do not release it yourself in that case.

```go title="Signature"
func AcquireFile(setter ...SetFileFunc) *File
```

### ReleaseFile

**ReleaseFile** returns the `File` to the pool. Do not use the file afterward.

```go title="Signature"
func ReleaseFile(f *File)
```

### SetFileFunc

**SetFileFunc** is a function that modifies a `File`, the option type that `AcquireFile` accepts. These helpers create the common ones:

- **SetFileName** sets the file name that is sent in the multipart form.
- **SetFileFieldName** sets the name of the form field that carries the file.
- **SetFilePath** sets the path the file is read from.
- **SetFileReader** sets the `io.ReadCloser` the file is read from. It is closed automatically when the request body is built.

```go title="Signature"
type SetFileFunc func(f *File)

func SetFileName(n string) SetFileFunc
func SetFileFieldName(p string) SetFileFunc
func SetFilePath(p string) SetFileFunc
func SetFileReader(r io.ReadCloser) SetFileFunc
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.AddFiles(client.AcquireFile(
    client.SetFilePath("./avatar.png"),
    client.SetFileFieldName("avatar"),
))

resp, err := req.Post("https://httpbin.org/post")
if err != nil {
    panic(err)
}

fmt.Println(string(resp.Body()))
```

</details>

### SetName

**SetName** sets the file's name.

```go title="Signature"
func (f *File) SetName(n string)
```

### SetFieldName

**SetFieldName** sets the field name of the file in the multipart form.

```go title="Signature"
func (f *File) SetFieldName(n string)
```

### SetPath

**SetPath** sets the file's path.

```go title="Signature"
func (f *File) SetPath(p string)
```

### SetReader

**SetReader** sets the file's `io.ReadCloser`. The reader is closed automatically when the request body is parsed.

```go title="Signature"
func (f *File) SetReader(r io.ReadCloser)
```

### Reset

**Reset** clears the file's fields.

```go title="Signature"
func (f *File) Reset()
```

## SetValWithStruct

**SetValWithStruct** fills a `WithStruct` from the fields of a struct. The struct setters of the request and the client, such as `SetParamsWithStruct`, use it with the struct tags `param`, `cookie`, `path` and `form`. Call it yourself to fill a collection of your own that implements `WithStruct`.

`v` must be a struct or a non-nil pointer to a struct. `nil` panics, and so does a nil pointer to a struct that has exported fields. Any other value is ignored, including a nil map, a nil slice, a nil pointer to a type that is not a struct, and a pointer to a pointer. Each exported field is added under the name in its `tagName` struct tag, or under the field name when the tag is missing or empty. Fields of type string, integer, float, complex or bool are written as text, and a slice or array of those adds one value per element under the same name. Fields of other types, such as nested structs, maps and pointers, are skipped. A name that is already present is deleted first, even when the field itself is skipped, so the struct's values replace the earlier ones. A cookie or path parameter holds one value per name, so for a slice only the last element remains.

```go title="Signature"
type WithStruct interface {
    Add(name, obj string)
    Del(name string)
}

func SetValWithStruct(p WithStruct, tagName string, v any)
```

<details>
<summary>Example</summary>

```go title="Example"
type filter struct {
    Query string   `param:"q"`
    Tags  []string `param:"tag"`
    Page  int
    skip  string
}

args := fasthttp.AcquireArgs()
defer fasthttp.ReleaseArgs(args)

// *fasthttp.Args has the Add and Del methods that WithStruct needs.
client.SetValWithStruct(args, "param", filter{
    Query: "fiber",
    Tags:  []string{"go", "http"},
    Page:  2,
    skip:  "ignored",
})

fmt.Println(args.String())
```

```text
q=fiber&tag=go&tag=http&Page=2
```

</details>
