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

### Example JSON response benchmark

A loopback HTTP benchmark on a shared Linux amd64 host (AMD EPYC 9V74, Go 1.27.4,
`GOMAXPROCS=2`, Fiber v3.7.0) measured a route returning the same 20 typed
records from `c.JSON`. Each record contains an ID, name, price, boolean,
and two tags. Only the encoder changed. One client sent requests sequentially
over a persistent HTTP/1.1 connection without TLS or compression and read the
full response. Before timing, every encoder's response was decoded and
checked against the original records.

| JSON encoder | Median µs/request | Range (µs/request) | Allocations/request |
| :----------- | ----------------: | -----------------: | ------------------: |
| Default (`encoding/json`) | 71.6 | 62.5–95.2 | 61 |
| `go-json` v0.12.0 | 61.3 | 46.5–77.2 | 60 |
| `sonic.ConfigStd` v1.15.5 | 51.3 | 44.9–67.6 | 61 |

The median and range are over seven runs per encoder. These measurements
include client, TCP, and handler work; they are not server CPU time, maximum
throughput, or a benchmark of decoding. Both alternatives reduced median
latency in this run, but all three ranges overlapped. Benchmark your own
payloads and verify the JSON output before changing encoders.

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
