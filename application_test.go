package fiber

import (
	"net/http"
	"testing"
)

func Test_Methods(t *testing.T) {
	app := New()

	handler := func(c *Ctx) {}
	methods := []string{"CONNECT", "PUT", "POST", "DELETE", "HEAD", "PATCH", "OPTIONS", "TRACE", "GET", "ALL", "USE"}

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
