// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package fiber

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2/utils"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func testEmptyHandler(_ *Ctx) error {
	return nil
}

func testStatus200(t *testing.T, app *App, url, method string) {
	t.Helper()

	req := httptest.NewRequest(method, url, nil)

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
}

func testErrorResponse(t *testing.T, err error, resp *http.Response, expectedBodyError string) {
	t.Helper()

	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedBodyError, string(body), "Response body")
}

func Test_App_MethodNotAllowed(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "hi, i'm an custom error", string(body))
}

func Test_App_ErrorHandler_HandlerStack(t *testing.T) {
	t.Parallel()
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1: USE error", string(body))
}

func Test_App_ErrorHandler_RouteStack(t *testing.T) {
	t.Parallel()
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "1: USE error", string(body))
}

func Test_App_serverErrorHandler_Internal_Error(t *testing.T) {
	t.Parallel()
	app := New()
	msg := "test err"
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	app.serverErrorHandler(c.fasthttp, errors.New(msg))
	utils.AssertEqual(t, string(c.fasthttp.Response.Body()), msg)
	utils.AssertEqual(t, c.fasthttp.Response.StatusCode(), StatusBadRequest)
}

func Test_App_serverErrorHandler_Network_Error(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	app.serverErrorHandler(c.fasthttp, &net.DNSError{
		Err:       "test error",
		Name:      "test host",
		IsTimeout: false,
	})
	utils.AssertEqual(t, string(c.fasthttp.Response.Body()), utils.StatusMessage(StatusBadGateway))
	utils.AssertEqual(t, c.fasthttp.Response.StatusCode(), StatusBadGateway)
}

