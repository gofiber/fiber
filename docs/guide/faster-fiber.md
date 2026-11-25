---
id: faster-fiber
title: ⚡ Make Fiber Faster
sidebar_position: 11
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

### Encoder benchmarks

Time per response for the [HttpArena](https://github.com/MDA2AV/HttpArena) `GET /json/20` handler of Fiber, a 3.4 KB response of 20 items, measured in process with `go test -bench` (`-count=10`, one CPU, Go 1.27, Fiber v3.7). Every encoder writes the same bytes. **Marshal** is the encoder alone, **Request** the whole Fiber request without the network.

| Encoder | amd64 Marshal | amd64 Request | arm64 Marshal | arm64 Request |
|:--------|--------------:|--------------:|--------------:|--------------:|
| `encoding/json` | 20.5 µs | 22.5 µs | 13.8 µs | 14.9 µs |
| [goccy/go-json](https://github.com/goccy/go-json) v0.12.0 | 3.6 µs | 5.4 µs | 2.6 µs | 3.5 µs |
| [bytedance/sonic](https://github.com/bytedance/sonic) v1.15.5 | 3.8 µs | 5.6 µs | 11.9 µs | 12.8 µs |
| [segmentio/encoding](https://github.com/segmentio/encoding) v0.5.4 | 5.0 µs | 6.7 µs | 3.8 µs | 4.7 µs |

amd64 is an AMD EPYC 7763 and arm64 an Apple M1. `goccy/go-json` and `segmentio/encoding` make one allocation fewer per response than `encoding/json`, and `sonic` as many.

- `sonic` compiles its encoders just in time only on amd64. On arm64 it runs them in an interpreter and stays close to `encoding/json`, so prefer `goccy/go-json` or `segmentio/encoding` on arm64 servers such as AWS Graviton.
- The gain depends on the shape of the response: measure your own handlers before switching.

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

Set `RegexHandler` to the compile function you want Fiber to use for `regex()`
constraints.

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
- `RegexHandler` must be a non-nil function of the form `func(string) T`, where
  `T` has a `MatchString(string) bool` method; `fiber.New` panics for any other
  shape, so `regexp.Compile`, which also returns an error, does not fit, and
  registering a route panics when the handler returns a nil matcher
- invalid patterns still panic during route registration because Fiber uses
  `MustCompile`-style semantics
- Fiber may invoke `RegexHandler` more than once per route while parsing raw and
  normalized route patterns during registration
- compiled matchers are reused across requests, so custom matchers must be safe
  for concurrent use

## Related Performance Reading

- [Zero Allocation](../intro.md#zero-allocation): why `Ctx` values are reused across requests and how to copy them safely
- [Benchmarks](../extra/benchmarks.md): how Fiber compares to other frameworks
