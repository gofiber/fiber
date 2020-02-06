package fiber

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func Test_Accepts(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := ".xml"
		result := c.Accepts(expect)
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_AcceptsCharsets(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "utf-8"
		result := c.AcceptsCharsets(expect)
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_AcceptsEncodings(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "gzip"
		result := c.AcceptsEncodings(expect)
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "deflate, gzip;q=1.0, *;q=0.5")
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_AcceptsLanguages(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "fr"
		result := c.AcceptsLanguages(expect)
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_BaseURL(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "http://google.com"
		result := c.BaseURL()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "http://google.com/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_BasicAuth(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect1 := "john"
		expect2 := "doe"
		result1, result2, _ := c.BasicAuth()
		if result1 != expect1 {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect1, expect1)
		}
		if result2 != expect2 {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), result2, expect2)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("john", "doe")
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_Body(t *testing.T) {
	app := New()
	app.Post("/test", func(c *Ctx) {
		expect := "john=doe"
		result := c.Body()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
		expect = "doe"
		result = c.Body("john")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
		c.Body(func(k, v string) {
			expect = "john"
			if k != "john" {
				t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, k)
			}
			expect = "doe"
			if v != "doe" {
				t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, v)
			}
		})
	})
	data := url.Values{}
	data.Set("john", "doe")
	req, _ := http.NewRequest("POST", "/test", strings.NewReader(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_Cookies(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "john=doe"
		result := c.Cookies()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
		expect = "doe"
		result = c.Cookies("john")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
		c.Cookies(func(k, v string) {
			expect = "john"
			if k != "john" {
				t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, k)
			}
			expect = "doe"
			if v != "doe" {
				t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, v)
			}
		})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "john", Value: "doe"})
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_FormFile(t *testing.T) {

}

// TODO: add all functions from request.go
