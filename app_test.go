// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/eventemitter"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
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

func testErrorResponse(t *testing.T, err error, resp *http.Response, expectedBodyError string) {
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedBodyError, string(body), "Response body")
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

	g := app.Get("/with-next", func(c *Ctx) error {
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hi, i'm an error", string(body))

	_, err = app.Test(httptest.NewRequest(MethodGet, "/", strings.NewReader("big body")))
	if err != nil {
		utils.AssertEqual(t, "body size exceeds the given limit", err.Error(), "app.Test(req)")
	}
}

func Test_App_ErrorHandler_Custom(t *testing.T) {
	app := New()

	app.Use(func(c *Ctx, err error) error {
		return c.Status(200).SendString("hi, i'm an custom error")
	})

	app.Get("/", func(c *Ctx) error {
		return errors.New("hi, i'm an error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hi, i'm an custom error", string(body))
}

func Test_App_ErrorHandler_HandlerStack(t *testing.T) {
	app := New()

	app.Use(func(c *Ctx, err error) error {
		utils.AssertEqual(t, "1: USE error", err.Error())
		return DefaultErrorHandler(c, err)
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1: USE error", string(body))
}

func Test_App_ErrorHandler_RouteStack(t *testing.T) {
	app := New()

	app.Use(func(c *Ctx, err error) error {
		utils.AssertEqual(t, "1: USE error", err.Error())
		return DefaultErrorHandler(c, err)
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1: USE error", string(body))
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
	app := New()
	micro := New()

	app.Use("/john", micro)

	micro.Get("/doe", func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Path(t *testing.T) {
	parent := New()
	sub1 := New()
	sub2 := New()

	parent.Use("/sub1", sub1)
	sub1.Use("/sub2", sub2)

	utils.AssertEqual(t, "/", parent.Path())
	utils.AssertEqual(t, "/sub1/sub2", sub2.Path())
}

func Benchmark_App_Path(b *testing.B) {
	parent := New()
	sub := New()

	parent.Use("/sub", sub)

	var p string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		p = sub.Path()
	}

	utils.AssertEqual(b, "/sub", p)
}

func Test_App_MountPath(t *testing.T) {
	parent := New()
	sub1 := New()
	sub2 := New()

	parent.Use("/sub1", sub1)
	sub1.Use("/sub2", sub2)

	utils.AssertEqual(t, "", parent.MountPath())
	utils.AssertEqual(t, "/sub1", sub1.MountPath())
	utils.AssertEqual(t, "/sub2", sub2.MountPath())
}

func Benchmark_App_MountPath(b *testing.B) {
	parent := New()
	sub := New()

	parent.Use("/sub", sub)

	var mp string
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		mp = sub.MountPath()
	}

	utils.AssertEqual(b, "/sub", mp)
}

func Test_App_OnMount(t *testing.T) {
	app := New()
	sub := New()
	sub1 := New()

	app.Use("/sub", sub)

	sub.OnMount(func(parent *App) {
		//Check parent app
		utils.AssertEqual(t, app.mountpath, parent.mountpath)
	})

	sub.OnMount(func(parent *App) {
		utils.AssertEqual(t, parent != nil, true)
	})

	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "not mounted sub app to parent app", fmt.Sprintf("%s", err))
		}
	}()

	sub1.OnMount(func(parent *App) {})

	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "onmount cannot be used on parent app", fmt.Sprintf("%s", err))
		}
	}()
	app.OnMount(func(parent *App) {})
}

func Test_App_Use_Params(t *testing.T) {
	t.Parallel()
	app := New()

	{
		app.Use("/prefix/:param", func(c *Ctx) error {
			utils.AssertEqual(t, "john", c.Params("param"))
			return nil
		})

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/prefix/john", nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	}

	{
		app.Use("/foo/:bar?", func(c *Ctx) error {
			utils.AssertEqual(t, "foobar", c.Params("bar", "foobar"))
			return nil
		})

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	}

	{
		app.Use("/:param/*", func(c *Ctx) error {
			utils.AssertEqual(t, "john", c.Params("param"))
			utils.AssertEqual(t, "doe", c.Params("*"))
			return nil
		})

		resp, err := app.Test(httptest.NewRequest(MethodGet, "/john/doe", nil))
		utils.AssertEqual(t, nil, err, "app.Test(req)")
		utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	}

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

	body, err := io.ReadAll(resp.Body)
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	// check the detected path result
	utils.AssertEqual(t, "/AbC", app.getString(body))
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

	body, err := io.ReadAll(resp.Body)
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

func Test_App_Router(t *testing.T) {
	app := New()
	r := app.Router()

	r.Get("/", testEmptyHandler)

	testStatus200(t, app, "/", MethodGet)
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

// go test -run Test_App_Mixed_Routes_WithSameLen
func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	app := New()

	// middleware
	app.Use(func(c *Ctx) error {
		c.Set("TestHeader", "TestValue")
		return c.Next()
	})
	// routes with the same length
	app.Use("/tesbar", NewStatic("./.github"))
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

	body, err := io.ReadAll(resp.Body)
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

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"), "Response: "+string(body))
	utils.AssertEqual(t, true, strings.HasPrefix(string(body), "<!DOCTYPE html>"), "Response: "+string(body))
}

// go test -run Test_App_Next_Method
func Test_App_Next_Method(t *testing.T) {
	app := New()
	app.config.DisableStartupMessage = true

	app.Use(func(c *Ctx) error {
		utils.AssertEqual(t, MethodGet, c.Method())
		err := c.Next()
		utils.AssertEqual(t, MethodGet, c.Method())
		return err
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_AcquireCtx -benchmem -count=4
func Benchmark_AcquireCtx(b *testing.B) {
	app := New()
	for n := 0; n < b.N; n++ {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		app.ReleaseCtx(c)
	}
}

// go test -run Test_NewError
func Test_NewError(t *testing.T) {
	err := NewError(StatusForbidden, "permission denied")
	utils.AssertEqual(t, StatusForbidden, err.Code)
	utils.AssertEqual(t, "permission denied", err.Message)
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
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	_, err = app.Test(httptest.NewRequest(MethodGet, "/timeout", nil), 20)
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
	app := New()
	app.Engine(invalidView{})

	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "implement me", fmt.Sprintf("%v", err))
		}
	}()
	_ = app.engine.Render(nil, "", nil)
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

func Test_App_Server(t *testing.T) {
	app := New()

	utils.AssertEqual(t, false, app.Server() == nil)
}

func Test_App_Error_In_Fasthttp_Server(t *testing.T) {
	app := New()
	app.errorHandler = func(ctx *Ctx, err error) error {
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
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "io.ReadAll(resp.Body)")
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
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "io.ReadAll(resp.Body)")

	utils.AssertEqual(t, testString, string(body))
}

func Test_App_UseMountedErrorHandler(t *testing.T) {
	app := New()

	fiber := New()
	fiber.Use(func(ctx *Ctx, err error) error {
		return ctx.Status(500).SendString("hi, i'm a custom error")
	})

	fiber.Get("/", func(c *Ctx) error {
		return errors.New("something happened")
	})

	app.Use("/api", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_UseMountedErrorHandlerRootLevel(t *testing.T) {
	app := New()

	fiber := New()
	fiber.Use(func(ctx *Ctx, err error) error {
		return ctx.Status(500).SendString("hi, i'm a custom error")
	})

	fiber.Get("/api", func(c *Ctx) error {
		return errors.New("something happened")
	})

	app.Use("/", fiber)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api", nil))
	testErrorResponse(t, err, resp, "hi, i'm a custom error")
}

func Test_App_Locals(t *testing.T) {
	app := New()
	app.Locals["framework"] = "fiber"

	utils.AssertEqual(t, "fiber", app.Locals["framework"])
}

func Test_App_EventEmitter(t *testing.T) {
	app := New()
	fiberEvent := func(message string) {
		utils.AssertEqual(t, message, "fiber is amazing")
	}

	app.On("fiber", &fiberEvent)
	app.Emit("fiber", "fiber is amazing")
	utils.AssertEqual(t, 1, app.ListenerCount("fiber"))
	app.RemoveListener("fiber", &fiberEvent)
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, err, eventemitter.ErrEventNotExists)
		}
	}()
	app.ListenerCount("fiber")

	events := []string{
		"event_1", "event_1", "event_1",
	}
	for _, event := range events {
		app.AddListener(event, func() {})
	}
	app.RemoveAllListeners()
	utils.AssertEqual(t, 0, app.ListenerCount("event_1"))
}

// TODO: Rewrite this tests for new mounting app
// func Test_App_UseMountedErrorHandlerForBestPrefixMatch(t *testing.T) {
// 	app := New()

// 	tsf := func(ctx *Ctx, err error) error {
// 		return ctx.Status(200).SendString("hi, i'm a custom sub sub fiber error")
// 	}
// 	tripleSubFiber := New(Config{
// 		ErrorHandler: tsf,
// 	})
// 	tripleSubFiber.Get("/", func(c *Ctx) error {
// 		return errors.New("something happened")
// 	})

// 	sf := func(ctx *Ctx, err error) error {
// 		return ctx.Status(200).SendString("hi, i'm a custom sub fiber error")
// 	}
// 	subfiber := New(Config{
// 		ErrorHandler: sf,
// 	})
// 	subfiber.Get("/", func(c *Ctx) error {
// 		return errors.New("something happened")
// 	})
// 	subfiber.Use("/third", tripleSubFiber)

// 	f := func(ctx *Ctx, err error) error {
// 		return ctx.Status(200).SendString("hi, i'm a custom error")
// 	}
// 	fiber := New(Config{
// 		ErrorHandler: f,
// 	})
// 	fiber.Get("/", func(c *Ctx) error {
// 		return errors.New("something happened")
// 	})
// 	fiber.Use("/sub", subfiber)

// 	app.Use("/api", fiber)

// 	resp, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub", nil))
// 	utils.AssertEqual(t, nil, err, "/api/sub req")
// 	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

// 	b, err := io.ReadAll(resp.Body)
// 	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
// 	utils.AssertEqual(t, "hi, i'm a custom sub fiber error", string(b), "Response body")

// 	resp2, err := app.Test(httptest.NewRequest(MethodGet, "/api/sub/third", nil))
// 	utils.AssertEqual(t, nil, err, "/api/sub/third req")
// 	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

// 	b, err = io.ReadAll(resp2.Body)
// 	utils.AssertEqual(t, nil, err, "iotuil.ReadAll()")
// 	utils.AssertEqual(t, "hi, i'm a custom sub sub fiber error", string(b), "Third fiber Response body")
// }
