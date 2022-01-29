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
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

var testEmptyHandler = func(c *Ctx) error {
	return nil
}

func testStatus200(t *testing.T, app *App, url string, method string) {
	t.Helper()

	req := httptest.NewRequest(method, url, nil)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_MethodNotAllowed(t *testing.T) {
	app := New()

	app.Use(func(c *Ctx) error {
		return c.Next()
	})

	app.Post("/", testEmptyHandler)

	app.Options("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode)
	utils.AssertEqual(t, "", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPatch, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPut, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	app.Get("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodTrace, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPatch, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPut, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 405, resp.StatusCode)
	utils.AssertEqual(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))
}

func Test_App_Custom_Middleware_404_Should_Not_SetMethodNotAllowed(t *testing.T) {
	app := New()

	app.Use(func(c *Ctx) error {
		return c.SendStatus(404)
	})

	app.Post("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 404, resp.StatusCode)

	g := app.Group("/with-next", func(c *Ctx) error {
		return c.Status(404).Next()
	})

	g.Post("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/with-next", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 404, resp.StatusCode)
}

func Test_App_ServerErrorHandler_SmallReadBuffer(t *testing.T) {
	expectedError := regexp.MustCompile(
		`error when reading request headers: small read buffer\. Increase ReadBufferSize\. Buffer size=4096, contents: "GET / HTTP/1.1\\r\\nHost: example\.com\\r\\nVery-Long-Header: -+`,
	)
	app := New()

	app.Get("/", func(c *Ctx) error {
		panic(errors.New("should never called"))
	})

	request := httptest.NewRequest(MethodGet, "/", nil)
	logHeaderSlice := make([]string, 5000)
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

func Test_App_Errors(t *testing.T) {
	app := New(Config{
		BodyLimit: 4,
	})

	app.Get("/", func(c *Ctx) error {
		return errors.New("hi, i'm an error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hi, i'm an error", string(body))

	_, err = app.Test(httptest.NewRequest(MethodGet, "/", strings.NewReader("big body")))
	if err != nil {
		utils.AssertEqual(t, "body size exceeds the given limit", err.Error(), "app.Test(req)")
	}
}

func Test_App_ErrorHandler_Custom(t *testing.T) {
	app := New(Config{
		ErrorHandler: func(c *Ctx, err error) error {
			return c.Status(200).SendString("hi, i'm an custom error")
		},
	})

	app.Get("/", func(c *Ctx) error {
		return errors.New("hi, i'm an error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hi, i'm an custom error", string(body))
}

func Test_App_ErrorHandler_HandlerStack(t *testing.T) {
	app := New(Config{
		ErrorHandler: func(c *Ctx, err error) error {
			utils.AssertEqual(t, "1: USE error", err.Error())
			return DefaultErrorHandler(c, err)
		},
	})
	app.Use("/", func(c *Ctx) error {
		err := c.Next() // call next USE
		utils.AssertEqual(t, "2: USE error", err.Error())
		return errors.New("1: USE error")
	}, func(c *Ctx) error {
		err := c.Next() // call [0] GET
		utils.AssertEqual(t, "0: GET error", err.Error())
		return errors.New("2: USE error")
	})
	app.Get("/", func(c *Ctx) error {
		return errors.New("0: GET error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1: USE error", string(body))
}

func Test_App_ErrorHandler_RouteStack(t *testing.T) {
	app := New(Config{
		ErrorHandler: func(c *Ctx, err error) error {
			utils.AssertEqual(t, "1: USE error", err.Error())
			return DefaultErrorHandler(c, err)
		},
	})
	app.Use("/", func(c *Ctx) error {
		err := c.Next()
		utils.AssertEqual(t, "0: GET error", err.Error())
		return errors.New("1: USE error") // [2] call ErrorHandler
	})
	app.Get("/test", func(c *Ctx) error {
		return errors.New("0: GET error") // [1] return to USE
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1: USE error", string(body))
}

func Test_App_ErrorHandler_GroupMount(t *testing.T) {
	micro := New(Config{
		ErrorHandler: func(c *Ctx, err error) error {
			utils.AssertEqual(t, "0: GET error", err.Error())
			return c.Status(500).SendString("1: custom error")
		},
	})
	micro.Get("/doe", func(c *Ctx) error {
		return errors.New("0: GET error")
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Mount("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1: custom error", string(body))
}

func Test_App_Nested_Params(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param", func(c *Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param/test", func(c *Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param/test/:param2", func(c *Ctx) error {
		return c.Status(200).Send([]byte("Good job"))
	})

	req := httptest.NewRequest(MethodGet, "/test/john/test/doe", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

// go test -run Test_App_Mount
func Test_App_Mount(t *testing.T) {
	micro := New()
	micro.Get("/doe", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	app.Mount("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, uint32(2), app.handlersCount)
}

func Test_App_Use_Params(t *testing.T) {
	app := New()

	app.Use("/prefix/:param", func(c *Ctx) error {
		utils.AssertEqual(t, "john", c.Params("param"))
		return nil
	})

	app.Use("/foo/:bar?", func(c *Ctx) error {
		utils.AssertEqual(t, "foobar", c.Params("bar", "foobar"))
		return nil
	})

	app.Use("/:param/*", func(c *Ctx) error {
		utils.AssertEqual(t, "john", c.Params("param"))
		utils.AssertEqual(t, "doe", c.Params("*"))
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/prefix/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
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

func Test_App_Use_UnescapedPath(t *testing.T) {
	app := New(Config{UnescapePath: true, CaseSensitive: true})

	app.Use("/cRéeR/:param", func(c *Ctx) error {
		utils.AssertEqual(t, "/cRéeR/اختبار", c.Path())
		return c.SendString(c.Params("param"))
	})

	app.Use("/abc", func(c *Ctx) error {
		utils.AssertEqual(t, "/AbC", c.Path())
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/cR%C3%A9eR/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	// check the param result
	utils.AssertEqual(t, "اختبار", app.getString(body))

	// with lowercase letters
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")
}

func Test_App_Use_CaseSensitive(t *testing.T) {
	app := New(Config{CaseSensitive: true})

	app.Use("/abc", func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong letters in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/AbC", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")

	// right letters in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// check the detected path when the case insensitive recognition is activated
	app.config.CaseSensitive = false
	// check the case sensitive feature
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/AbC", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	// check the detected path result
	utils.AssertEqual(t, "/AbC", app.getString(body))
}

func Test_App_Add_Method_Test(t *testing.T) {
	app := New()
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "add: invalid http method JOHN\n", fmt.Sprintf("%v", err))
		}
	}()
	app.Add("JOHN", "/doe", testEmptyHandler)
}

// go test -run Test_App_GETOnly
func Test_App_GETOnly(t *testing.T) {
	app := New(Config{
		GETOnly: true,
	})

	app.Post("/", func(c *Ctx) error {
		return c.SendString("Hello 👋!")
	})

	req := httptest.NewRequest(MethodPost, "/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusMethodNotAllowed, resp.StatusCode, "Status code")
}

func Test_App_Use_Params_Group(t *testing.T) {
	app := New()

	group := app.Group("/prefix/:param/*")
	group.Use("/", func(c *Ctx) error {
		return c.Next()
	})
	group.Get("/test", func(c *Ctx) error {
		utils.AssertEqual(t, "john", c.Params("param"))
		utils.AssertEqual(t, "doe", c.Params("*"))
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/prefix/john/doe/test", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Chaining(t *testing.T) {
	n := func(c *Ctx) error {
		return c.Next()
	}
	app := New()
	app.Use("/john", n, n, n, n, func(c *Ctx) error {
		return c.SendStatus(202)
	})
	// check handler count for registered HEAD route
	utils.AssertEqual(t, 5, len(app.stack[methodInt(MethodHead)][0].Handlers), "app.Test(req)")

	req := httptest.NewRequest(MethodPost, "/john", nil)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 202, resp.StatusCode, "Status code")

	app.Get("/test", n, n, n, n, func(c *Ctx) error {
		return c.SendStatus(203)
	})

	req = httptest.NewRequest(MethodGet, "/test", nil)

	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 203, resp.StatusCode, "Status code")
}

func Test_App_Order(t *testing.T) {
	app := New()

	app.Get("/test", func(c *Ctx) error {
		c.Write([]byte("1"))
		return c.Next()
	})

	app.All("/test", func(c *Ctx) error {
		c.Write([]byte("2"))
		return c.Next()
	})

	app.Use(func(c *Ctx) error {
		c.Write([]byte("3"))
		return nil
	})

	req := httptest.NewRequest(MethodGet, "/test", nil)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "123", string(body))
}

func Test_App_Methods(t *testing.T) {
	dummyHandler := testEmptyHandler

	app := New()

	app.Connect("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", "CONNECT")

	app.Put("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPut)

	app.Post("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPost)

	app.Delete("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodDelete)

	app.Head("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodHead)

	app.Patch("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPatch)

	app.Options("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodOptions)

	app.Trace("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodTrace)

	app.Get("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodGet)

	app.All("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodPost)

	app.Use("/:john?/:doe?", dummyHandler)
	testStatus200(t, app, "/john/doe", MethodGet)
}

func Test_App_Route_Naming(t *testing.T) {
	app := New()
	handler := func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	}
	app.Get("/john", handler).Name("john")
	app.Delete("/doe", handler)
	app.Name("doe")

	jane := app.Group("/jane").Name("jane.")
	jane.Get("/test", handler).Name("test")
	jane.Trace("/trace", handler).Name("trace")

	group := app.Group("/group")
	group.Get("/test", handler).Name("test")

	app.Post("/post", handler).Name("post")

	subGroup := jane.Group("/sub-group").Name("sub.")
	subGroup.Get("/done", handler).Name("done")

	utils.AssertEqual(t, "post", app.GetRoute("post").Name)
	utils.AssertEqual(t, "john", app.GetRoute("john").Name)
	utils.AssertEqual(t, "jane.test", app.GetRoute("jane.test").Name)
	utils.AssertEqual(t, "jane.trace", app.GetRoute("jane.trace").Name)
	utils.AssertEqual(t, "jane.sub.done", app.GetRoute("jane.sub.done").Name)
	utils.AssertEqual(t, "test", app.GetRoute("test").Name)
}

func Test_App_New(t *testing.T) {
	app := New()
	app.Get("/", testEmptyHandler)

	appConfig := New(Config{
		Immutable: true,
	})
	appConfig.Get("/", testEmptyHandler)
}

func Test_App_Config(t *testing.T) {
	app := New(Config{
		DisableStartupMessage: true,
	})
	utils.AssertEqual(t, true, app.Config().DisableStartupMessage)
}

func Test_App_Shutdown(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		app := New(Config{
			DisableStartupMessage: true,
		})
		utils.AssertEqual(t, true, app.Shutdown() == nil)
	})

	t.Run("no server", func(t *testing.T) {
		app := &App{}
		if err := app.Shutdown(); err != nil {
			if err.Error() != "shutdown: server is not running" {
				t.Fatal()
			}
		}
	})
}

// go test -run Test_App_Static_Index_Default
func Test_App_Static_Index_Default(t *testing.T) {
	app := New()

	app.Static("/prefix", "./.github/workflows")
	app.Static("", "./.github/")
	app.Static("test", "", Static{Index: "index.html"})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/not-found", nil))
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

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/testdata/testRoutes.json", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMEApplicationJSON, resp.Header.Get("Content-Type"))
	utils.AssertEqual(t, "", resp.Header.Get(HeaderCacheControl), "CacheControl Control")

	body, err = ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "testRoutes"))
}

// go test -run Test_App_Static_MaxAge
func Test_App_Static_MaxAge(t *testing.T) {
	app := New()

	app.Static("/", "./.github", Static{MaxAge: 100})

	resp, err := app.Test(httptest.NewRequest("GET", "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))
	utils.AssertEqual(t, "public, max-age=100", resp.Header.Get(HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_App_Static_Group
func Test_App_Static_Group(t *testing.T) {
	app := New()

	grp := app.Group("/v1", func(c *Ctx) error {
		c.Set("Test-Header", "123")
		return c.Next()
	})

	grp.Static("/v2", "./.github/index.html")

	req := httptest.NewRequest(MethodGet, "/v1/v2", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))
	utils.AssertEqual(t, "123", resp.Header.Get("Test-Header"))

	grp = app.Group("/v2")
	grp.Static("/v3*", "./.github/index.html")

	req = httptest.NewRequest(MethodGet, "/v2/v3/john/doe", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))
}

func Test_App_Static_Wildcard(t *testing.T) {
	app := New()

	app.Static("*", "./.github/index.html")

	req := httptest.NewRequest(MethodGet, "/yesyes/john/doe", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Test file"))
}

func Test_App_Static_Prefix_Wildcard(t *testing.T) {
	app := New()

	app.Static("/test/*", "./.github/index.html")

	req := httptest.NewRequest(MethodGet, "/test/john/doe", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/my/nameisjohn*", "./.github/index.html")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/my/nameisjohn/no/its/not", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Test file"))
}

func Test_App_Static_Prefix(t *testing.T) {
	app := New()
	app.Static("/john", "./.github")

	req := httptest.NewRequest(MethodGet, "/john/index.html", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/prefix", "./.github/testdata")

	req = httptest.NewRequest(MethodGet, "/prefix/template.html", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/single", "./.github/testdata/testRoutes.json")

	req = httptest.NewRequest(MethodGet, "/single", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMEApplicationJSON, resp.Header.Get(HeaderContentType))
}

func Test_App_Static_Trailing_Slash(t *testing.T) {
	app := New()
	app.Static("/john", "./.github")

	req := httptest.NewRequest(MethodGet, "/john/", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/john_without_index", "./.github/testdata/fs/css")

	req = httptest.NewRequest(MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/john/", "./.github")

	req = httptest.NewRequest(MethodGet, "/john/", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	req = httptest.NewRequest(MethodGet, "/john", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/john_without_index/", "./.github/testdata/fs/css")

	req = httptest.NewRequest(MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
}

func Test_App_Static_Next(t *testing.T) {
	app := New()
	app.Static("/", ".github", Static{
		Next: func(c *Ctx) bool {
			// If value of the header is any other from "skip"
			// c.Next() will be invoked
			return c.Get("X-Custom-Header") == "skip"
		},
	})
	app.Get("/", func(c *Ctx) error {
		return c.SendString("You've skipped app.Static")
	})

	t.Run("app.Static is skipped: invoking Get handler", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Custom-Header", "skip")
		resp, err := app.Test(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.StatusCode)
		utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
		utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, true, strings.Contains(string(body), "You've skipped app.Static"))
	})

	t.Run("app.Static is not skipped: serving index.html", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Custom-Header", "don't skip")
		resp, err := app.Test(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.StatusCode)
		utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
		utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := ioutil.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))
	})
}

// go test -run Test_App_Mixed_Routes_WithSameLen
func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	app := New()

	// middleware
	app.Use(func(c *Ctx) error {
		c.Set("TestHeader", "TestValue")
		return c.Next()
	})
	// routes with the same length
	app.Static("/tesbar", "./.github")
	app.Get("/foobar", func(c *Ctx) error {
		c.Type("html")
		return c.Send([]byte("FOO_BAR"))
	})

	// match get route
	req := httptest.NewRequest(MethodGet, "/foobar", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, "TestValue", resp.Header.Get("TestHeader"))
	utils.AssertEqual(t, "text/html", resp.Header.Get(HeaderContentType))

	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "FOO_BAR", string(body))

	// match static route
	req = httptest.NewRequest(MethodGet, "/tesbar", nil)
	resp, err = app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, "TestValue", resp.Header.Get("TestHeader"))
	utils.AssertEqual(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))

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

// go test -run Test_App_Group_Mount
func Test_App_Group_Mount(t *testing.T) {
	micro := New()
	micro.Get("/doe", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	app := New()
	v1 := app.Group("/v1")
	v1.Mount("/john", micro)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/v1/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, uint32(2), app.handlersCount)
}

func Test_App_Group(t *testing.T) {
	dummyHandler := testEmptyHandler

	app := New()

	grp := app.Group("/test")
	grp.Get("/", dummyHandler)
	testStatus200(t, app, "/test", MethodGet)

	grp.Get("/:demo?", dummyHandler)
	testStatus200(t, app, "/test/john", MethodGet)

	grp.Connect("/CONNECT", dummyHandler)
	testStatus200(t, app, "/test/CONNECT", MethodConnect)

	grp.Put("/PUT", dummyHandler)
	testStatus200(t, app, "/test/PUT", MethodPut)

	grp.Post("/POST", dummyHandler)
	testStatus200(t, app, "/test/POST", MethodPost)

	grp.Delete("/DELETE", dummyHandler)
	testStatus200(t, app, "/test/DELETE", MethodDelete)

	grp.Head("/HEAD", dummyHandler)
	testStatus200(t, app, "/test/HEAD", MethodHead)

	grp.Patch("/PATCH", dummyHandler)
	testStatus200(t, app, "/test/PATCH", MethodPatch)

	grp.Options("/OPTIONS", dummyHandler)
	testStatus200(t, app, "/test/OPTIONS", MethodOptions)

	grp.Trace("/TRACE", dummyHandler)
	testStatus200(t, app, "/test/TRACE", MethodTrace)

	grp.All("/ALL", dummyHandler)
	testStatus200(t, app, "/test/ALL", MethodPost)

	grp.Use(dummyHandler)
	testStatus200(t, app, "/test/oke", MethodGet)

	grp.Use("/USE", dummyHandler)
	testStatus200(t, app, "/test/USE/oke", MethodGet)

	api := grp.Group("/v1")
	api.Post("/", dummyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/test/v1/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	// utils.AssertEqual(t, "/test/v1", resp.Header.Get("Location"), "Location")

	api.Get("/users", dummyHandler)
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1/UsErS", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	// utils.AssertEqual(t, "/test/v1/users", resp.Header.Get("Location"), "Location")
}

func Test_App_Route(t *testing.T) {
	dummyHandler := testEmptyHandler

	app := New()

	grp := app.Route("/test", func(grp Router) {
		grp.Get("/", dummyHandler)
		grp.Get("/:demo?", dummyHandler)
		grp.Connect("/CONNECT", dummyHandler)
		grp.Put("/PUT", dummyHandler)
		grp.Post("/POST", dummyHandler)
		grp.Delete("/DELETE", dummyHandler)
		grp.Head("/HEAD", dummyHandler)
		grp.Patch("/PATCH", dummyHandler)
		grp.Options("/OPTIONS", dummyHandler)
		grp.Trace("/TRACE", dummyHandler)
		grp.All("/ALL", dummyHandler)
		grp.Use(dummyHandler)
		grp.Use("/USE", dummyHandler)
	})

	testStatus200(t, app, "/test", MethodGet)
	testStatus200(t, app, "/test/john", MethodGet)
	testStatus200(t, app, "/test/CONNECT", MethodConnect)
	testStatus200(t, app, "/test/PUT", MethodPut)
	testStatus200(t, app, "/test/POST", MethodPost)
	testStatus200(t, app, "/test/DELETE", MethodDelete)
	testStatus200(t, app, "/test/HEAD", MethodHead)
	testStatus200(t, app, "/test/PATCH", MethodPatch)
	testStatus200(t, app, "/test/OPTIONS", MethodOptions)
	testStatus200(t, app, "/test/TRACE", MethodTrace)
	testStatus200(t, app, "/test/ALL", MethodPost)
	testStatus200(t, app, "/test/oke", MethodGet)
	testStatus200(t, app, "/test/USE/oke", MethodGet)

	grp.Route("/v1", func(grp Router) {
		grp.Post("/", dummyHandler)
		grp.Get("/users", dummyHandler)
	})

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/test/v1/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1/UsErS", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Deep_Group(t *testing.T) {
	runThroughCount := 0
	dummyHandler := func(c *Ctx) error {
		runThroughCount++
		return c.Next()
	}

	app := New()
	gAPI := app.Group("/api", dummyHandler)
	gV1 := gAPI.Group("/v1", dummyHandler)
	gUser := gV1.Group("/user", dummyHandler)
	gUser.Get("/authenticate", func(c *Ctx) error {
		runThroughCount++
		return c.SendStatus(200)
	})
	testStatus200(t, app, "/api/v1/user/authenticate", MethodGet)
	utils.AssertEqual(t, 4, runThroughCount, "Loop count")
}

// go test -run Test_App_Next_Method
func Test_App_Next_Method(t *testing.T) {
	app := New()
	app.config.DisableStartupMessage = true

	app.Use(func(c *Ctx) error {
		utils.AssertEqual(t, MethodGet, c.Method())
		c.Next()
		utils.AssertEqual(t, MethodGet, c.Method())
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
}

// go test -run Test_App_Listen
func Test_App_Listen(t *testing.T) {
	app := New(Config{DisableStartupMessage: true})

	utils.AssertEqual(t, false, app.Listen(":99999") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":4003"))
}

// go test -run Test_App_Listen_Prefork
func Test_App_Listen_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	utils.AssertEqual(t, nil, app.Listen(":99999"))
}

// go test -run Test_App_ListenTLS
func Test_App_ListenTLS(t *testing.T) {
	app := New()

	// invalid port
	utils.AssertEqual(t, false, app.ListenTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key") == nil)
	// missing perm/cert file
	utils.AssertEqual(t, false, app.ListenTLS(":0", "", "./.github/testdata/ssl.key") == nil)

	go func() {
		time.Sleep(1000 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.ListenTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key"))
}

// go test -run Test_App_ListenTLS_Prefork
func Test_App_ListenTLS_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	// invalid key file content
	utils.AssertEqual(t, false, app.ListenTLS(":0", "./.github/testdata/ssl.pem", "./.github/testdata/template.html") == nil)

	utils.AssertEqual(t, nil, app.ListenTLS(":99999", "./.github/testdata/ssl.pem", "./.github/testdata/ssl.key"))
}

// go test -run Test_App_Listener
func Test_App_Listener(t *testing.T) {
	app := New()

	go func() {
		time.Sleep(500 * time.Millisecond)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listener(ln))
}

// go test -run Test_App_Listener_Prefork
func Test_App_Listener_Prefork(t *testing.T) {
	testPreforkMaster = true

	app := New(Config{DisableStartupMessage: true, Prefork: true})

	ln := fasthttputil.NewInmemoryListener()
	utils.AssertEqual(t, nil, app.Listener(ln))
}

func Test_App_Listener_TLS_Listener(t *testing.T) {
	// Create tls certificate
	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
	if err != nil {
		utils.AssertEqual(t, nil, err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}

	ln, err := tls.Listen(NetworkTCP4, ":0", config)
	utils.AssertEqual(t, nil, err)

	app := New()

	go func() {
		time.Sleep(time.Millisecond * 500)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listener(ln))
}

// go test -v -run=^$ -bench=Benchmark_AcquireCtx -benchmem -count=4
func Benchmark_AcquireCtx(b *testing.B) {
	app := New()
	for n := 0; n < b.N; n++ {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		app.ReleaseCtx(c)
	}
}

// go test -v -run=^$ -bench=Benchmark_App_ETag -benchmem -count=4
func Benchmark_App_ETag(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send([]byte("Hello, World!"))
	for n := 0; n < b.N; n++ {
		setETag(c, false)
	}
	utils.AssertEqual(b, `"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
}

// go test -v -run=^$ -bench=Benchmark_App_ETag_Weak -benchmem -count=4
func Benchmark_App_ETag_Weak(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	c.Send([]byte("Hello, World!"))
	for n := 0; n < b.N; n++ {
		setETag(c, true)
	}
	utils.AssertEqual(b, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
}

// go test -run Test_NewError
func Test_NewError(t *testing.T) {
	e := NewError(StatusForbidden, "permission denied")
	utils.AssertEqual(t, StatusForbidden, e.Code)
	utils.AssertEqual(t, "permission denied", fmt.Sprint(e.Message))
}

func Test_NewErrors(t *testing.T) {
	e := NewErrors(StatusBadRequest, "error 1", "error 2")
	messages := e.Message.([]interface{})
	utils.AssertEqual(t, StatusBadRequest, e.Code)
	utils.AssertEqual(t, "error 1", fmt.Sprint(messages[0]))
	utils.AssertEqual(t, "error 2", fmt.Sprint(messages[1]))
}

// go test -run Test_Test_Timeout
func Test_Test_Timeout(t *testing.T) {
	app := New()
	app.config.DisableStartupMessage = true

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil), -1)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	app.Get("timeout", func(c *Ctx) error {
		time.Sleep(55 * time.Millisecond)
		return nil
	})

	_, err = app.Test(httptest.NewRequest(MethodGet, "/timeout", nil), 50)
	utils.AssertEqual(t, true, err != nil, "app.Test(req)")
}

type errorReader int

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("errorReader")
}

// go test -run Test_Test_DumpError
func Test_Test_DumpError(t *testing.T) {
	app := New()
	app.config.DisableStartupMessage = true

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", errorReader(0)))
	utils.AssertEqual(t, true, resp == nil)
	utils.AssertEqual(t, "errorReader", err.Error())
}

// go test -run Test_App_Handler
func Test_App_Handler(t *testing.T) {
	h := New().Handler()
	utils.AssertEqual(t, "fasthttp.RequestHandler", reflect.TypeOf(h).String())
}

type invalidView struct{}

func (invalidView) Load() error { return errors.New("invalid view") }

func (i invalidView) Render(io.Writer, string, interface{}, ...string) error { panic("implement me") }

// go test -run Test_App_Init_Error_View
func Test_App_Init_Error_View(t *testing.T) {
	app := New(Config{Views: invalidView{}})

	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "implement me", fmt.Sprintf("%v", err))
		}
	}()
	_ = app.config.Views.Render(nil, "", nil)
}

// go test -run Test_App_Stack
func Test_App_Stack(t *testing.T) {
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path1", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)

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

// go test -run Test_App_HandlersCount
func Test_App_HandlersCount(t *testing.T) {
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)

	count := app.HandlersCount()
	utils.AssertEqual(t, uint32(4), count)
}

// go test -run Test_App_ReadTimeout
func Test_App_ReadTimeout(t *testing.T) {
	app := New(Config{
		ReadTimeout:           time.Nanosecond,
		IdleTimeout:           time.Minute,
		DisableStartupMessage: true,
		DisableKeepalive:      true,
	})

	app.Get("/read-timeout", func(c *Ctx) error {
		return c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)

		conn, err := net.Dial(NetworkTCP4, "127.0.0.1:4004")
		utils.AssertEqual(t, nil, err)
		defer conn.Close()

		_, err = conn.Write([]byte("HEAD /read-timeout HTTP/1.1\r\n"))
		utils.AssertEqual(t, nil, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, true, bytes.Contains(buf[:n], []byte("408 Request Timeout")))

		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":4004"))
}

// go test -run Test_App_BadRequest
func Test_App_BadRequest(t *testing.T) {
	app := New(Config{
		DisableStartupMessage: true,
	})

	app.Get("/bad-request", func(c *Ctx) error {
		return c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		conn, err := net.Dial(NetworkTCP4, "127.0.0.1:4005")
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

	utils.AssertEqual(t, nil, app.Listen(":4005"))
}

// go test -run Test_App_SmallReadBuffer
func Test_App_SmallReadBuffer(t *testing.T) {
	app := New(Config{
		ReadBufferSize:        1,
		DisableStartupMessage: true,
	})

	app.Get("/small-read-buffer", func(c *Ctx) error {
		return c.SendString("I should not be sent")
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

	utils.AssertEqual(t, nil, app.Listen(":4006"))
}

func captureOutput(f func()) string {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	stdout := os.Stdout
	stderr := os.Stderr
	defer func() {
		os.Stdout = stdout
		os.Stderr = stderr
		log.SetOutput(os.Stderr)
	}()
	os.Stdout = writer
	os.Stderr = writer
	log.SetOutput(writer)
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		var buf bytes.Buffer
		wg.Done()
		io.Copy(&buf, reader)
		out <- buf.String()
	}()
	wg.Wait()
	f()
	writer.Close()
	return <-out
}

func Test_App_Master_Process_Show_Startup_Message(t *testing.T) {
	startupMessage := captureOutput(func() {
		New(Config{Prefork: true}).
			startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10))
	})
	fmt.Println(startupMessage)
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "https://127.0.0.1:3000"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "(bound on host 0.0.0.0 and port 3000)"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "Child PIDs"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "11111, 22222, 33333, 44444, 55555, 60000"))
	utils.AssertEqual(t, true, strings.Contains(startupMessage, "Prefork ........ Enabled"))
}

func Test_App_Master_Process_Show_Startup_MessageWithAppName(t *testing.T) {
	app := New(Config{Prefork: true, AppName: "Test App v1.0.1"})
	startupMessage := captureOutput(func() {
		app.startupMessage(":3000", true, strings.Repeat(",11111,22222,33333,44444,55555,60000", 10))
	})
	fmt.Println(startupMessage)
	utils.AssertEqual(t, "Test App v1.0.1", app.Config().AppName)
	utils.AssertEqual(t, true, strings.Contains(startupMessage, app.Config().AppName))
}

func Test_App_Server(t *testing.T) {
	app := New()

	utils.AssertEqual(t, false, app.Server() == nil)
}

func Test_App_Error_In_Fasthttp_Server(t *testing.T) {
	app := New()
	app.config.ErrorHandler = func(ctx *Ctx, err error) error {
		return errors.New("fake error")
	}
	app.server.GetOnly = true

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", nil))
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 500, resp.StatusCode)
}

// go test -race -run Test_App_New_Test_Parallel
func Test_App_New_Test_Parallel(t *testing.T) {
	t.Run("Test_App_New_Test_Parallel_1", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		app.Test(httptest.NewRequest("GET", "/", nil))
	})
	t.Run("Test_App_New_Test_Parallel_2", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		app.Test(httptest.NewRequest("GET", "/", nil))
	})
}

func Test_App_ReadBodyStream(t *testing.T) {
	app := New(Config{StreamRequestBody: true})
	app.Post("/", func(c *Ctx) error {
		// Calling c.Body() automatically reads the entire stream.
		return c.SendString(fmt.Sprintf("%v %s", c.Request().IsBodyStream(), c.Body()))
	})
	testString := "this is a test"
	resp, err := app.Test(httptest.NewRequest("POST", "/", bytes.NewBufferString(testString)))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "ioutil.ReadAll(resp.Body)")
	utils.AssertEqual(t, fmt.Sprintf("true %s", testString), string(body))
}

func Test_App_DisablePreParseMultipartForm(t *testing.T) {
	// Must be used with both otherwise there is no point.
	testString := "this is a test"

	app := New(Config{DisablePreParseMultipartForm: true, StreamRequestBody: true})
	app.Post("/", func(c *Ctx) error {
		req := c.Request()
		mpf, err := req.MultipartForm()
		if err != nil {
			return err
		}
		if !req.IsBodyStream() {
			return fmt.Errorf("not a body stream")
		}
		file, err := mpf.File["test"][0].Open()
		if err != nil {
			return err
		}
		buffer := make([]byte, len(testString))
		n, err := file.Read(buffer)
		if err != nil {
			return err
		}
		if n != len(testString) {
			return fmt.Errorf("bad read length")
		}
		return c.Send(buffer)
	})
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)
	writer, err := w.CreateFormFile("test", "test")
	utils.AssertEqual(t, nil, err, "w.CreateFormFile")
	n, err := writer.Write([]byte(testString))
	utils.AssertEqual(t, nil, err, "writer.Write")
	utils.AssertEqual(t, len(testString), n, "writer n")
	utils.AssertEqual(t, nil, w.Close(), "w.Close()")

	req := httptest.NewRequest("POST", "/", b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	body, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "ioutil.ReadAll(resp.Body)")

	utils.AssertEqual(t, testString, string(body))
}

func Test_App_UseMountedErrorHandler(t *testing.T) {
	app := New()

	fiber := New(Config{
		ErrorHandler: func(ctx *Ctx, err error) error {
			return ctx.Status(200).SendString("hi, i'm a custom error")
		},
	})
	fiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})

	app.Mount("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
	utils.AssertEqual(t, "hi, i'm a custom error", string(b), "Response body")
}

func Test_App_UseMountedErrorHandlerForBestPrefixMatch(t *testing.T) {
	app := New()

	tsf := func(ctx *Ctx, err error) error {
		return ctx.Status(200).SendString("hi, i'm a custom sub sub fiber error")
	}
	tripleSubFiber := New(Config{
		ErrorHandler: tsf,
	})
	tripleSubFiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})

	sf := func(ctx *Ctx, err error) error {
		return ctx.Status(200).SendString("hi, i'm a custom sub fiber error")
	}
	subfiber := New(Config{
		ErrorHandler: sf,
	})
	subfiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})
	subfiber.Mount("/third", tripleSubFiber)

	f := func(ctx *Ctx, err error) error {
		return ctx.Status(200).SendString("hi, i'm a custom error")
	}
	fiber := New(Config{
		ErrorHandler: f,
	})
	fiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})
	fiber.Mount("/sub", subfiber)

	app.Mount("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub", nil))
	utils.AssertEqual(t, nil, err, "/api/sub req")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	b, err := ioutil.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
	utils.AssertEqual(t, "hi, i'm a custom sub fiber error", string(b), "Response body")

	resp2, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub/third", nil))
	utils.AssertEqual(t, nil, err, "/api/sub/third req")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	b, err = ioutil.ReadAll(resp2.Body)
	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
	utils.AssertEqual(t, "hi, i'm a custom sub sub fiber error", string(b), "Third fiber Response body")
}

func emptyHandler(c *Ctx) error {
	return nil
}
func Test_App_print_Route(t *testing.T) {
	app := New(Config{EnablePrintRoutes: true})
	app.Get("/", emptyHandler).Name("routeName")
	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})
	fmt.Println(printRoutesMessage)
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "GET"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "emptyHandler"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "routeName"))
}

func Test_App_print_Route_with_group(t *testing.T) {
	app := New(Config{EnablePrintRoutes: true})
	app.Get("/", emptyHandler)
	v1 := app.Group("v1")
	v1.Get("/test", emptyHandler).Name("v1")
	v1.Post("/test/fiber", emptyHandler)
	v1.Put("/test/fiber/*", emptyHandler)
	printRoutesMessage := captureOutput(func() {
		app.printRoutesMessage()
	})
	fmt.Println(printRoutesMessage)
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "GET"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "emptyHandler"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "POST"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "PUT"))
	utils.AssertEqual(t, true, strings.Contains(printRoutesMessage, "/v1/test/fiber/*"))
}
