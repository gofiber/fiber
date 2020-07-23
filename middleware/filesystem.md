# FileSystem

FileSystem middleware for Fiber

### Example
The middleware packages comes with the official Fiber framework.
```go
import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/fiber/middleware"
)
```

### Signature
```go
embed.New(config ...embed.Config) func(c *fiber.Ctx)
```

### Config
| Property | Type | Description | Default |
| :--- | :--- | :--- | :--- |
| Index | `string` | Index file name | `index.html` |
| Browse | `bool` | Enable directory browsing | `false` |
| Root | `http.FileSystem` | http.FileSystem to use | `nil` |
| ErrorHandler | `func(*fiber.Ctx, error)` | Error handler | `InternalServerError` |

### pkger

```go
package main

import (
  "net/http"

  "github.com/gofiber/fiber"
  "github.com/gofiber/middleware/filesystem"
)

func main() {
  app := fiber.New()

  // Pass a FileSystem 
  app.Use("/assets", middleware.FileSystem(http.Dir("./assets")))

  // Define the index file for serving a directory
  app.Use("/assets", middleware.FileSystem(http.Dir("./assets"), "index.html"))

  // Enable directory browsing
  app.Use("/assets", middleware.FileSystem(http.Dir("./assets"), true))

  // Pass a config
  app.Use("/assets", middleware.FileSystem(middleware.FileSystemConfig{
      Root:   http.Dir("./assets"),
      Index:  "index.html",
      Browse: true,
  }))

  app.Listen(8080)
}
```

### packr

```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/middleware/filesystem"

  "github.com/gobuffalo/packr/v2"
)

func main() {
  app := fiber.New()

  app.Use("/assets", middleware.FileSystem(packr.New("Assets Box", "/assets")))

  app.Listen(8080)
}
```

### go.rice

```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/middleware/filesystem"

  "github.com/GeertJohan/go.rice"
)

func main() {
  app := fiber.New()

  app.Use("/assets", middleware.FileSystem(rice.MustFindBox("assets").HTTPBox()))

  app.Listen(8080)
}
```

### fileb0x

```go
package main

import (
  "github.com/gofiber/fiber"
  "github.com/gofiber/middleware/filesystem"
  "<Your go module>/myEmbeddedFiles"
)

func main() {
  app := fiber.New()

  app.Use("/assets", middleware.FileSystem(myEmbeddedFiles.HTTP))

  app.Listen(8080)
}
```

### statik

```go
package main

import (
  "log"
  "github.com/gofiber/fiber"
  "github.com/gofiber/middleware/filesystem"
	
  "<Your go module>/statik"
  fs "github.com/rakyll/statik/fs"
)

func main() {
  statik, err := fs.New()
  if err != nil {
    log.Fatal(err)
  }

  app := fiber.New()

  app.Use("/", middleware.FileSystem.New(statikFS))

  app.Listen(8080)
}
```