func Test_App_Nested_Params(t *testing.T) {
	t.Parallel()
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

func Test_App_Use_Params(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func Test_App_Not_Use_StrictRouting(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/abc", func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/foo")
	g.Use("/", func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong path in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/abc/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// right path in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// right path with group in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_App_Use_MultiplePrefix(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use([]string{"/john", "/doe"}, func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/test")
	g.Use([]string{"/john", "/doe"}, func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "/john", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "/doe", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "/test/john", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "/test/doe", string(body))
}

func Test_App_Use_StrictRouting(t *testing.T) {
	t.Parallel()
	app := New(Config{StrictRouting: true})

	app.Get("/abc", func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/foo")
	g.Get("/", func(c *Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong path in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/abc/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path with group in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_App_Add_Method_Test(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "add: invalid http method JANE\n", fmt.Sprintf("%v", err))
		}
	}()

	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := New(Config{
		RequestMethods: methods,
	})

	app.Add("JOHN", "/doe", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest("JOHN", "/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusMethodNotAllowed, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("UNKNOWN", "/doe", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, StatusBadRequest, resp.StatusCode, "Status code")

	app.Add("JANE", "/doe", testEmptyHandler)
}

// go test -run Test_App_GETOnly
func Test_App_GETOnly(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	n := func(c *Ctx) error {
		return c.Next()
	}
	app := New()
	app.Use("/john", n, n, n, n, func(c *Ctx) error {
		return c.SendStatus(202)
	})
	// check handler count for registered HEAD route
	utils.AssertEqual(t, 5, len(app.stack[app.methodInt(MethodHead)][0].Handlers), "app.Test(req)")

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
	t.Parallel()
	app := New()

	app.Get("/test", func(c *Ctx) error {
		_, err := c.Write([]byte("1"))
		utils.AssertEqual(t, nil, err)
		return c.Next()
	})

	app.All("/test", func(c *Ctx) error {
		_, err := c.Write([]byte("2"))
		utils.AssertEqual(t, nil, err)
		return c.Next()
	})

	app.Use(func(c *Ctx) error {
		_, err := c.Write([]byte("3"))
		utils.AssertEqual(t, nil, err)
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
	t.Parallel()
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
	t.Parallel()
	app := New()
	handler := func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	}
	app.Get("/john", handler).Name("john")
	app.Delete("/doe", handler)
	app.Name("doe")

	jane := app.Group("/jane").Name("jane.")
	group := app.Group("/group")
	subGroup := jane.Group("/sub-group").Name("sub.")

	jane.Get("/test", handler).Name("test")
	jane.Trace("/trace", handler).Name("trace")

	group.Get("/test", handler).Name("test")

	app.Post("/post", handler).Name("post")

	subGroup.Get("/done", handler).Name("done")

	utils.AssertEqual(t, "post", app.GetRoute("post").Name)
	utils.AssertEqual(t, "john", app.GetRoute("john").Name)
	utils.AssertEqual(t, "jane.test", app.GetRoute("jane.test").Name)
	utils.AssertEqual(t, "jane.trace", app.GetRoute("jane.trace").Name)
	utils.AssertEqual(t, "jane.sub.done", app.GetRoute("jane.sub.done").Name)
	utils.AssertEqual(t, "test", app.GetRoute("test").Name)
}

func Test_App_New(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", testEmptyHandler)

	appConfig := New(Config{
		Immutable: true,
	})
	appConfig.Get("/", testEmptyHandler)
}

func Test_App_Config(t *testing.T) {
	t.Parallel()
	app := New(Config{
		DisableStartupMessage: true,
	})
	utils.AssertEqual(t, true, app.Config().DisableStartupMessage)
}

func Test_App_Shutdown(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := New(Config{
			DisableStartupMessage: true,
		})
		utils.AssertEqual(t, true, app.Shutdown() == nil)
	})

	t.Run("no server", func(t *testing.T) {
		t.Parallel()
		app := &App{}
		if err := app.Shutdown(); err != nil {
			utils.AssertEqual(t, "shutdown: server is not running", err.Error())
		}
	})
}

func Test_App_ShutdownWithTimeout(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(ctx *Ctx) error {
		time.Sleep(5 * time.Second)
		return ctx.SendString("body")
	})
	ln := fasthttputil.NewInmemoryListener()
	go func() {
		utils.AssertEqual(t, nil, app.Listener(ln))
	}()
	time.Sleep(1 * time.Second)
	go func() {
		conn, err := ln.Dial()
		if err != nil {
			t.Errorf("unexepcted error: %v", err)
		}

		if _, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: google.com\r\n\r\n")); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)

	shutdownErr := make(chan error)
	go func() {
		shutdownErr <- app.ShutdownWithTimeout(1 * time.Second)
	}()

	timer := time.NewTimer(time.Second * 5)
	select {
	case <-timer.C:
		t.Fatal("idle connections not closed on shutdown")
	case err := <-shutdownErr:
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unexpected err %v. Expecting %v", err, context.DeadlineExceeded)
		}
	}
}

func Test_App_ShutdownWithContext(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(ctx *Ctx) error {
		time.Sleep(5 * time.Second)
		return ctx.SendString("body")
	})

	ln := fasthttputil.NewInmemoryListener()

	go func() {
		utils.AssertEqual(t, nil, app.Listener(ln))
	}()

	time.Sleep(1 * time.Second)

	go func() {
		conn, err := ln.Dial()
		if err != nil {
			t.Errorf("unexepcted error: %v", err)
		}

		if _, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: google.com\r\n\r\n")); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}()

	time.Sleep(1 * time.Second)

	shutdownErr := make(chan error)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		shutdownErr <- app.ShutdownWithContext(ctx)
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("idle connections not closed on shutdown")
	case err := <-shutdownErr:
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unexpected err %v. Expecting %v", err, context.DeadlineExceeded)
		}
	}
}

// go test -run Test_App_Static_Index_Default
func Test_App_Static_Index_Default(t *testing.T) {
	t.Parallel()
	app := New()

	app.Static("/prefix", "./.github/workflows")
	app.Static("", "./.github/")
	app.Static("test", "", Static{Index: "index.html"})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/not-found", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 404, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "Cannot GET /not-found", string(body))
}

// go test -run Test_App_Static_Index
func Test_App_Static_Direct(t *testing.T) {
	t.Parallel()
	app := New()

	app.Static("/", "./.github")

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/testdata/testRoutes.json", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMEApplicationJSON, resp.Header.Get("Content-Type"))
	utils.AssertEqual(t, "", resp.Header.Get(HeaderCacheControl), "CacheControl Control")

	body, err = io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "test_routes"))
}

