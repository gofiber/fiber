package csrf

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_CSRF(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	methods := [4]string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace}

	for _, method := range methods {
		// Generate CSRF token
		ctx.Request.Header.SetMethod(method)
		h(ctx)

		// Without CSRF cookie
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		h(ctx)
		require.Equal(t, 403, ctx.Response.StatusCode())

		// Invalid CSRF token
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.Set(HeaderName, "johndoe")
		h(ctx)
		require.Equal(t, 403, ctx.Response.StatusCode())

		// Valid CSRF token
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(method)
		h(ctx)
		token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
		token = strings.Split(strings.Split(token, ";")[0], "=")[1]

		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.Set(HeaderName, token)
		ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
		h(ctx)
		require.Equal(t, 200, ctx.Response.StatusCode())
	}
}

func Test_CSRF_WithSession(t *testing.T) {
	t.Parallel()

	// session store
	store := session.NewStore(session.Config{
		KeyLookup: "cookie:_session",
	})

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := &fasthttp.RequestCtx{}
	defer app.ReleaseCtx(app.AcquireCtx(ctx))

	// get session
	sess, err := store.Get(app.AcquireCtx(ctx))
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	// the session string is no longer be 123
	newSessionIDString := sess.ID()
	require.NoError(t, sess.Save())

	app.AcquireCtx(ctx).Request().Header.SetCookie("_session", newSessionIDString)

	// middleware config
	config := Config{
		Session: store,
	}

	// middleware
	app.Use(New(config))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	methods := [4]string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace}

	for _, method := range methods {
		// Generate CSRF token
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		ctx.Request.Header.SetCookie("_session", newSessionIDString)
		h(ctx)

		// Without CSRF cookie
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.SetCookie("_session", newSessionIDString)
		h(ctx)
		require.Equal(t, 403, ctx.Response.StatusCode())

		// Empty/invalid CSRF token
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.Set(HeaderName, "johndoe")
		ctx.Request.Header.SetCookie("_session", newSessionIDString)
		h(ctx)
		require.Equal(t, 403, ctx.Response.StatusCode())

		// Valid CSRF token
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(method)
		ctx.Request.Header.SetCookie("_session", newSessionIDString)
		h(ctx)
		token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
		for _, header := range strings.Split(token, ";") {
			if strings.Split(utils.Trim(header, ' '), "=")[0] == ConfigDefault.CookieName {
				token = strings.Split(header, "=")[1]
				break
			}
		}

		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.Set(HeaderName, token)
		ctx.Request.Header.SetCookie("_session", newSessionIDString)
		ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
		h(ctx)
		require.Equal(t, 200, ctx.Response.StatusCode())
	}
}

// go test -run Test_CSRF_WithSession_Middleware
func Test_CSRF_WithSession_Middleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// session mw
	smh, sstore := session.NewWithStore()

	// csrf mw
	cmh := New(Config{
		Session: sstore,
	})

	app.Use(smh)

	app.Use(cmh)

	app.Get("/", func(c fiber.Ctx) error {
		sess := session.FromContext(c)
		sess.Set("hello", "world")
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/", func(c fiber.Ctx) error {
		sess := session.FromContext(c)
		if sess.Get("hello") != "world" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token and session_id
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	csrfTokenParts := strings.Split(string(ctx.Response.Header.Peek(fiber.HeaderSetCookie)), ";")
	require.Greater(t, len(csrfTokenParts), 2)
	csrfToken := strings.Split(csrfTokenParts[0], "=")[1]
	require.NotEmpty(t, csrfToken)
	sessionID := strings.Split(csrfTokenParts[1], "=")[1]
	require.NotEmpty(t, sessionID)

	// Use the CSRF token and session_id
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, csrfToken)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, csrfToken)
	ctx.Request.Header.SetCookie("session_id", sessionID)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
}

// go test -run Test_CSRF_ExpiredToken
func Test_CSRF_ExpiredToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		IdleTimeout: 1 * time.Second,
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Use the CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Wait for the token to expire
	time.Sleep(1250 * time.Millisecond)

	// Expired CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

