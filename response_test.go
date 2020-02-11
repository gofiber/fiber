package fiber

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

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
	app.Get("/test", func(c *Ctx) {
		options := &Cookie{
			MaxAge:   60,
			Domain:   "example.com",
			Path:     "/",
			HTTPOnly: true,
			Secure:   false,
			SameSite: "lax",
		}
		c.Cookie("name", "john", options)
	})
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf(`%s: %s`, t.Name(), err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf(`%s: StatusCode %v`, t.Name(), resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Set-Cookie"), "name=john; max-age=60; domain=example.com; path=/; HttpOnly; SameSite=Lax") {
		t.Fatalf(`%s: Expecting %s`, t.Name(), "name=john; max-age=60; domain=example.com; path=/; HttpOnly; SameSite=Lax")
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
func Test_JSONBytes(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.JsonBytes([]byte(""))
		c.JSONBytes([]byte(`{"Name":"Grame","Age":20}`))
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
func Test_JSONString(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) {
		c.JsonString("")
		c.JSONString(`{"Name":"Grame","Age":20}`)
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
func Test_XML(t *testing.T) {
	type person struct {
		Name  string `xml:"name"`
		Stars int    `xml:"stars"`
	}
	app := New()
	app.Get("/test", func(c *Ctx) {
		if err := c.Xml(""); err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		if err := c.XML(""); err != nil {
			t.Fatalf(`%s: %s`, t.Name(), err)
		}
		if err := c.XML(person{"John", 50}); err != nil {
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
	if resp.Header.Get("Content-Type") != "application/xml" {
		t.Fatalf(`%s: Expected %v`, t.Name(), "application/xml")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf(`%s: Error %s`, t.Name(), err)
	}
	if string(body) != `<person><name>John</name><stars>50</stars></person>` {
		t.Fatalf(`%s: Expecting %s`, t.Name(), `<person><name>John</name><stars>50</stars></person>`)
	}
}
