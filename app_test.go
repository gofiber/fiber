// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//nolint:goconst // Much easier to just ignore memory leaks in tests
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
	"sync/atomic"

	"github.com/gofiber/utils/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func testEmptyHandler(_ Ctx) error {
	return nil
}

func testStatus200(t *testing.T, app *App, url, method string) {
	t.Helper()

	req := httptest.NewRequest(method, url, nil)

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func testErrorResponse(t *testing.T, err error, resp *http.Response, expectedBodyError string) {
	t.Helper()

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, expectedBodyError, string(body), "Response body")
}

func Test_App_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		return c.Next()
	})

	app.Post("/", testEmptyHandler)

	app.Options("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPatch, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPut, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "POST, OPTIONS", resp.Header.Get(HeaderAllow))

	app.Get("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodTrace, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "GET, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	resp, err = app.Test(httptest.NewRequest(MethodPatch, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "GET, POST, OPTIONS", resp.Header.Get(HeaderAllow))

	app.Head("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodPut, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 405, resp.StatusCode)
	require.Equal(t, "GET, HEAD, POST, OPTIONS", resp.Header.Get(HeaderAllow))
}

func Test_App_Custom_Middleware_404_Should_Not_SetMethodNotAllowed(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		return c.SendStatus(404)
	})

	app.Post("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)

	g := app.Group("/with-next", func(c Ctx) error {
		return c.Status(404).Next()
	})

	g.Post("/", testEmptyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/with-next", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}

func Test_App_ServerErrorHandler_SmallReadBuffer(t *testing.T) {
	t.Parallel()
	expectedError := regexp.MustCompile(
		`error when reading request headers: small read buffer\. Increase ReadBufferSize\. Buffer size=4096, contents: "GET / HTTP/1.1\\r\\nHost: example\.com\\r\\nVery-Long-Header: -+`,
	)
	app := New()

	app.Get("/", func(_ Ctx) error {
		panic(errors.New("should never called"))
	})

	request := httptest.NewRequest(MethodGet, "/", nil)
	logHeaderSlice := make([]string, 5000)
	request.Header.Set("Very-Long-Header", strings.Join(logHeaderSlice, "-"))
	_, err := app.Test(request)
	if err == nil {
		t.Error("Expect an error at app.Test(request)")
	}

	require.Regexp(t, expectedError, err.Error())
}

func Test_App_Errors(t *testing.T) {
	t.Parallel()
	app := New(Config{
		BodyLimit: 4,
	})

	app.Get("/", func(_ Ctx) error {
		return errors.New("hi, i'm an error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hi, i'm an error", string(body))

	_, err = app.Test(httptest.NewRequest(MethodGet, "/", strings.NewReader("big body")))
	if err != nil {
		require.Equal(t, "body size exceeds the given limit", err.Error(), "app.Test(req)")
	}
}

type customConstraint struct{}

func (*customConstraint) Name() string {
	return "test"
}

func (*customConstraint) Execute(param string, args ...string) bool {
	if param == "test" && len(args) == 1 && args[0] == "test" {
		return true
	}

	if len(args) == 0 && param == "c" {
		return true
	}

	return false
}

func Test_App_CustomConstraint(t *testing.T) {
	t.Parallel()
	app := New()
	app.RegisterCustomConstraint(&customConstraint{})

	app.Get("/test/:param<test(test)>", func(c Ctx) error {
		return c.SendString("test")
	})

	app.Get("/test2/:param<test>", func(c Ctx) error {
		return c.SendString("test")
	})

	app.Get("/test3/:param<test()>", func(c Ctx) error {
		return c.SendString("test")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/test2", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/c", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test2/cc", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test3/cc", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
}

func Test_App_ErrorHandler_Custom(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, _ error) error {
			return c.Status(200).SendString("hi, i'm an custom error")
		},
	})

	app.Get("/", func(_ Ctx) error {
		return errors.New("hi, i'm an error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hi, i'm an custom error", string(body))
}

func Test_App_ErrorHandler_HandlerStack(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.Equal(t, "1: USE error", err.Error())
			return DefaultErrorHandler(c, err)
		},
	})
	app.Use("/", func(c Ctx) error {
		err := c.Next() // call next USE
		require.Equal(t, "2: USE error", err.Error())
		return errors.New("1: USE error")
	}, func(c Ctx) error {
		err := c.Next() // call [0] GET
		require.Equal(t, "0: GET error", err.Error())
		return errors.New("2: USE error")
	})
	app.Get("/", func(_ Ctx) error {
		return errors.New("0: GET error")
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1: USE error", string(body))
}

func Test_App_ErrorHandler_RouteStack(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			require.Equal(t, "1: USE error", err.Error())
			return DefaultErrorHandler(c, err)
		},
	})
	app.Use("/", func(c Ctx) error {
		err := c.Next()
		require.Equal(t, "0: GET error", err.Error())
		return errors.New("1: USE error") // [2] call ErrorHandler
	})
	app.Get("/test", func(_ Ctx) error {
		return errors.New("0: GET error") // [1] return to USE
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "1: USE error", string(body))
}

func Test_App_serverErrorHandler_Internal_Error(t *testing.T) {
	t.Parallel()
	app := New()
	msg := "test err"
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	app.serverErrorHandler(c.fasthttp, errors.New(msg))
	require.Equal(t, string(c.fasthttp.Response.Body()), msg)
	require.Equal(t, StatusBadRequest, c.fasthttp.Response.StatusCode())
}

func Test_App_serverErrorHandler_Network_Error(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	app.serverErrorHandler(c.fasthttp, &net.DNSError{
		Err:       "test error",
		Name:      "test host",
		IsTimeout: false,
	})
	require.Equal(t, string(c.fasthttp.Response.Body()), utils.StatusMessage(StatusBadGateway))
	require.Equal(t, StatusBadGateway, c.fasthttp.Response.StatusCode())
}

func Test_App_Nested_Params(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/test", func(c Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param", func(c Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param/test", func(c Ctx) error {
		return c.Status(400).Send([]byte("Should move on"))
	})
	app.Get("/test/:param/test/:param2", func(c Ctx) error {
		return c.Status(200).Send([]byte("Good job"))
	})

	req := httptest.NewRequest(MethodGet, "/test/john/test/doe", nil)
	resp, err := app.Test(req)

	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Use_Params(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/prefix/:param", func(c Ctx) error {
		require.Equal(t, "john", c.Params("param"))
		return nil
	})

	app.Use("/foo/:bar?", func(c Ctx) error {
		require.Equal(t, "foobar", c.Params("bar", "foobar"))
		return nil
	})

	app.Use("/:param/*", func(c Ctx) error {
		require.Equal(t, "john", c.Params("param"))
		require.Equal(t, "doe", c.Params("*"))
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/prefix/john", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/john/doe", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	defer func() {
		if err := recover(); err != nil {
			require.Equal(t, "use: invalid handler func()\n", fmt.Sprintf("%v", err))
		}
	}()

	app.Use("/:param/*", func() {
		// this should panic
	})
}

func Test_App_Use_UnescapedPath(t *testing.T) {
	t.Parallel()
	app := New(Config{UnescapePath: true, CaseSensitive: true})

	app.Use("/cRéeR/:param", func(c Ctx) error {
		require.Equal(t, "/cRéeR/اختبار", c.Path())
		return c.SendString(c.Params("param"))
	})

	app.Use("/abc", func(c Ctx) error {
		require.Equal(t, "/AbC", c.Path())
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/cR%C3%A9eR/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	// check the param result
	require.Equal(t, "اختبار", app.getString(body))

	// with lowercase letters
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/cr%C3%A9er/%D8%A7%D8%AE%D8%AA%D8%A8%D8%A7%D8%B1", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")
}

func Test_App_Use_CaseSensitive(t *testing.T) {
	t.Parallel()
	app := New(Config{CaseSensitive: true})

	app.Use("/abc", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong letters in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/AbC", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right letters in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// check the detected path when the case insensitive recognition is activated
	app.config.CaseSensitive = false
	// check the case sensitive feature
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/AbC", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "app.Test(req)")
	// check the detected path result
	require.Equal(t, "/AbC", app.getString(body))
}

func Test_App_Not_Use_StrictRouting(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/abc", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/foo")
	g.Use("/", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong path in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/abc/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// right path in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// right path with group in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_App_Use_MultiplePrefix(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use([]string{"/john", "/doe"}, func(c Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/test")
	g.Use([]string{"/john", "/doe"}, func(c Ctx) error {
		return c.SendString(c.Path())
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/john", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/john", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/doe", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/doe", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/john", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/test/john", string(body))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/doe", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "/test/doe", string(body))
}

func Test_App_Use_StrictRouting(t *testing.T) {
	t.Parallel()
	app := New(Config{StrictRouting: true})

	app.Get("/abc", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	g := app.Group("/foo")
	g.Get("/", func(c Ctx) error {
		return c.SendString(c.Path())
	})

	// wrong path in the requested route -> 404
	resp, err := app.Test(httptest.NewRequest(MethodGet, "/abc/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path with group in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")
}

func Test_App_Add_Method_Test(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err != nil {
			require.Equal(t, "add: invalid http method JANE\n", fmt.Sprintf("%v", err))
		}
	}()

	methods := append(DefaultMethods, "JOHN") //nolint:gocritic // We want a new slice here
	app := New(Config{
		RequestMethods: methods,
	})

	app.Add([]string{"JOHN"}, "/doe", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest("JOHN", "/doe", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/doe", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest("UNKNOWN", "/doe", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusNotImplemented, resp.StatusCode, "Status code")

	app.Add([]string{"JANE"}, "/doe", testEmptyHandler)
}

// go test -run Test_App_GETOnly
func Test_App_GETOnly(t *testing.T) {
	t.Parallel()
	app := New(Config{
		GETOnly: true,
	})

	app.Post("/", func(c Ctx) error {
		return c.SendString("Hello 👋!")
	})

	req := httptest.NewRequest(MethodPost, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, StatusMethodNotAllowed, resp.StatusCode, "Status code")
}

func Test_App_Use_Params_Group(t *testing.T) {
	t.Parallel()
	app := New()

	group := app.Group("/prefix/:param/*")
	group.Use("/", func(c Ctx) error {
		return c.Next()
	})
	group.Get("/test", func(c Ctx) error {
		require.Equal(t, "john", c.Params("param"))
		require.Equal(t, "doe", c.Params("*"))
		return nil
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/prefix/john/doe/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Chaining(t *testing.T) {
	t.Parallel()
	n := func(c Ctx) error {
		return c.Next()
	}
	app := New()
	app.Use("/john", n, n, n, n, func(c Ctx) error {
		return c.SendStatus(202)
	})
	// check handler count for registered HEAD route
	require.Len(t, app.stack[app.methodInt(MethodHead)][0].Handlers, 5, "app.Test(req)")

	req := httptest.NewRequest(MethodPost, "/john", nil)

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 202, resp.StatusCode, "Status code")

	app.Get("/test", n, n, n, n, func(c Ctx) error {
		return c.SendStatus(203)
	})

	req = httptest.NewRequest(MethodGet, "/test", nil)

	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 203, resp.StatusCode, "Status code")
}

func Test_App_Order(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/test", func(c Ctx) error {
		_, err := c.Write([]byte("1"))
		require.NoError(t, err)

		return c.Next()
	})

	app.All("/test", func(c Ctx) error {
		_, err := c.Write([]byte("2"))
		require.NoError(t, err)

		return c.Next()
	})

	app.Use(func(c Ctx) error {
		_, err := c.Write([]byte("3"))
		require.NoError(t, err)

		return nil
	})

	req := httptest.NewRequest(MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "123", string(body))
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
	handler := func(c Ctx) error {
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

	require.Equal(t, "post", app.GetRoute("post").Name)
	require.Equal(t, "john", app.GetRoute("john").Name)
	require.Equal(t, "jane.test", app.GetRoute("jane.test").Name)
	require.Equal(t, "jane.trace", app.GetRoute("jane.trace").Name)
	require.Equal(t, "jane.sub.done", app.GetRoute("jane.sub.done").Name)
	require.Equal(t, "test", app.GetRoute("test").Name)
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
		StrictRouting: true,
	})
	require.True(t, app.Config().StrictRouting)
}

func Test_App_Shutdown(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		app := New()
		require.NoError(t, app.Shutdown())
	})

	t.Run("no server", func(t *testing.T) {
		t.Parallel()
		app := &App{}
		if err := app.Shutdown(); err != nil {
			require.ErrorContains(t, err, "shutdown: server is not running")
		}
	})
}

func Test_App_ShutdownWithTimeout(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		time.Sleep(5 * time.Second)
		return c.SendString("body")
	})

	ln := fasthttputil.NewInmemoryListener()
	go func() {
		err := app.Listener(ln)
		assert.NoError(t, err)
	}()

	time.Sleep(1 * time.Second)
	go func() {
		conn, err := ln.Dial()
		assert.NoError(t, err)

		_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: google.com\r\n\r\n"))
		assert.NoError(t, err)
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
	var shutdownHookCalled int32
    app.Hooks().OnShutdown(func() error {
        atomic.StoreInt32(&shutdownHookCalled, 1)
        return nil
    })

	app.Get("/", func(ctx Ctx) error {
		time.Sleep(5 * time.Second)
		return ctx.SendString("body")
	})

	ln := fasthttputil.NewInmemoryListener()

    serverErr := make(chan error, 1)
    go func() {
        serverErr <- app.Listener(ln)
    }()

    time.Sleep(100 * time.Millisecond)

    clientDone := make(chan struct{})
    go func() {
        conn, err := ln.Dial()
        assert.NoError(t, err)
        _, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
        assert.NoError(t, err)
        close(clientDone)
    }()
	
	<-clientDone
    time.Sleep(100 * time.Millisecond)

	shutdownErr := make(chan error, 1)
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
        defer cancel()
        shutdownErr <- app.ShutdownWithContext(ctx)
    }()

	select {
    case <-time.After(2 * time.Second):
        t.Fatal("shutdown did not complete in time")
    case err := <-shutdownErr:
        assert.Error(t, err, "Expected shutdown to return an error due to timeout")
        assert.True(t, errors.Is(err, context.DeadlineExceeded), "Expected DeadlineExceeded error")
    }

	assert.Equal(t, int32(1), atomic.LoadInt32(&shutdownHookCalled), "Shutdown hook was not called")

	select {
    case err := <-serverErr:
        assert.NoError(t, err, "Server should have shut down without error")
    // default:
        // Server is still running, which is expected as the long-running request prevented full shutdown
    }
}

// go test -run Test_App_Mixed_Routes_WithSameLen
func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	t.Parallel()
	app := New()

	// middleware
	app.Use(func(c Ctx) error {
		c.Set("TestHeader", "TestValue")
		return c.Next()
	})
	// routes with the same length
	app.Get("/tesbar", func(c Ctx) error {
		c.Type("html")
		return c.Send([]byte("TEST_BAR"))
	})
	app.Get("/foobar", func(c Ctx) error {
		c.Type("html")
		return c.Send([]byte("FOO_BAR"))
	})

	// match get route
	req := httptest.NewRequest(MethodGet, "/foobar", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(HeaderContentLength))
	require.Equal(t, "TestValue", resp.Header.Get("TestHeader"))
	require.Equal(t, "text/html", resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "FOO_BAR", string(body))

	// match static route
	req = httptest.NewRequest(MethodGet, "/tesbar", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.NotEmpty(t, resp.Header.Get(HeaderContentLength))
	require.Equal(t, "TestValue", resp.Header.Get("TestHeader"))
	require.Equal(t, "text/html", resp.Header.Get(HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "TEST_BAR")
}

func Test_App_Group_Invalid(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err != nil {
			require.Equal(t, "use: invalid handler int\n", fmt.Sprintf("%v", err))
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
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	api.Get("/users", dummyHandler)
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1/UsErS", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Route(t *testing.T) {
	t.Parallel()
	dummyHandler := testEmptyHandler

	app := New()

	register := app.Route("/test").
		Get(dummyHandler).
		Head(dummyHandler).
		Post(dummyHandler).
		Put(dummyHandler).
		Delete(dummyHandler).
		Connect(dummyHandler).
		Options(dummyHandler).
		Trace(dummyHandler).
		Patch(dummyHandler)

	testStatus200(t, app, "/test", MethodGet)
	testStatus200(t, app, "/test", MethodHead)
	testStatus200(t, app, "/test", MethodPost)
	testStatus200(t, app, "/test", MethodPut)
	testStatus200(t, app, "/test", MethodDelete)
	testStatus200(t, app, "/test", MethodConnect)
	testStatus200(t, app, "/test", MethodOptions)
	testStatus200(t, app, "/test", MethodTrace)
	testStatus200(t, app, "/test", MethodPatch)

	register.Route("/v1").Get(dummyHandler).Post(dummyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/test/v1", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	register.Route("/v1").Route("/v2").Route("/v3").Get(dummyHandler).Trace(dummyHandler)

	resp, err = app.Test(httptest.NewRequest(MethodTrace, "/test/v1/v2/v3", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1/v2/v3", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
}

func Test_App_Deep_Group(t *testing.T) {
	t.Parallel()
	runThroughCount := 0
	dummyHandler := func(c Ctx) error {
		runThroughCount++
		return c.Next()
	}

	app := New()
	gAPI := app.Group("/api", dummyHandler)
	gV1 := gAPI.Group("/v1", dummyHandler)
	gUser := gV1.Group("/user", dummyHandler)
	gUser.Get("/authenticate", func(c Ctx) error {
		runThroughCount++
		return c.SendStatus(200)
	})
	testStatus200(t, app, "/api/v1/user/authenticate", MethodGet)
	require.Equal(t, 4, runThroughCount, "Loop count")
}

// go test -run Test_App_Next_Method
func Test_App_Next_Method(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use(func(c Ctx) error {
		require.Equal(t, MethodGet, c.Method())
		err := c.Next()
		require.Equal(t, MethodGet, c.Method())
		return err
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
}

// go test -v -run=^$ -bench=Benchmark_NewError -benchmem -count=4
func Benchmark_NewError(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewError(200, "test") //nolint:errcheck // not needed
	}
}

// go test -run Test_NewError
func Test_NewError(t *testing.T) {
	t.Parallel()
	e := NewError(StatusForbidden, "permission denied")
	require.Equal(t, StatusForbidden, e.Code)
	require.Equal(t, "permission denied", e.Message)
}

// go test -run Test_Test_Timeout
func Test_Test_Timeout(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil), -1)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")

	app.Get("timeout", func(_ Ctx) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	_, err = app.Test(httptest.NewRequest(MethodGet, "/timeout", nil), 20*time.Millisecond)
	require.Error(t, err, "app.Test(req)")
}

type errorReader int

var errErrorReader = errors.New("errorReader")

func (errorReader) Read([]byte) (int, error) {
	return 0, errErrorReader
}

// go test -run Test_Test_DumpError
func Test_Test_DumpError(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", errorReader(0)))
	require.Nil(t, resp)
	require.ErrorIs(t, err, errErrorReader)
}

// go test -run Test_App_Handler
func Test_App_Handler(t *testing.T) {
	t.Parallel()
	h := New().Handler()
	require.Equal(t, "fasthttp.RequestHandler", reflect.TypeOf(h).String())
}

type invalidView struct{}

func (invalidView) Load() error { return errors.New("invalid view") }

func (invalidView) Render(io.Writer, string, any, ...string) error { panic("implement me") }

// go test -run Test_App_Init_Error_View
func Test_App_Init_Error_View(t *testing.T) {
	t.Parallel()
	app := New(Config{Views: invalidView{}})

	defer func() {
		if err := recover(); err != nil {
			require.Equal(t, "implement me", fmt.Sprintf("%v", err))
		}
	}()

	err := app.config.Views.Render(nil, "", nil)
	require.NoError(t, err)
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
	require.Equal(t, len(methodList), len(stack))
	require.Len(t, stack[app.methodInt(MethodGet)], 3)
	require.Len(t, stack[app.methodInt(MethodHead)], 1)
	require.Len(t, stack[app.methodInt(MethodPost)], 2)
	require.Len(t, stack[app.methodInt(MethodPut)], 1)
	require.Len(t, stack[app.methodInt(MethodPatch)], 1)
	require.Len(t, stack[app.methodInt(MethodDelete)], 1)
	require.Len(t, stack[app.methodInt(MethodConnect)], 1)
	require.Len(t, stack[app.methodInt(MethodOptions)], 1)
	require.Len(t, stack[app.methodInt(MethodTrace)], 1)
}

// go test -run Test_App_HandlersCount
func Test_App_HandlersCount(t *testing.T) {
	t.Parallel()
	app := New()

	app.Use("/path0", testEmptyHandler)
	app.Get("/path2", testEmptyHandler)
	app.Post("/path3", testEmptyHandler)

	count := app.HandlersCount()
	require.Equal(t, uint32(3), count)
}

// go test -run Test_App_ReadTimeout
func Test_App_ReadTimeout(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ReadTimeout:      time.Nanosecond,
		IdleTimeout:      time.Minute,
		DisableKeepalive: true,
	})

	app.Get("/read-timeout", func(c Ctx) error {
		return c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)

		conn, err := net.Dial(NetworkTCP4, "127.0.0.1:4004")
		assert.NoError(t, err)
		defer func(conn net.Conn) {
			err := conn.Close()
			assert.NoError(t, err)
		}(conn)

		_, err = conn.Write([]byte("HEAD /read-timeout HTTP/1.1\r\n"))
		assert.NoError(t, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)

		assert.NoError(t, err)
		assert.True(t, bytes.Contains(buf[:n], []byte("408 Request Timeout")))

		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":4004", ListenConfig{DisableStartupMessage: true}))
}

// go test -run Test_App_BadRequest
func Test_App_BadRequest(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/bad-request", func(c Ctx) error {
		return c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		conn, err := net.Dial(NetworkTCP4, "127.0.0.1:4005")
		assert.NoError(t, err)
		defer func(conn net.Conn) {
			err := conn.Close()
			assert.NoError(t, err)
		}(conn)

		_, err = conn.Write([]byte("BadRequest\r\n"))
		assert.NoError(t, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)

		assert.NoError(t, err)
		assert.True(t, bytes.Contains(buf[:n], []byte("400 Bad Request")))
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":4005", ListenConfig{DisableStartupMessage: true}))
}

// go test -run Test_App_SmallReadBuffer
func Test_App_SmallReadBuffer(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ReadBufferSize: 1,
	})

	app.Get("/small-read-buffer", func(c Ctx) error {
		return c.SendString("I should not be sent")
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		req, err := http.NewRequestWithContext(context.Background(), MethodGet, "http://127.0.0.1:4006/small-read-buffer", nil)
		assert.NoError(t, err)
		var client http.Client
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, 431, resp.StatusCode)
		assert.NoError(t, app.Shutdown())
	}()

	require.NoError(t, app.Listen(":4006", ListenConfig{DisableStartupMessage: true}))
}

func Test_App_Server(t *testing.T) {
	t.Parallel()
	app := New()

	require.NotNil(t, app.Server())
}

func Test_App_Error_In_Fasthttp_Server(t *testing.T) {
	app := New()
	app.config.ErrorHandler = func(_ Ctx, _ error) error {
		return errors.New("fake error")
	}
	app.server.GetOnly = true

	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", nil))
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)
}

// go test -race -run Test_App_New_Test_Parallel
func Test_App_New_Test_Parallel(t *testing.T) {
	t.Parallel()
	t.Run("Test_App_New_Test_Parallel_1", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		_, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
		require.NoError(t, err)
	})
	t.Run("Test_App_New_Test_Parallel_2", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		_, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
		require.NoError(t, err)
	})
}

