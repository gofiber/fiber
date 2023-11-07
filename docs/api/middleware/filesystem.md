---
id: filesystem
---

# FileSystem

Filesystem middleware for [Fiber](https://github.com/gofiber/fiber) that enables you to serve files from a directory.

:::caution
**`:params` & `:optionals?` within the prefix path are not supported!**

**To handle paths with spaces (or other url encoded values) make sure to set `fiber.Config{ UnescapePath: true }`**
:::

## Signatures

```go
func New(config Config) fiber.Handler
```

## Examples

Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/filesystem"
)
```

After you initiate your Fiber app, you can use the following possibilities:

```go
// Provide a minimal config
app.Use(filesystem.New(filesystem.Config{
	Root: http.Dir("./assets"),
}))

// Or extend your config for customization
app.Use(filesystem.New(filesystem.Config{
    Root:         http.Dir("./assets"),
    Browse:       true,
    Index:        "index.html",
    NotFoundFile: "404.html",
    MaxAge:       3600,
}))
```


> If your environment (Go 1.16+) supports it, we recommend using Go Embed instead of the other solutions listed as this one is native to Go and the easiest to use.

## embed

[Embed](https://golang.org/pkg/embed/) is the native method to embed files in a Golang excecutable. Introduced in Go 1.16.

```go
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

// Embed a single file
//go:embed index.html
var f embed.FS

// Embed a directory
//go:embed static/*
var embedDirStatic embed.FS

func main() {
	app := fiber.New()

	app.Use("/", filesystem.New(filesystem.Config{
		Root: http.FS(f),
	}))

	// Access file "image.png" under `static/` directory via URL: `http://<server>/static/image.png`.
	// Without `PathPrefix`, you have to access it via URL:
	// `http://<server>/static/static/image.png`.
	app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(embedDirStatic),
		PathPrefix: "static",
		Browse: true,
	}))

	log.Fatal(app.Listen(":3000"))
}
```

## pkger

[https://github.com/markbates/pkger](https://github.com/markbates/pkger)

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/filesystem"

    "github.com/markbates/pkger"
)

func main() {
    app := fiber.New()

    app.Use("/assets", filesystem.New(filesystem.Config{
        Root: pkger.Dir("/assets"),
	}))

    log.Fatal(app.Listen(":3000"))
}
```

## packr

[https://github.com/gobuffalo/packr](https://github.com/gobuffalo/packr)

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/filesystem"

    "github.com/gobuffalo/packr/v2"
)

func main() {
    app := fiber.New()

    app.Use("/assets", filesystem.New(filesystem.Config{
        Root: packr.New("Assets Box", "/assets"),
	}))

    log.Fatal(app.Listen(":3000"))
}
```

## go.rice

[https://github.com/GeertJohan/go.rice](https://github.com/GeertJohan/go.rice)

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/filesystem"

    "github.com/GeertJohan/go.rice"
)

func main() {
    app := fiber.New()

    app.Use("/assets", filesystem.New(filesystem.Config{
        Root: rice.MustFindBox("assets").HTTPBox(),
	}))

    log.Fatal(app.Listen(":3000"))
}
```

## fileb0x

[https://github.com/UnnoTed/fileb0x](https://github.com/UnnoTed/fileb0x)

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/filesystem"

    "<Your go module>/myEmbeddedFiles"
)

func main() {
    app := fiber.New()

    app.Use("/assets", filesystem.New(filesystem.Config{
        Root: myEmbeddedFiles.HTTP,
	}))

    log.Fatal(app.Listen(":3000"))
}
```

## statik

[https://github.com/rakyll/statik](https://github.com/rakyll/statik)

```go
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	// Use blank to invoke init function and register data to statik
	_ "<Your go module>/statik" 
	"github.com/rakyll/statik/fs"
)

func main() {
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	app := fiber.New()

	app.Use("/", filesystem.New(filesystem.Config{
		Root: statikFS,
	}))

	log.Fatal(app.Listen(":3000"))
}
```

## Config

| Property           | Type                    | Description                                                                                                 | Default      |
|:-------------------|:------------------------|:------------------------------------------------------------------------------------------------------------|:-------------|
| Next               | `func(*fiber.Ctx) bool` | Next defines a function to skip this middleware when returned true.                                         | `nil`        |
| Root               | `http.FileSystem`       | Root is a FileSystem that provides access to a collection of files and directories.                         | `nil`        |
| PathPrefix         | `string`                | PathPrefix defines a prefix to be added to a filepath when reading a file from the FileSystem.              | ""           |
| Browse             | `bool`                  | Enable directory browsing.                                                                                  | `false`      |
| Index              | `string`                | Index file for serving a directory.                                                                         | "index.html" |
| MaxAge             | `int`                   | The value for the Cache-Control HTTP-header that is set on the file response. MaxAge is defined in seconds. | 0            |
| NotFoundFile       | `string`                | File to return if the path is not found. Useful for SPA's.                                                  | ""           |
| ContentTypeCharset | `string`                | The value for the Content-Type HTTP-header that is set on the file response.                                | ""           |

## Default Config

```go
var ConfigDefault = Config{
    Next:   nil,
    Root:   nil,
    PathPrefix: "",
    Browse: false,
    Index:  "/index.html",
    MaxAge: 0,
    ContentTypeCharset: "",
}
```

## Utils

### SendFile

Serves a file from an [HTTP file system](https://pkg.go.dev/net/http#FileSystem) at the specified path.

```go title="Signature" title="Signature"
func SendFile(c *fiber.Ctx, filesystem http.FileSystem, path string) error
```
Import the middleware package that is part of the Fiber web framework

```go
import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/filesystem"
)
```

```go title="Example"
// Define a route to serve a specific file
app.Get("/download", func(c *fiber.Ctx) error {
    // Serve the file using SendFile function
    err := filesystem.SendFile(c, http.Dir("your/filesystem/root"), "path/to/your/file.txt")
    if err != nil {
        // Handle the error, e.g., return a 404 Not Found response
        return c.Status(fiber.StatusNotFound).SendString("File not found")
    }
    
    return nil
})
```

```go title="Example"
// Serve static files from the "build" directory using Fiber's built-in middleware.
app.Use("/", filesystem.New(filesystem.Config{
    Root:       http.FS(f),         // Specify the root directory for static files.
    PathPrefix: "build",            // Define the path prefix where static files are served.
}))

// For all other routes (wildcard "*"), serve the "index.html" file from the "build" directory.
app.Use("*", func(ctx *fiber.Ctx) error {
    return filesystem.SendFile(ctx, http.FS(f), "build/index.html")
})
```
