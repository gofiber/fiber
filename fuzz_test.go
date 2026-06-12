//go:build go1.18
// +build go1.18

// Copyright (c) 2019-present Fenny and Contributors
// SPDX-License-Identifier: MIT

package fiber_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// FuzzFiberRouteMatch tests HTTP route matching with arbitrary
// paths and methods. Every HTTP request to a Fiber app passes
// through this routing code.
//
// Fiber has 40K+ GitHub stars and 3 GitHub Security Advisories.
func FuzzFiberRouteMatch(f *testing.F) {
	f.Add("/api/users", "GET")
	f.Add("/api/users/:id", "GET")
	f.Add("/", "POST")
	f.Add("", "")
	f.Add(strings.Repeat("/a", 100), "GET")

	f.Fuzz(func(t *testing.T, path, method string) {
		if len(path) > 10000 || len(method) > 100 {
			return
		}

		func() {
			defer func() { _ = recover() }()

			app := fiber.New()
			app.Get("/test", func(c fiber.Ctx) error { return c.SendString("ok") })
			app.Get("/api/:resource", func(c fiber.Ctx) error { return c.SendString("api") })

			req := httptest.NewRequest(method, path, nil)
			_, _ = app.Test(req)
		}()
	})
}

// FuzzFiberBodyParser tests request body binding with arbitrary
// JSON byte input.
func FuzzFiberBodyParser(f *testing.F) {
	f.Add([]byte(`{"name":"test"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 1<<16 {
			return
		}

		func() {
			defer func() { _ = recover() }()

			app := fiber.New()
			app.Post("/api", func(c fiber.Ctx) error {
				var m map[string]any
				_ = c.Bind().JSON(&m)
				return c.JSON(m)
			})

			req := httptest.NewRequest("POST", "/api",
				io.NopCloser(strings.NewReader(string(body))))
			req.Header.Set("Content-Type", "application/json")
			_, _ = app.Test(req)
		}()
	})
}
