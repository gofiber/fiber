---
id: response
title: 📥 Response
description: >-
  Response methods of Gofiber HTTP client.
sidebar_position: 3
---

The `Response` structure in Gofiber's HTTP client represents the server's response to an HTTP request. It contains all the necessary information received from the server. This includes:

- **Status Code**: The HTTP status code returned by the server (e.g., 200 OK, 404 Not Found).
- **Headers**: HTTP headers received from the server that provide additional information about the response.
- **Body**: The data received from the server, typically in the form of a JSON, XML, or plain text format.
- **Cookies**: Any cookies sent by the server along with the response.

This structure allows users to easily access and manage the data returned by the server, facilitating efficient handling of HTTP responses.

```go
type Response struct {
    client  *Client
    request *Request
    cookie  []*fasthttp.Cookie

    RawResponse *fasthttp.Response
}
```

## AcquireResponse

AcquireResponse returns an empty response object from the pool.
The returned response may be returned to the pool with ReleaseResponse when no longer needed.
This allows reducing GC load.

```go title="Signature"
func AcquireResponse() *Response
```

## ReleaseResponse

ReleaseResponse returns the object acquired via AcquireResponse to the pool.
Do not access the released Response object; otherwise, data races may occur.

```go title="Signature"
func ReleaseResponse(resp *Response)
```

## Status

Status method returns the HTTP status string for the executed request.

```go title="Signature"
func (r *Response) Status() string
```

## StatusCode

StatusCode method returns the HTTP status code for the executed request.

```go title="Signature"
func (r *Response) StatusCode() int
```

## Protocol

Protocol method returns the HTTP response protocol used for the request.

```go title="Signature"
func (r *Response) Protocol() string
```

```go title="Example"
resp, err := client.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}

fmt.Println(resp.Protocol())
```

<details>
<summary>Click here to see the result</summary>

```text
HTTP/1.1
```

</details>

## Header

Header method returns the response headers.

```go title="Signature"
func (r *Response) Header(key string) string
```

## Headers

Headers returns all headers in the response using an iterator. You can use `maps.Collect()` to collect all headers into a map.
The returned value is valid until the response object is released. Any future calls to Headers method will return the modified value. Do not store references to returned value. Make copies instead.

```go title="Signature"
func (r *Response) Headers() iter.Seq2[string, []string] 
```

```go title="Example"
resp, err := client.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}

for key, values := range resp.Headers() {
    fmt.Printf("%s => %s\n", key, strings.Join(values, ", "))
}
```

<details>

<summary>Click here to see the result</summary>

```text
Date => Wed, 04 Dec 2024 15:28:29 GMT
Connection => keep-alive
Access-Control-Allow-Origin => *
Access-Control-Allow-Credentials => true
```

</details>

```go title="Example with maps.Collect()"
resp, err := client.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}

headers := maps.Collect(resp.Headers()) // Collect all headers into a map
for key, values := range headers {
    fmt.Printf("%s => %s\n", key, strings.Join(values, ", "))
}
```

<details>

<summary>Click here to see the result</summary>

```text
Date => Wed, 04 Dec 2024 15:28:29 GMT
Connection => keep-alive
Access-Control-Allow-Origin => *
Access-Control-Allow-Credentials => true
```

</details>

## Cookies

Cookies method to access all the response cookies.
The returned value is valid until the response object is released. Any future calls to Cookies method will return the modified value. Do not store references to returned value. Make copies instead.

```go title="Signature"
func (r *Response) Cookies() []*fasthttp.Cookie
```

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

<details>
<summary>Click here to see the result</summary>

```text
go => fiber
```

</details>

## Body

Body method returns HTTP response as []byte array for the executed request.

```go title="Signature"
func (r *Response) Body() []byte
```

## String

String method returns the body of the server response as String.

```go title="Signature"
func (r *Response) String() string
```

## JSON

JSON method will unmarshal body to json.

```go title="Signature"
func (r *Response) JSON(v any) error
```

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

err = resp.JSON(&out)
if err != nil {
    panic(err)
}

fmt.Printf("%+v\n", out)
```

<details>
<summary>Click here to see the result</summary>

```text
{Slideshow:{Author:Yours Truly Date:date of publication Title:Sample Slide Show}}
```

</details>

## XML

XML method will unmarshal body to xml.

```go title="Signature"
func (r *Response) XML(v any) error
```

## CBOR

CBOR method will unmarshal body to CBOR.

```go title="Signature"
func (r *Response) CBOR(v any) error
```

## Save

Save method will save the body to a file or io.Writer.

```go title="Signature"
func (r *Response) Save(v any) error
```

## Reset

Reset clears the Response object.

```go title="Signature"
func (r *Response) Reset() 
```

## Close

Close method will release the Request and Response objects; after calling Close, please do not use these objects.

```go title="Signature"
func (r *Response) Close()
```
