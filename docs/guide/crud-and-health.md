---
id: crud-and-health
title: 🚀 Minimal CRUD and Health Check
description: >-
  A minimal in-memory CRUD example with a /health endpoint. Useful as a
  starting point for learning, smoke tests, and CI checks. Not for production
  persistence.
sidebar_position: 12
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';
This guide shows a minimal Fiber app with two things most "hello world" examples leave out:

- A `/health` endpoint that returns `{ "status": "ok" }`
- A tiny in-memory CRUD for `Item` (`list`, `create`, `get`, `update`, `delete`)

:::caution
This example stores data in memory. It is intended for **learning, smoke tests, and CI checks**, not for production persistence. Restarting the app clears all data.
:::

## Full example

<Tabs>
<TabItem value="example" label="Example">
```go title="main.go"
package main

import (
	"log"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v3"
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

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

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

	log.Fatal(app.Listen(":3000"))
}
```
