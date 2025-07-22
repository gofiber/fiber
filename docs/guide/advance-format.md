---
id: advance-format
title: 🐛 Advance Format
description: >-
  Learn how to use MessagePack (MsgPack) and CBOR for efficient binary serialization in Fiber applications.
sidebar_position: 9
---

## Msgpack

Fiber enables efficient binary serialization using MessagePack (MsgPack). You can leverage popular Go libraries to encode and decode MsgPack data within your route handlers.

- Fiber supports binding requests with the `application/vnd.msgpack` content type by default. For more details, see the [Binding documentation](../api/bind.md#msgpack).
- Use `Ctx.MsgPack()` to bind MsgPack data directly to structs, similar to how you would use JSON binding. Alternatively, use `Ctx.AutoFormat()` to send response as MsgPack when the Accept HTTP header is `application/vnd.msgpack`, For more details, see the [AutoFormat documentation](../api/ctx.md#autoformat).

### Recommended Libraries

- [github.com/vmihailenco/msgpack](https://pkg.go.dev/github.com/vmihailenco/msgpack) — A widely used, feature-rich MsgPack library.
- [github.com/shamaton/msgpack/v2](https://pkg.go.dev/github.com/shamaton/msgpack/v2) — High-performance MsgPack library.

### Installation

Install either library using:

```bash
go get github.com/vmihailenco/msgpack
# or
go get github.com/shamaton/msgpack/v2
```

> **Note:** Fiber does **not** register MsgPack by default because it is not part of the Go standard library. You can choose from several popular MsgPack libraries in the Go ecosystem. The two recommended packages below are widely used and compatible with Go.

### Example: Using `shamaton/msgpack/v2`

```go
import (
    "github.com/gofiber/fiber/v3"
    "github.com/shamaton/msgpack/v2"
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
        return c.MsgPack(data)
    })

    app.Listen(":3000")
}
```

## CBOR

Fiber doesn't include a CBOR implementation by default. To enable CBOR encoding and decoding you need to choose a library, for example [fxamacker/cbor](https://github.com/fxamacker/cbor).

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
