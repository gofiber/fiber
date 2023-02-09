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

	app.Get("/", func(c Ctx) error {
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

	app.Get("/", func(c Ctx) error {
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

func Test_App_ErrorHandler_Custom(t *testing.T) {
	t.Parallel()
	app := New(Config{
		ErrorHandler: func(c Ctx, err error) error {
			return c.Status(200).SendString("hi, i'm an custom error")
		},
	})

	app.Get("/", func(c Ctx) error {
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
	app.Get("/", func(c Ctx) error {
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
	app.Get("/test", func(c Ctx) error {
		return errors.New("0: GET error") // [1] return to USE
	})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/test", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")

	body, err := io.ReadAll(resp.Body)
	require.Equal(t, nil, err)
	require.Equal(t, "1: USE error", string(body))
}

func Test_App_serverErrorHandler_Internal_Error(t *testing.T) {
	t.Parallel()
	app := New()
	msg := "test err"
	c := app.NewCtx(&fasthttp.RequestCtx{}).(*DefaultCtx)

	app.serverErrorHandler(c.fasthttp, errors.New(msg))
	require.Equal(t, string(c.fasthttp.Response.Body()), msg)
	require.Equal(t, c.fasthttp.Response.StatusCode(), StatusBadRequest)
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
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// right path in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// right path with group in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", nil))
	require.Equal(t, nil, err, "app.Test(req)")
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
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/abc", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, StatusOK, resp.StatusCode, "Status code")

	// wrong path with group in the requested route -> 404
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, StatusNotFound, resp.StatusCode, "Status code")

	// right path with group in the requrested route -> 200
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/foo/", nil))
	require.Equal(t, nil, err, "app.Test(req)")
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
	require.Equal(t, 5, len(app.stack[app.methodInt(MethodHead)][0].Handlers), "app.Test(req)")

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
		require.True(t, app.Shutdown() == nil)
	})

	t.Run("no server", func(t *testing.T) {
		t.Parallel()
		app := &App{}
		if err := app.Shutdown(); err != nil {
			if err.Error() != "shutdown: server is not running" {
				t.Fatal()
			}
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
		require.NoError(t, app.Listener(ln))
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

// go test -run Test_App_Static_Index_Default
func Test_App_Static_Index_Default(t *testing.T) {
	app := New()

	app.Static("/prefix", "./.github/workflows")
	app.Static("", "./.github/")
	app.Static("test", "", Static{Index: "index.html"})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/not-found", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Cannot GET /not-found", string(body))
}

// go test -run Test_App_Static_Index
func Test_App_Static_Direct(t *testing.T) {
	app := New()

	app.Static("/", "./.github")

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), "Hello, World!"))

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/testdata/testRoutes.json", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMEApplicationJSON, resp.Header.Get("Content-Type"))
	require.Equal(t, "", resp.Header.Get(HeaderCacheControl), "CacheControl Control")

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), "test_routes"))
}

// go test -run Test_App_Static_MaxAge
func Test_App_Static_MaxAge(t *testing.T) {
	app := New()

	app.Static("/", "./.github", Static{MaxAge: 100})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))
	require.Equal(t, "public, max-age=100", resp.Header.Get(HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_App_Static_Custom_CacheControl
func Test_App_Static_Custom_CacheControl(t *testing.T) {
	app := New()

	app.Static("/", "./.github", Static{ModifyResponse: func(c Ctx) error {
		if strings.Contains(c.GetRespHeader("Content-Type"), "text/html") {
			c.Response().Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		return nil
	}})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/index.html", nil))
	require.Equal(t, nil, err, "app.Test(req)")
	require.Equal(t, "no-cache, no-store, must-revalidate", resp.Header.Get(HeaderCacheControl), "CacheControl Control")

	normalResp, normalErr := app.Test(httptest.NewRequest(MethodGet, "/config.yml", nil))
	require.Equal(t, nil, normalErr, "app.Test(req)")
	require.Equal(t, "", normalResp.Header.Get(HeaderCacheControl), "CacheControl Control")
}

// go test -run Test_App_Static_Download
func Test_App_Static_Download(t *testing.T) {
	app := New()

	app.Static("/fiber.png", "./.github/testdata/fs/img/fiber.png", Static{Download: true})

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/fiber.png", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, "image/png", resp.Header.Get(HeaderContentType))
	require.Equal(t, `attachment`, resp.Header.Get(HeaderContentDisposition))
}

// go test -run Test_App_Static_Group
func Test_App_Static_Group(t *testing.T) {
	app := New()

	grp := app.Group("/v1", func(c Ctx) error {
		c.Set("Test-Header", "123")
		return c.Next()
	})

	grp.Static("/v2", "./.github/index.html")

	req := httptest.NewRequest(MethodGet, "/v1/v2", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))
	require.Equal(t, "123", resp.Header.Get("Test-Header"))

	grp = app.Group("/v2")
	grp.Static("/v3*", "./.github/index.html")

	req = httptest.NewRequest(MethodGet, "/v2/v3/john/doe", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))
}

func Test_App_Static_Wildcard(t *testing.T) {
	app := New()

	app.Static("*", "./.github/index.html")

	req := httptest.NewRequest(MethodGet, "/yesyes/john/doe", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), "Test file"))
}

