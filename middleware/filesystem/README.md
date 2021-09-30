# Filesystem Middleware

Filesystem middleware for [Fiber](https://github.com/gofiber/fiber) that enables you to serve files from a directory.

⚠️ **`:params` & `:optionals?` within the prefix path are not supported!**

## Table of Contents

- [Filesystem Middleware](#filesystem-middleware)
	- [Table of Contents](#table-of-contents)
	- [Signatures](#signatures)
	- [Examples](#examples)
		- [Config](#config)
		- [embed](#embed)
		- [pkger](#pkger)
		- [packr](#packr)
		- [go.rice](#gorice)
		- [fileb0x](#fileb0x)
		- [statik](#statik)
	- [Config](#config-1)
		- [Default Config](#default-config)

## Signatures

```go
func New(config Config) fiber.Handler
```

## Examples

First import the middleware from Fiber,

```go
import (
  "github.com/gofiber/fiber/v2"
  "github.com/gofiber/fiber/v2/middleware/filesystem"
)
```

Then create a Fiber app with `app := fiber.New()`.

### Config

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

### embed

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

### pkger

[Pkger](https://github.com/markbates/pkger) can be used to embed files in a Golang excecutable.

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
	})

	log.Fatal(app.Listen(":3000"))
}
```

### packr

[Packr](https://github.com/gobuffalo/packr) can be used to embed files in a Golang excecutable.

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
	})

	log.Fatal(app.Listen(":3000"))
}
```

### go.rice

https://github.com/GeertJohan/go.rice

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
	})

	log.Fatal(app.Listen(":3000"))
}
```

### fileb0x

[Fileb0x](https://github.com/UnnoTed/fileb0x) can be used to embed files in a Golang excecutable.

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
	})

	log.Fatal(app.Listen(":3000"))
}
```

### statik

[Statik](https://github.com/rakyll/statik) can be used to embed files in a Golang excecutable.

```go
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"<Your go module>/statik"
	fs "github.com/rakyll/statik/fs"
)

func main() {
	statik, err := fs.New()
	if err != nil {
		panic(err)
	}

	app := fiber.New()

	app.Use("/", filesystem.New(filesystem.Config{
		Root: statikFS,
	})

	log.Fatal(app.Listen(":3000"))
}
```

## Config

```go
// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// Root is a FileSystem that provides access
	// to a collection of files and directories.
	//
	// Required. Default: nil
	Root http.FileSystem `json:"-"`

	// PathPrefix defines a prefix to be added to a filepath when
	// reading a file from the FileSystem.
	//
	// Use when using Go 1.16 embed.FS
	//
	// Optional. Default ""
	PathPrefix string `json:"path_prefix"`

	// Enable directory browsing.
	//
	// Optional. Default: false
	Browse bool `json:"browse"`

	// Index file for serving a directory.
	//
	// Optional. Default: "index.html"
	Index string `json:"index"`

	// The value for the Cache-Control HTTP-header
	// that is set on the file response. MaxAge is defined in seconds.
	//
	// Optional. Default value 0.
	MaxAge    int `json:"max_age"`

	// File to return if path is not found. Useful for SPA's.
	//
	// Optional. Default: ""
	NotFoundFile string `json:"not_found_file"`
}
```

### Default Config

```go
var ConfigDefault = Config{
	Next:       nil,
	Root:       nil,
	PathPrefix: "",
	Browse:     false,
	Index:      "/index.html",
	MaxAge:     0,
}
```
