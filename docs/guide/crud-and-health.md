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
    "cmp"
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
    out := make([]Item, 0, len(s.items))
    for _, it := range s.items {
        out = append(out, it)
    }
    s.mu.Unlock()
    // Map iteration is unordered, so sort before answering, and do it off the
    // lock: out is local from here on.
    slices.SortFunc(out, func(a, b Item) int { return cmp.Compare(a.ID, b.ID) })
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

    // Readiness needs a probe of its own: the default one always answers
    // "healthy", so it could never take this instance out of rotation.
    var ready atomic.Bool
    readiness := healthcheck.Config{Probe: func(fiber.Ctx) bool { return ready.Load() }}

    app.Get(healthcheck.LivenessEndpoint, healthcheck.New())
    app.Get(healthcheck.ReadinessEndpoint, healthcheck.New(readiness))
    app.Get(healthcheck.StartupEndpoint, healthcheck.New())

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

    // In memory, so ready right away and the flag is never observed false. A
    // real service starts Listen first and flips this once its database is up.
    ready.Store(true)

    log.Fatal(app.Listen(":3000"))
}
```

The route path `:id` is a parameter; see
[Routing](./routing.md#parameters). `c.Bind().Body` maps the request body onto
a struct, see [Bind](../api/bind.md#body), and `fiber.NewError` hands the
status and message to the central error handler, see
[Error handling](./error-handling.md). `atomic.Bool` is a flag that is safe to
read and write from many requests at once.

## Try it out

Start the app:

```bash
go run main.go
```

Check the probes. Each answers `OK` as plain text, and `-f` makes `curl` exit
non-zero on a 4xx or 5xx, which is what lets a CI step fail. Use
`--fail-with-body` instead when the log should also carry the error message:

```bash
curl -fsS http://localhost:3000/livez
curl -fsS http://localhost:3000/readyz
curl -fsS http://localhost:3000/startupz
```

Pass `ResponseFormat: healthcheck.FormatJSON` in the config to answer
`{"status":"OK"}` instead; see
[Healthcheck](../middleware/healthcheck.md).

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

A request for an item that is gone answers `404` with the message as plain
text, not as JSON. [Error handling](./error-handling.md) shows how to answer
JSON instead:

```bash
curl -sS -w '\n%{http_code}\n' http://localhost:3000/items/1
# not found
# 404
```

## Notes

- The store uses a `sync.Mutex` so it is safe to call from multiple handlers.
- The handlers accept any name, including an empty one. See
  [Validation](./validation.md) for rejecting bad input.
- Store `false` in `ready` while shutting down so a load balancer drains this
  instance; `app.Hooks().OnPreShutdown` runs while the server still answers.
- For production persistence, replace the `store` with a database of your choice.
