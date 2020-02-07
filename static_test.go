package fiber

import (
	"net/http"
	"testing"
)

func Test_Static(t *testing.T) {
	app := New()
	app.Static("./middleware")
	app.Static("/john", "./middleware")
	app.Static("*", "./middleware/cors.go")
	req, _ := http.NewRequest("GET", "/cors.go", nil)
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
	req, _ = http.NewRequest("GET", "/john/cors.go", nil)
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
