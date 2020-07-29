// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	utils "github.com/gofiber/utils"
	fasthttp "github.com/valyala/fasthttp"
	fasthttputil "github.com/valyala/fasthttp/fasthttputil"
)

func testStatus200(t *testing.T, app *App, url string, method string) {
	req := httptest.NewRequest(method, url, nil)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_MethodNotAllowed(t *testing.T) {
	app := New()

	app.Use(func(ctx *Ctx) { ctx.Next() })

	app.Post("/", func(c *Ctx) {})

	app.Options("/", func(c *Ctx) {})

	resp, err := app.Test(httptest.NewRequest("POST", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, "", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest("PATCH", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest("PUT", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	app.Get("/", func(c *Ctx) {})

	resp, err = app.Test(httptest.NewRequest("TRACE", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest("PATCH", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest("PUT", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))
}

func Test_App_Custom_Middleware_404_Should_Not_SetMethodNotAllowed(t *testing.T) {
	app := New()

	app.Use(func(ctx *Ctx) {
		ctx.Status(404)
	})

	app.Post("/", func(c *Ctx) {})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 404, resp.StatusCode)

	g := app.Group("/with-next", func(ctx *Ctx) {
		ctx.Status(404)
		ctx.Next()
	})

	g.Post("/", func(c *Ctx) {})

	resp, err = app.Test(httptest.NewRequest("GET", "/with-next", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 404, resp.StatusCode)
}

func Test_App_ServerErrorHandler_SmallReadBuffer(t *testing.T) {
	expectedError := regexp.MustCompile(
		`error when reading request headers: small read buffer\. Increase ReadBufferSize\. Buffer size=4096, contents: "GET / HTTP/1.1\\r\\nHost: example\.com\\r\\nVery-Long-Header: -+`,
	)
	app := New()

	app.Get("/", func(c *Ctx) {
		panic(errors.New("should never called"))
	})

	request := httptest.NewRequest("GET", "/", nil)
	logHeaderSlice := make([]string, 5000, 5000)
	request.Header.Set("Very-Long-Header", strings.Join(logHeaderSlice, "-"))
	_, err := app.Test(request)

	if err == nil {
		t.Error("Expect an error at app.Test(request)")
	}

	utils.AssertEqual(
		t,
		true,
		expectedError.MatchString(err.Error()),
		fmt.Sprintf("Has: %s, expected pattern: %s", err.Error(), expectedError.String()),
	)
}

func Test_App_ErrorHandler(t *testing.T) {
	app := New(&Settings{
		BodyLimit: 4,
	})

	app.Get("/", func(c *Ctx) {
		c.Next(errors.New("hi, i'm an error"))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hi, i'm an error", string(body))

	_, err = app.Test(httptest.NewRequest("GET", "/", strings.NewReader("big body")))
	if err != nil {
		utils.AssertEqual(t, "body size exceeds the given limit", err.Error(), "app.Test(req)")
	}
}

func Test_App_ErrorHandler_Custom(t *testing.T) {
	app := New(&Settings{
		ErrorHandler: func(ctx *Ctx, err error) {
			ctx.Status(200).SendString("hi, i'm an custom error")
		},
	})

	app.Get("/", func(c *Ctx) {
		c.Next(errors.New("hi, i'm an error"))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hi, i'm an custom error", string(body))
}

func Test_App_Nested_Params(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Status(400).Send("Should move on")
	})
	app.Get("/test/:param", func(c *Ctx) {
		c.Status(400).Send("Should move on")
	})
	app.Get("/test/:param/test", func(c *Ctx) {
		c.Status(400).Send("Should move on")
	})
	app.Get("/test/:param/test/:param2", func(c *Ctx) {
		c.Status(200).Send("Good job")
	})

	req := httptest.NewRequest("GET", "/test/john/test/doe", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Use_Params(t *testing.T) {
	app := New()

	app.Use("/prefix/:param", func(c *Ctx) {
		utils.AssertEqual(t, "john", c.Params("param"))
	})

	app.Use("/foo/:bar?", func(c *Ctx) {
		utils.AssertEqual(t, "foobar", c.Params("bar", "foobar"))
	})

	app.Use("/:param/*", func(c *Ctx) {
		utils.AssertEqual(t, "john", c.Params("param"))
		utils.AssertEqual(t, "doe", c.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/prefix/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("GET", "/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("GET", "/foo", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "use: invalid handler func()\n", fmt.Sprintf("%v", err))
		}
	}()

	app.Use("/:param/*", func() {
		// this should panic
	})
}

func Test_App_Add_Method_Test(t *testing.T) {
	app := New()
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "add: invalid http method JOHN\n", fmt.Sprintf("%v", err))
		}
	}()
	app.Add("JOHN", "/doe", func(c *Ctx) {

	})
}

func Test_App_Listen_TLS(t *testing.T) {
	app := New()

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/TEST_DATA/ssl.pem", "./.github/TEST_DATA/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(3078, config))
}

func Test_App_Listener_TLS(t *testing.T) {
	app := New()

	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/TEST_DATA/ssl.pem", "./.github/TEST_DATA/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	ln, err := net.Listen("tcp4", ":3055")
	utils.AssertEqual(t, nil, err)

	utils.AssertEqual(t, nil, app.Listener(ln, config))
}
func Test_App_Use_Params_Group(t *testing.T) {
	app := New()

	group := app.Group("/prefix/:param/*")
	group.Use("/", func(c *Ctx) {
		c.Next()
	})
	group.Get("/test", func(c *Ctx) {
		utils.AssertEqual(t, "john", c.Params("param"))
		utils.AssertEqual(t, "doe", c.Params("*"))
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/prefix/john/doe/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Chaining(t *testing.T) {
	n := func(c *Ctx) {
		c.Next()
	}
	app := New()
	app.Use("/john", n, n, n, n, func(c *Ctx) {
		c.Status(202)
	})
	// check handler count for registered HEAD route
	utils.AssertEqual(t, 5, len(app.stack[methodInt(MethodHead)][0].Handlers), "app.Test(req)")

	req := httptest.NewRequest("POST", "/john", nil)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 202, resp.StatusCode, "Status code")

	app.Get("/test", n, n, n, n, func(c *Ctx) {
		c.Status(203)
	})

	req = httptest.NewRequest("GET", "/test", nil)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 203, resp.StatusCode, "Status code")

}

func Test_App_Order(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) {
		c.Write("1")
		c.Next()
	})

	app.All("/test", func(c *Ctx) {
		c.Write("2")
		c.Next()
	})

	app.Use(func(c *Ctx) {
		c.Write("3")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "123", string(body))
}
func Test_App_Methods(t *testing.T) {
	var dummyHandler = func(c *Ctx) {}

	app := New()

	app.Connect("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "CONNECT")

	app.Put("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "PUT")

	app.Post("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "POST")

	app.Delete("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "DELETE")

	app.Head("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "HEAD")

	app.Patch("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "PATCH")

	app.Options("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "OPTIONS")

	app.Trace("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "TRACE")

	app.Get("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "GET")

	app.All("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "POST")

	app.Use("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "GET")

}

func Test_App_New(t *testing.T) {
	app := New()
	app.Get("/", func(*Ctx) {

	})

	appConfig := New(&Settings{
		Immutable: true,
	})
	appConfig.Get("/", func(*Ctx) {

	})
}

func Test_App_Shutdown(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})
	if err := app.Shutdown(); err != nil {
		if err.Error() != "shutdown: server is not running" {
			t.Fatal()
		}
	}
}

// go test -run Test_App_Static_Index_Default
func Test_App_Static_Index_Default(t *testing.T) {
	app := New()

	app.Static("/prefix", "./.github/workflows")
	app.Static("", "./.github/")
	app.Static("test", "", Static{Index: "index.html"})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest("GET", "/not-found", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Cannot GET /not-found", string(body))
}

// go test -run Test_App_Static_Index
func Test_App_Static_Direct(t *testing.T) {
	app := New()

	app.Static("/", "./.github")

	resp, err := app.Test(httptest.NewRequest("GET", "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest("GET", "/FUNDING.yml", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "gofiber.io/support"))
}
func Test_App_Static_Group(t *testing.T) {
	app := New()

	grp := app.Group("/v1", func(c *Ctx) {
		c.Set("Test-Header", "123")
		c.Next()
	})

	grp.Static("/v2", "./.github/FUNDING.yml")

	req := httptest.NewRequest("GET", "/v1/v2", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
	utils.AssertEqual(t, "123", resp.Header.Get("Test-Header"))

	grp = app.Group("/v2")
	grp.Static("/v3*", "./.github/FUNDING.yml")

	req = httptest.NewRequest("GET", "/v2/v3/john/doe", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

}

func Test_App_Static_Wildcard(t *testing.T) {
	app := New()

	app.Static("*", "./.github/FUNDING.yml")

	req := httptest.NewRequest("GET", "/yesyes/john/doe", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "gofiber.io/support"))

}

func Test_App_Static_Prefix_Wildcard(t *testing.T) {
	app := New()

	app.Static("/test/*", "./.github/FUNDING.yml")

	req := httptest.NewRequest("GET", "/test/john/doe", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	app.Static("/my/nameisjohn*", "./.github/FUNDING.yml")

	resp, err = app.Test(httptest.NewRequest("GET", "/my/nameisjohn/no/its/not", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "gofiber.io/support"))
}

func Test_App_Static_Prefix(t *testing.T) {
	app := New()
	app.Static("/john", "./.github")

	req := httptest.NewRequest("GET", "/john/stale.yml", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	app.Static("/prefix", "./.github/workflows")

	req = httptest.NewRequest("GET", "/prefix/test.yml", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))

	app.Static("/single", "./.github/workflows/test.yml")

	req = httptest.NewRequest("GET", "/single", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
}

// go test -run Test_App_Mixed_Routes_WithSameLen
func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	app := New()

	// middleware
	app.Use(func(ctx *Ctx) {
		ctx.Set("TestHeader", "TestValue")
		ctx.Next()
	})
	// routes with the same length
	app.Static("/tesbar", "./.github")
	app.Get("/foobar", func(ctx *Ctx) {
		ctx.Send("FOO_BAR")
		ctx.Type("html")
	})

	// match get route
	req := httptest.NewRequest("GET", "/foobar", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "TestValue", resp.Header.Get("TestHeader"))
	utils.AssertEqual(t, "text/html", resp.Header.Get("Content-Type"))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "FOO_BAR", string(body))

	// match static route
	req = httptest.NewRequest("GET", "/tesbar", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get("Content-Length") == "")
	utils.AssertEqual(t, "TestValue", resp.Header.Get("TestHeader"))
	utils.AssertEqual(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"), "Response: "+string(body))
	utils.AssertEqual(t, true, strings.HasPrefix(string(body), "<!DOCTYPE html>"), "Response: "+string(body))
}

func Test_App_Group_Invalid(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "use: invalid handler int\n", fmt.Sprintf("%v", err))
		}
	}()
	New().Group("/").Use(1)
}

func Test_App_Group(t *testing.T) {
	var dummyHandler = func(c *Ctx) {}

	app := New()

	grp := app.Group("/test")
	grp.Get("/", dummyHandler)
	testStatus200(t, app, "/test", "GET")

	grp.Get("/:demo?", dummyHandler)
	testStatus200(t, app, "/test/john", "GET")

	grp.Connect("/CONNECT", dummyHandler)
	testStatus200(t, app, "/test/CONNECT", "CONNECT")

	grp.Put("/PUT", dummyHandler)
	testStatus200(t, app, "/test/PUT", "PUT")

	grp.Post("/POST", dummyHandler)
	testStatus200(t, app, "/test/POST", "POST")

	grp.Delete("/DELETE", dummyHandler)
	testStatus200(t, app, "/test/DELETE", "DELETE")

	grp.Head("/HEAD", dummyHandler)
	testStatus200(t, app, "/test/HEAD", "HEAD")

	grp.Patch("/PATCH", dummyHandler)
	testStatus200(t, app, "/test/PATCH", "PATCH")

	grp.Options("/OPTIONS", dummyHandler)
	testStatus200(t, app, "/test/OPTIONS", "OPTIONS")

	grp.Trace("/TRACE", dummyHandler)
	testStatus200(t, app, "/test/TRACE", "TRACE")

	grp.All("/ALL", dummyHandler)
	testStatus200(t, app, "/test/ALL", "POST")

	grp.Use("/USE", dummyHandler)
	testStatus200(t, app, "/test/USE/oke", "GET")

	api := grp.Group("/v1")
	api.Post("/", dummyHandler)

	resp, err := app.Test(httptest.NewRequest("POST", "/test/v1/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	//utils.AssertEqual(t, "/test/v1", resp.Header.Get("Location"), "Location")

	api.Get("/users", dummyHandler)
	resp, err = app.Test(httptest.NewRequest("GET", "/test/v1/UsErS", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	//utils.AssertEqual(t, "/test/v1/users", resp.Header.Get("Location"), "Location")
}

func Test_App_Deep_Group(t *testing.T) {
	runThroughCount := 0
	var dummyHandler = func(c *Ctx) {
		runThroughCount++
		c.Next()
	}

	app := New()
	gAPI := app.Group("/api", dummyHandler)
	gV1 := gAPI.Group("/v1", dummyHandler)
	gUser := gV1.Group("/user", dummyHandler)
	gUser.Get("/authenticate", func(ctx *Ctx) {
		runThroughCount++
		ctx.SendStatus(200)
	})
	testStatus200(t, app, "/api/v1/user/authenticate", "GET")
	utils.AssertEqual(t, 4, runThroughCount, "Loop count")
}

// go test -run Test_App_Next_Method
func Test_App_Next_Method(t *testing.T) {
	app := New()
	app.Settings.DisableStartupMessage = true

	app.Use(func(c *Ctx) {
		utils.AssertEqual(t, "GET", c.Method())
		c.Next()
		utils.AssertEqual(t, "GET", c.Method())
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
}

// go test -run Test_App_Listen
func Test_App_Listen(t *testing.T) {
	app := New()

	utils.AssertEqual(t, false, app.Listen(1.23) == nil)

	utils.AssertEqual(t, false, app.Listen(":1.23") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(4003))

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen("[::]:4010"))
}

// go test -run Test_App_Listener
func Test_App_Listener(t *testing.T) {
	app := New(&Settings{
		Prefork: true,
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listener(ln))
}

// go test -v -run=^$ -bench=Benchmark_App_ETag -benchmem -count=4
func Benchmark_App_ETag(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send("Hello, World!")
	for n := 0; n < b.N; n++ {
		setETag(c, false)
	}
	utils.AssertEqual(b, `"13-1831710635"`, string(c.Fasthttp.Response.Header.Peek(HeaderETag)))
}

// go test -v -run=^$ -bench=Benchmark_App_ETag_Weak -benchmem -count=4
func Benchmark_App_ETag_Weak(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send("Hello, World!")
	for n := 0; n < b.N; n++ {
		setETag(c, true)
	}
	utils.AssertEqual(b, `W/"13-1831710635"`, string(c.Fasthttp.Response.Header.Peek(HeaderETag)))
}

// go test -run Test_NewError
func Test_NewError(t *testing.T) {
	e := NewError(StatusForbidden, "permission denied")
	utils.AssertEqual(t, StatusForbidden, e.Code)
	utils.AssertEqual(t, "permission denied", e.Message)
}

func Test_Test_Timeout(t *testing.T) {
	app := New()
	app.Settings.DisableStartupMessage = true

	app.Get("/", func(_ *Ctx) {})

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil), -1)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	app.Get("timeout", func(c *Ctx) {
		time.Sleep(55 * time.Millisecond)
	})

	_, err = app.Test(httptest.NewRequest("GET", "/timeout", nil), 50)
	utils.AssertEqual(t, true, err != nil, "app.Test(req)")
}

type errorReader int

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("errorReader")
}

func Test_Test_DumpError(t *testing.T) {
	app := New()
	app.Settings.DisableStartupMessage = true

	app.Get("/", func(_ *Ctx) {})

	resp, err := app.Test(httptest.NewRequest("GET", "/", errorReader(0)))
	utils.AssertEqual(t, true, resp == nil)
	utils.AssertEqual(t, "errorReader", err.Error())
}

func Test_App_Handler(t *testing.T) {
	h := New().Handler()
	utils.AssertEqual(t, "fasthttp.RequestHandler", reflect.TypeOf(h).String())
}

type invalidView struct{}

func (invalidView) Load() error { return errors.New("invalid view") }

func (i invalidView) Render(io.Writer, string, interface{}, ...string) error { panic("implement me") }

func Test_App_Init_Error_View(t *testing.T) {
	app := New(&Settings{Views: invalidView{}})
	app.init()

	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "implement me", fmt.Sprintf("%v", err))
		}
	}()
	_ = app.Settings.Views.Render(nil, "", nil)
}

func Test_App_Stack(t *testing.T) {
	app := New()

	app.Use("/path0", func(_ *Ctx) {})
	app.Get("/path1", func(_ *Ctx) {})
	app.Get("/path2", func(_ *Ctx) {})
	app.Post("/path3", func(_ *Ctx) {})

	stack := app.Stack()
	utils.AssertEqual(t, 9, len(stack))
	utils.AssertEqual(t, 3, len(stack[methodInt(MethodGet)]))
	utils.AssertEqual(t, 3, len(stack[methodInt(MethodHead)]))
	utils.AssertEqual(t, 2, len(stack[methodInt(MethodPost)]))
	utils.AssertEqual(t, 1, len(stack[methodInt(MethodPut)]))
	utils.AssertEqual(t, 1, len(stack[methodInt(MethodPatch)]))
	utils.AssertEqual(t, 1, len(stack[methodInt(MethodDelete)]))
	utils.AssertEqual(t, 1, len(stack[methodInt(MethodConnect)]))
	utils.AssertEqual(t, 1, len(stack[methodInt(MethodOptions)]))
	utils.AssertEqual(t, 1, len(stack[methodInt(MethodTrace)]))
}

// go test -run Test_App_ReadTimeout
//func Test_App_ReadTimeout(t *testing.T) {
//	app := New(&Settings{
//		ReadTimeout:           time.Nanosecond,
//		IdleTimeout:           time.Minute,
//		DisableStartupMessage: true,
//		DisableKeepalive:      true,
//	})
//
//	app.Get("/read-timeout", func(c *Ctx) {
//		c.SendString("I should not be sent")
//	})
//
//	go func() {
//		time.Sleep(500 * time.Millisecond)
//
//		conn, err := net.Dial("tcp4", "127.0.0.1:4004")
//		utils.AssertEqual(t, nil, err)
//		defer conn.Close()
//
//		_, err = conn.Write([]byte("HEAD /read-timeout HTTP/1.1\r\n"))
//		utils.AssertEqual(t, nil, err)
//
//		buf := make([]byte, 1024)
//		var n int
//		n, err = conn.Read(buf)
//
//		utils.AssertEqual(t, nil, err)
//		utils.AssertEqual(t, true, bytes.Contains(buf[:n], []byte("408 Request Timeout")))
//
//		utils.AssertEqual(t, nil, app.Shutdown())
//	}()
//
//	utils.AssertEqual(t, nil, app.Listen(4004))
//}

// go test -run Test_App_BadRequest
func Test_App_BadRequest(t *testing.T) {
	app := New(&Settings{
		DisableStartupMessage: true,
	})

	app.Get("/bad-request", func(c *Ctx) {
		c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		conn, err := net.Dial("tcp4", "127.0.0.1:4005")
		utils.AssertEqual(t, nil, err)
		defer conn.Close()

		_, err = conn.Write([]byte("BadRequest\r\n"))
		utils.AssertEqual(t, nil, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)
		utils.AssertEqual(t, nil, err)

		utils.AssertEqual(t, true, bytes.Contains(buf[:n], []byte("400 Bad Request")))

		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(4005))
}

// go test -run Test_App_SmallReadBuffer
func Test_App_SmallReadBuffer(t *testing.T) {
	app := New(&Settings{
		ReadBufferSize:        1,
		DisableStartupMessage: true,
	})

	app.Get("/small-read-buffer", func(c *Ctx) {
		c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Get("http://127.0.0.1:4006/small-read-buffer")
		if resp != nil {
			utils.AssertEqual(t, 431, resp.StatusCode)
		}
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(4006))
}
