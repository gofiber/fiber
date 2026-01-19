---
id: advance-format
title: 🐛 Advanced Format
description: >-
  Learn how to use MessagePack (MsgPack) and CBOR for efficient binary serialization in Fiber applications.
sidebar_position: 9
---

## MsgPack

Fiber lets you use MessagePack for efficient binary serialization. Use one of the popular Go libraries below to encode and decode data in handlers.

- Fiber can bind requests with the `application/vnd.msgpack` content type out of the box. See the [Binding documentation](../api/bind.md#msgpack) for details.
- Use `Bind().MsgPack()` to bind data to structs, similar to JSON. `Ctx.AutoFormat()` responds with MsgPack when the `Accept` header is `application/vnd.msgpack`. See the [AutoFormat documentation](../api/ctx.md#autoformat) for more.

### Recommended Libraries

- [github.com/vmihailenco/msgpack](https://pkg.go.dev/github.com/vmihailenco/msgpack) — A widely used, feature-rich MsgPack library.
- [github.com/shamaton/msgpack/v3](https://pkg.go.dev/github.com/shamaton/msgpack/v3) — High-performance MsgPack library.

### Installation

Install either library using:

```bash
go get github.com/vmihailenco/msgpack
# or
go get github.com/shamaton/msgpack/v3
```

> **Note:** Fiber doesn't bundle a MsgPack implementation because it's outside the Go standard library. Pick one of the popular libraries in the ecosystem; the two below are widely used and well maintained.

### Example: Using `shamaton/msgpack/v3`

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/shamaton/msgpack/v3"
)

type User struct {
    Name string `msgpack:"name"` // tag may vary depending on your MsgPack library
    Age  int   `msgpack:"age"`
}

func main() {
    app := fiber.New(fiber.Config{
        // Optional: Set custom MsgPack encoder/decoder
        MsgPackEncoder: msgpack.Marshal,
        MsgPackDecoder: msgpack.Unmarshal,
    })

    app.Post("/msgpack", func(c fiber.Ctx) error {
        var user User
        if err := c.Bind().MsgPack(&user); err != nil {
            return err
        }
        // Content type will be set automatically to application/vnd.msgpack
        return c.MsgPack(user)
    })

    app.Listen(":3000")
}
```

## CBOR

Fiber doesn't ship with a CBOR implementation. Use a library such as [fxamacker/cbor](https://github.com/fxamacker/cbor) to add encoding and decoding.

- Use `Bind().CBOR()` to bind CBOR to structs. `Ctx.AutoFormat()` replies with CBOR when the `Accept` header is `application/cbor`. See the [AutoFormat documentation](../api/ctx.md#autoformat) for details.

```bash
go get github.com/fxamacker/cbor/v2
```

Configure Fiber with the chosen library:

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/fxamacker/cbor/v2"
)

func main() {
    app := fiber.New(fiber.Config{
        CBOREncoder: cbor.Marshal,
        CBORDecoder: cbor.Unmarshal,
    })

    type User struct {
        Name string `cbor:"name"`
        Age  int    `cbor:"age"`
    }

    app.Post("/cbor", func(c fiber.Ctx) error {
        var user User
        if err := c.Bind().CBOR(&user); err != nil {
            return err
        }

        // Content type will be set automatically to application/cbor
        return c.CBOR(user)
    })

    app.Listen(":3000")
}
```
