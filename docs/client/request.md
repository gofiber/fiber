---
id: request
title: 📤 Request
description: >-
  Request methods of Gofiber HTTP client.
sidebar_position: 2
---

The `Request` structure in Gofiber's HTTP client represents an HTTP request. It encapsulates all the necessary information required to send a request to a server. This includes:

- **URL**: The URL to which the request is sent.
- **Method**: The HTTP method used (GET, POST, PUT, DELETE, etc.).
- **Headers**: HTTP headers that provide additional information about the request or the needed responses.
- **Body**: The data sent with the request, typically used with POST and PUT methods.
- **Query Parameters**: Parameters that are appended to the URL, used to modify the request or to provide additional information.

This structure is designed to be flexible and efficient, allowing users to easily construct and modify HTTP requests according to their needs.

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

Get sends the GET request.
It sets the URL and HTTP method, and then it sends the request.

```go title="Signature"
func (r *Request) Get(url string) (*Response, error)
```

### Post

Post sends the POST request.
It sets the URL and HTTP method, and then it sends the request.

```go title="Signature"
func (r *Request) Post(url string) (*Response, error)
```

### Put

Put sends the PUT request.
It sets the URL and HTTP method, and then it sends the request.

```go title="Signature"
func (r *Request) Put(url string) (*Response, error)
```

### Patch

Patch sends the PATCH request.
It sets the URL and HTTP method, and then it sends the request.

```go title="Signature"
func (r *Request) Patch(url string) (*Response, error)
```

### Delete

Delete sends the DELETE request.
It sets the URL and HTTP method, and then it sends the request.

```go title="Signature"
func (r *Request) Delete(url string) (*Response, error)
```

### Head

Head sends the HEAD request.
It sets the URL and HTTP method, and then it sends the request.

```go title="Signature"
func (r *Request) Head(url string) (*Response, error)
```

### Options

Options sends the OPTIONS request.
It sets the URL and HTTP method, and then it sends the request.

```go title="Signature"
func (r *Request) Options(url string) (*Response, error)
```

### Custom

Custom sends a request with custom HTTP method.
It sets the URL and HTTP method, and then it sends the request.
You can use Custom to send requests with methods like TRACE, CONNECT.

```go title="Signature"
func (r *Request) Custom(url, method string) (*Response, error)
```

## AcquireRequest

AcquireRequest returns an empty request object from the pool.
The returned request may be returned to the pool with ReleaseRequest when no longer needed.
This allows reducing GC load.

```go title="Signature"
func AcquireRequest() *Request
```

## ReleaseRequest

ReleaseRequest returns the object acquired via AcquireRequest to the pool.
Do not access the released Request object; otherwise, data races may occur.

```go title="Signature"
func ReleaseRequest(req *Request)
```

## Method

Method returns HTTP method in request.

```go title="Signature"
func (r *Request) Method() string
```

## SetMethod

SetMethod will set method for Request object. The user should use request method to set method.

```go title="Signature"
func (r *Request) SetMethod(method string) *Request
```

## URL

URL returns request url in Request instance.

```go title="Signature"
func (r *Request) URL() string
```

## SetURL

SetURL will set url for Request object.

```go title="Signature"
func (r *Request) SetURL(url string) *Request
```

## Client

Client gets the Client instance of Request.

```go title="Signature"
func (r *Request) Client() *Client
```

## SetClient

SetClient method sets client of request instance.
If the client given is null, it will panic.

```go title="Signature"
func (r *Request) SetClient(c *Client) *Request
```

## Context

Context returns the Context if it's already set in the request; otherwise, it returns `context.Background()`.

```go title="Signature"
func (r *Request) Context() context.Context
```

## SetContext