// go test -run Test_App_Static_MaxAge
func Test_App_Static_MaxAge(t *testing.T) {
	t.Parallel()
	app := New()

	app.Static("/", "./.github", Static{MaxAge: 100})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))
	utils.AssertEqual(t, "public, max-age=100", resp.Header.Get(HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_App_Static_Custom_CacheControl
func Test_App_Static_Custom_CacheControl(t *testing.T) {
	t.Parallel()
	app := New()

	app.Static("/", "./.github", Static{ModifyResponse: func(c *Ctx) error {
		if strings.Contains(c.GetRespHeader("Content-Type"), "text/html") {
			c.Response().Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		return nil
	}})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, "no-cache, no-store, must-revalidate", resp.Header.Get(HeaderCacheControl), "CacheControl Control")

	respNormal, errNormal := app.Test(httptest.NewRequest(MethodGet, "/config.yml", nil))
	utils.AssertEqual(t, nil, errNormal, "app.Test(req)")
	utils.AssertEqual(t, "", respNormal.Header.Get(HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_App_Static_Download
func Test_App_Static_Download(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	app.Static("/fiber.png", "./.github/testdata/fs/img/fiber.png", Static{Download: true})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/fiber.png", nil))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, "image/png", resp.Header.Get(HeaderContentType))
	utils.AssertEqual(t, `attachment`, resp.Header.Get(HeaderContentDisposition))
}

// go test -run Test_App_Static_Group
func Test_App_Static_Group(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	app := New()

	app.Static("*", "./.github/index.html")

	req := httptest.NewRequest(MethodGet, "/yesyes/john/doe", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Test file"))
}

func Test_App_Static_Prefix_Wildcard(t *testing.T) {
	t.Parallel()
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

	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, true, strings.Contains(string(body), "Test file"))
}

func Test_App_Static_Prefix(t *testing.T) {
	t.Parallel()
	app := New()
	app.Static("/john", "./.github")

	req := httptest.NewRequest(MethodGet, "/john/index.html", nil)
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
	utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/prefix", "./.github/testdata")

	req = httptest.NewRequest(MethodGet, "/prefix/index.html", nil)
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
	t.Parallel()
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
	t.Parallel()
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
		t.Parallel()
		req := httptest.NewRequest(MethodGet, "/", nil)
		req.Header.Set("X-Custom-Header", "skip")
		resp, err := app.Test(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.StatusCode)
		utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
		utils.AssertEqual(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, true, strings.Contains(string(body), "You've skipped app.Static"))
	})

	t.Run("app.Static is not skipped: serving index.html", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(MethodGet, "/", nil)
		req.Header.Set("X-Custom-Header", "don't skip")
		resp, err := app.Test(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.StatusCode)
		utils.AssertEqual(t, false, resp.Header.Get(HeaderContentLength) == "")
		utils.AssertEqual(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, true, strings.Contains(string(body), "Hello, World!"))
	})
}

// go test -run Test_App_Mixed_Routes_WithSameLen
func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	t.Parallel()
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

func Test_App_Group_Invalid(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "use: invalid handler int\n", fmt.Sprintf("%v", err))
		}
	}()
	New().Group("/").Use(1)
}

func Test_App_Group(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// go test -v -run=^$ -bench=Benchmark_App_ETag -benchmem -count=4
func Benchmark_App_ETag(b *testing.B) {
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)
	err := c.Send([]byte("Hello, World!"))
	utils.AssertEqual(b, nil, err)
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
	utils.AssertEqual(b, nil, c.Send([]byte("Hello, World!")))
	for n := 0; n < b.N; n++ {
		setETag(c, true)
	}
	utils.AssertEqual(b, `W/"13-1831710635"`, string(c.Response().Header.Peek(HeaderETag)))
}

// go test -run Test_NewError
func Test_NewError(t *testing.T) {
	t.Parallel()
	err := NewError(StatusForbidden, "permission denied")
	utils.AssertEqual(t, StatusForbidden, err.Code)
	utils.AssertEqual(t, "permission denied", err.Message)
}

