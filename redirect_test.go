// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	defer app.ReleaseCtx(c)

	err := c.Redirect().To("http://default.com")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "http://example.com", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(t, err)
	_, err = msgs.UnmarshalMsg(decoded)
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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())

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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	err := c.Redirect().With("success", "1").With("message", "test").Back("/")
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(t, err)
	_, err = msgs.UnmarshalMsg(decoded)
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
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	err := c.Redirect().With("success", "1").With("message", "test").Route("user")

	require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
	require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})

	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, c.Response().StatusCode())
	require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(t, err)
	_, err = msgs.UnmarshalMsg(decoded)
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

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

		c.Request().URI().SetQueryString("id=1&name=tom")
		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, StatusSeeOther, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
		require.NoError(t, err)
		_, err = msgs.UnmarshalMsg(decoded)
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

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

		c.Request().Header.Set(HeaderContentType, MIMEApplicationForm)
		c.Request().SetBodyString("id=1&name=tom")
		err := c.Redirect().With("success", "1").With("message", "test").WithInput().Route("user")

		require.Contains(t, c.redirect.messages, redirectionMsg{key: "success", value: "1", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "message", value: "test", level: 0, isOldInput: false})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "id", value: "1", isOldInput: true})
		require.Contains(t, c.redirect.messages, redirectionMsg{key: "name", value: "tom", isOldInput: true})

		require.NoError(t, err)
		require.Equal(t, StatusSeeOther, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
		require.NoError(t, err)
		_, err = msgs.UnmarshalMsg(decoded)
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

		c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

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
		require.Equal(t, StatusSeeOther, c.Response().StatusCode())
		require.Equal(t, "/user", string(c.Response().Header.Peek(HeaderLocation)))

		c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

		var msgs redirectionMsgs
		decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
		require.NoError(t, err)
		_, err = msgs.UnmarshalMsg(decoded)
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

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

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))

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

	c.Request().Header.Set(HeaderCookie, "fiber_flash=test")

	c.Redirect().parseAndClearFlashMessages()

	require.Empty(t, c.Redirect().messages)
}

// Test_Redirect_parseAndClearFlashMessages_InvalidHex tests the case where hex decoding fails
func Test_Redirect_parseAndClearFlashMessages_InvalidHex(t *testing.T) {
	t.Parallel()

	app := New()

	// Setup request and response
	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed
	defer app.ReleaseCtx(c)

	// Create redirect instance
	r := AcquireRedirect()
	r.c = c

	// Set invalid hex value in flash cookie
	c.Request().Header.SetCookie(FlashCookieName, "not-a-valid-hex-string")

	// Call parseAndClearFlashMessages
	r.parseAndClearFlashMessages()

	// Verify that no flash messages are processed (should be empty)
	require.Empty(t, r.messages)

	// Release redirect
	ReleaseRedirect(r)
}

func Test_Redirect_CompleteFlowWithFlashMessages(t *testing.T) {
	t.Parallel()

	app := New()

	// First handler that sets flash messages and redirects
	app.Get("/source", func(c Ctx) error {
		// Redirect to the target handler
		return c.Redirect().With("string_message", "Hello, World!").
			With("number_message", "12345").
			With("bool_message", "true").
			To("/target")
	})

	// Second handler that receives and processes flash messages
	app.Get("/target", func(c Ctx) error {
		// Get all flash messages and return them as a JSON response
		return c.JSON(Map{
			"string_message": c.Redirect().Message("string_message").Value,
			"number_message": c.Redirect().Message("number_message").Value,
			"bool_message":   c.Redirect().Message("bool_message").Value,
		})
	})

	// Step 1: Make the initial request to the source route
	req := httptest.NewRequest(MethodGet, "/source", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/target", resp.Header.Get(HeaderLocation))

	// Verify and get the cookie from the response
	cookies := resp.Cookies()
	var flashCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "fiber_flash" {
			flashCookie = cookie
			break
		}
	}
	require.NotNil(t, flashCookie, "Flash cookie should be set")

	// Step 2: Make the second request to the target route with the cookie
	req = httptest.NewRequest(MethodGet, "/target", nil)
	req.Header.Set("Cookie", flashCookie.Name+"="+flashCookie.Value)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	// Parse the JSON response and verify flash messages
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Verify all flash messages were received correctly
	require.Equal(t, "Hello, World!", result["string_message"])
	require.Equal(t, "12345", result["number_message"]) // JSON numbers are float64
	require.Equal(t, "true", result["bool_message"])
}