func Test_App_Static_Prefix_Wildcard(t *testing.T) {
	app := New()

	app.Static("/test/*", "./.github/index.html")

	req := httptest.NewRequest(MethodGet, "/test/john/doe", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/my/nameisjohn*", "./.github/index.html")

	resp, err = app.Test(httptest.NewRequest(MethodGet, "/my/nameisjohn/no/its/not", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), "Test file"))
}

func Test_App_Static_Prefix(t *testing.T) {
	app := New()
	app.Static("/john", "./.github")

	req := httptest.NewRequest(MethodGet, "/john/index.html", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/prefix", "./.github/testdata")

	req = httptest.NewRequest(MethodGet, "/prefix/index.html", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/single", "./.github/testdata/testRoutes.json")

	req = httptest.NewRequest(MethodGet, "/single", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMEApplicationJSON, resp.Header.Get(HeaderContentType))
}

func Test_App_Static_Trailing_Slash(t *testing.T) {
	app := New()
	app.Static("/john", "./.github")

	req := httptest.NewRequest(MethodGet, "/john/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/john_without_index", "./.github/testdata/fs/css")

	req = httptest.NewRequest(MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/john/", "./.github")

	req = httptest.NewRequest(MethodGet, "/john/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	req = httptest.NewRequest(MethodGet, "/john", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

	app.Static("/john_without_index/", "./.github/testdata/fs/css")

	req = httptest.NewRequest(MethodGet, "/john_without_index/", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 404, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))
}

func Test_App_Static_Next(t *testing.T) {
	app := New()
	app.Static("/", ".github", Static{
		Next: func(c Ctx) bool {
			// If value of the header is any other from "skip"
			// c.Next() will be invoked
			return c.Get("X-Custom-Header") == "skip"
		},
	})
	app.Get("/", func(c Ctx) error {
		return c.SendString("You've skipped app.Static")
	})

	t.Run("app.Static is skipped: invoking Get handler", func(t *testing.T) {
		req := httptest.NewRequest(MethodGet, "/", nil)
		req.Header.Set("X-Custom-Header", "skip")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.False(t, resp.Header.Get(HeaderContentLength) == "")
		require.Equal(t, MIMETextPlainCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.True(t, strings.Contains(string(body), "You've skipped app.Static"))
	})

	t.Run("app.Static is not skipped: serving index.html", func(t *testing.T) {
		req := httptest.NewRequest(MethodGet, "/", nil)
		req.Header.Set("X-Custom-Header", "don't skip")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.False(t, resp.Header.Get(HeaderContentLength) == "")
		require.Equal(t, MIMETextHTMLCharsetUTF8, resp.Header.Get(HeaderContentType))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.True(t, strings.Contains(string(body), "Hello, World!"))
	})
}

// go test -run Test_App_Mixed_Routes_WithSameLen
func Test_App_Mixed_Routes_WithSameLen(t *testing.T) {
	app := New()

	// middleware
	app.Use(func(c Ctx) error {
		c.Set("TestHeader", "TestValue")
		return c.Next()
	})
	// routes with the same length
	app.Static("/tesbar", "./.github")
	app.Get("/foobar", func(c Ctx) error {
		c.Type("html")
		return c.Send([]byte("FOO_BAR"))
	})

	// match get route
	req := httptest.NewRequest(MethodGet, "/foobar", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
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
	require.False(t, resp.Header.Get(HeaderContentLength) == "")
	require.Equal(t, "TestValue", resp.Header.Get("TestHeader"))
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get(HeaderContentType))

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(body), "Hello, World!"), "Response: "+string(body))
	require.True(t, strings.HasPrefix(string(body), "<!DOCTYPE html>"), "Response: "+string(body))
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
	// require.Equal(t, "/test/v1", resp.Header.Get("Location"), "Location")

	api.Get("/users", dummyHandler)
	resp, err = app.Test(httptest.NewRequest(MethodGet, "/test/v1/UsErS", nil))
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	// require.Equal(t, "/test/v1/users", resp.Header.Get("Location"), "Location")
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

// go test -v -run=^$ -bench=Benchmark_AcquireCtx -benchmem -count=4
func Benchmark_AcquireCtx(b *testing.B) {
	app := New()
	for n := 0; n < b.N; n++ {
		c := app.AcquireCtx()
		c.Reset(&fasthttp.RequestCtx{})

		app.ReleaseCtx(c)
	}
}

// go test -v -run=^$ -bench=Benchmark_NewError -benchmem -count=4
func Benchmark_NewError(b *testing.B) {
	for n := 0; n < b.N; n++ {
		NewError(200, "test")
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

	app.Get("timeout", func(c Ctx) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	_, err = app.Test(httptest.NewRequest(MethodGet, "/timeout", nil), 20*time.Millisecond)
	require.True(t, err != nil, "app.Test(req)")
}

type errorReader int

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("errorReader")
}

// go test -run Test_Test_DumpError
func Test_Test_DumpError(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", testEmptyHandler)

	resp, err := app.Test(httptest.NewRequest(MethodGet, "/", errorReader(0)))
	require.True(t, resp == nil)
	require.Equal(t, "failed to dump request: errorReader", err.Error())
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
	require.Equal(t, 3, len(stack[app.methodInt(MethodGet)]))
	require.Equal(t, 1, len(stack[app.methodInt(MethodHead)]))
	require.Equal(t, 2, len(stack[app.methodInt(MethodPost)]))
	require.Equal(t, 1, len(stack[app.methodInt(MethodPut)]))
	require.Equal(t, 1, len(stack[app.methodInt(MethodPatch)]))
	require.Equal(t, 1, len(stack[app.methodInt(MethodDelete)]))
	require.Equal(t, 1, len(stack[app.methodInt(MethodConnect)]))
	require.Equal(t, 1, len(stack[app.methodInt(MethodOptions)]))
	require.Equal(t, 1, len(stack[app.methodInt(MethodTrace)]))
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
		require.NoError(t, err)
		defer func(conn net.Conn) {
			err := conn.Close()
			require.NoError(t, err)
		}(conn)

		_, err = conn.Write([]byte("HEAD /read-timeout HTTP/1.1\r\n"))
		require.NoError(t, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)

		require.NoError(t, err)
		require.True(t, bytes.Contains(buf[:n], []byte("408 Request Timeout")))

		require.Nil(t, app.Shutdown())
	}()

	require.Nil(t, app.Listen(":4004", ListenConfig{DisableStartupMessage: true}))
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
		require.NoError(t, err)
		defer func(conn net.Conn) {
			err := conn.Close()
			require.NoError(t, err)
		}(conn)

		_, err = conn.Write([]byte("BadRequest\r\n"))
		require.NoError(t, err)

		buf := make([]byte, 1024)
		var n int
		n, err = conn.Read(buf)
		require.NoError(t, err)

		require.True(t, bytes.Contains(buf[:n], []byte("400 Bad Request")))

		require.Nil(t, app.Shutdown())
	}()

	require.Nil(t, app.Listen(":4005", ListenConfig{DisableStartupMessage: true}))
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
		req, err := http.NewRequestWithContext(context.Background(), MethodGet, "http://127.0.0.1:4006/small-read-buffer", http.NoBody)
		require.NoError(t, err)
		var client http.Client
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, 431, resp.StatusCode)
		require.Nil(t, app.Shutdown())
	}()

	require.Nil(t, app.Listen(":4006", ListenConfig{DisableStartupMessage: true}))
}