// go test -run Test_Test_Timeout
func Test_Test_Timeout(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	app := New()
	app.config.DisableStartupMessage = true

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", errorReader(0)))
	utils.AssertEqual(t, true, resp == nil)
	utils.AssertEqual(t, "failed to dump request: errorReader", err.Error())
}

// go test -run Test_App_Handler
func Test_App_Handler(t *testing.T) {
	t.Parallel()
	h := New().Handler()
	utils.AssertEqual(t, "fasthttp.RequestHandler", reflect.TypeOf(h).String())
}

type invalidView struct{}

func (invalidView) Load() error { return errors.New("invalid view") }

func (invalidView) Render(io.Writer, string, interface{}, ...string) error { panic("implement me") }

// go test -run Test_App_Init_Error_View
func Test_App_Init_Error_View(t *testing.T) {
	app := New(Config{Views: invalidView{}})

	defer func() {
		if err := recover(); err != nil {
			utils.AssertEqual(t, "implement me", fmt.Sprintf("%v", err))
		}
	}()

	err := app.config.Views.Render(nil, "", nil)
	utils.AssertEqual(t, nil, err)
}

// go test -run Test_App_Stack
func Test_App_Stack(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path1", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)

	stack := app.Stack()
	methodList := app.config.RequestMethods
	utils.AssertEqual(t, len(methodList), len(stack))
	utils.AssertEqual(t, 3, len(stack[app.methodInt(MethodGet)]))
	utils.AssertEqual(t, 3, len(stack[app.methodInt(MethodHead)]))
	utils.AssertEqual(t, 2, len(stack[app.methodInt(MethodPost)]))
	utils.AssertEqual(t, 1, len(stack[app.methodInt(MethodPut)]))
	utils.AssertEqual(t, 1, len(stack[app.methodInt(MethodPatch)]))
	utils.AssertEqual(t, 1, len(stack[app.methodInt(MethodDelete)]))
	utils.AssertEqual(t, 1, len(stack[app.methodInt(MethodConnect)]))
	utils.AssertEqual(t, 1, len(stack[app.methodInt(MethodOptions)]))
	utils.AssertEqual(t, 1, len(stack[app.methodInt(MethodTrace)]))
}

// go test -run Test_App_HandlersCount
func Test_App_HandlersCount(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)

	count := app.HandlersCount()
	utils.AssertEqual(t, uint32(4), count)
}

// go test -run Test_App_ReadTimeout
func Test_App_ReadTimeout(t *testing.T) {
	t.Parallel()
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
		defer func(conn net.Conn) {
			err := conn.Close()
			utils.AssertEqual(t, nil, err)
		}(conn)

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
	t.Parallel()
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
		defer func(conn net.Conn) {
			err := conn.Close()
			utils.AssertEqual(t, nil, err)
		}(conn)

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
	t.Parallel()
	app := New(Config{
		ReadBufferSize:        1,
		DisableStartupMessage: true,
	})

	app.Get("/small-read-buffer", func(c *Ctx) error {
		return c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		req, err := http.NewRequestWithContext(context.Background(), MethodGet, "http://127.0.0.1:4006/small-read-buffer", http.NoBody)
		utils.AssertEqual(t, nil, err)
		var client http.Client
		resp, err := client.Do(req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 431, resp.StatusCode)
		utils.AssertEqual(t, nil, app.Shutdown())
	}()

	utils.AssertEqual(t, nil, app.Listen(":4006"))
}

func Test_App_Server(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	t.Run("Test_App_New_Test_Parallel_1", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		_, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err)
	})
	t.Run("Test_App_New_Test_Parallel_2", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		_, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
		utils.AssertEqual(t, nil, err)
	})
}

func Test_App_ReadBodyStream(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true})
	app.Post("/", func(c *Ctx) error {
		// Calling c.Body() automatically reads the entire stream.
		return c.SendString(fmt.Sprintf("%v %s", c.Request().IsBodyStream(), c.Body()))
	})
	testString := "this is a test"
	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", bytes.NewBufferString(testString)))
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "io.ReadAll(resp.Body)")
	utils.AssertEqual(t, fmt.Sprintf("true %s", testString), string(body))
}

