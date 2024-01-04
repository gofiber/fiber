---
id: client
title: 🌎 Client
description: The Client struct represents the Fiber HTTP Client.
sidebar_position: 5
---

## Start request

Start a http request with http method and url.

```go title="Signatures"
// Client http methods
func (c *Client) Get(url string) *Agent
func (c *Client) Head(url string) *Agent
func (c *Client) Post(url string) *Agent
func (c *Client) Put(url string) *Agent
func (c *Client) Patch(url string) *Agent
func (c *Client) Delete(url string) *Agent
```

Here we present a brief example demonstrating the simulation of a proxy using our `*fiber.Agent` methods.
```go
// Get something
func getSomething(c *fiber.Ctx) (err error) {
	agent := fiber.Get("<URL>")
	statusCode, body, errs := agent.Bytes()
	if len(errs) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errs": errs,
		})
	}

	var something fiber.Map
	err = json.Unmarshal(body, &something)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"err": err,
		})
	}

	return c.Status(statusCode).JSON(something)
}

// Post something
func createSomething(c *fiber.Ctx) (err error) {
	agent := fiber.Post("<URL>")
	agent.Body(c.Body()) // set body received by request
	statusCode, body, errs := agent.Bytes()
	if len(errs) > 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"errs": errs,
		})
	}

    // pass status code and body received by the proxy
	return c.Status(statusCode).Send(body)
}
```
Based on this short example, we can perceive that using the `*fiber.Client` is very straightforward and intuitive.


## ✨ Agent
`Agent` is built on top of FastHTTP's [`HostClient`](https://github.com/valyala/fasthttp/blob/master/client.go#L603) which has lots of convenient helper methods such as dedicated methods for request methods.

### Parse

Parse initializes a HostClient.

```go title="Parse"
a := AcquireAgent()
req := a.Request()
req.Header.SetMethod(MethodGet)
req.SetRequestURI("http://example.com")

if err := a.Parse(); err != nil {
    panic(err)
}

code, body, errs := a.Bytes() // ...
```

### Set

Set sets the given `key: value` header.

```go title="Signature"
func (a *Agent) Set(k, v string) *Agent
func (a *Agent) SetBytesK(k []byte, v string) *Agent
func (a *Agent) SetBytesV(k string, v []byte) *Agent
func (a *Agent) SetBytesKV(k []byte, v []byte) *Agent
```

```go title="Example"
agent.Set("k1", "v1").
    SetBytesK([]byte("k1"), "v1").
    SetBytesV("k1", []byte("v1")).
    SetBytesKV([]byte("k2"), []byte("v2"))
// ...
```

### Add

Add adds the given `key: value` header. Multiple headers with the same key may be added with this function.

```go title="Signature"
func (a *Agent) Add(k, v string) *Agent
func (a *Agent) AddBytesK(k []byte, v string) *Agent
func (a *Agent) AddBytesV(k string, v []byte) *Agent
func (a *Agent) AddBytesKV(k []byte, v []byte) *Agent
```

```go title="Example"
agent.Add("k1", "v1").
    AddBytesK([]byte("k1"), "v1").
    AddBytesV("k1", []byte("v1")).
    AddBytesKV([]byte("k2"), []byte("v2"))
// Headers:
// K1: v1
// K1: v1
// K1: v1
// K2: v2
```

### ConnectionClose

ConnectionClose adds the `Connection: close` header.

```go title="Signature"
func (a *Agent) ConnectionClose() *Agent
```

```go title="Example"
agent.ConnectionClose()
// ...
```

### UserAgent

UserAgent sets `User-Agent` header value.

```go title="Signature"
func (a *Agent) UserAgent(userAgent string) *Agent
func (a *Agent) UserAgentBytes(userAgent []byte) *Agent
```

```go title="Example"
agent.UserAgent("fiber")
// ...
```

### Cookie

