<h1 align="center">Fiber Client</h1>
<p align="center">Easy-to-use HTTP client based on fasthttp (inspired by <a href="https://github.com/go-resty/resty">resty</a> and <a href="https://github.com/axios/axios">axios</a>)</p>
<p align="center"><a href="#features">Features</a> section describes in detail about Resty capabilities</p>

## Features

> The characteristics have not yet been written.

- GET, POST, PUT, DELETE, HEAD, PATCH, OPTIONS, etc.
- Simple and chainable methods for settings and request
- Request Body can be `string`, `[]byte`, `map`, `slice`
  - Auto detects `Content-Type`
  - Buffer processing for `files`
  - Native `*fasthttp.Request` instance can be accessed during middleware and request execution via `Request.RawRequest`
  - Request Body can be read multiple time via `Request.RawRequest.GetBody()`
- Response object gives you more possibility
  - Access as `[]byte` by `response.Body()` or access as `string` by `response.String()`
- Automatic marshal and unmarshal for JSON and XML content type
  - Default is JSON, if you supply struct/map without header Content-Type
  - For auto-unmarshal, refer to -
    - Success scenario Request.SetResult() and Response.Result().
    - Error scenario Request.SetError() and Response.Error().
    - Supports RFC7807 - application/problem+json & application/problem+xml
  - Provide an option to override JSON Marshal/Unmarshal and XML Marshal/Unmarshal

## Usage

The following samples will assist you to become as comfortable as possible with `Fiber Client` library.

```go
// Import Fiber Client into your code and refer it as `client`.
import "github.com/gofiber/fiber/client"
```

### Simple GET