func Test_App_DisablePreParseMultipartForm(t *testing.T) {
	t.Parallel()
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
			return fmt.Errorf("failed to open: %w", err)
		}
		buffer := make([]byte, len(testString))
		n, err := file.Read(buffer)
		if err != nil {
			return fmt.Errorf("failed to read: %w", err)
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

	req := httptest.NewRequest(MethodPost, "/", b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err, "io.ReadAll(resp.Body)")

	utils.AssertEqual(t, testString, string(body))
}

func Test_App_Test_no_timeout_infinitely(t *testing.T) {
	t.Parallel()
	var err error
	c := make(chan int)

	go func() {
		defer func() { c <- 0 }()
		app := New()
		app.Get("/", func(c *Ctx) error {
			runtime.Goexit()
			return nil
		})

		req := httptest.NewRequest(MethodGet, "/", http.NoBody)
		_, err = app.Test(req, -1)
	}()

	tk := time.NewTimer(5 * time.Second)
	defer tk.Stop()

	select {
	case <-tk.C:
		t.Error("hanging test")
		t.FailNow()
	case <-c:
	}

	if err == nil {
		t.Error("unexpected success request")
		t.FailNow()
	}
}

func Test_App_SetTLSHandler(t *testing.T) {
	t.Parallel()
	tlsHandler := &TLSHandler{clientHelloInfo: &tls.ClientHelloInfo{
		ServerName: "example.golang",
	}}

	app := New()
	app.SetTLSHandler(tlsHandler)

	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	utils.AssertEqual(t, "example.golang", c.ClientHelloInfo().ServerName)
}

func Test_App_AddCustomRequestMethod(t *testing.T) {
	t.Parallel()
	methods := append(DefaultMethods, "TEST") //nolint:gocritic // We want a new slice here
	app := New(Config{
		RequestMethods: methods,
	})
	appMethods := app.config.RequestMethods

	// method name is always uppercase - https://datatracker.ietf.org/doc/html/rfc7231#section-4.1
	utils.AssertEqual(t, len(app.stack), len(appMethods))
	utils.AssertEqual(t, len(app.stack), len(appMethods))
	utils.AssertEqual(t, "TEST", appMethods[len(appMethods)-1])
}

func TestApp_GetRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c *Ctx) error {
		return c.Next()
	})
	handler := func(c *Ctx) error {
		return c.SendStatus(StatusOK)
	}
	app.Delete("/delete", handler).Name("delete")
	app.Post("/post", handler).Name("post")
	routes := app.GetRoutes(false)
	utils.AssertEqual(t, 2+len(app.config.RequestMethods), len(routes))
	methodMap := map[string]string{"/delete": "delete", "/post": "post"}
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		if ok {
			utils.AssertEqual(t, name, route.Name)
		}
	}

	routes = app.GetRoutes(true)
	utils.AssertEqual(t, 2, len(routes))
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		utils.AssertEqual(t, true, ok)
		utils.AssertEqual(t, name, route.Name)
	}
}

func Test_Middleware_Route_Naming_With_Use(t *testing.T) {
	named := "named"
	app := New()

	app.Get("/unnamed", func(c *Ctx) error {
		return c.Next()
	})

	app.Post("/named", func(c *Ctx) error {
		return c.Next()
	}).Name(named)

	app.Use(func(c *Ctx) error {
		return c.Next()
	}) // no name - logging MW

	app.Use(func(c *Ctx) error {
		return c.Next()
	}).Name("corsMW")

	app.Use(func(c *Ctx) error {
		return c.Next()
	}).Name("compressMW")

	app.Use(func(c *Ctx) error {
		return c.Next()
	}) // no name - cache MW

	grp := app.Group("/pages").Name("pages.")
	grp.Use(func(c *Ctx) error {
		return c.Next()
	}).Name("csrfMW")

	grp.Get("/home", func(c *Ctx) error {
		return c.Next()
	}).Name("home")

	grp.Get("/unnamed", func(c *Ctx) error {
		return c.Next()
	})

	for _, route := range app.GetRoutes() {
		switch route.Path {
		case "/":
			utils.AssertEqual(t, "compressMW", route.Name)
		case "/unnamed":
			utils.AssertEqual(t, "", route.Name)
		case "named":
			utils.AssertEqual(t, named, route.Name)
		case "/pages":
			utils.AssertEqual(t, "pages.csrfMW", route.Name)
		case "/pages/home":
			utils.AssertEqual(t, "pages.home", route.Name)
		case "/pages/unnamed":
			utils.AssertEqual(t, "", route.Name)
		}
	}
}