func Test_App_ReadBodyStream(t *testing.T) {
	t.Parallel()
	app := New(Config{StreamRequestBody: true})
	app.Post("/", func(c Ctx) error {
		// Calling c.Body() automatically reads the entire stream.
		return c.SendString(fmt.Sprintf("%v %s", c.Request().IsBodyStream(), c.Body()))
	})
	testString := "this is a test"
	resp, err := app.Test(httptest.NewRequest(MethodPost, "/", bytes.NewBufferString(testString)))
	require.NoError(t, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")
	require.Equal(t, "true "+testString, string(body))
}

func Test_App_DisablePreParseMultipartForm(t *testing.T) {
	t.Parallel()
	// Must be used with both otherwise there is no point.
	testString := "this is a test"

	app := New(Config{DisablePreParseMultipartForm: true, StreamRequestBody: true})
	app.Post("/", func(c Ctx) error {
		req := c.Request()
		mpf, err := req.MultipartForm()
		if err != nil {
			return err
		}
		if !req.IsBodyStream() {
			return errors.New("not a body stream")
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
			return errors.New("bad read length")
		}
		return c.Send(buffer)
	})
	b := &bytes.Buffer{}
	w := multipart.NewWriter(b)
	writer, err := w.CreateFormFile("test", "test")
	require.NoError(t, err, "w.CreateFormFile")
	n, err := writer.Write([]byte(testString))
	require.NoError(t, err, "writer.Write")
	require.Len(t, testString, n, "writer n")
	require.NoError(t, w.Close(), "w.Close()")

	req := httptest.NewRequest(MethodPost, "/", b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "io.ReadAll(resp.Body)")

	require.Equal(t, testString, string(body))
}

func Test_App_Test_no_timeout_infinitely(t *testing.T) {
	t.Parallel()
	var err error
	c := make(chan int)

	go func() {
		defer func() { c <- 0 }()
		app := New()
		app.Get("/", func(_ Ctx) error {
			runtime.Goexit()
			return nil
		})

		req := httptest.NewRequest(MethodGet, "/", nil)
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

func Test_App_Test_timeout(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/", func(_ Ctx) error {
		time.Sleep(1 * time.Second)
		return nil
	})

	_, err := app.Test(httptest.NewRequest(MethodGet, "/", nil), 100*time.Millisecond)
	require.Equal(t, errors.New("test: timeout error after 100ms"), err)
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

	require.Equal(t, "example.golang", c.ClientHelloInfo().ServerName)
}

func Test_App_AddCustomRequestMethod(t *testing.T) {
	t.Parallel()
	methods := append(DefaultMethods, "TEST") //nolint:gocritic // We want a new slice here
	app := New(Config{
		RequestMethods: methods,
	})
	appMethods := app.config.RequestMethods

	// method name is always uppercase - https://datatracker.ietf.org/doc/html/rfc7231#section-4.1
	require.Equal(t, len(app.stack), len(appMethods))
	require.Equal(t, len(app.stack), len(appMethods))
	require.Equal(t, "TEST", appMethods[len(appMethods)-1])
}

func Test_App_GetRoutes(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		return c.Next()
	})
	handler := func(c Ctx) error {
		return c.SendStatus(StatusOK)
	}
	app.Delete("/delete", handler).Name("delete")
	app.Post("/post", handler).Name("post")
	routes := app.GetRoutes(false)
	require.Len(t, routes, 2+len(app.config.RequestMethods))
	methodMap := map[string]string{"/delete": "delete", "/post": "post"}
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		if ok {
			require.Equal(t, name, route.Name)
		}
	}

	routes = app.GetRoutes(true)
	require.Len(t, routes, 2)
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		require.True(t, ok)
		require.Equal(t, name, route.Name)
	}
}

