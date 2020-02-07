package fiber

import (
	"net/http"
	"testing"
)

func Test_Methods(t *testing.T) {
	app := New()
	app.Connect(func(c *Ctx) {})
	app.Put(func(c *Ctx) {})
	app.Post(func(c *Ctx) {})
	app.Delete(func(c *Ctx) {})
	app.Head(func(c *Ctx) {})
	app.Patch(func(c *Ctx) {})
	app.Options(func(c *Ctx) {})
	app.Trace(func(c *Ctx) {})
	app.Get(func(c *Ctx) {})
	app.All("/special", func(c *Ctx) {})
	app.Use("/special/john", func(c *Ctx) {})
	req, _ := http.NewRequest("CONNECT", "/", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("PUT", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("DELETE", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("HEAD", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("PATCH", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("OPTIONS", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("TRACE", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", "/", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", "/special", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", "/special/john", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
