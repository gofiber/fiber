// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://fiber.wiki
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"net/http"
	"testing"
)

var handler = func(c *Ctx) {}

func is200(t *testing.T, app *Fiber, url string, m ...string) {

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
func Test_Methods(t *testing.T) {
	app := New()

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

func Test_Static(t *testing.T) {
	app := New()
	grp := app.Group("/v1")
	grp.Static("/v2", ".travis.yml")
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
	app := New()

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

// func Test_Listen(t *testing.T) {
// t.Parallel()
// 	app := New()
// 	app.Banner = false
// 	go func() {
// 		time.Sleep(1 * time.Second)
// 		_ = app.Shutdown()
// 	}()
// 	app.Listen(3002)
// 	go func() {
// 		time.Sleep(1 * time.Second)
// 		_ = app.Shutdown()
// 	}()
// 	app.Listen("3002")
// }
