package fiber

import (
	"net/http"
	"testing"
)

var handler = func(c *Ctx) {}

func Test_Methods(t *testing.T) {
	app := New()

	methods := []string{"CONNECT", "PUT", "POST", "DELETE", "HEAD", "PATCH", "OPTIONS", "TRACE", "GET", "ALL", "USE"}
	app.Connect("", handler)
	app.Connect("/CONNECT", handler)
	app.Put("/PUT", handler)
	app.Post("/POST", handler)
	app.Delete("/DELETE", handler)
	app.Head("/HEAD", handler)
	app.Patch("/PATCH", handler)
	app.Options("/OPTIONS", handler)
	app.Trace("/TRACE", handler)
	app.Get("/GET", handler)
	app.All("/ALL", handler)
	app.Use("/USE", handler)

	for _, method := range methods {
		var req *http.Request
		if method == "ALL" {
			req, _ = http.NewRequest("CONNECT", "/"+method, nil)
		} else if method == "USE" {
			req, _ = http.NewRequest("OPTIONS", "/"+method+"/test", nil)
		} else {
			req, _ = http.NewRequest(method, "/"+method, nil)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf(`%s: %s %s`, t.Name(), method, err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf(`%s: %s expecting 200 but received %v`, t.Name(), method, resp.StatusCode)
		}
	}
}

func Test_Static(t *testing.T) {
	app := New()
	app.Static("./.github")
	app.Static("/john", "./.github")
	app.Static("*", "./.github/stale.yml")
	req, _ := http.NewRequest("GET", "/stale.yml", nil)
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
}
func Test_Group(t *testing.T) {
	app := New()
	grp := app.Group("/test")
	grp.Get("/", handler)
	grp.Get("/:demo?", handler)
	grp.Connect("/CONNECT", handler)
	grp.Put("/PUT", handler)
	grp.Post("/POST", handler)
	grp.Delete("/DELETE", handler)
	grp.Head("/HEAD", handler)
	grp.Patch("/PATCH", handler)
	grp.Options("/OPTIONS", handler)
	grp.Trace("/TRACE", handler)
	grp.All("/ALL", handler)
	grp.Use("/USE", handler)
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", "/test/test", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}

// func Test_Listen(t *testing.T) {
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
