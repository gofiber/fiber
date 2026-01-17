---
id: response
title: 📥 Response
description: >-
  Response methods of Gofiber HTTP client.
sidebar_position: 3
---

The `Response` struct in Fiber's HTTP client represents the server's reply and exposes:

- **Status Code**: The HTTP status code returned by the server (e.g., `200 OK`, `404 Not Found`).
- **Headers**: All HTTP headers returned by the server, providing additional response-related information.
- **Body**: The response body content, which can be JSON, XML, plain text, or other formats.
- **Cookies**: Any cookies the server sent along with the response.

It makes it easy to inspect and handle data returned by the server.

```go
type Response struct {
    client      *Client
    request     *Request
    cookie      []*fasthttp.Cookie
    RawResponse *fasthttp.Response
}
```

## AcquireResponse

**AcquireResponse** returns a new pooled `Response`. Call `ReleaseResponse` when you're done to return it to the pool and limit allocations.

```go title="Signature"
func AcquireResponse() *Response
```

## ReleaseResponse

**ReleaseResponse** puts the `Response` back into the pool. Do not use it after releasing; doing so can trigger data races.

```go title="Signature"
func ReleaseResponse(resp *Response)
```

## Status

**Status** returns the HTTP status message (e.g., `OK`, `Not Found`) associated with the response.

```go title="Signature"
func (r *Response) Status() string
```

## StatusCode

**StatusCode** returns the numeric HTTP status code of the response.

```go title="Signature"
func (r *Response) StatusCode() int
```

## Protocol

**Protocol** returns the HTTP protocol used (e.g., `HTTP/1.1`, `HTTP/2`) for the response.

```go title="Signature"
func (r *Response) Protocol() string
```

<details>
<summary>Example</summary>

```go title="Example"
resp, err := client.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}

fmt.Println(resp.Protocol())
```

**Output:**

```text
HTTP/1.1
```

</details>

## Header

**Header** retrieves the value of a specific response header by key. If multiple values exist for the same header, this returns the first one.

```go title="Signature"
func (r *Response) Header(key string) string
```

## Headers

**Headers** returns an iterator over all response headers. Use `maps.Collect()` to convert them into a map if desired. The returned values are only valid until the response is released, so make copies if needed.

```go title="Signature"
func (r *Response) Headers() iter.Seq2[string, []string] 
```

<details>
<summary>Example</summary>

```go title="Example"
resp, err := client.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}

for key, values := range resp.Headers() {
    fmt.Printf("%s => %s\n", key, strings.Join(values, ", "))
}
```

**Output:**

```text
Date => Wed, 04 Dec 2024 15:28:29 GMT
Connection => keep-alive
Access-Control-Allow-Origin => *
Access-Control-Allow-Credentials => true
```

</details>

<details>
<summary>Example with maps.Collect()</summary>

```go title="Example with maps.Collect()"
resp, err := client.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}

headers := maps.Collect(resp.Headers())
for key, values := range headers {
    fmt.Printf("%s => %s\n", key, strings.Join(values, ", "))
}
```

**Output:**

```text
Date => Wed, 04 Dec 2024 15:28:29 GMT
Connection => keep-alive
Access-Control-Allow-Origin => *
Access-Control-Allow-Credentials => true
```

</details>

## Cookies

**Cookies** returns a slice of all cookies set by the server in this response. The slice is only valid until the response is released.

```go title="Signature"
func (r *Response) Cookies() []*fasthttp.Cookie
```

<details>
<summary>Example</summary>

```go title="Example"
resp, err := client.Get("https://httpbin.org/cookies/set/go/fiber")
if err != nil {
    panic(err)
}

cookies := resp.Cookies()
for _, cookie := range cookies {
    fmt.Printf("%s => %s\n", string(cookie.Key()), string(cookie.Value()))
}
```

**Output:**

```text
go => fiber
```

</details>

## Body

