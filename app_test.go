// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"io/ioutil"
	"net"
	"net/http/httptest"
	"testing"
	"time"
)

func testStatus200(t *testing.T, app *App, url string, method string) {
	req := httptest.NewRequest(method, url, nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Nested_Params(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Status(400).Send("Should move on")
	})
	app.Get("/test/:param", func(c *Ctx) {
		c.Status(400).Send("Should move on")
	})
	app.Get("/test/:param/test", func(c *Ctx) {
		c.Status(400).Send("Should move on")
	})
	app.Get("/test/:param/test/:param2", func(c *Ctx) {
		c.Status(200).Send("Good job")
	})

	req := httptest.NewRequest("GET", "/test/john/test/doe", nil)
	resp, err := app.Test(req)

	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Use_Params(t *testing.T) {
	app := New()

	app.Use("/prefix/:param", func(c *Ctx) {
		assertEqual(t, "john", c.Params("param"))
	})

	app.Use("/:param/*", func(c *Ctx) {
		assertEqual(t, "john", c.Params("param"))
		assertEqual(t, "doe", c.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/prefix/john", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("GET", "/john/doe", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_App_Order(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Write("1")
		c.Next()
	})

	app.All("/test", func(c *Ctx) {
		c.Write("2")
		c.Next()
	})

	app.Use(func(c *Ctx) {
		c.Write("3")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, "123", string(body))
}
func Test_App_Methods(t *testing.T) {

	var dummyHandler = func(c *Ctx) {}

	app := New()

	app.Connect("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "CONNECT")

	app.Put("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "CONNECT")

	app.Post("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "POST")

	app.Delete("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "DELETE")

	app.Head("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "HEAD")

	app.Patch("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "PATCH")

	app.Options("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "OPTIONS")

	app.Trace("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "TRACE")

	app.Get("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "GET")

	app.All("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "POST")

	app.Use("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "GET")

}

func Test_App_New(t *testing.T) {
	app := New()
	app.Get("/", func(*Ctx) {

	})

	appConfig := New(&Settings{
		Immutable: true,
	})
	appConfig.Get("/", func(*Ctx) {

	})
}

func Test_App_Shutdown(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})
	_ = app.Shutdown()
}

func Test_App_Static(t *testing.T) {
	app := New()

	grp := app.Group("/v1")

	grp.Static("/v2", ".github/auth_assign.yml")
	app.Static("/*", ".github/FUNDING.yml")
	app.Static("/john", "./.github")

	req := httptest.NewRequest("GET", "/john/stale.yml", nil)
	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, false, resp.Header.Get("Content-Length") == "")

	req = httptest.NewRequest("GET", "/yesyes/john/doe", nil)
	resp, err = app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, false, resp.Header.Get("Content-Length") == "")

	req = httptest.NewRequest("GET", "/john/stale.yml", nil)
	resp, err = app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, false, resp.Header.Get("Content-Length") == "")

	req = httptest.NewRequest("GET", "/v1/v2", nil)
	resp, err = app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, false, resp.Header.Get("Content-Length") == "")
}

func Test_App_Group(t *testing.T) {
	var dummyHandler = func(c *Ctx) {}

	app := New()

	grp := app.Group("/test")
	grp.Get("/", dummyHandler)
	testStatus200(t, app, "/test", "GET")

	grp.Get("/:demo?", dummyHandler)
	testStatus200(t, app, "/test/john", "GET")

	grp.Connect("/CONNECT", dummyHandler)
	testStatus200(t, app, "/test/CONNECT", "CONNECT")

	grp.Put("/PUT", dummyHandler)
	testStatus200(t, app, "/test/PUT", "PUT")

	grp.Post("/POST", dummyHandler)
	testStatus200(t, app, "/test/POST", "POST")

	grp.Delete("/DELETE", dummyHandler)
	testStatus200(t, app, "/test/DELETE", "DELETE")

	grp.Head("/HEAD", dummyHandler)
	testStatus200(t, app, "/test/HEAD", "HEAD")

	grp.Patch("/PATCH", dummyHandler)
	testStatus200(t, app, "/test/PATCH", "PATCH")

	grp.Options("/OPTIONS", dummyHandler)
	testStatus200(t, app, "/test/OPTIONS", "OPTIONS")

	grp.Trace("/TRACE", dummyHandler)
	testStatus200(t, app, "/test/TRACE", "TRACE")

	grp.All("/ALL", dummyHandler)
	testStatus200(t, app, "/test/ALL", "POST")

	grp.Use("/USE", dummyHandler)
	testStatus200(t, app, "/test/USE/oke", "GET")

	api := grp.Group("/v1")
	api.Post("/", dummyHandler)
	testStatus200(t, app, "/test/v1/", "POST")

	api.Get("/users", dummyHandler)
	testStatus200(t, app, "/test/v1/users", "GET")
}

func Test_App_Listen(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})
	go func() {
		time.Sleep(1 * time.Millisecond)
		_ = app.Shutdown()
	}()
	err := app.Listen(3002)
	assertEqual(t, nil, err)
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = app.Shutdown()
	}()
	err = app.Listen("3003")
	assertEqual(t, nil, err)
}

func Test_App_Serve(t *testing.T) {
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
	err = app.Serve(ln)
	assertEqual(t, nil, err)
}
