---
id: hooks
title: 🎣 Hooks
description: >-
  Hooks are used to manipulate request/response proccess of Fiber client.
sidebar_position: 4
---

With hooks, you can manipulate the client on before request/after response stages or more complex logging/tracing cases.

There are 2 kinds of hooks:

## Request Hooks

They are called before the HTTP request has been sent. You can use them make changes on Request object.

You need to use `RequestHook func(*Client, *Request) error` function signature while creating the hooks. You can use request hooks to change host URL, log request properties etc. Here is an example about how to create request hooks:

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

There are also some builtin request hooks provide some functionalities for Fiber client. Here is a list of them:

- [parserRequestURL](https://github.com/gofiber/fiber/blob/main/client/hooks.go#L62): parserRequestURL customizes the URL according to the path params and query params. It's necessary for `PathParam` and `QueryParam` methods.

- [parserRequestHeader](https://github.com/gofiber/fiber/blob/main/client/hooks.go#L113): parserRequestHeader sets request headers, cookies, body type, referer, user agent according to client and request proeprties. It's necessary to make request header and cookiejar methods functional.

- [parserRequestBody](https://github.com/gofiber/fiber/blob/main/client/hooks.go#L178): parserRequestBody serializes the body automatically. It is useful for XML, JSON, form, file bodies.

:::info
If any error returns from request hook execution, it will interrupt the request and return the error.
:::

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

## Response Hooks

They are called after the HTTP response has been completed. You can use them to get some information about response and request.

You need to use `ResponseHook func(*Client, *Response, *Request) error` function signature while creating the hooks. You can use response hook for logging, tracing etc. Here is an example about how to create response hooks:

```go
func main() {
    cc := client.New()

    cc.AddResponseHook(func(c *client.Client, resp *client.Response, req *client.Request) error {
        fmt.Printf("Response Status Code: %d\n", resp.StatusCode())
        fmt.Printf("HTTP protocol: %s\n\n", resp.Protocol())

        fmt.Println("Response Headers:")
        resp.RawResponse.Header.VisitAll(func(key, value []byte) {
            fmt.Printf("%s: %s\n", key, value)
        })

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

There are also some builtin request hooks provide some functionalities for Fiber client. Here is a list of them:

- [parserResponseCookie](https://github.com/gofiber/fiber/blob/main/client/hooks.go#L293): parserResponseCookie parses cookies and saves into the response objects and cookiejar if it's exists.

- [logger](https://github.com/gofiber/fiber/blob/main/client/hooks.go#L319): logger prints some RawRequest and RawResponse information. It uses [log.CommonLogger](https://github.com/gofiber/fiber/blob/main/log/log.go#L49) interface for logging.

:::info
If any error is returned from executing the response hook, it will return the error without executing other response hooks.
:::

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

:::info
Hooks work as FIFO (first-in-first-out). You need to check the order while adding the hooks.
:::

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