Cookie sets a cookie in `key: value` form. `Cookies` can be used to set multiple cookies.

```go title="Signature"
func (a *Agent) Cookie(key, value string) *Agent
func (a *Agent) CookieBytesK(key []byte, value string) *Agent
func (a *Agent) CookieBytesKV(key, value []byte) *Agent
func (a *Agent) Cookies(kv ...string) *Agent
func (a *Agent) CookiesBytesKV(kv ...[]byte) *Agent
```

```go title="Example"
agent.Cookie("k", "v")
agent.Cookies("k1", "v1", "k2", "v2")
// ...
```

### Referer

Referer sets the Referer header value.

```go title="Signature"
func (a *Agent) Referer(referer string) *Agent
func (a *Agent) RefererBytes(referer []byte) *Agent
```

```go title="Example"
agent.Referer("https://docs.gofiber.io")
// ...
```

### ContentType

ContentType sets Content-Type header value.

```go title="Signature"
func (a *Agent) ContentType(contentType string) *Agent
func (a *Agent) ContentTypeBytes(contentType []byte) *Agent
```

```go title="Example"
agent.ContentType("custom-type")
// ...
```

### Host

Host sets the Host header.

```go title="Signature"
func (a *Agent) Host(host string) *Agent
func (a *Agent) HostBytes(host []byte) *Agent
```

```go title="Example"
agent.Host("example.com")
// ...
```

### QueryString

QueryString sets the URI query string.

```go title="Signature"
func (a *Agent) QueryString(queryString string) *Agent
func (a *Agent) QueryStringBytes(queryString []byte) *Agent
```

```go title="Example"
agent.QueryString("foo=bar")
// ...
```

### BasicAuth

BasicAuth sets the URI username and password using HTTP Basic Auth.

```go title="Signature"
func (a *Agent) BasicAuth(username, password string) *Agent
func (a *Agent) BasicAuthBytes(username, password []byte) *Agent
```

```go title="Example"
agent.BasicAuth("foo", "bar")
// ...
```

### Body

There are several ways to set request body.

```go title="Signature"
func (a *Agent) BodyString(bodyString string) *Agent
func (a *Agent) Body(body []byte) *Agent

// BodyStream sets request body stream and, optionally body size.
//
// If bodySize is >= 0, then the bodyStream must provide exactly bodySize bytes
// before returning io.EOF.
//
// If bodySize < 0, then bodyStream is read until io.EOF.
//
// bodyStream.Close() is called after finishing reading all body data
// if it implements io.Closer.
//
// Note that GET and HEAD requests cannot have body.
func (a *Agent) BodyStream(bodyStream io.Reader, bodySize int) *Agent
```

```go title="Example"
agent.BodyString("foo=bar")
agent.Body([]byte("bar=baz"))
agent.BodyStream(strings.NewReader("body=stream"), -1)
// ...
```

### JSON

JSON sends a JSON request by setting the Content-Type header to the `ctype` parameter. If no `ctype` is passed in, the header is set to `application/json`.

```go title="Signature"
func (a *Agent) JSON(v interface{}, ctype ...string) *Agent
```

```go title="Example"
agent.JSON(fiber.Map{"success": true})
// ...
```

### XML

XML sends an XML request by setting the Content-Type header to `application/xml`.

```go title="Signature"
func (a *Agent) XML(v interface{}) *Agent
```

```go title="Example"
agent.XML(fiber.Map{"success": true})
// ...
```

### Form

Form sends a form request by setting the Content-Type header to `application/x-www-form-urlencoded`.

```go title="Signature"
// Form sends form request with body if args is non-nil.
//
// It is recommended obtaining args via AcquireArgs and release it
// manually in performance-critical code.
func (a *Agent) Form(args *Args) *Agent
```

```go title="Example"
args := AcquireArgs()
args.Set("foo", "bar")

agent.Form(args)
// ...
ReleaseArgs(args)
```

