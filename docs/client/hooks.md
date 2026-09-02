---
id: hooks
title: 🎣 Hooks
description: >-
  Hooks are used to manipulate the request/response process of the Fiber client.
sidebar_position: 4
---

Hooks let you intercept and modify the request or response flow of the Fiber client. They are useful for:

- Changing request parameters (e.g., URL, headers) before sending the request.
- Logging request and response details.
- Integrating complex tracing or monitoring tools.
- Handling authentication, retries, or other custom logic.

There are two kinds of hooks:

## Request Hooks

**Request hooks** are functions executed before the HTTP request is sent. They follow the signature:

```go
type RequestHook func(*Client, *Request) error
```

A request hook receives both the `Client` and the `Request` objects, allowing you to modify the request before it leaves your application. For example, you could:

- Change the host URL.
- Log request details (method, URL, headers).
- Add or modify headers or query parameters.
- Intercept and apply custom authentication logic.

**Example:**

```go
type Repository struct {
    Name        string `json:"name"`
    FullName    string `json:"full_name"`
    Description string `json:"description"`
    Homepage    string `json:"homepage"`

    Owner struct {
        Login string `json:"login"`
    } `json:"owner"`
}

func main() {
    cc := client.New()

    // Add a request hook that modifies the request URL before sending.
    cc.AddRequestHook(func(c *client.Client, r *client.Request) error {
        r.SetURL("https://api.github.com/" + r.URL())
        return nil
    })

    resp, err := cc.Get("repos/gofiber/fiber")
    if err != nil {
        panic(err)
    }

    var repo Repository
    if err := resp.JSON(&repo); err != nil {
        panic(err)
    }

    fmt.Printf("Status code: %d\n", resp.StatusCode())
    fmt.Printf("Repository: %s\n", repo.FullName)
    fmt.Printf("Description: %s\n", repo.Description)
    fmt.Printf("Homepage: %s\n", repo.Homepage)
    fmt.Printf("Owner: %s\n", repo.Owner.Login)
    fmt.Printf("Name: %s\n", repo.Name)
    fmt.Printf("Full Name: %s\n", repo.FullName)
}
```

<details>
<summary>Click here to see the result</summary>

```plaintext
Status code: 200
Repository: gofiber/fiber
Description: ⚡️ Express inspired web framework written in Go
Homepage: https://gofiber.io
Owner: gofiber
Name: fiber
Full Name: gofiber/fiber
```

</details>

### Built-in Request Hooks

Fiber includes built-in request hooks:

- **parserRequestURL**: Normalizes and customizes the URL based on path and query parameters. Required for `PathParam` and `QueryParam` methods.
- **parserRequestHeader**: Sets request headers, cookies, content type, referer, and user agent based on client and request properties. Client headers are applied first and request headers with the same key replace them.
- **parserRequestBody**: Automatically serializes the request body (JSON, XML, form, file uploads, etc.).

:::info
If a request hook returns an error, Fiber stops the request and returns the error immediately.
:::

**Example with Multiple Hooks:**

```go
func main() {
    cc := client.New()

    cc.AddRequestHook(func(c *client.Client, r *client.Request) error {
        fmt.Println("Hook 1")
        return errors.New("error")
    })

    cc.AddRequestHook(func(c *client.Client, r *client.Request) error {
        fmt.Println("Hook 2")
        return nil
    })

    _, err := cc.Get("https://example.com/")
    if err != nil {
        panic(err)
    }
}
```

<details>
<summary>Click here to see the result</summary>

```shell
Hook 1.
panic: error

goroutine 1 [running]:
main.main()
        main.go:25 +0xaa
exit status 2
```

</details>

### Final Request Hooks

Hooks added with `AddRequestHook` run before Fiber's built-in hooks, so they can
configure high-level request fields such as URL parameters, headers, and body.
Hooks added with `AddFinalRequestHook` run after the built-in hooks and
immediately before the request is sent, when `RawRequest` holds the resolved
URL, the merged headers and cookies, and the serialized body. This is the place
for request signing:

```go
cc.AddFinalRequestHook(func(_ *client.Client, req *client.Request) error {
    raw := req.RawRequest
    if raw.IsBodyStream() {
        return errors.New("cannot sign a streamed body")
    }

    signature := sign(raw.URI().RequestURI(), raw.URI().Host(), raw.Body())
    raw.Header.Set("X-Signature", signature)
    return nil
})
```