func Test_App_Server(t *testing.T) {
	t.Parallel()
	app := New()

	require.False(t, app.Server() == nil)
}

func Test_App_Error_In_Fasthttp_Server(t *testing.T) {
	t.Parallel()
	app := New()
	app.config.ErrorHandler = func(c Ctx, err error) error {
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
		require.Equal(t, nil, err)
	})
	t.Run("Test_App_New_Test_Parallel_2", func(t *testing.T) {
		t.Parallel()
		app := New(Config{Immutable: true})
		_, err := app.Test(httptest.NewRequest(MethodGet, "/", nil))
		require.Equal(t, nil, err)
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
	require.Equal(t, fmt.Sprintf("true %s", testString), string(body))
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
	require.NoError(t, err, "w.CreateFormFile")
	n, err := writer.Write([]byte(testString))
	require.NoError(t, err, "writer.Write")
	require.Equal(t, len(testString), n, "writer n")
	require.Nil(t, w.Close(), "w.Close()")

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
		app.Get("/", func(c Ctx) error {
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

	c := app.NewCtx(&fasthttp.RequestCtx{})
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

func TestApp_GetRoutes(t *testing.T) {
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
	require.Equal(t, 2+len(app.config.RequestMethods), len(routes))
	methodMap := map[string]string{"/delete": "delete", "/post": "post"}
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		if ok {
			require.Equal(t, name, route.Name)
		}
	}

	routes = app.GetRoutes(true)
	require.Equal(t, 2, len(routes))
	for _, route := range routes {
		name, ok := methodMap[route.Path]
		require.Equal(t, true, ok)
		require.Equal(t, name, route.Name)
	}
}