### MultipartForm

MultipartForm sends multipart form request by setting the Content-Type header to `multipart/form-data`. These requests can include key-value's and files.

```go title="Signature"
// MultipartForm sends multipart form request with k-v and files.
//
// It is recommended to obtain args via AcquireArgs and release it
// manually in performance-critical code.
func (a *Agent) MultipartForm(args *Args) *Agent
```

```go title="Example"
args := AcquireArgs()
args.Set("foo", "bar")

agent.MultipartForm(args)
// ...
ReleaseArgs(args)
```

Fiber provides several methods for sending files. Note that they must be called before `MultipartForm`.

#### Boundary

Boundary sets boundary for multipart form request.

```go title="Signature"
func (a *Agent) Boundary(boundary string) *Agent
```

```go title="Example"
agent.Boundary("myBoundary")
    .MultipartForm(nil)
// ...
```

#### SendFile\(s\)

SendFile read a file and appends it to a multipart form request. Sendfiles can be used to append multiple files.

```go title="Signature"
func (a *Agent) SendFile(filename string, fieldname ...string) *Agent
func (a *Agent) SendFiles(filenamesAndFieldnames ...string) *Agent
```

```go title="Example"
agent.SendFile("f", "field name")
    .SendFiles("f1", "field name1", "f2").
    .MultipartForm(nil)
// ...
```

#### FileData

FileData appends file data for multipart form request.

```go
// FormFile represents multipart form file
type FormFile struct {
    // Fieldname is form file's field name
    Fieldname string
    // Name is form file's name
    Name string
    // Content is form file's content
    Content []byte
}
```

```go title="Signature"
// FileData appends files for multipart form request.
//
// It is recommended obtaining formFile via AcquireFormFile and release it
// manually in performance-critical code.
func (a *Agent) FileData(formFiles ...*FormFile) *Agent
```

```go title="Example"
ff1 := &FormFile{"filename1", "field name1", []byte("content")}
ff2 := &FormFile{"filename2", "field name2", []byte("content")}
agent.FileData(ff1, ff2).
    MultipartForm(nil)
// ...
```

### Debug

Debug mode enables logging request and response detail to `io.writer`\(default is `os.Stdout`\).

```go title="Signature"
func (a *Agent) Debug(w ...io.Writer) *Agent
```

```go title="Example"
agent.Debug()
// ...
```

### Timeout

Timeout sets request timeout duration.

```go title="Signature"
func (a *Agent) Timeout(timeout time.Duration) *Agent
```

```go title="Example"
agent.Timeout(time.Second)
// ...
```

### Reuse

Reuse enables the Agent instance to be used again after one request. If agent is reusable, then it should be released manually when it is no longer used.

```go title="Signature"
func (a *Agent) Reuse() *Agent
```

```go title="Example"
agent.Reuse()
// ...
```

### InsecureSkipVerify

InsecureSkipVerify controls whether the Agent verifies the server certificate chain and host name.

```go title="Signature"
func (a *Agent) InsecureSkipVerify() *Agent
```

```go title="Example"
agent.InsecureSkipVerify()
// ...
```

### TLSConfig

TLSConfig sets tls config.

```go title="Signature"
func (a *Agent) TLSConfig(config *tls.Config) *Agent
```

```go title="Example"
// Create tls certificate
cer, _ := tls.LoadX509KeyPair("pem", "key")

config := &tls.Config{
    Certificates: []tls.Certificate{cer},
}

agent.TLSConfig(config)
// ...
```

### MaxRedirectsCount

MaxRedirectsCount sets max redirect count for GET and HEAD.

```go title="Signature"
func (a *Agent) MaxRedirectsCount(count int) *Agent
```

```go title="Example"
agent.MaxRedirectsCount(7)
// ...
```

### JSONEncoder

JSONEncoder sets custom json encoder.

