# Expvar Middleware

Expvar middleware for [Fiber](https://github.com/gofiber/fiber) that serves via its HTTP server runtime exposed variants in the JSON format. The package is typically only imported for the side effect of registering its HTTP handlers. The handled path is `/debug/vars`.

- [Expvar Middleware](#expvar-middleware)
	- [Signatures](#signatures)
	- [Example](#example)

## Signatures

```go
func New() fiber.Handler
```

## Example

Import the expvar package that is part of the Fiber web framework

```go
package main

import (
	"expvar"
	"fmt"

	"github.com/gofiber/fiber/v2"
	expvarmw "github.com/gofiber/fiber/v2/middleware/expvar"
)

var count = expvar.NewInt("count")

func main() {
	app := fiber.New()
	app.Use(expvarmw.New())
	app.Get("/", func(c *fiber.Ctx) error {
		count.Add(1)

		return c.SendString(fmt.Sprintf("hello expvar count %d", count.Value()))
	})

	fmt.Println(app.Listen(":3000"))
}
```

Visit path `/debug/vars` to see all vars and use query `r=key` to filter exposed variables.

```bash
curl 127.0.0.1:3000
hello expvar count 1

curl 127.0.0.1:3000/debug/vars
{
	"cmdline": ["xxx"],
	"count": 1,
	"expvarHandlerCalls": 33,
	"expvarRegexpErrors": 0,
	"memstats": {...}
}

curl 127.0.0.1:3000/debug/vars?r=c
{
	"cmdline": ["xxx"],
	"count": 1
}
```
