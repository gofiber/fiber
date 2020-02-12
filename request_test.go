package fiber

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func Test_Accepts(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := ""
		result := c.Accepts(expect)
		if c.Accepts() != "" {
			t.Fatalf(`Expecting %s, got %s`, expect, result)
		}
		expect = ".xml"
		result = c.Accepts(expect)
		t.Log(result)
		if result != expect {
			t.Fatalf(`Expecting %s, got %s`, expect, result)
		}
		expect = ".whaaaaat"
		result = c.Accepts(expect)
		if result != "" {
			t.Fatalf(`Expecting %s, got %s`, "", result)
		}
	})
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_AcceptsCharsets(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.AcceptsCharsets()

		expect := "utf-8"
		result := c.AcceptsCharsets(expect)
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_AcceptsEncodings(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.AcceptsEncodings()
		expect := "gzip"
		result := c.AcceptsEncodings(expect)
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "deflate, gzip;q=1.0, *;q=0.5")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_AcceptsLanguages(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.AcceptsLanguages()
		expect := "fr"
		result := c.AcceptsLanguages(expect)
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Language", "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_BaseURL(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.BaseUrl() // deprecated
		expect := "http://google.com"
		result := c.BaseURL()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "http://google.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_BasicAuth(t *testing.T) {
	t.Parallel()
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
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Body(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/test", func(c *Ctx) {
		c.Body(1)
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
		expect = "doe"
		result = c.Body([]byte("john"))
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
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_BodyParser(t *testing.T) {
	t.Parallel()
	app := New()
	type Demo struct {
		Name string `json:"name"`
	}
	app.Post("/test", func(c *Ctx) {
		d := new(Demo)
		err := c.BodyParser(&d)
		if err != nil {
			t.Fatalf(`%s: BodyParser %v`, t.Name(), err)
		}
		if d.Name != "john" {
			t.Fatalf(`%s: Expect %s got %s`, t.Name(), "john", d)
		}
	})
	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte(`{"name":"john"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len([]byte(`{"name":"john"}`))))

	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_Cookies(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Cookies(1)
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
		expect = "doe"
		result = c.Cookies([]byte("john"))
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
	t.Parallel()
	// TODO
}
func Test_FormValue(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/test", func(c *Ctx) {
		expect := "john"
		result := c.FormValue("name")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("name", "john"); err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	writer.Close()
	req, _ := http.NewRequest("POST", "/test", body)
	contentType := fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary())

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Fresh(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Fresh()
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_Get(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "utf-8, iso-8859-1;q=0.5"
		result := c.Get("Accept-Charset")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
		expect = "Monster"
		result = c.Get("referrer")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	req.Header.Set("Referer", "Monster")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Hostname(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "google.com"
		result := c.Hostname()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "http://google.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_IP(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Ip() // deprecated
		expect := "0.0.0.0"
		result := c.IP()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "http://google.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_IPs(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Ips() // deprecated
		expect := []string{"0.0.0.0", "1.1.1.1"}
		result := c.IPs()
		if result[0] != expect[0] && result[1] != expect[1] {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "0.0.0.0, 1.1.1.1")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Is(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Is(".json")
		expect := true
		result := c.Is("html")
		if result != expect {
			t.Fatalf(`%s: Expecting %v, got %v`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Content-Type", "text/html")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Locals(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c *Ctx) {
		c.Locals("john", "doe")
		c.Next()
	})
	app.Get("/test", func(c *Ctx) {
		expect := "doe"
		result := c.Locals("john")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Method(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get(func(c *Ctx) {
		expect := "GET"
		result := c.Method()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	app.Post(func(c *Ctx) {
		expect := "POST"
		result := c.Method()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	app.Put(func(c *Ctx) {
		expect := "PUT"
		result := c.Method()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", "/test", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("PUT", "/test", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_MultipartForm(t *testing.T) {
	t.Parallel()
	app := New()
	app.Post("/test", func(c *Ctx) {
		expect := "john"
		result, err := c.MultipartForm()
		if err != nil {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, err)
		}
		if result.Value["name"][0] != expect {
			t.Fatalf(`%s: Expecting %s, got %v`, t.Name(), expect, result)
		}
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("name", "john"); err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	writer.Close()
	req, _ := http.NewRequest("POST", "/test", body)
	contentType := fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary())

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_OriginalURL(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.OriginalUrl() // deprecated
		expect := "/test?search=demo"
		result := c.OriginalURL()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "http://google.com/test?search=demo", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Params(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) {
		expect := "john"
		result := c.Params("user")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	app.Get("/test2/*", func(c *Ctx) {
		expect := "im/a/cookie"
		result := c.Params("*")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test/john", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", "/test2/im/a/cookie", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Path(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test/:user", func(c *Ctx) {
		expect := "/test/john"
		result := c.Path()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test/john", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Query(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "john"
		result := c.Query("search")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
		expect = "20"
		result = c.Query("age")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test?search=john&age=20", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Range(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Range()
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_Route(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "/test"
		result := c.Route().Path
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_SaveFile(t *testing.T) {
	t.Parallel()
	// TODO
}
func Test_Secure(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := false
		result := c.Secure()
		if result != expect {
			t.Fatalf(`%s: Expecting %v, got %v`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_SignedCookies(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.SignedCookies()
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_Stale(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Stale()
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_Subdomains(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := []string{"john", "doe"}
		result := c.Subdomains()
		if result[0] != expect[0] && result[1] != expect[1] {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "http://john.doe.google.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_XHR(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Xhr() // deprecated
		expect := true
		result := c.XHR()
		if result != expect {
			t.Fatalf(`%s: Expecting %v, got %v`, t.Name(), expect, result)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