```go title="Signature"
func (a *Agent) JSONEncoder(jsonEncoder utils.JSONMarshal) *Agent
```

```go title="Example"
agent.JSONEncoder(json.Marshal)
// ...
```

### JSONDecoder

JSONDecoder sets custom json decoder.

```go title="Signature"
func (a *Agent) JSONDecoder(jsonDecoder utils.JSONUnmarshal) *Agent
```

```go title="Example"
agent.JSONDecoder(json.Unmarshal)
// ...
```

### Request

Request returns Agent request instance.

```go title="Signature"
func (a *Agent) Request() *Request
```

```go title="Example"
req := agent.Request()
// ...
```

### SetResponse

SetResponse sets custom response for the Agent instance. It is recommended obtaining custom response via AcquireResponse and release it manually in performance-critical code.

```go title="Signature"
func (a *Agent) SetResponse(customResp *Response) *Agent
```

```go title="Example"
resp := AcquireResponse()
agent.SetResponse(resp)
// ...
ReleaseResponse(resp)
```

<details><summary>Example handling for response values</summary>

```go title="Example handling response"
// Create a Fiber HTTP client agent
agent := fiber.Get("https://httpbin.org/get")

// Acquire a response object to store the result
resp := fiber.AcquireResponse()
agent.SetResponse(resp)

// Perform the HTTP GET request
code, body, errs := agent.String()
if errs != nil {
    // Handle any errors that occur during the request
    panic(errs)
}

// Print the HTTP response code and body
fmt.Println("Response Code:", code)
fmt.Println("Response Body:", body)

// Visit and print all the headers in the response
resp.Header.VisitAll(func(key, value []byte) {
    fmt.Println("Header", string(key), "value", string(value))
})

// Release the response to free up resources
fiber.ReleaseResponse(resp)
```

Output:
```txt title="Output"
Response Code: 200
Response Body: {
  "args": {}, 
  "headers": {
    "Host": "httpbin.org", 
    "User-Agent": "fiber", 
    "X-Amzn-Trace-Id": "Root=1-653763d0-2555d5ba3838f1e9092f9f72"
  }, 
  "origin": "83.137.191.1", 
  "url": "https://httpbin.org/get"
}

Header Content-Length value 226
Header Content-Type value application/json
Header Server value gunicorn/19.9.0
Header Date value Tue, 24 Oct 2023 06:27:28 GMT
Header Connection value keep-alive
Header Access-Control-Allow-Origin value *
Header Access-Control-Allow-Credentials value true
```

</details>

### Dest

Dest sets custom dest. The contents of dest will be replaced by the response body, if the dest is too small a new slice will be allocated.

```go title="Signature"
func (a *Agent) Dest(dest []byte) *Agent {
```

```go title="Example"
agent.Dest(nil)
// ...
```

### Bytes

Bytes returns the status code, bytes body and errors of url.

```go title="Signature"
func (a *Agent) Bytes() (code int, body []byte, errs []error)
```

```go title="Example"
code, body, errs := agent.Bytes()
// ...
```

### String

String returns the status code, string body and errors of url.

```go title="Signature"
func (a *Agent) String() (int, string, []error)
```

```go title="Example"
code, body, errs := agent.String()
// ...
```

### Struct

Struct returns the status code, bytes body and errors of url. And bytes body will be unmarshalled to given v.

```go title="Signature"
func (a *Agent) Struct(v interface{}) (code int, body []byte, errs []error)
```

```go title="Example"
var d data
code, body, errs := agent.Struct(&d)
// ...
```

### RetryIf

RetryIf controls whether a retry should be attempted after an error.
By default, will use isIdempotent function from fasthttp

```go title="Signature"
func (a *Agent) RetryIf(retryIf RetryIfFunc) *Agent
```

```go title="Example"
agent.Get("https://example.com").RetryIf(func (req *fiber.Request) bool {
    return req.URI() == "https://example.com"
})
// ...
```