func Test_Middleware_Route_Naming_With_Use(t *testing.T) {
	t.Parallel()
	named := "named"
	app := New()

	app.Get("/unnamed", func(c Ctx) error {
		return c.Next()
	})

	app.Post("/named", func(c Ctx) error {
		return c.Next()
	}).Name(named)

	app.Use(func(c Ctx) error {
		return c.Next()
	}) // no name - logging MW

	app.Use(func(c Ctx) error {
		return c.Next()
	}).Name("corsMW")

	app.Use(func(c Ctx) error {
		return c.Next()
	}).Name("compressMW")

	app.Use(func(c Ctx) error {
		return c.Next()
	}) // no name - cache MW

	grp := app.Group("/pages").Name("pages.")
	grp.Use(func(c Ctx) error {
		return c.Next()
	}).Name("csrfMW")

	grp.Get("/home", func(c Ctx) error {
		return c.Next()
	}).Name("home")

	grp.Get("/unnamed", func(c Ctx) error {
		return c.Next()
	})

	for _, route := range app.GetRoutes() {
		switch route.Path {
		case "/":
			require.Equal(t, "compressMW", route.Name)
		case "/unnamed":
			require.Equal(t, "", route.Name)
		case "named":
			require.Equal(t, named, route.Name)
		case "/pages":
			require.Equal(t, "pages.csrfMW", route.Name)
		case "/pages/home":
			require.Equal(t, "pages.home", route.Name)
		case "/pages/unnamed":
			require.Equal(t, "", route.Name)
		}
	}
}

