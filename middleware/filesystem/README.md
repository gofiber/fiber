# Filesystem middleware
Filesystem middleware for [Fiber](https://github.com/gofiber/fiber) that enables you to serve files from a directory. 

⚠️ **`:params` & `:optionals?` within the prefix path are not supported!**

### Table of Contents
- [Signatures](#signatures)
- [Examples](#examples)
- [Config](#config)
- [Default Config](#default-config)


### Signatures
```go
func New(config Config) fiber.Handler
```

### Examples
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
	Root: http.Dir("./assets")
}))

// Or extend your config for customization
app.Use(filesystem.New(filesystem.Config{
	Root:         http.Dir("./assets"),
	Index:        "index.html",
	Browse:       true,
	NotFoundFile: "404.html"
}))
```

## pkger
https://github.com/markbates/pkger

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

## packr
https://github.com/gobuffalo/packr

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

## go.rice
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

## fileb0x
https://github.com/UnnoTed/fileb0x

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

## statik
https://github.com/rakyll/statik

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

### Config
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
	Root http.FileSystem

	// Index file for serving a directory.
	//
	// Optional. Default: "index.html"
	Index string

	// Enable directory browsing.
	//
	// Optional. Default: false
	Browse bool

	// File to return if path is not found. Useful for SPA's.
	//
	// Optional. Default: ""
	NotFoundFile string
}
```

### Default Config
```go
var ConfigDefault = Config{
	Next:   nil,
	Root:   nil,
	Index:  "/index.html",
	Browse: false,
}
```