// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"mime/multipart"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
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

// go test -run Test_Redirect_To_WithFlashMessages
func Test_Redirect_To_WithFlashMessages(t *testing.T) {
	t.Parallel()
	app := New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := c.Redirect().With("success", "2").With("success", "1").With("message", "test", 2).To("http://example.com")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))

	c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	_, err = msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
	require.NoError(t, err)

	require.Len(t, msgs, 2)
	require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 2, isOldInput: false})
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

// go test -run Test_Redirect_Back_WithFlashMessages
func Test_Redirect_Back_WithFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	err := c.Redirect().With("success", "1").With("message", "test").Back("/")
	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	_, err = msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
	require.NoError(t, err)

	require.Len(t, msgs, 2)
	require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
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

	require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})

	require.NoError(t, err)
	require.Equal(t, 302, c.Response().StatusCode())
	require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	_, err = msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
	require.NoError(t, err)

	require.Len(t, msgs, 2)
	require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

// go test -run Test_Redirect_Route_WithOldInput
func Test_Redirect_Route_WithOldInput(t *testing.T) {
	t.Parallel()

	t.Run("Query", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Get("/user", func(c Ctx) error {
			return c.SendString("user")
		}).Name("user")

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

		c.Request().URI().SetQueryString("id=1&name=tom")
		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, 302, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		_, err = msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
		require.NoError(t, err)

		require.Len(t, msgs, 4)
		require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "id", value: "1", level: 0, isOldInput: true})
		require.Contains(t, msgs, redirectionMsg{key: "name", value: "tom", level: 0, isOldInput: true})
	})

	t.Run("Form", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Post("/user", func(c Ctx) error {
			return c.SendString("user")
		}).Name("user")

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

		c.Request().Header.Set(HeaderContentType, MIMEApplicationForm)
		c.Request().SetBodyString("id=1&name=tom")
		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, 302, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		_, err = msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
		require.NoError(t, err)

		require.Len(t, msgs, 4)
		require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "id", value: "1", level: 0, isOldInput: true})
		require.Contains(t, msgs, redirectionMsg{key: "name", value: "tom", level: 0, isOldInput: true})
	})

	t.Run("MultipartForm", func(t *testing.T) {
		t.Parallel()

		app := New()
		app.Get("/user", func(c Ctx) error {
			return c.SendString("user")
		}).Name("user")

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		require.NoError(t, writer.WriteField("id", "1"))
		require.NoError(t, writer.WriteField("name", "tom"))
		require.NoError(t, writer.Close())

		c.Request().SetBody(body.Bytes())
		c.Request().Header.Set(HeaderContentType, writer.FormDataContentType())

		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, 302, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		_, err = msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
		require.NoError(t, err)

		require.Len(t, msgs, 4)
		require.Contains(t, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, msgs, redirectionMsg{key: "id", value: "1", level: 0, isOldInput: true})
		require.Contains(t, msgs, redirectionMsg{key: "name", value: "tom", level: 0, isOldInput: true})
	})
}

