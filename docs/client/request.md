---
id: request
title: 📤 Request
description: >-
  Request methods of Gofiber HTTP client.
sidebar_position: 2
---

The `Request` structure in Gofiber's HTTP client represents an HTTP request. It encapsulates all the necessary information needed to send a request to a server, including:

- **URL**: The endpoint to which the request is sent.
- **Method**: The HTTP method (GET, POST, PUT, DELETE, etc.).
- **Headers**: Key-value pairs that provide additional information about the request or guide how the response should be processed.
- **Body**: The data sent with the request, commonly used with methods like POST and PUT.
- **Query Parameters**: Parameters appended to the URL to pass additional data or modify the request's behavior.

This structure is designed to be both flexible and efficient, allowing you to easily build and modify HTTP requests as needed.

```go
type Request struct {
    url       string
    method    string
    userAgent string
    boundary  string
    referer   string
    ctx       context.Context
    header    *Header
    params    *QueryParam
    cookies   *Cookie
    path      *PathParam

    timeout      time.Duration
    maxRedirects int

    client *Client

    body     any
    formData *FormData
    files    []*File
    bodyType bodyType

    RawRequest *fasthttp.Request
}
```

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

### Custom

**Custom** sends a request using a custom HTTP method. For example, you can use this to send a TRACE or CONNECT request.

```go title="Signature"
func (r *Request) Custom(url, method string) (*Response, error)
```

## AcquireRequest

**AcquireRequest** returns a new (pooled) `Request` object. When you are done with the request, call `ReleaseRequest` to return it to the pool and reduce GC load.

```go title="Signature"
func AcquireRequest() *Request
```

## ReleaseRequest

**ReleaseRequest** returns the `Request` object back to the pool. Do not use the request after releasing it, as this may cause data races.

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

**SetURL** sets the URL for the `Request` object.

```go title="Signature"
func (r *Request) SetURL(url string) *Request
```

## Client

**Client** retrieves the `Client` instance associated with the `Request`.

```go title="Signature"
func (r *Request) Client() *Client
```

## SetClient

**SetClient** assigns a `Client` to the `Request`. If the provided client is `nil`, it will panic.

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
    "Referer": "", 
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
    "Referer": "", 
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
  "name": "efectn"
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

**SetParamsWithStruct** sets multiple query parameters from a struct. Nested structs are not supported.

```go title="Signature"
func (r *Request) SetParamsWithStruct(v any) *Request
```

<details>
<summary>Example</summary>

```go title="Example"
req := client.AcquireRequest()
defer client.ReleaseRequest(req)

req.SetParamsWithStruct(struct {
    Name    string   `json:"name"`
    Hobbies []string `json:"hobbies"`
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
  "Hobbies": [
    "Football", 
    "Basketball"
  ], 
  "Name": "John Doe"
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

**SetUserAgent** sets the user agent header for the request, overriding the one set at the client level if any.

```go title="Signature"
func (r *Request) SetUserAgent(ua string) *Request
```

## Boundary

**Boundary** returns the multipart boundary used by the request.

```go title="Signature"
func (r *Request) Boundary() string
```

## SetBoundary

**SetBoundary** sets the multipart boundary for file uploads.

```go title="Signature"
func (r *Request) SetBoundary(b string) *Request
```

## Referer

**Referer** returns the Referer header value currently set in the request.

```go title="Signature"
func (r *Request) Referer() string
```

## SetReferer

**SetReferer** sets the Referer header for the request, overriding the one set at the client level if any.

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
    "test": "123"
  }
}
```

</details>

### SetCookiesWithStruct

**SetCookiesWithStruct** sets multiple cookies from a struct.

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

**SetPathParamsWithStruct** sets multiple path parameters from a struct.

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

**SetJSON** sets the request body to a JSON-encoded payload.

```go title="Signature"
func (r *Request) SetJSON(v any) *Request
```

## SetXML

**SetXML** sets the request body to an XML-encoded payload.

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

**SetFormDataWithStruct** sets multiple form data fields from a struct.

```go title="Signature"
func (r *Request) SetFormDataWithStruct(v any) *Request
```

### DelFormData

**DelFormData** deletes one or more form data fields by their keys.

```go title="Signature"
func (r *Request) DelFormData(key ...string) *Request
```

## File

**File** returns a file from the request by its name. If no name was provided, it attempts to match by path.

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

**SetTimeout** sets a timeout for the request, overriding any timeout set at the client level.

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

**MaxRedirects** returns the maximum number of redirects allowed for the request.

```go title="Signature"
func (r *Request) MaxRedirects() int
```

## SetMaxRedirects

**SetMaxRedirects** sets the maximum number of redirects for the request, overriding the client's setting.

```go title="Signature"
func (r *Request) SetMaxRedirects(count int) *Request
```

## Send

**Send** executes the HTTP request and returns a `Response`.

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

**SetParamsWithStruct** sets multiple query parameters from a struct. Nested structs are not supported.

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

**SetCookiesWithStruct** sets multiple cookies from a struct.

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

**SetParamsWithStruct** sets multiple path parameters from a struct.

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

**SetWithStruct** sets multiple form fields from a struct.

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

### AcquireFile

**AcquireFile** returns a `File` from the pool and applies any provided `SetFileFunc` functions to it. Release it with `ReleaseFile` when done.

```go title="Signature"
func AcquireFile(setter ...SetFileFunc) *File
```

### ReleaseFile

**ReleaseFile** returns the `File` to the pool. Do not use the file afterward.

```go title="Signature"
func ReleaseFile(f *File)
```

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
