// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

//nolint:wrapcheck // We must not wrap errors in tests
package fiber

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// go test -run Test_Redirect_To
func Test_Redirect_To(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().To("http://default.com")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "http://default.com", string(c.Response().Header.Peek(HeaderLocation)))

	err = c.Redirect().Status(301).To("http://example.com")
	require.NoError(t, err)
	require.Equal(t, 301, c.Response().StatusCode())
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithParams
func Test_Redirect_Route_WithParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithParams_WithQueries
func Test_Redirect_Route_WithParams_WithQueries(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
		Queries: map[string]string{"data[0][name]": "john", "data[0][age]": "10", "test": "doe"},
	})
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	require.NoError(t, err, "url.Parse(location)")
	require.Equal(t, "/user/fiber", location.Path)
	require.Equal(t, url.Values{"data[0][name]": []string{"john"}, "data[0][age]": []string{"10"}, "test": []string{"doe"}}, location.Query())
}

// go test -run Test_Redirect_Route_WithOptionalParams
func Test_Redirect_Route_WithOptionalParams(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"name": "fiber",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithOptionalParamsWithoutValue
func Test_Redirect_Route_WithOptionalParamsWithoutValue(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/:name?", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/user/", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithGreedyParameters
func Test_Redirect_Route_WithGreedyParameters(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/user/+", func(c Ctx) error {
		return c.JSON(c.Params("+"))
	}).Name("user")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Route("user", RedirectConfig{
		Params: Map{
			"+": "test/routes",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/user/test/routes", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Back
func Test_Redirect_Back(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	err = c.Redirect().Back()
	require.Equal(t, 500, c.Response().StatusCode())
	require.ErrorAs(t, err, &ErrRedirectBackNoFallback)
}

// go test -run Test_Redirect_Back_WithReferer
func Test_Redirect_Back_WithReferer(t *testing.T) {
	t.Parallel()
	app := New()
	app.Get("/", func(c Ctx) error {
		return c.JSON("Home")
	}).Name("home")
	app.Get("/back", func(c Ctx) error {
		return c.JSON("Back")
	}).Name("back")
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	c.Request().Header.Set(HeaderReferer, "/back")
	err := c.Redirect().Back("/")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/back", c.Get(HeaderReferer))
	require.Equal(t, "/back", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -run Test_Redirect_Route_WithFlashMessages
func Test_Redirect_Route_WithFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	err := c.Redirect().With("success", "1").With("message", "test").Route("user")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	equal := c.GetRespHeader(HeaderSetCookie) == "fiber_flash=success:1,message:test; path=/; SameSite=Lax" || c.GetRespHeader(HeaderSetCookie) == "fiber_flash=message:test,success:1; path=/; SameSite=Lax"
	require.True(t, equal)

	c.Redirect().setFlash()
	require.Equal(t, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))
}

// go test -run Test_Redirect_Route_WithOldInput
func Test_Redirect_Route_WithOldInput(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().URI().SetQueryString("id=1&name=tom")
	err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	require.Contains(t, c.GetRespHeader(HeaderSetCookie), "fiber_flash=")
	require.Contains(t, c.GetRespHeader(HeaderSetCookie), "success:1")
	require.Contains(t, c.GetRespHeader(HeaderSetCookie), "message:test")

	require.Contains(t, c.GetRespHeader(HeaderSetCookie), ",old_input_data_id:1")
	require.Contains(t, c.GetRespHeader(HeaderSetCookie), ",old_input_data_name:tom")

	c.Redirect().setFlash()
	require.Equal(t, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))
}

// go test -run Test_Redirect_setFlash
func Test_Redirect_setFlash(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderCookie, "fiber_flash=success:1,message:test,old_input_data_name:tom,old_input_data_id:1")

	c.Redirect().setFlash()

	require.Equal(t, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))

	require.Equal(t, "1", c.Redirect().Message("success"))
	require.Equal(t, "test", c.Redirect().Message("message"))
	require.Equal(t, map[string]string{"success": "1", "message": "test"}, c.Redirect().Messages())

	require.Equal(t, "1", c.Redirect().OldInput("id"))
	require.Equal(t, "tom", c.Redirect().OldInput("name"))
	require.Equal(t, map[string]string{"id": "1", "name": "tom"}, c.Redirect().OldInputs())
}

// go test -run Test_Redirect_Request
func Test_Redirect_Request(t *testing.T) {
	t.Parallel()
	app := New()

	app.Get("/", func(c Ctx) error {
		return c.Redirect().With("key", "value").With("key2", "value2").With("co\\:m\\,ma", "Fi\\:ber\\, v3").Route("name")
	})

	app.Get("/with-inputs", func(c Ctx) error {
		return c.Redirect().WithInput().With("key", "value").With("key2", "value2").Route("name")
	})

	app.Get("/just-inputs", func(c Ctx) error {
		return c.Redirect().WithInput().Route("name")
	})

	app.Get("/redirected", func(c Ctx) error {
		return c.JSON(Map{
			"messages": c.Redirect().Messages(),
			"inputs":   c.Redirect().OldInputs(),
		})
	}).Name("name")

	// Start test server
	ln := fasthttputil.NewInmemoryListener()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()

		err := app.Listener(ln, ListenConfig{
			DisableStartupMessage: true,
			GracefulContext:       ctx,
		})

		assert.NoError(t, err)
	}()

	// Test cases
	testCases := []struct {
		URL                string
		CookieValue        string
		ExpectedBody       string
		ExpectedStatusCode int
		ExpectedErr        error
	}{
		{
			URL:                "/",
			CookieValue:        "key:value,key2:value2,co\\:m\\,ma:Fi\\:ber\\, v3",
			ExpectedBody:       `{"inputs":{},"messages":{"co:m,ma":"Fi:ber, v3","key":"value","key2":"value2"}}`,
			ExpectedStatusCode: StatusOK,
			ExpectedErr:        nil,
		},
		{
			URL:                "/with-inputs?name=john&surname=doe",
			CookieValue:        "key:value,key2:value2,key:value,key2:value2,old_input_data_name:john,old_input_data_surname:doe",
			ExpectedBody:       `{"inputs":{"name":"john","surname":"doe"},"messages":{"key":"value","key2":"value2"}}`,
			ExpectedStatusCode: StatusOK,
			ExpectedErr:        nil,
		},
		{
			URL:                "/just-inputs?name=john&surname=doe",
			CookieValue:        "old_input_data_name:john,old_input_data_surname:doe",
			ExpectedBody:       `{"inputs":{"name":"john","surname":"doe"},"messages":{}}`,
			ExpectedStatusCode: StatusOK,
			ExpectedErr:        nil,
		},
	}

	for _, tc := range testCases {
		client := &fasthttp.HostClient{
			Dial: func(_ string) (net.Conn, error) {
				return ln.Dial()
			},
		}
		req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
		req.SetRequestURI("http://example.com" + tc.URL)
		req.Header.SetCookie(FlashCookieName, tc.CookieValue)
		err := client.DoRedirects(req, resp, 1)

		require.NoError(t, err)
		require.Equal(t, tc.ExpectedBody, string(resp.Body()))
		require.Equal(t, tc.ExpectedStatusCode, resp.StatusCode())
	}
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route -benchmem -count=4
func Benchmark_Redirect_Route(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()

	var err error

	for n := 0; n < b.N; n++ {
		err = c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
		})
	}

	require.NoError(b, err)
	require.Equal(b, 302, c.Response().StatusCode())
	require.Equal(b, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route_WithQueries -benchmem -count=4
func Benchmark_Redirect_Route_WithQueries(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()

	var err error

	for n := 0; n < b.N; n++ {
		err = c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
			Queries: map[string]string{"a": "a", "b": "b"},
		})
	}

	require.NoError(b, err)
	require.Equal(b, 302, c.Response().StatusCode())
	// analysis of query parameters with url parsing, since a map pass is always randomly ordered
	location, err := url.Parse(string(c.Response().Header.Peek(HeaderLocation)))
	require.NoError(b, err, "url.Parse(location)")
	require.Equal(b, "/user/fiber", location.Path)
	require.Equal(b, url.Values{"a": []string{"a"}, "b": []string{"b"}}, location.Query())
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route_WithFlashMessages -benchmem -count=4
func Benchmark_Redirect_Route_WithFlashMessages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	b.ReportAllocs()
	b.ResetTimer()

	var err error

	for n := 0; n < b.N; n++ {
		err = c.Redirect().With("success", "1").With("message", "test").Route("user")
	}

	require.NoError(b, err)
	require.Equal(b, 302, c.Response().StatusCode())
	require.Equal(b, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	equal := c.GetRespHeader(HeaderSetCookie) == "fiber_flash=success:1,message:test; path=/; SameSite=Lax" || c.GetRespHeader(HeaderSetCookie) == "fiber_flash=message:test,success:1; path=/; SameSite=Lax"
	require.True(b, equal)

	c.Redirect().setFlash()
	require.Equal(b, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_setFlash -benchmem -count=4
func Benchmark_Redirect_setFlash(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderCookie, "fiber_flash=success:1,message:test,old_input_data_name:tom,old_input_data_id:1")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.Redirect().setFlash()
	}

	require.Equal(b, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))

	require.Equal(b, "1", c.Redirect().Message("success"))
	require.Equal(b, "test", c.Redirect().Message("message"))
	require.Equal(b, map[string]string{"success": "1", "message": "test"}, c.Redirect().Messages())

	require.Equal(b, "1", c.Redirect().OldInput("id"))
	require.Equal(b, "tom", c.Redirect().OldInput("name"))
	require.Equal(b, map[string]string{"id": "1", "name": "tom"}, c.Redirect().OldInputs())
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Messages -benchmem -count=4
func Benchmark_Redirect_Messages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderCookie, "fiber_flash=success:1,message:test,old_input_data_name:tom,old_input_data_id:1")
	c.Redirect().setFlash()

	var msgs map[string]string

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		msgs = c.Redirect().Messages()
	}

	require.Equal(b, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))
	require.Equal(b, map[string]string{"success": "1", "message": "test"}, msgs)
}

