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

```go
package main

import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
  app := fiber.New()
  
  app.Get("/*", static.New("./public"))
  
  app.Listen(":3000")
}
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/hello.html
curl http://localhost:3000/css/style.css
```

</details>

```go
package main

import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
  app := fiber.New()
  
  app.Use("/", static.New("./public"))
  
  app.Listen(":3000")
}
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/hello.html
curl http://localhost:3000/css/style.css
```

</details>

```go
package main

import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
  app := fiber.New()
  
  app.Use("/static", static.New("./public/hello.html"))
  
  app.Listen(":3000")
}
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/static # will show hello.html
curl http://localhost:3000/static/john/doee # will show hello.html
```

</details>

```go
package main

import (
  "github.com/gofiber/fiber/v3"
  "github.com/gofiber/fiber/v3/middleware/static"
)

func main() {
  app := fiber.New()
  
  app.Get("/files*", static.New("", static.Config{
		FS:     os.DirFS("files"),
		Browse: true,
	}))
  
  app.Listen(":3000")
}
```

<details>
<summary>Test</summary>

```sh
curl http://localhost:3000/files/css/style.css
curl http://localhost:3000/files/index.html
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
| Compress       | `bool` | When set to true, the server tries minimizing CPU usage by caching compressed files.<br /><br />This works differently than the github.com/gofiber/compression middleware.                                                                              | `false`                  |
| ByteRange       | `bool` | When set to true, enables byte range requests.                                                                             | `false`                  |
| Browse       | `bool` | When set to true, enables directory browsing.                                                                             | `false`                  |
| Download       | `bool` | When set to true, enables direct download.                                                                             | `false`                  |
| IndexNames       | `[]string` | The names of the index files for serving a directory.                                                                             | `[]string{"index.html"}`                  |
| CacheDuration       | `string` | Expiration duration for inactive file handlers.<br /><br />Use a negative time.Duration to disable it.                                                                             | `10 * time.Second`                  |
| MaxAge       | `int` | The value for the Cache-Control HTTP-header that is set on the file response. MaxAge is defined in seconds.                                                                             | `0`                  |
| ModifyResponse       | `fiber.Handler` | ModifyResponse defines a function that allows you to alter the response.                                                                             | `nil`                  |

## Default Config

```go
var ConfigDefault = Config{
	Index:         []string{"index.html"},
	CacheDuration: 10 * time.Second,
}
```
