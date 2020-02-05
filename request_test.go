package fiber

import (
	"strings"
	"testing"
)

func Test_Accepts(t *testing.T) {
	// Raw http request
	req := "GET / HTTP/1.1\r\nHost: localhost:8080\r\nAccept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n\r\n"
	// Create fiber app
	app := New()
	app.Get("/", func(c *Ctx) {
		expecting := "html"
		result := c.Accepts(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}

		expecting = ".xml"
		result = c.Accepts(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}
	})
	// Send fake request
	res, err := app.FakeRequest(req)
	// Check for errors and if route was handled
	if err != nil || !strings.Contains(res, "HTTP/1.1 200 OK") {
		t.Fatalf(`%s: Error serving FakeRequest %s`, t.Name(), err)
	}
}
func Test_AcceptsCharsets(t *testing.T) {
	// Raw http request
	req := "GET / HTTP/1.1\r\nHost: localhost:8080\r\nAccept-Charset: utf-8, iso-8859-1;q=0.5\r\n\r\n"
	// Raw http request
	app := New()
	app.Get("/", func(c *Ctx) {
		expecting := "utf-8"
		result := c.AcceptsCharsets(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}

		expecting = "iso-8859-1"
		result = c.AcceptsCharsets(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}
	})
	// Send fake request
	res, err := app.FakeRequest(req)
	// Check for errors and if route was handled
	if err != nil || !strings.Contains(res, "HTTP/1.1 200 OK") {
		t.Fatalf(`%s: Error serving FakeRequest %s`, t.Name(), err)
	}
}
func Test_AcceptsEncodings(t *testing.T) {
	// Raw http request
	req := "GET / HTTP/1.1\r\nHost: localhost:8080\r\nAccept-Encoding: deflate, gzip;q=1.0, *;q=0.5\r\n\r\n"
	// Raw http request
	app := New()
	app.Get("/", func(c *Ctx) {
		expecting := "gzip"
		result := c.AcceptsEncodings(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}

		expecting = "*"
		result = c.AcceptsEncodings(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}
	})
	// Send fake request
	res, err := app.FakeRequest(req)
	// Check for errors and if route was handled
	if err != nil || !strings.Contains(res, "HTTP/1.1 200 OK") {
		t.Fatalf(`%s: Error serving FakeRequest %s`, t.Name(), err)
	}
}
func Test_AcceptsLanguages(t *testing.T) {
	// Raw http request
	req := "GET / HTTP/1.1\r\nHost: localhost:8080\r\nAccept-Language: fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5\r\n\r\n"
	// Raw http request
	app := New()
	app.Get("/", func(c *Ctx) {
		expecting := "fr"
		result := c.AcceptsLanguages(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}

		expecting = "en"
		result = c.AcceptsLanguages(expecting)
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}
	})
	// Send fake request
	res, err := app.FakeRequest(req)
	// Check for errors and if route was handled
	if err != nil || !strings.Contains(res, "HTTP/1.1 200 OK") {
		t.Fatalf(`%s: Error serving FakeRequest %s`, t.Name(), err)
	}
}
func Test_BaseURL(t *testing.T) {
	// Raw http request
	req := "GET / HTTP/1.1\r\nHost: localhost:8080\r\n\r\n"
	// Raw http request
	app := New()
	app.Get("/", func(c *Ctx) {
		expecting := "http://localhost:8080"
		result := c.BaseURL()
		if result != expecting {
			t.Fatalf(`%s: Expecting %s`, t.Name(), expecting)
		}
	})
	// Send fake request
	res, err := app.FakeRequest(req)
	// Check for errors and if route was handled
	if err != nil || !strings.Contains(res, "HTTP/1.1 200 OK") {
		t.Fatalf(`%s: Error serving FakeRequest %s`, t.Name(), err)
	}
}
func Test_BasicAuth(t *testing.T) {
	// Raw http request
	req := "GET / HTTP/1.1\r\nHost: localhost:8080\r\nAuthorization: Basic am9objpkb2U=\r\n\r\n"
	// Raw http request
	app := New()
	app.Get("/", func(c *Ctx) {
		user, pass, ok := c.BasicAuth()
		if !ok {
			t.Fatalf(`%s: Expecting %s`, t.Name(), "ok")
		}
		if user != "john" || pass != "doe" {
			if !ok {
				t.Fatalf(`%s: Expecting john & doe`, t.Name())
			}
		}
	})
	// Send fake request
	res, err := app.FakeRequest(req)
	// Check for errors and if route was handled
	if err != nil || !strings.Contains(res, "HTTP/1.1 200 OK") {
		t.Fatalf(`%s: Error serving FakeRequest %s`, t.Name(), err)
	}
}

// TODO: add all functions from request.go
