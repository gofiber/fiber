# Examples

#### Multiple File Upload

```go
package main

import "github.com/fenny/fiber"

func main() {
  app := fiber.New()

  app.Post("/", func(c *fiber.Ctx) {
    if form := c.MultipartForm(); form != nil {
      files := form.File["documents"]
      for _, file := range files {
        fmt.Println(file.Filename, file.Size, file.Header["Content-Type"][0])
        // => "tutorial.pdf" 360641 "application/pdf"
        c.SaveFile(file, fmt.Sprintf("./%s", file.Filename))
        // Saves the file to disk
      }
    }
  })

  app.Listen(8080)
}
```

#### 404 Handling

```go
package main

import "github.com/fenny/fiber"

func main() {
  app := fiber.New()

  app.Static("./static")
  app.Use(func (c *fiber.Ctx) {
    c.SendStatus(404)
    // => 404 "Not Found"
  })

  app.Listen(8080)
}
```

#### Static Caching

```go
package main

import "github.com/fenny/fiber"

func main() {
  app := fiber.New()

  app.Use(func(c *fiber.Ctx) {
    c.Set("Cache-Control", "max-age=2592000, public")
    c.Next()
  })
  app.Static("./static")

  app.Listen(8080)
}
```

#### Enable CORS

```go
package main

import "./fiber"

func main() {
  app := fiber.New()

  app.Use("/api", func(c *fiber.Ctx) {
    c.Set("Access-Control-Allow-Origin", "*")
    c.Set("Access-Control-Allow-Headers", "X-Requested-With")
    c.Next()
  })
  app.Get("/api", func(c *fiber.Ctx) {
    c.Send("Hi, I'm API!")
  })

  app.Listen(8080)
}
```

#### Returning JSON

```go
package main

import "./fiber"

type Data struct {
  Name string
  Age  int
}

func main() {
  app := fiber.New()

  app.Get("/json", func(c *fiber.Ctx) {
    data := Data{
      Name: "John", `json:"name"`
      Age:  20, `json:"age"`
    }
    err := c.JSON(data)
    if err != nil {
      c.SendStatus(500)
    }
  })

  app.Listen(8080)
}
```

#### TLS/HTTPS

```go
package main

import "./fiber"

func main() {
  app := fiber.New()

  app.Get("/", func(c *fiber.Ctx) {
    c.Send(c.Protocol()) // => "https"
  })

  app.Listen(443, "server.crt", "server.key")
}
```

_Caught a mistake? [Edit this page on GitHub!](https://github.com/Fenny/fiber/blob/master/docs/examples.md)_
