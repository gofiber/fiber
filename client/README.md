<h1 align="center">Fiber Client</h1>
<p align="center">Easy-to-use HTTP client based on fasthttp (inspired by <a href="https://github.com/go-resty/resty">resty</a> and <a href="https://github.com/axios/axios">axios</a>)</p>
<p align="center"><a href="#features">Features</a> section describes Fiber Client capabilities in detail</p>

## Features

- GET, POST, PUT, DELETE, HEAD, PATCH, OPTIONS, QUERY, and custom methods
- Simple and chainable methods for settings and request
- The request body is set with the method that matches its format, and the `Content-Type` follows from it
  - `SetJSON`, `SetXML` and `SetCBOR` marshal a value, `SetFormData` sends URL-encoded form fields
  - `AddFile`, `AddFileWithReader` and `AddFiles` send a multipart body
  - `SetRawBody` sends bytes as given and sets no `Content-Type` of its own
  - Native `*fasthttp.Request` instance can be accessed in request hooks via `Request.RawRequest`
- Response object gives you more possibility
  - Access as `[]byte` by `response.Body()` or access as `string` by `response.String()`
  - Unmarshal the body with `response.JSON(&v)`, `response.XML(&v)` or `response.CBOR(&v)`
  - Write the body to a file or an `io.Writer` with `response.Save()`
- Provide an option to override the JSON, XML and CBOR marshal and unmarshal functions with `SetJSONMarshal`, `SetJSONUnmarshal`, `SetXMLMarshal`, `SetXMLUnmarshal`, `SetCBORMarshal` and `SetCBORUnmarshal`
- Request and response hooks, a cookie jar, retries and timeouts

## Usage

The following samples will assist you to become as comfortable as possible with `Fiber Client` library.

```go
// Import Fiber Client into your code and refer it as `client`.
import "github.com/gofiber/fiber/v3/client"
```

### Simple GET

```go
cc := client.New()
cc.SetTimeout(10 * time.Second)

resp, err := cc.Get("https://httpbin.org/get")
if err != nil {
    panic(err)
}
defer resp.Close()

fmt.Printf("Status: %d\n", resp.StatusCode())
fmt.Printf("Body: %s\n", resp.String())
```

See the [client documentation](../docs/client/rest.md) for the full API.
