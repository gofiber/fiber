// 🚀 Fiber is an Express inspired web framework written in Go with 💖
// 📌 API Documentation: https://docs.gofiber.io
// 📝 Github Repository: https://github.com/gofiber/fiber

package fiber

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func Test_Accepts(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := ""
		result := c.Accepts(expect)
		if c.Accepts() != "" {
			t.Fatalf(`Expecting %s, got %s`, expect, result)
		}
		expect = ".xml"
		result = c.Accepts(expect)
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
	app := New()
	app.Get("/test", func(c *Ctx) {
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
func Test_Body(t *testing.T) {
	app := New()
	app.Post("/test", func(c *Ctx) {
		expect := "john=doe"
		result := c.Body()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
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
	app := New()
	type Demo struct {
		Name string `json:"name" xml:"name" form:"name" query:"name"`
	}
	app.Post("/test", func(c *Ctx) {
		d := new(Demo)
		err := c.BodyParser(d)
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

	// data := url.Values{}
	// data.Set("name", "john")
	// req = httptest.NewRequest("POST", "/test", strings.NewReader(data.Encode()))
	// req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	// req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	// _, err = app.Test(req)
	// if err != nil {
	// 	t.Fatalf(`%s: %s`, t.Name(), err)
	// }

	// req = httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte(`<name>john</name>`)))
	// req.Header.Set("Content-Type", "application/xml")
	// req.Header.Set("Content-Length", strconv.Itoa(len([]byte(`<name>john</name>`))))

	// _, err = app.Test(req)
	// if err != nil {
	// 	t.Fatalf(`%s: %s`, t.Name(), err)
	// }
}
func Test_Cookies(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "doe"
		result := c.Cookies("john")
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "john", Value: "doe"})
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_FormFile(t *testing.T) {
	app := New()
	app.Post("/test", func(c *Ctx) {
		expectFileName := "test"
		expectFileContent := "hello world"
		fh, err := c.FormFile("file")
		if err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		if fh.Filename != expectFileName {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expectFileName, fh.Filename)
		}
		f, err := fh.Open()
		if err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		b := new(bytes.Buffer)
		_, err = io.Copy(b, f)
		if err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		f.Close()
		if b.String() != expectFileContent {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expectFileContent, b.String())
		}
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}

	_, err = ioWriter.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	writer.Close()

	req, _ := http.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
}
func Test_FormValue(t *testing.T) {
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
	app := New()
	app.Get("/test", func(c *Ctx) {
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
	app := New()
	app.Get("/test", func(c *Ctx) {
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

// func Test_Is(t *testing.T) {
// 	app := New()
// 	app.Get("/test", func(c *Ctx) {
// 		c.Is(".json")
// 		expect := true
// 		result := c.Is("html")
// 		if result != expect {
// 			t.Fatalf(`%s: Expecting %v, got %v`, t.Name(), expect, result)
// 		}
// 	})
// 	req, _ := http.NewRequest("GET", "/test", nil)
// 	req.Header.Set("Content-Type", "text/html")
// 	resp, err := app.Test(req)
// 	if err != nil {
// 		t.Fatalf(`%s: %s`, t.Name(), err)
// 	}
// 	if resp.StatusCode != 200 {
// 		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
// 	}
// }
func Test_Locals(t *testing.T) {
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
	app := New()
	app.Get("/test", func(c *Ctx) {
		expect := "GET"
		result := c.Method()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	app.Post("/test", func(c *Ctx) {
		expect := "POST"
		result := c.Method()
		if result != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result)
		}
	})
	app.Put("/test", func(c *Ctx) {
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
	app := New()
	app.Get("/test", func(c *Ctx) {
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
	app := New()
	app.Get("/test", func(c *Ctx) {
		result, err := c.Range(1000)
		if err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
			return
		}
		expect := "bytes"
		if result.Type != expect {
			t.Fatalf(`%s: Expecting %s, got %s`, t.Name(), expect, result.Type)
		}
		expectNum := 500
		if result.Ranges[0].Start != expectNum {
			t.Fatalf(`%s: Expecting %v, got %v`, t.Name(), expectNum, result.Ranges[0].Start)
		}
		expectNum = 700
		if result.Ranges[0].End != expectNum {
			t.Fatalf(`%s: Expecting %v, got %v`, t.Name(), expectNum, result.Ranges[0].End)
		}
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("range", "bytes=500-700")
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
}
func Test_Route(t *testing.T) {
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
	// TODO
}
func Test_Secure(t *testing.T) {
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
func Test_Stale(t *testing.T) {
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
	app := New()
	app.Get("/test", func(c *Ctx) {
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

func Test_Append(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Append("X-Test", "hel")
		c.Append("X-Test", "lo", "world")
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("X-Test") != "hel, lo, world" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "X-Test: hel, lo, world")
	}
}
func Test_Attachment(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Attachment()
		c.Attachment("./static/img/logo.png")
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Disposition") != `attachment; filename="logo.png"` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `attachment; filename="logo.png"`)
	}
	if resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "image/png")
	}
}
func Test_ClearCookie(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.ClearCookie()
	})
	app.Get("/test2", func(c *Ctx) {
		c.ClearCookie("john")
	})
	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "john", Value: "doe"})
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "expires=") {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "expires=")
	}
	req, _ = http.NewRequest("GET", "/test2", nil)
	req.AddCookie(&http.Cookie{Name: "john", Value: "doe"})
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "expires=") {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "expires=")
	}
}
func Test_Cookie(t *testing.T) {
	app := New()
	expire := time.Now().Add(24 * time.Hour)
	var dst []byte
	dst = expire.In(time.UTC).AppendFormat(dst, time.RFC1123)
	httpdate := strings.Replace(string(dst), "UTC", "GMT", -1)
	app.Get("/test", func(c *Ctx) {
		cookie := new(Cookie)
		cookie.Name = "username"
		cookie.Value = "jon"
		cookie.Expires = expire
		c.Cookie(cookie)
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "username=jon; expires="+string(httpdate)+"; path=/") {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "username=jon; expires="+string(httpdate)+"; path=/")
	}
}
func Test_Download(t *testing.T) {
	// TODO
}
func Test_Format(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Format("Hello, World!")
	})
	app.Get("/test2", func(c *Ctx) {
		c.Format([]byte("Hello, World!"))
		c.Format("Hello, World!")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != "<p>Hello, World!</p>" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "<p>Hello, World!</p>")
	}

	req, _ = http.NewRequest("GET", "http://example.com/test2", nil)
	req.Header.Set("Accept", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `"Hello, World!"` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `"Hello, World!"`)
	}
}
func Test_HeadersSent(t *testing.T) {
	// TODO
}
func Test_JSON(t *testing.T) {
	type SomeStruct struct {
		Name string
		Age  uint8
	}
	app := New()
	app.Get("/test", func(c *Ctx) {
		if err := c.JSON(""); err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		data := SomeStruct{
			Name: "Grame",
			Age:  20,
		}
		if err := c.JSON(data); err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "application/json")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `{"Name":"Grame","Age":20}` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `{"Name":"Grame","Age":20}`)
	}
}
func Test_JSONP(t *testing.T) {
	type SomeStruct struct {
		Name string
		Age  uint8
	}
	app := New()
	app.Get("/test", func(c *Ctx) {
		if err := c.JSONP(""); err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		data := SomeStruct{
			Name: "Grame",
			Age:  20,
		}
		if err := c.JSONP(data, "alwaysjohn"); err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/javascript" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "application/javascript")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `alwaysjohn({"Name":"Grame","Age":20});` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `alwaysjohn({"Name":"Grame","Age":20});`)
	}
}
func Test_Links(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Links(
			"http://api.example.com/users?page=2", "next",
			"http://api.example.com/users?page=5", "last",
		)
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Link") != `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `Link: <http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`)
	}
}
func Test_Location(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Location("http://example.com")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Location") != "http://example.com" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "http://example.com")
	}
}
func Test_Next(t *testing.T) {
	app := New()
	app.Use("/", func(c *Ctx) {
		c.Next()
	})
	app.Get("/test", func(c *Ctx) {
		c.Set("X-Next-Result", "Works")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("X-Next-Result") != "Works" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "X-Next-Results: Works")
	}
}
func Test_Redirect(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Redirect("http://example.com", 301)
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 301 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Location") != "http://example.com" {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "Location: http://example.com")
	}
}
func Test_Render(t *testing.T) {
	// TODO
}
func Test_Send(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Send([]byte("Hello, World"))
		c.Send("Don't crash please")
		c.Send(1337)
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `1337` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `1337`)
	}
}
func Test_SendBytes(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.SendBytes([]byte("Hello, World"))
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `Hello, World` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `Hello, World`)
	}
}
func Test_SendStatus(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.SendStatus(415)
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 415 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `Unsupported Media Type` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `Unsupported Media Type`)
	}
}
func Test_SendString(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.SendString("Don't crash please")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `Don't crash please` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `Don't crash please`)
	}
}
func Test_Set(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Set("X-1", "1")
		c.Set("X-2", "2")
		c.Set("X-3", "3")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("X-1") != "1" {
		t.Fatalf(`%s: Expected %v`, t.Name(), "X-1: 1")
	}
	if resp.Header.Get("X-2") != "2" {
		t.Fatalf(`%s: Expected %v`, t.Name(), "X-2: 2")
	}
	if resp.Header.Get("X-3") != "3" {
		t.Fatalf(`%s: Expected %v`, t.Name(), "X-3: 3")
	}
}
func Test_Status(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Status(400)
		c.Status(415).Send("Hello, World")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 415 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `Hello, World` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `Hello, World`)
	}
}
func Test_Type(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Type(".json")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf(`%s: Expected %v`, t.Name(), `Content-Type: application/json`)
	}
}
func Test_Vary(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Vary("Origin")
		c.Vary("User-Agent")
		c.Vary("Accept-Encoding", "Accept")
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if resp.Header.Get("Vary") != "Origin, User-Agent, Accept-Encoding, Accept" {
		t.Fatalf(`%s: Expected %v`, t.Name(), `Vary: Origin, User-Agent, Accept-Encoding, Accept`)
	}
}
func Test_Write(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.Write("Hello, ")
		c.Write([]byte("World! "))
		c.Write(123)
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `Hello, World! 123` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `Hello, World! 123`)
	}
}
