---
id: crud-and-health
title: 🚀 Minimal CRUD and Health Check
description: >-
  A minimal in-memory CRUD example with healthcheck probes. Useful as a
  starting point for learning, smoke tests, and CI checks. Not for production
  persistence.
sidebar_position: 12
---

This guide shows a minimal Fiber app with two things most "hello world"
examples leave out:

- Liveness, readiness, and startup probes using the
  [`healthcheck`](../middleware/healthcheck.md) middleware
- A tiny in-memory CRUD for `Item` (`list`, `create`, `get`, `update`, `delete`)

:::caution
This example stores data in memory. It is intended for **learning, smoke
tests, and CI checks**, not for production persistence. Restarting the app
clears all data.
:::

## Set up the module

```bash
mkdir crud && cd crud
go mod init example.com/crud
go get github.com/gofiber/fiber/v3
```

## Full example

```go title="main.go"
package main

import (
    "log"
    "slices"
    "strconv"
    "sync"
    "sync/atomic"

    "github.com/gofiber/fiber/v3"
    "github.com/gofiber/fiber/v3/middleware/healthcheck"
)

type Item struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type store struct {
    mu     sync.Mutex
    items  map[int]Item
    nextID int
}

func newStore() *store {
    return &store{items: make(map[int]Item), nextID: 1}
}

func (s *store) list() []Item {
    s.mu.Lock()
    defer s.mu.Unlock()
    out := make([]Item, 0, len(s.items))
    for _, it := range s.items {
        out = append(out, it)
    }
    // Map iteration is unordered, so sort before answering.
    slices.SortFunc(out, func(a, b Item) int { return a.ID - b.ID })
    return out
}

func (s *store) create(name string) Item {
    s.mu.Lock()
    defer s.mu.Unlock()
    it := Item{ID: s.nextID, Name: name}
    s.items[it.ID] = it
    s.nextID++
    return it
}

func (s *store) get(id int) (Item, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    it, ok := s.items[id]
    return it, ok
}

func (s *store) update(id int, name string) (Item, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    it, ok := s.items[id]
    if !ok {
        return Item{}, false
    }
    it.Name = name
    s.items[id] = it
    return it, true
}

func (s *store) delete(id int) bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, ok := s.items[id]; !ok {
        return false
    }
    delete(s.items, id)
    return true
}

func main() {
    app := fiber.New()
    s := newStore()

    // A probe that always answers "healthy" can never take the instance out of
    // rotation, so readiness and startup report this flag instead.
    var ready atomic.Bool
    isReady := func(fiber.Ctx) bool { return ready.Load() }

    // Liveness answers as long as the process serves at all.
    app.Get(healthcheck.LivenessEndpoint, healthcheck.New()) // /livez
    app.Get(healthcheck.ReadinessEndpoint, healthcheck.New(healthcheck.Config{
        Probe: isReady,
    })) // /readyz
    app.Get(healthcheck.StartupEndpoint, healthcheck.New(healthcheck.Config{
        Probe: isReady,
    })) // /startupz

    app.Get("/items", func(c fiber.Ctx) error {
        return c.JSON(s.list())
    })

    app.Post("/items", func(c fiber.Ctx) error {
        var body struct {
            Name string `json:"name"`
        }
        if err := c.Bind().Body(&body); err != nil {
            return fiber.NewError(fiber.StatusBadRequest, "invalid body")
        }
        return c.Status(fiber.StatusCreated).JSON(s.create(body.Name))
    })

    app.Get("/items/:id", func(c fiber.Ctx) error {
        id, err := strconv.Atoi(c.Params("id"))
        if err != nil {
            return fiber.NewError(fiber.StatusBadRequest, "invalid id")
        }
        it, ok := s.get(id)
        if !ok {
            return fiber.NewError(fiber.StatusNotFound, "not found")
        }
        return c.JSON(it)
    })

    app.Put("/items/:id", func(c fiber.Ctx) error {
        id, err := strconv.Atoi(c.Params("id"))
        if err != nil {
            return fiber.NewError(fiber.StatusBadRequest, "invalid id")
        }
        var body struct {
            Name string `json:"name"`
        }
        if err := c.Bind().Body(&body); err != nil {
            return fiber.NewError(fiber.StatusBadRequest, "invalid body")
        }
        it, ok := s.update(id, body.Name)
        if !ok {
            return fiber.NewError(fiber.StatusNotFound, "not found")
        }
        return c.JSON(it)
    })

    app.Delete("/items/:id", func(c fiber.Ctx) error {
        id, err := strconv.Atoi(c.Params("id"))
        if err != nil {
            return fiber.NewError(fiber.StatusBadRequest, "invalid id")
        }
        if !s.delete(id) {
            return fiber.NewError(fiber.StatusNotFound, "not found")
        }
        return c.SendStatus(fiber.StatusNoContent)
    })

    // Everything this app needs is in memory, so it is ready right away. A real
    // service flips this once its database or queue is connected.
    ready.Store(true)

    log.Fatal(app.Listen(":3000"))
}
```

The route path `:id` is a parameter; see
[Routing](./routing.md#parameters). `c.Bind().Body` maps the request body onto
a struct, see [Bind](../api/bind.md#body), and `fiber.NewError` hands the
status and message to the central error handler, see
[Error handling](./error-handling.md).

## Try it out

Start the app:

```bash
go run main.go
```

Check the probes. `-f` makes `curl` exit non-zero on an error status, which is
what lets a CI step fail:

```bash
curl -fsS http://localhost:3000/livez    # OK
curl -fsS http://localhost:3000/readyz   # OK
curl -fsS http://localhost:3000/startupz # OK
```

Create an item:

```bash
curl -fsS -X POST http://localhost:3000/items \
  -H "Content-Type: application/json" \
  -d '{"name":"first"}'
# {"id":1,"name":"first"}
```

List, get, update, delete:

```bash
curl -fsS http://localhost:3000/items
curl -fsS http://localhost:3000/items/1
curl -fsS -X PUT http://localhost:3000/items/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"updated"}'
curl -fsS -X DELETE http://localhost:3000/items/1
```

A request for an item that is gone answers with the status and the message,
not with JSON:

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:3000/items/1
# 404
```

## Notes

- Storage is in-memory only; restarting the app clears all items.
- The store uses a `sync.Mutex` so it is safe to call from multiple handlers.
- Errors travel through the default error handler, which answers `text/plain`.
  [Error handling](./error-handling.md) shows how to answer JSON instead.
- The handlers accept any name, including an empty one. See
  [Validation](./validation.md) for rejecting bad input.
- `Content-Type` decides which parser `c.Bind().Body` uses, and `curl -d` sends
  `application/x-www-form-urlencoded` unless told otherwise. Dropping the `-H`
  line above therefore does not fail: the JSON text is read as a form and the
  item is created with an empty name.
- For production persistence, replace the `store` with a database of your choice.
