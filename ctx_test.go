// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

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
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func Test_Ctx_Accepts(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "", c.Accepts(""))
		assertEqual(t, ".xml", c.Accepts(".xml"))
		assertEqual(t, "", c.Accepts(".john"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_AcceptsCharsets(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "utf-8", c.AcceptsCharsets("utf-8"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_AcceptsEncodings(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "gzip", c.AcceptsEncodings("gzip"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "deflate, gzip;q=1.0, *;q=0.5")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_AcceptsLanguages(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "fr", c.AcceptsLanguages("fr"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Language", "fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_BaseURL(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "http://google.com", c.BaseURL())
	})

	req := httptest.NewRequest("GET", "http://google.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Body(t *testing.T) {
	app := New()

	app.Post("/test", func(c *Ctx) {
		assertEqual(t, "john=doe", c.Body())
	})

	data := url.Values{}
	data.Set("john", "doe")

	req := httptest.NewRequest("POST", "/test", strings.NewReader(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_BodyParser(t *testing.T) {
	app := New()

	type Demo struct {
		Name string `json:"name" xml:"name" form:"name" query:"name"`
	}
	type Query struct {
		ID    int
		Name  string
		Hobby []string
	}

	app.Post("/test", func(c *Ctx) {
		d := new(Demo)
		assertEqual(t, nil, c.BodyParser(d))
		assertEqual(t, "john", d.Name)
	})

	app.Get("/query", func(c *Ctx) {
		d := new(Query)
		assertEqual(t, nil, c.BodyParser(d))
		assertEqual(t, 2, len(d.Hobby))
	})

	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer([]byte(`{"name":"john"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len([]byte(`{"name":"john"}`))))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	req = httptest.NewRequest("GET", "/query?id=1&name=tom&hobby=basketball&hobby=football", nil)

	resp, err = app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Cookies(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "doe", c.Cookies("john"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "john", Value: "doe"})

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_FormFile(t *testing.T) {
	app := New()

	app.Post("/test", func(c *Ctx) {
		fh, err := c.FormFile("file")
		assertEqual(t, nil, err)
		assertEqual(t, "test", fh.Filename)

		f, err := fh.Open()
		assertEqual(t, nil, err)

		b := new(bytes.Buffer)
		_, err = io.Copy(b, f)
		assertEqual(t, nil, err)

		f.Close()
		assertEqual(t, "hello world", b.String())
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	assertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	assertEqual(t, nil, err)

	writer.Close()

	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_FormValue(t *testing.T) {
	app := New()

	app.Post("/test", func(c *Ctx) {
		assertEqual(t, "john", c.FormValue("name"))
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	assertEqual(t, nil, writer.WriteField("name", "john"))

	writer.Close()
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Fresh(t *testing.T) {
	app := New()
	// TODO
	app.Get("/test", func(c *Ctx) {
		c.Fresh()
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Get(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "utf-8, iso-8859-1;q=0.5", c.Get("Accept-Charset"))
		assertEqual(t, "Monster", c.Get("referrer"))
	})
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Charset", "utf-8, iso-8859-1;q=0.5")
	req.Header.Set("Referer", "Monster")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Hostname(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "google.com", c.Hostname())
	})

	req := httptest.NewRequest("GET", "http://google.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_IP(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "0.0.0.0", c.IP())
	})

	req := httptest.NewRequest("GET", "http://google.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_IPs(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, []string{"0.0.0.0", "1.1.1.1"}, c.IPs())
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "0.0.0.0, 1.1.1.1")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}

// func Test_Ctx_Is(t *testing.T) {
// 	app := New()
// 	app.Get("/test", func(c *Ctx) {
// 		c.Is(".json")
// 		expect := true
// 		result := c.Is("html")
// 		if result != expect {
// 			t.Fatalf(`%s: Expecting %v, got %v`, t.Name(), expect, result)
// 		}
// 	})
// 	req := httptest.NewRequest("GET", "/test", nil)
// 	req.Header.Set("Content-Type", "text/html")
// 	resp, err := app.Test(req)
// 	if err != nil {
// 		t.Fatalf(`%s: %s`, t.Name(), err)
// 	}
// 	if resp.StatusCode != 200 {
// 		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
// 	}
// }
func Test_Ctx_Locals(t *testing.T) {
	app := New()

	app.Use(func(c *Ctx) {
		c.Locals("john", "doe")
		c.Next()
	})
	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "doe", c.Locals("john"))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Method(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "GET", c.Method())
	})
	app.Post("/test", func(c *Ctx) {
		assertEqual(t, "POST", c.Method())
	})
	app.Put("/test", func(c *Ctx) {
		assertEqual(t, "PUT", c.Method())
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("POST", "/test", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("PUT", "/test", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_MultipartForm(t *testing.T) {
	app := New()

	app.Post("/test", func(c *Ctx) {
		result, err := c.MultipartForm()
		assertEqual(t, nil, err)
		assertEqual(t, "john", result.Value["name"][0])
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	assertEqual(t, nil, writer.WriteField("name", "john"))

	writer.Close()
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_OriginalURL(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "http://google.com/test?search=demo", c.OriginalURL())
	})

	resp, err := app.Test(httptest.NewRequest("GET", "http://google.com/test?search=demo", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Params(t *testing.T) {
	app := New()

	app.Get("/test/:user", func(c *Ctx) {
		assertEqual(t, "john", c.Params("user"))
	})

	app.Get("/test2/*", func(c *Ctx) {
		assertEqual(t, "im/a/cookie", c.Params("*"))
	})

	app.Get("/test3/:optional?", func(c *Ctx) {
		assertEqual(t, "", c.Params("optional"))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/test/john", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("GET", "/test2/im/a/cookie", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("GET", "/test3", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Path(t *testing.T) {
	app := New()

	app.Get("/test/:user", func(c *Ctx) {
		assertEqual(t, "/test/john", c.Path())
	})

	req := httptest.NewRequest("GET", "/test/john", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Query(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "john", c.Query("search"))
		assertEqual(t, "20", c.Query("age"))
	})
	req := httptest.NewRequest("GET", "/test?search=john&age=20", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Range(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		result, err := c.Range(1000)
		assertEqual(t, nil, err)
		assertEqual(t, "bytes", result.Type)
		assertEqual(t, 500, result.Ranges[0].Start)
		assertEqual(t, 700, result.Ranges[0].End)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("range", "bytes=500-700")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Route(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, "/test", c.Route().Path)
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_SaveFile(t *testing.T) {
	app := New()

	app.Post("/test", func(c *Ctx) {
		fh, err := c.FormFile("file")
		assertEqual(t, nil, err)

		tempFile, err := ioutil.TempFile(os.TempDir(), "test-")
		assertEqual(t, nil, err)

		defer os.Remove(tempFile.Name())
		err = c.SaveFile(fh, tempFile.Name())
		assertEqual(t, nil, err)

		bs, err := ioutil.ReadFile(tempFile.Name())
		assertEqual(t, nil, err)
		assertEqual(t, "hello world", string(bs))
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	ioWriter, err := writer.CreateFormFile("file", "test")
	assertEqual(t, nil, err)

	_, err = ioWriter.Write([]byte("hello world"))
	assertEqual(t, nil, err)
	writer.Close()

	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", strconv.Itoa(len(body.Bytes())))

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Secure(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, false, c.Secure())
	})

	// app.Get("/secure", func(c *Ctx) {
	// 	assertEqual(t, true, c.Secure())
	// })

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	// req = httptest.NewRequest("GET", "https://google.com/secure", nil)

	// resp, err = app.Test(req)
	// assertEqual(t, nil, err, "app.Test(req)")
	// assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Stale(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Stale()
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
func Test_Ctx_Subdomains(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, []string{"john", "doe"}, c.Subdomains())
	})

	req := httptest.NewRequest("GET", "http://john.doe.google.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_Ctx_Append(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Append("X-Test", "Hello")
		c.Append("X-Test", "World")
		c.Append("X-Test", "Hello", "World")
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "Hello, World", resp.Header.Get("X-Test"))
}
func Test_Ctx_Attachment(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Attachment()
		c.Attachment("./static/img/logo.png")
	})
	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, `attachment; filename="logo.png"`, resp.Header.Get("Content-Disposition"))
	assertEqual(t, "image/png", resp.Header.Get("Content-Type"))
}

func Test_Ctx_ClearCookie(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.ClearCookie()
	})

	app.Get("/test2", func(c *Ctx) {
		c.ClearCookie("john")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "john", Value: "doe"})

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, true, strings.Contains(resp.Header.Get("Set-Cookie"), "expires="))

	req = httptest.NewRequest("GET", "/test2", nil)
	req.AddCookie(&http.Cookie{Name: "john", Value: "doe"})

	resp, err = app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, true, strings.Contains(resp.Header.Get("Set-Cookie"), "expires="))
}
func Test_Ctx_Cookie(t *testing.T) {
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
	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	expireDate := "username=jon; expires=" + string(httpdate) + "; path=/"
	assertEqual(t, true, strings.Contains(resp.Header.Get("Set-Cookie"), expireDate))
}
func Test_Ctx_Download(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Download("ctx.go")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)

	f, err := os.Open("./ctx.go")
	assertEqual(t, nil, err)

	defer f.Close()

	expect, err := ioutil.ReadAll(f)
	assertEqual(t, nil, err)
	assertEqual(t, true, bytes.Equal(expect, body))
}
func Test_Ctx_Format(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Format("Hello, World!")
	})

	app.Get("/test2", func(c *Ctx) {
		c.Format([]byte("Hello, World!"))
		c.Format("Hello, World!")
	})
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("Accept", "text/html")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, "<p>Hello, World!</p>", string(body))

	req = httptest.NewRequest("GET", "http://example.com/test2", nil)
	req.Header.Set("Accept", "application/json")

	resp, err = app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err = ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `"Hello, World!"`, string(body))
}

func Test_Ctx_JSON(t *testing.T) {
	app := New()

	type SomeStruct struct {
		Name string
		Age  uint8
	}

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, nil, c.JSON(""))

		data := SomeStruct{
			Name: "Grame",
			Age:  20,
		}
		assertEqual(t, nil, c.JSON(data))
	})
	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `{"Name":"Grame","Age":20}`, string(body))
}
func Test_Ctx_JSONP(t *testing.T) {
	app := New()

	type SomeStruct struct {
		Name string
		Age  uint8
	}

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, nil, c.JSONP(""))

		data := SomeStruct{
			Name: "Grame",
			Age:  20,
		}
		assertEqual(t, nil, c.JSONP(data, "john"))
	})
	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "application/javascript", resp.Header.Get("Content-Type"))

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `john({"Name":"Grame","Age":20});`, string(body))
}
func Test_Ctx_Links(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Links(
			"http://api.example.com/users?page=2", "next",
			"http://api.example.com/users?page=5", "last",
		)
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, `<http://api.example.com/users?page=2>; rel="next",<http://api.example.com/users?page=5>; rel="last"`, resp.Header.Get("Link"))
}
func Test_Ctx_Location(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Location("http://example.com")
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "http://example.com", resp.Header.Get("Location"))
}
func Test_Ctx_Next(t *testing.T) {
	app := New()

	app.Use("/", func(c *Ctx) {
		c.Next()
	})

	app.Get("/test", func(c *Ctx) {
		c.Set("X-Next-Result", "Works")
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "Works", resp.Header.Get("X-Next-Result"))
}
func Test_Ctx_Redirect(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Redirect("http://example.com", 301)
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 301, resp.StatusCode, "Status code")
	assertEqual(t, "http://example.com", resp.Header.Get("Location"))
}
func Test_Ctx_Render(t *testing.T) {
	// TODO
}
func Test_Ctx_Send(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Send([]byte("Hello, World"))
		c.Send("Don't crash please")
		c.Send(1337)
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `1337`, string(body))
}
func Test_Ctx_SendBytes(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.SendBytes([]byte("Hello, World"))
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `Hello, World`, string(body))
}
func Test_Ctx_SendStatus(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.SendStatus(415)
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 415, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `Unsupported Media Type`, string(body))
}
func Test_Ctx_SendString(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.SendString("Don't crash please")
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `Don't crash please`, string(body))
}
func Test_Ctx_Set(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Set("X-1", "1")
		c.Set("X-2", "2")
		c.Set("X-3", "3")
		c.Set("X-3", "1337")
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "1", resp.Header.Get("X-1"))
	assertEqual(t, "2", resp.Header.Get("X-2"))
	assertEqual(t, "1337", resp.Header.Get("X-3"))
}
func Test_Ctx_Status(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Status(400)
		c.Status(415).Send("Hello, World")
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 415, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `Hello, World`, string(body))
}
func Test_Ctx_Type(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Type(".json")
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "application/json", resp.Header.Get("Content-Type"))
}
func Test_Ctx_Vary(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Vary("Origin")
		c.Vary("User-Agent")
		c.Vary("Accept-Encoding", "Accept")
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
	assertEqual(t, "Origin, User-Agent, Accept-Encoding, Accept", resp.Header.Get("Vary"))
}
func Test_Ctx_Write(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Write("Hello, ")
		c.Write([]byte("World! "))
		c.Write(123)
	})

	req := httptest.NewRequest("GET", "http://example.com/test", nil)

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	assertEqual(t, nil, err)
	assertEqual(t, `Hello, World! 123`, string(body))
}

func Test_Ctx_XHR(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		assertEqual(t, true, c.XHR())
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := app.Test(req)
	assertEqual(t, nil, err, "app.Test(req)")
	assertEqual(t, 200, resp.StatusCode, "Status code")
}
