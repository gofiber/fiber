// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

// go test -v -coverprofile cover.out .
// go tool cover -html=cover.out -o cover.html
// open cover.html

package fiber

import (
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var handler = func(c *Ctx) {}

func is200(t *testing.T, app *App, url string, m ...string) {

	method := "GET"
	if len(m) > 0 {
		method = m[0]
	}
	req, _ := http.NewRequest(method, url, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s - %s - %v", method, url, err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("%s - %s - %v", method, url, resp.StatusCode)
	}
}

func Test_Order(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})
	app.Get("/", func(c *Ctx) {
		c.Write("1")
		c.Next()
	})
	app.All("/", func(c *Ctx) {
		c.Write("2")
		c.Next()
	})
	app.Use(func(c *Ctx) {
		c.Write("3")
		c.Next()
	})
	req := httptest.NewRequest("GET", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != "123" {
		t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), "123", string(body))
	}
}
func Test_Methods(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})

	app.Connect("/:john?/:doe?", handler)
	is200(t, app, "/", "CONNECT")

	app.Connect("/:john?/:doe?", handler)
	is200(t, app, "/", "CONNECT")

	app.Put("/:john?/:doe?", handler)
	is200(t, app, "/", "CONNECT")

	app.Post("/:john?/:doe?", handler)
	is200(t, app, "/", "POST")

	app.Delete("/:john?/:doe?", handler)
	is200(t, app, "/", "DELETE")

	app.Head("/:john?/:doe?", handler)
	is200(t, app, "/", "HEAD")

	app.Patch("/:john?/:doe?", handler)
	is200(t, app, "/", "PATCH")

	app.Options("/:john?/:doe?", handler)
	is200(t, app, "/", "OPTIONS")

	app.Trace("/:john?/:doe?", handler)
	is200(t, app, "/", "TRACE")

	app.Get("/:john?/:doe?", handler)
	is200(t, app, "/", "GET")

	app.All("/:john?/:doe?", handler)
	is200(t, app, "/", "POST")

	app.Use("/:john?/:doe?", handler)
	is200(t, app, "/", "GET")

}

func Test_New(t *testing.T) {
	app := New(&Settings{
		Immutable: true,
	})
	app.Get("/", func(*Ctx) {

	})
}

func Test_Shutdown(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})
	_ = app.Shutdown()
}

func Test_Static(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})
	grp := app.Group("/v1")
	grp.Static("/v2", ".github/auth_assign.yml")
	app.Static("/*", ".github/FUNDING.yml")
	app.Static("/john", "./.github")
	req, _ := http.NewRequest("GET", "/john/stale.yml", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Length") == "" {
		t.Fatalf(`%s: Missing Content-Length`, t.Name())
	}
	req, _ = http.NewRequest("GET", "/yesyes/john/doe", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Length") == "" {
		t.Fatalf(`%s: Missing Content-Length`, t.Name())
	}
	req, _ = http.NewRequest("GET", "/john/stale.yml", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Length") == "" {
		t.Fatalf(`%s: Missing Content-Length`, t.Name())
	}
	req, _ = http.NewRequest("GET", "/v1/v2", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Length") == "" {
		t.Fatalf(`%s: Missing Content-Length`, t.Name())
	}
}

func Test_Group(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})

	grp := app.Group("/test")
	grp.Get("/", handler)
	is200(t, app, "/test", "GET")

	grp.Get("/:demo?", handler)
	is200(t, app, "/test/john", "GET")

	grp.Connect("/CONNECT", handler)
	is200(t, app, "/test/CONNECT", "CONNECT")

	grp.Put("/PUT", handler)
	is200(t, app, "/test/PUT", "PUT")

	grp.Post("/POST", handler)
	is200(t, app, "/test/POST", "POST")

	grp.Delete("/DELETE", handler)
	is200(t, app, "/test/DELETE", "DELETE")

	grp.Head("/HEAD", handler)
	is200(t, app, "/test/HEAD", "HEAD")

	grp.Patch("/PATCH", handler)
	is200(t, app, "/test/PATCH", "PATCH")

	grp.Options("/OPTIONS", handler)
	is200(t, app, "/test/OPTIONS", "OPTIONS")

	grp.Trace("/TRACE", handler)
	is200(t, app, "/test/TRACE", "TRACE")

	grp.All("/ALL", handler)
	is200(t, app, "/test/ALL", "POST")

	grp.Use("/USE", handler)
	is200(t, app, "/test/USE/oke", "GET")

	api := grp.Group("/v1")
	api.Post("/", handler)
	is200(t, app, "/test/v1/", "POST")

	api.Get("/users", handler)
	is200(t, app, "/test/v1/users", "GET")
}

func Test_Listen(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = app.Shutdown()
	}()
	app.Listen(3002)
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = app.Shutdown()
	}()
	app.Listen("3003")
}

func Test_Serve(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
		Prefork:               true,
	})
	ln, err := net.Listen("tcp4", ":3004")
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = app.Shutdown()
	}()
	app.Serve(ln)
}
