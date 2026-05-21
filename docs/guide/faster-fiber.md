---
id: faster-fiber
title: ⚡ Make Fiber Faster
sidebar_position: 7
---

## Custom JSON Encoder/Decoder

Fiber defaults to the standard `encoding/json` for stability and reliability. If you need more speed, consider these libraries:

- [goccy/go-json](https://github.com/goccy/go-json)
- [bytedance/sonic](https://github.com/bytedance/sonic)
- [segmentio/encoding](https://github.com/segmentio/encoding)
- [minio/simdjson-go](https://github.com/minio/simdjson-go)

```go title="Example"
package main

import "github.com/gofiber/fiber/v3"
import "github.com/goccy/go-json"

func main() {
    app := fiber.New(fiber.Config{
        JSONEncoder: json.Marshal,
        JSONDecoder: json.Unmarshal,
    })

    // ...
}
```

### References

- [Set custom JSON encoder for client](../client/rest.md#setjsonmarshal)
- [Set custom JSON decoder for client](../client/rest.md#setjsonunmarshal)
- [Set custom JSON encoder for application](../api/fiber.md#jsonencoder)
- [Set custom JSON decoder for application](../api/fiber.md#jsondecoder)

## Alternative Regex Engines for `regex()` Constraints

Fiber route patterns still do not support general regex routes, but you can swap
the compiler used by `regex()` parameter constraints through
[`Config.RegexHandler`](../api/fiber.md#regexhandler). This lets you try
high-performance engines such as
[coregex](https://github.com/coregx/coregex) on the matching path.

### Configure `RegexHandler`

`RegexHandler` accepts a function with the shape `func(string) T`, where `T`
implements `fiber.RegexMatcher`.

```go title="Example"
package main

import (
    "github.com/coregx/coregex"
    "github.com/gofiber/fiber/v3"
)

func main() {
    app := fiber.New(fiber.Config{
        RegexHandler: coregex.MustCompile,
    })

    app.Get("/api/:id<regex(\\d+)>", func(c fiber.Ctx) error {
        return c.SendString("ID: " + c.Params("id"))
    })
}
```

You can also set it explicitly to the standard library default:

```go title="Example"
package main

import (
	"regexp"

	"github.com/gofiber/fiber/v3"
)

func main() {
	app := fiber.New(fiber.Config{
		RegexHandler: regexp.MustCompile,
	})

	_ = app
}
```

### Notes

- `RegexHandler` only affects `regex()` parameter constraints
- invalid patterns still panic during route registration because Fiber uses
  `MustCompile`-style semantics
- Fiber may invoke `RegexHandler` more than once per route while parsing raw and
  normalized route patterns during registration
- compiled matchers are reused across requests, so custom matchers must be safe
  for concurrent use