func Test_Redirect_FlashMessagesWithSpecialChars(t *testing.T) {
	t.Parallel()

	app := New()

	// Handler that sets flash messages with special characters and redirects
	app.Get("/special-source", func(c Ctx) error {
		// Create a large message to test encoding of larger data
		return c.Redirect().With("null_bytes", "Contains\x00null\x00bytes").
			With("control_chars", "Contains\r\ncontrol\tcharacters").
			With("unicode", "Unicode: 你好世界").
			With("emoji", "Emoji: 🔥🚀😊").
			To("/special-target")
	})

	// Target handler that receives the flash messages
	app.Get("/special-target", func(c Ctx) error {
		return c.JSON(Map{
			"null_bytes":    c.Redirect().Message("null_bytes").Value,
			"control_chars": c.Redirect().Message("control_chars").Value,
			"unicode":       c.Redirect().Message("unicode").Value,
			"emoji":         c.Redirect().Message("emoji").Value,
		})
	})

	// Step 1: Make the initial request
	req := httptest.NewRequest(MethodGet, "/special-source", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/special-target", resp.Header.Get(HeaderLocation))

	// Get the flash cookie
	var flashCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "fiber_flash" {
			flashCookie = cookie
			break
		}
	}
	require.NotNil(t, flashCookie, "Flash cookie should be set")

	// Step 2: Make the second request with the cookie
	req = httptest.NewRequest(MethodGet, "/special-target", nil)
	req.Header.Set("Cookie", flashCookie.Name+"="+flashCookie.Value)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, StatusOK, resp.StatusCode)

	// Parse and verify the response
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	// Verify special character handling
	require.Equal(t, "Contains\x00null\x00bytes", result["null_bytes"])
	require.Equal(t, "Contains\r\ncontrol\tcharacters", result["control_chars"])
	require.Equal(t, "Unicode: 你好世界", result["unicode"])
	require.Equal(t, "Emoji: 🔥🚀😊", result["emoji"])
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route -benchmem -count=4
func Benchmark_Redirect_Route(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()

	var err error

	for b.Loop() {
		err = c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
		})
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
	require.Equal(b, "/user/fiber", string(c.Response().Header.Peek(HeaderLocation)))
}

// go test -v -run=^$ -bench=Benchmark_Redirect_Route_WithQueries -benchmem -count=4
func Benchmark_Redirect_Route_WithQueries(b *testing.B) {
	app := New()
	app.Get("/user/:name", func(c Ctx) error {
		return c.JSON(c.Params("name"))
	}).Name("user")

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()

	var err error

	for b.Loop() {
		err = c.Redirect().Route("user", RedirectConfig{
			Params: Map{
				"name": "fiber",
			},
			Queries: map[string]string{"a": "a", "b": "b"},
		})
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	b.ReportAllocs()

	var err error

	for b.Loop() {
		err = c.Redirect().With("success", "1").With("message", "test").Route("user")
	}

	require.NoError(b, err)
	require.Equal(b, StatusSeeOther, c.Response().StatusCode())
	require.Equal(b, "/user", string(c.Response().Header.Peek(HeaderLocation)))

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(b, err)
	_, err = msgs.UnmarshalMsg(decoded)
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))

	b.ReportAllocs()

	for b.Loop() {
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	c.Redirect().With("success", "1").With("message", "test")

	b.ReportAllocs()

	for b.Loop() {
		c.Redirect().processFlashMessages()
	}

	c.RequestCtx().Request.Header.Set(HeaderCookie, c.GetRespHeader(HeaderSetCookie)) // necessary for testing

	var msgs redirectionMsgs
	decoded, err := hex.DecodeString(c.Cookies(FlashCookieName))
	require.NoError(b, err)
	_, err = msgs.UnmarshalMsg(decoded)
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var msgs []FlashMessage

	b.ReportAllocs()

	for b.Loop() {
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var oldInputs []OldInputData

	b.ReportAllocs()

	for b.Loop() {
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var msg FlashMessage

	b.ReportAllocs()

	for b.Loop() {
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

	c := app.AcquireCtx(&fasthttp.RequestCtx{}).(*DefaultCtx) //nolint:errcheck,forcetypeassert // not needed

	val, err := testredirectionMsgs.MarshalMsg(nil)
	require.NoError(b, err)

	c.Request().Header.Set(HeaderCookie, "fiber_flash="+hex.EncodeToString(val))
	c.Redirect().parseAndClearFlashMessages()

	var input OldInputData

	b.ReportAllocs()

	for b.Loop() {
		input = c.Redirect().OldInput("name")
	}

	require.Equal(b, OldInputData{
		Key:   "name",
		Value: "tom",
	}, input)
}