// go test -run Test_CSRF_ExpiredToken_WithSession
func Test_CSRF_ExpiredToken_WithSession(t *testing.T) {
	t.Parallel()

	// session store
	store := session.NewStore(session.Config{
		KeyLookup: "cookie:_session",
	})

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := &fasthttp.RequestCtx{}
	defer app.ReleaseCtx(app.AcquireCtx(ctx))

	// get session
	sess, err := store.Get(app.AcquireCtx(ctx))
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	// get session id
	newSessionIDString := sess.ID()
	require.NoError(t, sess.Save())

	app.AcquireCtx(ctx).Request().Header.SetCookie("_session", newSessionIDString)

	// middleware config
	config := Config{
		Session:     store,
		IdleTimeout: 1 * time.Second,
	}

	// middleware
	app.Use(New(config))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("_session", newSessionIDString)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	for _, header := range strings.Split(token, ";") {
		if strings.Split(utils.Trim(header, ' '), "=")[0] == ConfigDefault.CookieName {
			token = strings.Split(header, "=")[1]
			break
		}
	}

	// Use the CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie("_session", newSessionIDString)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Wait for the token to expire
	time.Sleep(1*time.Second + 100*time.Millisecond)

	// Expired CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie("_session", newSessionIDString)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

// go test -run Test_CSRF_MultiUseToken
func Test_CSRF_MultiUseToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		KeyLookup: "header:X-Csrf-Token",
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set("X-Csrf-Token", "johndoe")
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set("X-Csrf-Token", token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	newToken := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	newToken = strings.Split(strings.Split(newToken, ";")[0], "=")[1]
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Check if the token is not a dummy value
	require.Equal(t, token, newToken)
}

// go test -run Test_CSRF_SingleUseToken
func Test_CSRF_SingleUseToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		SingleUseToken: true,
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Use the CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	newToken := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	newToken = strings.Split(strings.Split(newToken, ";")[0], "=")[1]
	if token == newToken {
		t.Error("new token should not be the same as the old token")
	}

	// Use the CSRF token again
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

// go test -run Test_CSRF_Next
func Test_CSRF_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_CSRF_Invalid_KeyLookup(t *testing.T) {
	t.Parallel()
	defer func() {
		require.Equal(t, "[CSRF] KeyLookup must in the form of <source>:<key>", recover())
	}()
	app := fiber.New()

	app.Use(New(Config{KeyLookup: "I:am:invalid"}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
}

func Test_CSRF_From_Form(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{KeyLookup: "form:_csrf"}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	ctx.Request.SetBodyString("_csrf=" + token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
}

func Test_CSRF_From_Query(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{KeyLookup: "query:_csrf"}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/?_csrf=" + utils.UUIDv4())
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.SetRequestURI("/?_csrf=" + token)
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "OK", string(ctx.Response.Body()))
}

func Test_CSRF_From_Param(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	csrfGroup := app.Group("/:csrf", New(Config{KeyLookup: "param:csrf"}))

	csrfGroup.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/" + utils.UUIDv4())
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/" + utils.UUIDv4())
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.SetRequestURI("/" + token)
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "OK", string(ctx.Response.Body()))
}

func Test_CSRF_From_Cookie(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	csrfGroup := app.Group("/", New(Config{KeyLookup: "cookie:csrf"}))

	csrfGroup.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf="+utils.UUIDv4()+";")
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf="+token+";")
	ctx.Request.SetRequestURI("/")
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "OK", string(ctx.Response.Body()))
}

func Test_CSRF_From_Custom(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	extractor := func(c fiber.Ctx) (string, error) {
		body := string(c.Body())
		// Generate the correct extractor to get the token from the correct location
		selectors := strings.Split(body, "=")

		if len(selectors) != 2 || selectors[1] == "" {
			return "", ErrMissingParam
		}
		return selectors[1], nil
	}

	app.Use(New(Config{Extractor: extractor}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Invalid CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
	ctx.Request.SetBodyString("_csrf=" + token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
}

func Test_CSRF_Extractor_EmptyString(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	extractor := func(_ fiber.Ctx) (string, error) {
		return "", nil
	}

	errorHandler := func(c fiber.Ctx, err error) error {
		return c.Status(403).SendString(err.Error())
	}

	app.Use(New(Config{
		Extractor:    extractor,
		ErrorHandler: errorHandler,
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
	ctx.Request.SetBodyString("_csrf=" + token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
	require.Equal(t, ErrTokenNotFound.Error(), string(ctx.Response.Body()))
}

func Test_CSRF_Origin(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{CookieSecure: true}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "http")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Test Correct Origin with port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com:8080")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com:8080")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com:8080")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Correct Origin with wrong port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com:3000")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Test Correct Origin with null
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "null")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Correct Origin with ReverseProxy
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("10.0.1.42.com:8080")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("10.0.1.42:8080")
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "http")
	ctx.Request.Header.Set(fiber.HeaderXForwardedHost, "example.com")
	ctx.Request.Header.Set(fiber.HeaderXForwardedFor, `192.0.2.43, "[2001:db8:cafe::17]"`)
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Correct Origin with ReverseProxy Missing X-Forwarded-* Headers
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("10.0.1.42:8080")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("10.0.1.42:8080")
	ctx.Request.Header.Set(fiber.HeaderXUrlScheme, "http") // We need to set this header to make sure c.Protocol() returns http
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Test Wrong Origin
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "http")
	ctx.Request.Header.Set(fiber.HeaderXForwardedHost, "example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://csrf.example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_TrustedOrigins(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{
		CookieSecure: true,
		TrustedOrigins: []string{
			"http://safe.example.com",
			"https://safe.example.com",
			"http://*.domain-1.com",
			"https://*.domain-1.com",
		},
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Test Trusted Origin
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://safe.example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Origin Subdomain
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("domain-1.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://safe.domain-1.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Origin deeply nested subdomain
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("a.b.c.domain-1.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("a.b.c.domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "https://a.b.c.domain-1.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Origin Invalid
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("domain-1.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://evildomain-1.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Test Trusted Referer
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://safe.example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Referer Wildcard
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("domain-1.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://safe.domain-1.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Referer deeply nested subdomain
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("a.b.c.domain-1.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("a.b.c.domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://a.b.c.domain-1.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Referer Invalid
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("api.domain-1.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("api.domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://evildomain-1.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_TrustedOrigins_InvalidOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		origin string
	}{
		{name: "No Scheme", origin: "localhost"},
		{name: "Wildcard", origin: "https://*"},
		{name: "Wildcard domain", origin: "https://*example.com"},
		{name: "File Scheme", origin: "file://example.com"},
		{name: "FTP Scheme", origin: "ftp://example.com"},
		{name: "Port Wildcard", origin: "http://example.com:*"},
		{name: "Multiple Wildcards", origin: "https://*.*.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := tt.origin
			t.Parallel()
			require.Panics(t, func() {
				app := fiber.New()
				app.Use(New(Config{
					CookieSecure:   true,
					TrustedOrigins: []string{origin},
				}))
			}, "Expected panic")
		})
	}
}

func Test_CSRF_Referer(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{CookieSecure: true}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Test Correct Referer with port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com:8443")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com:8443")
	ctx.Request.Header.Set(fiber.HeaderReferer, ctx.Request.URI().String())
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Correct Referer with ReverseProxy
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("10.0.1.42.com:8443")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("10.0.1.42:8443")
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedHost, "example.com")
	ctx.Request.Header.Set(fiber.HeaderXForwardedFor, `192.0.2.43, "[2001:db8:cafe::17]"`)
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Correct Referer with ReverseProxy Missing X-Forwarded-* Headers
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("10.0.1.42:8443")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("10.0.1.42:8443")
	ctx.Request.Header.Set(fiber.HeaderXUrlScheme, "https") // We need to set this header to make sure c.Protocol() returns https
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Test Correct Referer with path
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedHost, "example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com/action/items?gogogo=true")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Wrong Referer
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedHost, "example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://csrf.example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_DeleteToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	config := ConfigDefault

	app.Use(New(config))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// DeleteToken after token generation and remove the cookie
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.Set(HeaderName, "")
	handler := HandlerFromContext(app.AcquireCtx(ctx))
	if handler != nil {
		ctx.Request.Header.DelAllCookies()
		err := handler.DeleteToken(app.AcquireCtx(ctx))
		require.ErrorIs(t, err, ErrTokenNotFound)
	}
	h(ctx)

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Delete the CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	handler = HandlerFromContext(app.AcquireCtx(ctx))
	if handler != nil {
		if err := handler.DeleteToken(app.AcquireCtx(ctx)); err != nil {
			t.Fatal(err)
		}
	}
	h(ctx)

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_DeleteToken_WithSession(t *testing.T) {
	t.Parallel()

	// session store
	store := session.NewStore(session.Config{
		KeyLookup: "cookie:_session",
	})

	// fiber instance
	app := fiber.New()

	// fiber context
	ctx := &fasthttp.RequestCtx{}

	// get session
	sess, err := store.Get(app.AcquireCtx(ctx))
	require.NoError(t, err)
	require.True(t, sess.Fresh())

	// the session string is no longer be 123
	newSessionIDString := sess.ID()
	require.NoError(t, sess.Save())

	app.AcquireCtx(ctx).Request().Header.SetCookie("_session", newSessionIDString)

	// middleware config
	config := Config{
		Session: store,
	}

	// middleware
	app.Use(New(config))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("_session", newSessionIDString)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Delete the CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	handler := HandlerFromContext(app.AcquireCtx(ctx))
	if handler != nil {
		if err := handler.DeleteToken(app.AcquireCtx(ctx)); err != nil {
			t.Fatal(err)
		}
	}
	h(ctx)

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	ctx.Request.Header.SetCookie("_session", newSessionIDString)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_ErrorHandler_InvalidToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	errHandler := func(ctx fiber.Ctx, err error) error {
		require.Equal(t, ErrTokenInvalid, err)
		return ctx.Status(419).Send([]byte("invalid CSRF token"))
	}

	app.Use(New(Config{ErrorHandler: errHandler}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)

	// invalid CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, "johndoe")
	h(ctx)
	require.Equal(t, 419, ctx.Response.StatusCode())
	require.Equal(t, "invalid CSRF token", string(ctx.Response.Body()))
}

func Test_CSRF_ErrorHandler_EmptyToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	errHandler := func(ctx fiber.Ctx, err error) error {
		require.Equal(t, ErrMissingHeader, err)
		return ctx.Status(419).Send([]byte("empty CSRF token"))
	}

	app.Use(New(Config{ErrorHandler: errHandler}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)

	// empty CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	h(ctx)
	require.Equal(t, 419, ctx.Response.StatusCode())
	require.Equal(t, "empty CSRF token", string(ctx.Response.Body()))
}

func Test_CSRF_ErrorHandler_MissingReferer(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	errHandler := func(ctx fiber.Ctx, err error) error {
		require.Equal(t, ErrRefererNotFound, err)
		return ctx.Status(419).Send([]byte("empty CSRF token"))
	}

	app.Use(New(Config{
		CookieSecure: true,
		ErrorHandler: errHandler,
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedHost, "example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 419, ctx.Response.StatusCode())
}

func Test_CSRF_Cookie_Injection_Exploit(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Inject CSRF token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf_=pwned;")
	ctx.Request.SetRequestURI("/")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Exploit CSRF token we just injected
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.Set(fiber.HeaderCookie, "csrf_=pwned;")
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode(), "CSRF exploit successful")
}

// TODO: use this test case and make the unsafe header value bug from https://github.com/gofiber/fiber/issues/2045 reproducible and permanently fixed/tested by this testcase
// func Test_CSRF_UnsafeHeaderValue(t *testing.T) {
//  t.Parallel()
// 	app := fiber.New()

// 	app.Use(New())
// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendStatus(fiber.StatusOK)
// 	})
// 	app.Get("/test", func(c fiber.Ctx) error {
// 		return c.SendStatus(fiber.StatusOK)
// 	})
// 	app.Post("/", func(c fiber.Ctx) error {
// 		return c.SendStatus(fiber.StatusOK)
// 	})

// 	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
// 	require.NoError(t, err)
// 	require.Equal(t, fiber.StatusOK, resp.StatusCode)

// 	var token string
// 	for _, c := range resp.Cookies() {
// 		if c.Name != ConfigDefault.CookieName {
// 			continue
// 		}
// 		token = c.Value
// 		break
// 	}

// 	fmt.Println("token", token)

// 	getReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
// 	getReq.Header.Set(HeaderName, token)
// 	resp, err = app.Test(getReq)

// 	getReq = httptest.NewRequest(fiber.MethodGet, "/test", nil)
// 	getReq.Header.Set("X-Requested-With", "XMLHttpRequest")
// 	getReq.Header.Set(fiber.HeaderCacheControl, "no")
// 	getReq.Header.Set(HeaderName, token)

// 	resp, err = app.Test(getReq)

// 	getReq.Header.Set(fiber.HeaderAccept, "*/*")
// 	getReq.Header.Del(HeaderName)
// 	resp, err = app.Test(getReq)

// 	postReq := httptest.NewRequest(fiber.MethodPost, "/", nil)
// 	postReq.Header.Set("X-Requested-With", "XMLHttpRequest")
// 	postReq.Header.Set(HeaderName, token)
// 	resp, err = app.Test(postReq)
// }

// go test -v -run=^$ -bench=Benchmark_Middleware_CSRF_Check -benchmem -count=4
func Benchmark_Middleware_CSRF_Check(b *testing.B) {
	app := fiber.New()

	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Test Correct Referer POST
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)

	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(ctx)
	}

	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}

// go test -v -run=^$ -bench=Benchmark_Middleware_CSRF_GenerateToken -benchmem -count=4
func Benchmark_Middleware_CSRF_GenerateToken(b *testing.B) {
	app := fiber.New()

	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusTeapot)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	b.ReportAllocs()
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		h(ctx)
	}

	// Ensure the GET request returns a 418 status code
	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}

func Test_CSRF_InvalidURLHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	errHandler := func(ctx fiber.Ctx, err error) error {
		return ctx.Status(419).Send([]byte(err.Error()))
	}

	app.Use(New(Config{ErrorHandler: errHandler}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "http")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// invalid Origin
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://[::1]:%38%30/Invalid Origin")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 419, ctx.Response.StatusCode())
	require.Equal(t, ErrOriginInvalid.Error(), string(ctx.Response.Body()))

	// invalid Referer
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "http://[::1]:%38%30/Invalid Referer")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 419, ctx.Response.StatusCode())
	require.Equal(t, ErrRefererInvalid.Error(), string(ctx.Response.Body()))
}

func Test_CSRF_TokenFromContext(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.NotEmpty(t, token)
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_CSRF_FromContextMethods(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.NotEmpty(t, token)

		handler := HandlerFromContext(c)
		require.NotNil(t, handler)

		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_CSRF_FromContextMethods_Invalid(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Get("/", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.Empty(t, token)

		handler := HandlerFromContext(c)
		require.Nil(t, handler)

		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test_CSRF_GoContext Test for getting token and handler from Go standard context
func Test_CSRF_Context(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		// Get token and handler from fiber.Ctx
		fiberToken := TokenFromContext(c)
		fiberHandler := HandlerFromContext(c)
		require.NotEmpty(t, fiberToken)
		require.NotNil(t, fiberHandler)

		// Get token and handle from context.Contextr
		goCtx := c.Context()
		goToken := TokenFromContext(goCtx)
		goHandler := HandlerFromContext(goCtx)

		// Verify that the data obtained in both ways is consistent
		require.Equal(t, fiberToken, goToken)
		require.Equal(t, fiberHandler, goHandler)

		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test_CSRF_ServiceLayer Testing the use of Go standard contexts in the service layer
func Test_CSRF_ServiceLayer(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/service", func(c fiber.Ctx) error {
		// Simulate passing context to a service layer function
		result := simulateServiceLayer(c.Context())
		return c.SendString(result)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/service", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Token found in service layer", string(body))
}

// simulateServiceLayer Simulates a service layer function that requires access to a CSRF token
func simulateServiceLayer(ctx context.Context) string {
	token := TokenFromContext(ctx)
	if token == "" {
		return "No token found in service layer"
	}
	return "Token found in service layer"
}

// Test_CSRF_TokenConsistency Test for token consistency over the request cycle
func Test_CSRF_TokenConsistency(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	var fiberCtxToken string
	var goCtxToken string

	// First middleware, logging tokens in advance
	app.Use(func(c fiber.Ctx) error {
		c.Locals("before_middleware", true)
		return c.Next()
	})

	// CSRF middleware
	app.Use(New())

	// Third middleware, check tokens
	app.Use(func(c fiber.Ctx) error {
		// Check that the token is set correctly
		fiberCtxToken = TokenFromContext(c)
		goCtxToken = TokenFromContext(c.Context())

		require.NotEmpty(t, fiberCtxToken)
		require.Equal(t, fiberCtxToken, goCtxToken)

		return c.Next()
	})

	app.Get("/", func(c fiber.Ctx) error {
		// Verify that the token in the route processor matches the one in the middleware
		routeToken := TokenFromContext(c)
		require.Equal(t, fiberCtxToken, routeToken)

		routeGoToken := TokenFromContext(c.Context())
		require.Equal(t, goCtxToken, routeGoToken)

		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

// Test_CSRF_CrossRequest Testing cross-request token passing
func Test_CSRF_CrossRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/token", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.NotEmpty(t, token)
		return c.SendString(token)
	})

	app.Post("/verify", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.NotEmpty(t, token)
		return c.SendStatus(fiber.StatusOK)
	})

	// Get token
	getReq := httptest.NewRequest(fiber.MethodGet, "/token", nil)
	resp, err := app.Test(getReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	token := string(body)
	require.NotEmpty(t, token)

	// Get cookie from response
	var csrfCookie string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == ConfigDefault.CookieName {
			csrfCookie = cookie.Value
			break
		}
	}
	require.NotEmpty(t, csrfCookie)

	// Validate token
	postReq := httptest.NewRequest(fiber.MethodPost, "/verify", nil)
	postReq.Header.Set(HeaderName, token)
	postReq.AddCookie(&http.Cookie{
		Name:  ConfigDefault.CookieName,
		Value: csrfCookie,
	})

	resp, err = app.Test(postReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}