func Test_Route_Naming_Issue_2671_2685(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", emptyHandler).Name("index")
	require.Equal(t, "/", app.GetRoute("index").Path)

	app.Get("/a/:a_id", emptyHandler).Name("a")
	require.Equal(t, "/a/:a_id", app.GetRoute("a").Path)

	app.Post("/b/:bId", emptyHandler).Name("b")
	require.Equal(t, "/b/:bId", app.GetRoute("b").Path)

	c := app.Group("/c")
	c.Get("", emptyHandler).Name("c.get")
	require.Equal(t, "/c", app.GetRoute("c.get").Path)

	c.Post("", emptyHandler).Name("c.post")
	require.Equal(t, "/c", app.GetRoute("c.post").Path)

	c.Get("/d", emptyHandler).Name("c.get.d")
	require.Equal(t, "/c/d", app.GetRoute("c.get.d").Path)

	d := app.Group("/d/:d_id")
	d.Get("", emptyHandler).Name("d.get")
	require.Equal(t, "/d/:d_id", app.GetRoute("d.get").Path)

	d.Post("", emptyHandler).Name("d.post")
	require.Equal(t, "/d/:d_id", app.GetRoute("d.post").Path)

	e := app.Group("/e/:eId")
	e.Get("", emptyHandler).Name("e.get")
	require.Equal(t, "/e/:eId", app.GetRoute("e.get").Path)

	e.Post("", emptyHandler).Name("e.post")
	require.Equal(t, "/e/:eId", app.GetRoute("e.post").Path)

	e.Get("f", emptyHandler).Name("e.get.f")
	require.Equal(t, "/e/:eId/f", app.GetRoute("e.get.f").Path)

	postGroup := app.Group("/post/:postId")
	postGroup.Get("", emptyHandler).Name("post.get")
	require.Equal(t, "/post/:postId", app.GetRoute("post.get").Path)

	postGroup.Post("", emptyHandler).Name("post.update")
	require.Equal(t, "/post/:postId", app.GetRoute("post.update").Path)

	// Add testcase for routes use the same PATH on different methods
	app.Get("/users", emptyHandler).Name("get-users")
	app.Post("/users", emptyHandler).Name("add-user")
	getUsers := app.GetRoute("get-users")
	require.Equal(t, "/users", getUsers.Path)

	addUser := app.GetRoute("add-user")
	require.Equal(t, "/users", addUser.Path)

	// Add testcase for routes use the same PATH on different methods (for groups)
	newGrp := app.Group("/name-test")
	newGrp.Get("/users", emptyHandler).Name("grp-get-users")
	newGrp.Post("/users", emptyHandler).Name("grp-add-user")
	getUsers = app.GetRoute("grp-get-users")
	require.Equal(t, "/name-test/users", getUsers.Path)

	addUser = app.GetRoute("grp-add-user")
	require.Equal(t, "/name-test/users", addUser.Path)

	// Add testcase for HEAD route naming
	app.Get("/simple-route", emptyHandler).Name("simple-route")
	app.Head("/simple-route", emptyHandler).Name("simple-route2")

	sRoute := app.GetRoute("simple-route")
	require.Equal(t, "/simple-route", sRoute.Path)

	sRoute2 := app.GetRoute("simple-route2")
	require.Equal(t, "/simple-route", sRoute2.Path)
}

// go test -v -run=^$ -bench=Benchmark_Communication_Flow -benchmem -count=4
func Benchmark_Communication_Flow(b *testing.B) {
	app := New()

	app.Get("/", func(c Ctx) error {
		return c.SendString("Hello, World!")
	})

	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(fctx)
	}

	require.Equal(b, 200, fctx.Response.Header.StatusCode())
	require.Equal(b, "Hello, World!", string(fctx.Response.Body()))
}

// go test -v -run=^$ -bench=Benchmark_Ctx_AcquireReleaseFlow -benchmem -count=4
func Benchmark_Ctx_AcquireReleaseFlow(b *testing.B) {
	app := New()

	fctx := &fasthttp.RequestCtx{}

	b.Run("withoutRequestCtx", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			c, _ := app.AcquireCtx(fctx).(*DefaultCtx) //nolint:errcheck // not needed
			app.ReleaseCtx(c)
		}
	})

	b.Run("withRequestCtx", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			c, _ := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck // not needed
			app.ReleaseCtx(c)
		}
	})
}