// go test -run Test_Redirect_parseAndClearFlashMessages
func Test_Redirect_parseAndClearFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	msgs := redirectionMsgs{
		{
			key:   "success",
			value: "1",
		},
		{
			key:   "message",
			value: "test",
		},
		{
			key:        "name",
			value:      "tom",
			isOldInput: true,
		},
		{
			key:        "id",
			value:      "1",
			isOldInput: true,
		},
	}

	val, err := msgs.MarshalMsg(nil)
	require.NoError(t, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+string(val))

	c.Redirect().parseAndClearFlashMessages()

	require.Equal(t, FlashMessage{
		Key:   "success",
		Value: "1",
		Level: 0,
	}, c.Redirect().Message("success"))

	require.Equal(t, FlashMessage{
		Key:   "message",
		Value: "test",
		Level: 0,
	}, c.Redirect().Message("message"))

	require.Equal(t, FlashMessage{}, c.Redirect().Message("not_message"))

	require.Equal(t, []FlashMessage{
		{
			Key:   "success",
			Value: "1",
			Level: 0,
		},
		{
			Key:   "message",
			Value: "test",
			Level: 0,
		},
	}, c.Redirect().Messages())

	require.Equal(t, OldInputData{
		Key:   "id",
		Value: "1",
	}, c.Redirect().OldInput("id"))

	require.Equal(t, OldInputData{
		Key:   "name",
		Value: "tom",
	}, c.Redirect().OldInput("name"))

	require.Equal(t, OldInputData{}, c.Redirect().OldInput("not_name"))

	require.Equal(t, []OldInputData{
		{
			Key:   "name",
			Value: "tom",
		},
		{
			Key:   "id",
			Value: "1",
		},
	}, c.Redirect().OldInputs())
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

	c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	_, err = msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
	require.NoError(b, err)

	require.Contains(b, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(b, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

var testredirectionMsgs = redirectionMsgs{
	{
		key:   "success",
		value: "1",
	},
	{
		key:   "message",
		value: "test",
	},
	{
		key:        "name",
		value:      "tom",
		isOldInput: true,
	},
	{
		key:        "id",
		value:      "1",
		isOldInput: true,
	},
}

// go test -v -run=^$ -bench=Benchmark_Redirect_parseAndClearFlashMessages -benchmem -count=4
func Benchmark_Redirect_parseAndClearFlashMessages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+string(val))

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.Redirect().parseAndClearFlashMessages()
	}

	require.Equal(b, FlashMessage{
		Key:   "success",
		Value: "1",
	}, c.Redirect().Message("success"))

	require.Equal(b, FlashMessage{
		Key:   "message",
		Value: "test",
	}, c.Redirect().Message("message"))

	require.Equal(b, OldInputData{
		Key:   "id",
		Value: "1",
	}, c.Redirect().OldInput("id"))

	require.Equal(b, OldInputData{
		Key:   "name",
		Value: "tom",
	}, c.Redirect().OldInput("name"))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_processFlashMessages -benchmem -count=4
func Benchmark_Redirect_processFlashMessages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	c.Redirect().With("success", "1").With("message", "test")

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		c.Redirect().processFlashMessages()
	}

	c.Context().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	_, err := msgs.UnmarshalMsg([]byte(c.Cookies(FlashCookieName)))
	require.NoError(b, err)

	require.Len(b, msgs, 2)
	require.Contains(b, msgs, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(b, msgs, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Messages -benchmem -count=4
func Benchmark_Redirect_Messages(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+string(val))
	c.Redirect().parseAndClearFlashMessages()

	var msgs []FlashMessage

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		msgs = c.Redirect().Messages()
	}

	require.Contains(b, msgs, FlashMessage{
		Key:   "success",
		Value: "1",
		Level: 0,
	})

	require.Contains(b, msgs, FlashMessage{
		Key:   "message",
		Value: "test",
		Level: 0,
	})
}

// go test -v -run=^$ -bench=Benchmark_Redirect_OldInputs -benchmem -count=4
func Benchmark_Redirect_OldInputs(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+string(val))
	c.Redirect().parseAndClearFlashMessages()

	var oldInputs []OldInputData

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		oldInputs = c.Redirect().OldInputs()
	}

	require.Contains(b, oldInputs, OldInputData{
		Key:   "name",
		Value: "tom",
	})

	require.Contains(b, oldInputs, OldInputData{
		Key:   "id",
		Value: "1",
	})
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Message -benchmem -count=4
func Benchmark_Redirect_Message(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+string(val))
	c.Redirect().parseAndClearFlashMessages()

	var msg FlashMessage

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		msg = c.Redirect().Message("message")
	}

	require.Equal(b, FlashMessage{
		Key:   "message",
		Value: "test",
		Level: 0,
	}, msg)
}

// go test -v -run=^$ -bench=Benchmark_Redirect_OldInput -benchmem -count=4
func Benchmark_Redirect_OldInput(b *testing.B) {
	app := New()
	app.Get("/user", func(c Ctx) error {
		return c.SendString("user")
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck, forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+string(val))
	c.Redirect().parseAndClearFlashMessages()

	var input OldInputData

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		input = c.Redirect().OldInput("name")
	}

	require.Equal(b, OldInputData{
		Key:   "name",
		Value: "tom",
	}, input)
}