:::caution
fasthttp writes the request line, `Host` and `Content-Length` when it sends the
request, which is after this hook. Take the target and the host from
`RawRequest.URI()`: `Header.Header()` still carries the absolute URL without its
query string, and `Header.Host()` is empty.

The hook runs once per call, not once per attempt: retries resend the request it
signed, and a redirect is followed below it, carrying a signature bound to the
previous URL. Sign with a nonce or a timestamp only when retries are off.

Hence the `IsBodyStream` guard above. Reading `RawRequest.Body()` drains a body
stream into memory, and a stream that fails mid-read leaves fasthttp's error
text in the body: that text is then sent as the payload, signed as if it were
genuine, while `Send` reports no error. Buffer such a body before attaching it.
:::

## Response Hooks

**Response hooks** are functions executed after the HTTP response is received. They follow the signature:

```go
type ResponseHook func(*Client, *Response, *Request) error
```

A response hook receives the `Client`, `Response`, and `Request` objects, allowing you to inspect and modify the response or perform additional actions such as logging, tracing, or processing response data.

**Example:**

```go
func main() {
    cc := client.New()

    cc.AddResponseHook(func(c *client.Client, resp *client.Response, req *client.Request) error {
        fmt.Printf("Response Status Code: %d\n", resp.StatusCode())
        fmt.Printf("HTTP protocol: %s\n\n", resp.Protocol())

        fmt.Println("Response Headers:")
       for key, value := range resp.RawResponse.Header.All() {
           fmt.Printf("%s: %s\n", key, value)
       }

        return nil
    })

    _, err := cc.Get("https://example.com/")
    if err != nil {
        panic(err)
    }
}
```

<details>
<summary>Click here to see the result</summary>

```plaintext
Response Status Code: 200
HTTP protocol: HTTP/1.1

Response Headers:
Content-Length: 1256
Content-Type: text/html; charset=UTF-8
Server: ECAcc (dcd/7D5A)
Age: 216114
Cache-Control: max-age=604800
Date: Fri, 10 May 2024 10:49:10 GMT
Etag: "3147526947+gzip+ident"
Expires: Fri, 17 May 2024 10:49:10 GMT
Last-Modified: Thu, 17 Oct 2019 07:18:26 GMT
Vary: Accept-Encoding
X-Cache: HIT
```

</details>

### Built-in Response Hooks

Fiber includes built-in response hooks:

- **parserResponseCookie**: Parses cookies from the response and stores them in the response object and cookie jar if available. A cookie with an attribute that cannot be parsed keeps its name and value.
- **logger**: Logs information about the raw request and response. It uses the `log.CommonLogger` interface. Only the headers are logged for a streamed body, and nothing is logged when the client has no logger.

:::info
If a response hook returns an error, Fiber skips the remaining hooks and returns that error.
:::

**Example with Multiple Response Hooks:**

```go
func main() {
    cc := client.New()

    cc.AddResponseHook(func(c *client.Client, r1 *client.Response, r2 *client.Request) error {
        fmt.Println("Hook 1")
        return nil
    })

    cc.AddResponseHook(func(c *client.Client, r1 *client.Response, r2 *client.Request) error {
        fmt.Println("Hook 2")
        return errors.New("error")
    })

    cc.AddResponseHook(func(c *client.Client, r1 *client.Response, r2 *client.Request) error {
        fmt.Println("Hook 3")
        return nil
    })

    _, err := cc.Get("https://example.com/")
    if err != nil {
        panic(err)
    }
}
```

<details>
<summary>Click here to see the result</summary>

```shell
Hook 1
Hook 2
panic: error

goroutine 1 [running]:
main.main()
        main.go:30 +0xd6
exit status 2
```

</details>

## Hook Execution Order

Each hook group runs in FIFO order. The complete request pipeline is regular
request hooks, built-in request hooks, final request hooks, transport, built-in
response hooks, and regular response hooks.

**Example:**

```go
func main() {
    cc := client.New()

    cc.AddRequestHook(func(c *client.Client, r *client.Request) error {
        fmt.Println("Hook 1")
        return nil
    })

    cc.AddRequestHook(func(c *client.Client, r *client.Request) error {
        fmt.Println("Hook 2")
        return nil
    })

    _, err := cc.Get("https://example.com/")
    if err != nil {
        panic(err)
    }
}
```

<details>
<summary>Click here to see the result</summary>

```plaintext
Hook 1
Hook 2
```

</details>
