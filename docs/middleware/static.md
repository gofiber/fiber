---
id: static
---

# Static

Static middleware for Fiber that serves static files such as **images**, **CSS,** and **JavaScript**.

:::info
By default, **Static** will serve `index.html` files in response to a request on a directory. You can change it from [Config](#config)`
:::

## Signatures

```go
func New(root string, cfg ...Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the [Fiber](https://github.com/gofiber/fiber) web framework

```go
import(
    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/static"
)
```

### Serving files from a directory

```go
app.Get("/*", static.New("./public"))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/hello.html
curl http://localhost:3000/css/style.css
```

</details>

### Serving files from a directory with Use

```go
app.Use("/", static.New("./public"))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/hello.html
curl http://localhost:3000/css/style.css
```

</details>

### Serving a file

```go
app.Use("/static", static.New("./public/hello.html"))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/static # will show hello.html
curl http://localhost:3000/static/john/doee # will show hello.html
```

</details>

### Serving files using os.DirFS

```go
app.Get("/files*", static.New("", static.Config{
    FS:     os.DirFS("files"),
    Browse: true,
}))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/files/css/style.css
curl http://localhost:3000/files/index.html
```

</details>

### Serving files using embed.FS

```go
//go:embed path/to/files
var myfiles embed.FS

app.Get("/files*", static.New("", static.Config{
    FS:     myfiles,
    Browse: true,
}))
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/files/css/style.css
curl http://localhost:3000/files/index.html
```

</details>

### SPA (Single Page Application)

```go
app.Use("/web", static.New("", static.Config{
    FS: os.DirFS("dist"),
}))

app.Get("/web*", func(c fiber.Ctx) error {
    return c.SendFile("dist/index.html")
})
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/web/css/style.css
curl http://localhost:3000/web/index.html
curl http://localhost:3000/web
```

</details>

:::caution
To define static routes using `Get`, append the wildcard (`*`) operator at the end of the route.
:::

## Config

| Property   | Type                    | Description                                                                                                                | Default                |
|:-----------|:------------------------|:---------------------------------------------------------------------------------------------------------------------------|:-----------------------|
| Next       | `func(fiber.Ctx) bool` | Next defines a function to skip this middleware when returned true.                                                                              | `nil`                  |
| FS       | `fs.FS` | FS is the file system to serve the static files from.<br /><br />You can use interfaces compatible with fs.FS like embed.FS, os.DirFS etc.                                                 | `nil`                  |
| Compress       | `bool` | When set to true, the server tries minimizing CPU usage by caching compressed files. The middleware will compress the response using `gzip`, `brotli`, or `zstd` compression depending on the [Accept-Encoding](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Accept-Encoding) header. <br /><br />This works differently than the github.com/gofiber/compression middleware.                                                                              | `false`                  |
| ByteRange       | `bool` | When set to true, enables byte range requests.                                                                             | `false`                  |
| Browse       | `bool` | When set to true, enables directory browsing.                                                                             | `false`                  |
| Download       | `bool` | When set to true, enables direct download.                                                                             | `false`                  |
| IndexNames       | `[]string` | The names of the index files for serving a directory.                                                                             | `[]string{"index.html"}`                  |
| CacheDuration       | `time.Duration` | Expiration duration for inactive file handlers.<br /><br />Use a negative time.Duration to disable it.                                                                             | `10 * time.Second`                  |
| MaxAge       | `int` | The value for the Cache-Control HTTP-header that is set on the file response. MaxAge is defined in seconds.                                                                             | `0`                  |
| ModifyResponse       | `fiber.Handler` | ModifyResponse defines a function that allows you to alter the response.                                                                             | `nil`                  |
| NotFoundHandler       | `fiber.Handler` | NotFoundHandler defines a function to handle when the path is not found.                                                                             | `nil`                  |

:::info
You can set `CacheDuration` config property to `-1` to disable caching.
:::

## Default Config

```go
var ConfigDefault = Config{
    Index:         []string{"index.html"},
    CacheDuration: 10 * time.Second,
}
```