func Test_Route_Naming_Issue_2671_2685(t *testing.T) {
	app := New()

	app.Get("/", emptyHandler).Name("index")
	utils.AssertEqual(t, "/", app.GetRoute("index").Path)

	app.Get("/a/:a_id", emptyHandler).Name("a")
	utils.AssertEqual(t, "/a/:a_id", app.GetRoute("a").Path)

	app.Post("/b/:bId", emptyHandler).Name("b")
	utils.AssertEqual(t, "/b/:bId", app.GetRoute("b").Path)

	c := app.Group("/c")
	c.Get("", emptyHandler).Name("c.get")
	utils.AssertEqual(t, "/c", app.GetRoute("c.get").Path)

	c.Post("", emptyHandler).Name("c.post")
	utils.AssertEqual(t, "/c", app.GetRoute("c.post").Path)

	c.Get("/d", emptyHandler).Name("c.get.d")
	utils.AssertEqual(t, "/c/d", app.GetRoute("c.get.d").Path)

	d := app.Group("/d/:d_id")
	d.Get("", emptyHandler).Name("d.get")
	utils.AssertEqual(t, "/d/:d_id", app.GetRoute("d.get").Path)

	d.Post("", emptyHandler).Name("d.post")
	utils.AssertEqual(t, "/d/:d_id", app.GetRoute("d.post").Path)

	e := app.Group("/e/:eId")
	e.Get("", emptyHandler).Name("e.get")
	utils.AssertEqual(t, "/e/:eId", app.GetRoute("e.get").Path)

	e.Post("", emptyHandler).Name("e.post")
	utils.AssertEqual(t, "/e/:eId", app.GetRoute("e.post").Path)

	e.Get("f", emptyHandler).Name("e.get.f")
	utils.AssertEqual(t, "/e/:eId/f", app.GetRoute("e.get.f").Path)

	postGroup := app.Group("/post/:postId")
	postGroup.Get("", emptyHandler).Name("post.get")
	utils.AssertEqual(t, "/post/:postId", app.GetRoute("post.get").Path)

	postGroup.Post("", emptyHandler).Name("post.update")
	utils.AssertEqual(t, "/post/:postId", app.GetRoute("post.update").Path)

	// Add testcase for routes use the same PATH on different methods
	app.Get("/users", nil).Name("get-users")
	app.Post("/users", nil).Name("add-user")
	getUsers := app.GetRoute("get-users")
	utils.AssertEqual(t, getUsers.Path, "/users")

	addUser := app.GetRoute("add-user")
	utils.AssertEqual(t, addUser.Path, "/users")

	// Add testcase for routes use the same PATH on different methods (for groups)
	newGrp := app.Group("/name-test")
	newGrp.Get("/users", nil).Name("grp-get-users")
	newGrp.Post("/users", nil).Name("grp-add-user")
	getUsers = app.GetRoute("grp-get-users")
	utils.AssertEqual(t, getUsers.Path, "/name-test/users")

	addUser = app.GetRoute("grp-add-user")
	utils.AssertEqual(t, addUser.Path, "/name-test/users")

	// Add testcase for HEAD route naming
	app.Get("/simple-route", emptyHandler).Name("simple-route")
	app.Head("/simple-route", emptyHandler).Name("simple-route2")

	sRoute := app.GetRoute("simple-route")
	utils.AssertEqual(t, sRoute.Path, "/simple-route")

	sRoute2 := app.GetRoute("simple-route2")
	utils.AssertEqual(t, sRoute2.Path, "/simple-route")
}
