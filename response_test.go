package fiber

import (
	"strings"
	"testing"
)

func Test_Append(t *testing.T) {
	// Raw http request
	req := "GET / HTTP/1.1\r\nHost: localhost:8080\r\n\r\n"
	// Create fiber app
	app := New()
	app.Get("/", func(c *Ctx) {
		c.Append("X-Test", "hello", "world")
	})
	// Send fake request
	res, err := app.FakeRequest(req)
	// Check for errors and if route was handled
	if err != nil || !strings.Contains(res, "HTTP/1.1 200 OK") {
		t.Fatalf(`%s: Error serving FakeRequest %s`, t.Name(), err)
	}
	// Check if function works correctly
	if !strings.Contains(res, "X-Test: hello, world") {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "X-Test: hello, world")
	}
}

// TODO: add all functions from response.go