**Body** returns the raw response body as a byte slice.

```go title="Signature"
func (r *Response) Body() []byte
```

## BodyStream

**BodyStream** returns the response body as an `io.Reader`, allowing incremental reading without loading the entire body into memory. This is particularly useful when `Client.SetStreamResponseBody(true)` is enabled.

When streaming is enabled, the underlying stream from fasthttp is returned directly. When streaming is not enabled, a `bytes.Reader` wrapping the body is returned as a fallback.

:::note
When using `BodyStream()`, the response body is consumed as you read. Calling `Body()` afterward may return an empty slice if the stream has been fully read.
:::

```go title="Signature"
func (r *Response) BodyStream() io.Reader
```

<details>
<summary>Example</summary>

```go title="Example"
cc := client.New()
cc.SetStreamResponseBody(true)

resp, err := cc.Get("https://httpbin.org/bytes/1024")
if err != nil {
    panic(err)
}
defer resp.Close()

reader := resp.BodyStream()
buf := make([]byte, 256)
var total int

for {
    n, err := reader.Read(buf)
    total += n
    if err == io.EOF {
        break
    }
    if err != nil {
        panic(err)
    }
}

fmt.Printf("Read %d bytes\n", total)
```

**Output:**

```text
Read 1024 bytes
```

</details>

## IsStreaming

**IsStreaming** returns `true` if the response body is being streamed (i.e., when `Client.SetStreamResponseBody(true)` was set and the underlying transport provided a stream).

```go title="Signature"
func (r *Response) IsStreaming() bool
```

<details>
<summary>Example</summary>

```go title="Example"
cc := client.New()
cc.SetStreamResponseBody(true)

resp, err := cc.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}
defer resp.Close()

if resp.IsStreaming() {
    fmt.Println("Response is streaming")
    // Use resp.BodyStream() to read incrementally
} else {
    fmt.Println("Response is buffered")
    // Use resp.Body() for direct access
}
```

</details>

## String

**String** returns the response body as a trimmed string.

```go title="Signature"
func (r *Response) String() string
```

## JSON

**JSON** unmarshal the response body into the provided variable `v` using JSON. `v` should be a pointer to a struct or a type compatible with JSON unmarshal.

```go title="Signature"
func (r *Response) JSON(v any) error
```

<details>
<summary>Example</summary>

```go title="Example"
type Body struct {
    Slideshow struct {
        Author string `json:"author"`
        Date   string `json:"date"`
        Title  string `json:"title"`
    } `json:"slideshow"`
}
var out Body

resp, err := client.Get("https://httpbin.org/json")
if err != nil {
    panic(err)
}

if err = resp.JSON(&out); err != nil {
    panic(err)
}

fmt.Printf("%+v\n", out)
```

**Output:**

```text
{Slideshow:{Author:Yours Truly Date:date of publication Title:Sample Slide Show}}
```

</details>

## XML

**XML** unmarshal the response body into the provided variable `v` using XML decoding.

```go title="Signature"
func (r *Response) XML(v any) error
```

## CBOR

**CBOR** unmarshal the response body into `v` using CBOR decoding.

```go title="Signature"
func (r *Response) CBOR(v any) error
```

## Save

**Save** writes the response body to a file or an `io.Writer`. If `v` is a string, it interprets it as a file path, creates the file (and directories if needed), and writes the response to it. If `v` is an `io.Writer`, it writes directly to it.

```go title="Signature"
func (r *Response) Save(v any) error
```

## Reset

**Reset** clears the `Response` object, making it ready for reuse by `ReleaseResponse`.

```go title="Signature"
func (r *Response) Reset()
```

## Close

**Close** releases both the associated `Request` and `Response` objects back to their pools.

:::warning
After calling `Close`, any attempt to use the request or response may result in data races or undefined behavior. Ensure all processing is complete before closing.
:::

```go title="Signature"
func (r *Response) Close()
```