SetContext sets the context.Context for current Request. It allows interruption of the request execution if the ctx.Done() channel is closed.
See [the article](https://blog.golang.org/context) and the [context](https://pkg.go.dev/context) package documentation.

```go title="Signature"
func (r *Request) SetContext(ctx context.Context) *Request
```

## Header

Header method returns header value via key, this method will visit all field in the header.

```go title="Signature"
func (r *Request) Header(key string) []string
```

### AddHeader

AddHeader method adds a single header field and its value in the request instance.

```go title="Signature"
func (r *Request) AddHeader(key, val string) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

SetHeader method sets a single header field and its value in the request instance.
It will override the header which has been set in the client instance.

```go title="Signature"
func (r *Request) SetHeader(key, val string) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

AddHeaders method adds multiple header fields and its values at one go in the request instance.

```go title="Signature"
func (r *Request) AddHeaders(h map[string][]string) *Request
```

### SetHeaders

SetHeaders method sets multiple header fields and its values at one go in the request instance.
It will override the header which has been set in the client instance.

```go title="Signature"
func (r *Request) SetHeaders(h map[string]string) *Request
```

## Param

Param method returns params value via key, this method will visit all field in the query param.

```go title="Signature"
func (r *Request) Param(key string) []string
```

### AddParam

AddParam method adds a single param field and its value in the request instance.

```go title="Signature"
func (r *Request) AddParam(key, val string) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

SetParam method sets a single param field and its value in the request instance.
It will override param, which has been set in client instance.

```go title="Signature"
func (r *Request) SetParam(key, val string) *Request
```

### AddParams

AddParams method adds multiple param fields and its values at one go in the request instance.

```go title="Signature"
func (r *Request) AddParams(m map[string][]string) *Request
```

### SetParams

SetParams method sets multiple param fields and its values at one go in the request instance.
It will override param, which has been set in client instance.

```go title="Signature"
func (r *Request) SetParams(m map[string]string) *Request
```

### SetParamsWithStruct

SetParamsWithStruct method sets multiple param fields and its values at one go in the request instance.
It will override param, which has been set in client instance.

```go title="Signature"
func (r *Request) SetParamsWithStruct(v any) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

DelParams method deletes single or multiple param fields and their values.

```go title="Signature"
func (r *Request) DelParams(key ...string) *Request
```

## UserAgent

UserAgent returns user agent in request instance.

```go title="Signature"
func (r *Request) UserAgent() string
```

## SetUserAgent

SetUserAgent method sets user agent in request.
It will override the user agent which has been set in the client instance.

```go title="Signature"
func (r *Request) SetUserAgent(ua string) *Request
```

## Boundary

Boundary returns boundary in multipart boundary.

```go title="Signature"
func (r *Request) Boundary() string
```

## SetBoundary

SetBoundary method sets multipart boundary.

```go title="Signature"
func (r *Request) SetBoundary(b string) *Request
```

## Referer

Referer returns referer in request instance.

```go title="Signature"
func (r *Request) Referer() string
```

## SetReferer

SetReferer method sets referer in request.
It will override referer which set in client instance.

```go title="Signature"
func (r *Request) SetReferer(referer string) *Request
```

## Cookie

Cookie returns the cookie set in the request instance. If the cookie doesn't exist, returns empty string.

```go title="Signature"
func (r *Request) Cookie(key string) string
```

### SetCookie

SetCookie method sets a single cookie field and its value in the request instance.
It will override the cookie which is set in the client instance.

```go title="Signature"
func (r *Request) SetCookie(key, val string) *Request
```

### SetCookies

SetCookies method sets multiple cookie fields and its values at one go in the request instance.
It will override the cookie which is set in the client instance.

```go title="Signature"
func (r *Request) SetCookies(m map[string]string) *Request
```

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

<details>
<summary>Click here to see the result</summary>

```json
{
  "cookies": {
    "test": "123"
  }
}
```

</details>

### SetCookiesWithStruct

SetCookiesWithStruct method sets multiple cookie fields and its values at one go in the request instance.
It will override the cookie which is set in the client instance.

```go title="Signature"
func (r *Request) SetCookiesWithStruct(v any) *Request
```

### DelCookies

DelCookies method deletes single or multiple cookie fields ant its values.

```go title="Signature"
func (r *Request) DelCookies(key ...string) *Request
```

## PathParam

PathParam returns the path param set in the request instance. If the path param doesn't exist, return empty string.

```go title="Signature"
func (r *Request) PathParam(key string) string
```

### SetPathParam

SetPathParam method sets a single path param field and its value in the request instance.
It will override path param which set in client instance.

```go title="Signature"
func (r *Request) SetPathParam(key, val string) *Request
```

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

<details>
<summary>Click here to see the result</summary>

```plaintext
Gofiber
```

</details>

### SetPathParams

SetPathParams method sets multiple path param fields and its values at one go in the request instance.
It will override path param which set in client instance.

```go title="Signature"
func (r *Request) SetPathParams(m map[string]string) *Request
```

### SetPathParamsWithStruct

SetPathParamsWithStruct method sets multiple path param fields and its values at one go in the request instance.
It will override path param which set in client instance.

```go title="Signature"
func (r *Request) SetPathParamsWithStruct(v any) *Request
```

### DelPathParams

DelPathParams method deletes single or multiple path param fields ant its values.

```go title="Signature"
func (r *Request) DelPathParams(key ...string) *Request
```

### ResetPathParams

ResetPathParams deletes all path params.

```go title="Signature"
func (r *Request) ResetPathParams() *Request
```

## SetJSON

SetJSON method sets JSON body in request.

```go title="Signature"
func (r *Request) SetJSON(v any) *Request
```

## SetXML

SetXML method sets XML body in request.

```go title="Signature"
func (r *Request) SetXML(v any) *Request
```

## SetCBOR

SetCBOR method sets the request body using CBOR (Concise Binary Object Representation) encoding format.
It automatically sets the Content-Type header to "application/cbor".

```go title="Signature"
func (r *Request) SetCBOR(v any) *Request
```

## SetRawBody

SetRawBody method sets body with raw data in request.

```go title="Signature"
func (r *Request) SetRawBody(v []byte) *Request
```

## FormData

FormData method returns form data value via key, this method will visit all field in the form data.

```go title="Signature"
func (r *Request) FormData(key string) []string
```

### AddFormData

AddFormData method adds a single form data field and its value in the request instance.

```go title="Signature"
func (r *Request) AddFormData(key, val string) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

SetFormData method sets a single form data field and its value in the request instance.

```go title="Signature"
func (r *Request) SetFormData(key, val string) *Request 
```

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

<details>
<summary>Click here to see the result</summary>

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

### AddFormDatas

AddFormDatas method adds multiple form data fields and its values in the request instance.

```go title="Signature"
func (r *Request) AddFormDatas(m map[string][]string) *Request
```

### SetFormDatas

SetFormDatas method sets multiple form data fields and its values in the request instance.

```go title="Signature"
func (r *Request) SetFormDatas(m map[string]string) *Request
```

### SetFormDatas

SetFormDatas method sets multiple form data fields and its values in the request instance.

```go title="Signature"
func (r *Request) SetFormDatas(m map[string]string) *Request
```

### SetFormDatasWithStruct

SetFormDatasWithStruct method sets multiple form data fields and its values in the request instance via struct.

```go title="Signature"
func (r *Request) SetFormDatasWithStruct(v any) *Request
```

### DelFormDatas

DelFormDatas method deletes multiple form data fields and its value in the request instance.

```go title="Signature"
func (r *Request) DelFormDatas(key ...string) *Request
```

## File

File returns file ptr store in request obj by name.
If the name field is empty, it will try to match path.

```go title="Signature"
func (r *Request) File(name string) *File
```

### FileByPath

FileByPath returns file ptr store in request obj by path.

```go title="Signature"
func (r *Request) FileByPath(path string) *File
```

### AddFile

AddFile method adds a single file field and its value in the request instance via file path.

```go title="Signature"
func (r *Request) AddFile(path string) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

AddFileWithReader method adds a single field and its value in the request instance via reader.

```go title="Signature"
func (r *Request) AddFileWithReader(name string, reader io.ReadCloser) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

AddFiles method adds multiple file fields and its value in the request instance via File instance.

```go title="Signature"
func (r *Request) AddFiles(files ...*File) *Request
```

## Timeout

Timeout returns the length of timeout in request.

```go title="Signature"
func (r *Request) Timeout() time.Duration
```

## SetTimeout

SetTimeout method sets the timeout field and its values at one go in the request instance.
It will override timeout which set in client instance.

```go title="Signature"
func (r *Request) SetTimeout(t time.Duration) *Request
```

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

<details>
<summary>Click here to see the result</summary>

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

<details>
<summary>Click here to see the result</summary>

```shell
panic: timeout or cancel

goroutine 1 [running]:
main.main()
        main.go:18 +0xeb
exit status 2
```

</details>

## MaxRedirects

MaxRedirects returns the max redirects count in the request.

```go title="Signature"
func (r *Request) MaxRedirects() int
```

## SetMaxRedirects

SetMaxRedirects method sets the maximum number of redirects at one go in the request instance.
It will override max redirect, which is set in the client instance.

```go title="Signature"
func (r *Request) SetMaxRedirects(count int) *Request
```

## Send

Send sends HTTP request.

```go title="Signature"
func (r *Request) Send() (*Response, error)
```

## Reset

Reset clears Request object, used by ReleaseRequest method.

```go title="Signature"
func (r *Request) Reset()
```

## Header

Header is a wrapper which wrap http.Header, the header in client and request will store in it.

```go
type Header struct {
    *fasthttp.RequestHeader
}
```

### PeekMultiple

PeekMultiple methods returns multiple field in header with same key.

```go title="Signature"
func (h *Header) PeekMultiple(key string) []string
```

### AddHeaders

AddHeaders receives a map and add each value to header.

```go title="Signature"
func (h *Header) AddHeaders(r map[string][]string)
```

### SetHeaders

SetHeaders will override all headers.

```go title="Signature"
func (h *Header) SetHeaders(r map[string]string)
```

## QueryParam

QueryParam is a wrapper which wrap url.Values, the query string and formdata in client and request will store in it.

```go
type QueryParam struct {
    *fasthttp.Args
}
```

### AddParams

AddParams receive a map and add each value to param.

```go title="Signature"
func (p *QueryParam) AddParams(r map[string][]string)
```

### SetParams

SetParams will override all params.

```go title="Signature"
func (p *QueryParam) SetParams(r map[string]string)
```

### SetParamsWithStruct

SetParamsWithStruct will override all params with struct or pointer of struct.
Nested structs are not currently supported.

```go title="Signature"
func (p *QueryParam) SetParamsWithStruct(v any)
```

## Cookie

Cookie is a map which to store the cookies.

```go
type Cookie map[string]string
```

### Add

Add method impl the method in WithStruct interface.

```go title="Signature"
func (c Cookie) Add(key, val string)
```

### Del

Del method impl the method in WithStruct interface.

```go title="Signature"
func (c Cookie) Del(key string)
```

### SetCookie

SetCookie method sets a single val in Cookie.

```go title="Signature"
func (c Cookie) SetCookie(key, val string)
```

### SetCookies

SetCookies method sets multiple val in Cookie.

```go title="Signature"
func (c Cookie) SetCookies(m map[string]string)
```

### SetCookiesWithStruct

SetCookiesWithStruct method sets multiple val in Cookie via a struct.

```go title="Signature"
func (c Cookie) SetCookiesWithStruct(v any)
```

### DelCookies

DelCookies method deletes multiple val in Cookie.

```go title="Signature"
func (c Cookie) DelCookies(key ...string)
```

### VisitAll

VisitAll method receive a function which can travel the all val.

```go title="Signature"
func (c Cookie) VisitAll(f func(key, val string))
```

### Reset

Reset clears the Cookie object.

```go title="Signature"
func (c Cookie) Reset()
```

## PathParam

PathParam is a map which to store path params.

```go
type PathParam map[string]string
```

### Add

Add method impl the method in WithStruct interface.

```go title="Signature"
func (p PathParam) Add(key, val string)
```

### Del

Del method impl the method in WithStruct interface.

```go title="Signature"
func (p PathParam) Del(key string)
```

### SetParam

SetParam method sets a single val in PathParam.

```go title="Signature"
func (p PathParam) SetParam(key, val string)
```

### SetParams

SetParams method sets multiple val in PathParam.

```go title="Signature"
func (p PathParam) SetParams(m map[string]string)
```

### SetParamsWithStruct

SetParamsWithStruct method sets multiple val in PathParam via a struct.

```go title="Signature"
func (p PathParam) SetParamsWithStruct(v any)
```

### DelParams

DelParams method deletes multiple val in PathParams.

```go title="Signature"
func (p PathParam) DelParams(key ...string)
```

### VisitAll

VisitAll method receive a function which can travel the all val.

```go title="Signature"
func (p PathParam) VisitAll(f func(key, val string))
```

### Reset

Reset clears the PathParam object.

```go title="Signature"
func (p PathParam) Reset()
```

## FormData

FormData is a wrapper of fasthttp.Args and it is used for url encode body and file body.

```go
type FormData struct {
    *fasthttp.Args
}
```

### AddData

AddData method is a wrapper of Args's Add method.

```go title="Signature"
func (f *FormData) AddData(key, val string)
```

### SetData

SetData method is a wrapper of Args's Set method.

```go title="Signature"
func (f *FormData) SetData(key, val string)
```

### AddDatas

AddDatas method supports add multiple fields.

```go title="Signature"
func (f *FormData) AddDatas(m map[string][]string)
```

### SetDatas

SetDatas method supports set multiple fields.

```go title="Signature"
func (f *FormData) SetDatas(m map[string]string)
```

### SetDatasWithStruct

SetDatasWithStruct method supports set multiple fields via a struct.

```go title="Signature"
func (f *FormData) SetDatasWithStruct(v any)
```

### DelDatas

DelDatas method deletes multiple fields.

```go title="Signature"
func (f *FormData) DelDatas(key ...string)
```

### Reset

Reset clear the FormData object.

```go title="Signature"
func (f *FormData) Reset()
```

## File

File is a struct which support send files via request.

```go
type File struct {
    name      string
    fieldName string
    path      string
    reader    io.ReadCloser
}
```

### AcquireFile

AcquireFile returns a File object from the pool.
And you can set field in the File with SetFileFunc.

The returned file may be returned to the pool with ReleaseFile when no longer needed.
This allows reducing GC load.

```go title="Signature"
func AcquireFile(setter ...SetFileFunc) *File
```

### ReleaseFile

ReleaseFile returns the object acquired via AcquireFile to the pool.
Do not access the released File object, otherwise data races may occur.

```go title="Signature"
func ReleaseFile(f *File)
```

### SetName

SetName method sets file name.

```go title="Signature"
func (f *File) SetName(n string)
```

### SetFieldName

SetFieldName method sets key of file in the body.

```go title="Signature"
func (f *File) SetFieldName(n string)
```

### SetPath

SetPath method set file path.

```go title="Signature"
func (f *File) SetPath(p string)
```

### SetReader

SetReader method can receive an io.ReadCloser which will be closed in parserBody hook.

```go title="Signature"
func (f *File) SetReader(r io.ReadCloser)
```

### Reset

Reset clear the File object.

```go title="Signature"
func (f *File) Reset()
```