// go test -v -run=^$ -bench=Benchmark_Redirect_OldInputs -benchmem -count=4
func Benchmark_Redirect_OldInputs(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderCookie, "fiber_flash=success:1,message:test,old_input_data_name:tom,old_input_data_id:1")
	c.Redirect().setFlash()

	var oldInputs map[string]string

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		oldInputs = c.Redirect().OldInputs()
	}

	require.Equal(b, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))
	require.Equal(b, map[string]string{"id": "1", "name": "tom"}, oldInputs)
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Message -benchmem -count=4
func Benchmark_Redirect_Message(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderCookie, "fiber_flash=success:1,message:test,old_input_data_name:tom,old_input_data_id:1")
	c.Redirect().setFlash()

	var msg string

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		msg = c.Redirect().Message("message")
	}

	require.Equal(b, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))
	require.Equal(b, "test", msg)
}

// go test -v -run=^$ -bench=Benchmark_Redirect_OldInput -benchmem -count=4
func Benchmark_Redirect_OldInput(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Request().Header.Set(HeaderCookie, "fiber_flash=success:1,message:test,old_input_data_name:tom,old_input_data_id:1")
	c.Redirect().setFlash()

	var input string

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		input = c.Redirect().OldInput("name")
	}

	require.Equal(b, "fiber_flash=; expires=Tue, 10 Nov 2009 23:00:00 GMT", c.GetRespHeader(HeaderSetCookie))
	require.Equal(b, "tom", input)
}
