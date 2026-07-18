package csrf

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/redact"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type failingCSRFStorage struct {
	data map[string][]byte
	errs map[string]error
}

func newFailingCSRFStorage() *failingCSRFStorage {
	return &failingCSRFStorage{
		data: make(map[string][]byte),
		errs: make(map[string]error),
	}
}

func (s *failingCSRFStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	if err, ok := s.errs["get|"+key]; ok && err != nil {
		return nil, err
	}
	if val, ok := s.data[key]; ok {
		return append([]byte(nil), val...), nil
	}
	return nil, nil
}

var trustedProxyConfig = fiber.Config{
	TrustProxy: true,
	TrustProxyConfig: fiber.TrustProxyConfig{
		Proxies: []string{"0.0.0.0"},
	},
}

func newTrustedApp() *fiber.App {
	return fiber.New(trustedProxyConfig)
}

func newTrustedRequestCtx() *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.SetRemoteAddr(net.Addr(&net.TCPAddr{IP: net.ParseIP("0.0.0.0")}))

	return ctx
}

func (s *failingCSRFStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *failingCSRFStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	if err, ok := s.errs["set|"+key]; ok && err != nil {
		return err
	}
	s.data[key] = append([]byte(nil), val...)
	return nil
}

func (s *failingCSRFStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (s *failingCSRFStorage) DeleteWithContext(_ context.Context, key string) error {
	if err, ok := s.errs["del|"+key]; ok && err != nil {
		return err
	}
	delete(s.data, key)
	return nil
}

func (s *failingCSRFStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *failingCSRFStorage) ResetWithContext(context.Context) error {
	s.data = make(map[string][]byte)
	s.errs = make(map[string]error)
	return nil
}

func (s *failingCSRFStorage) Reset() error {
	return s.ResetWithContext(context.Background())
}

func (*failingCSRFStorage) Close() error { return nil }

func TestCSRFStorageGetError(t *testing.T) {
	t.Parallel()

	storage := newFailingCSRFStorage()
	storage.errs["get|token"] = errors.New("boom")

	var captured error
	app := fiber.New()

	app.Use(New(Config{
		Storage: storage,
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return fiber.ErrTeapot
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: "token"})

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "csrf: failed to fetch token from storage")
}

func TestCSRFStorageSetError(t *testing.T) {
	t.Parallel()

	storage := newFailingCSRFStorage()
	storage.errs["set|token"] = errors.New("boom")

	var captured error
	app := fiber.New()

	app.Use(New(Config{
		Storage: storage,
		KeyGenerator: func() string {
			return "token"
		},
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return fiber.ErrTeapot
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "csrf: failed to store token in storage")
}

func TestCSRFStorageDeleteError(t *testing.T) {
	t.Parallel()

	storage := newFailingCSRFStorage()
	storage.data["token"] = []byte("value")
	storage.errs["del|token"] = errors.New("boom")

	var captured error
	app := fiber.New()

	app.Use(New(Config{
		Storage:        storage,
		SingleUseToken: true,
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return fiber.ErrTeapot
		},
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodPost, "/", http.NoBody)
	req.Header.Set(HeaderName, "token")
	req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: "token"})

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.Error(t, captured)
	require.ErrorContains(t, captured, "csrf: failed to delete token from storage")
}

func Test_CSRF(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	methods := [5]string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery}

	for _, method := range methods {
		// Generate CSRF token
		ctx.Request.Header.SetMethod(method)
		h(ctx)

		// Without CSRF cookie
		ctx.Request.Header.Reset()
		ctx.Request.ResetBody()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		h(ctx)
		require.Equal(t, 403, ctx.Response.StatusCode())

		// Invalid CSRF token
		ctx.Request.Header.Reset()
		ctx.Request.ResetBody()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.Header.Set(HeaderName, "johndoe")
		h(ctx)
		require.Equal(t, 403, ctx.Response.StatusCode())

		// Valid CSRF token
		ctx.Request.Header.Reset()
		ctx.Request.ResetBody()
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
		Extractor: extractors.FromCookie("_session"),
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

	methods := [5]string{fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace, fiber.MethodQuery}

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
		for header := range strings.SplitSeq(token, ";") {
			if strings.Split(utils.TrimSpace(header), "=")[0] == ConfigDefault.CookieName {
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

	csrfCookie := fasthttp.AcquireCookie()
	csrfCookie.SetKey(ConfigDefault.CookieName)
	require.True(t, ctx.Response.Header.Cookie(csrfCookie))
	csrfToken := string(csrfCookie.Value())
	require.NotEmpty(t, csrfToken)
	fasthttp.ReleaseCookie(csrfCookie)

	sessionCookie := fasthttp.AcquireCookie()
	sessionCookie.SetKey("session_id")
	require.True(t, ctx.Response.Header.Cookie(sessionCookie))
	sessionID := string(sessionCookie.Value())
	require.NotEmpty(t, sessionID)
	fasthttp.ReleaseCookie(sessionCookie)

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
		Extractor: extractors.FromCookie("_session"),
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
	for header := range strings.SplitSeq(token, ";") {
		if strings.Split(utils.TrimSpace(header), "=")[0] == ConfigDefault.CookieName {
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
		Extractor: extractors.FromHeader("X-Csrf-Token"),
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_CSRFLoggerTagRedactsToken(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New())
	app.Use(logger.New(logger.Config{
		Format: "${csrf-token}",
		Stream: &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	// CSRF tokens are randomly generated per request, so assert on the
	// redaction shape (4-byte prefix + Mask) rather than a fixed value.
	got := buf.String()
	require.Len(t, got, redact.PrefixLength+len(redact.Mask))
	require.True(t, strings.HasSuffix(got, redact.Mask), "expected suffix %q in %q", redact.Mask, got)
}

// Test_CSRFLogContextTagRedactsToken runs serially because it mutates
// package-global default logger output and context format.
func Test_CSRFLogContextTagRedactsToken(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, "csrf-token=${csrf-token} ")

	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Regexp(t, `\[Info\] csrf-token=.{`+strconv.Itoa(redact.PrefixLength)+`}`+regexp.QuoteMeta(redact.Mask)+` start`, buf.String())
}

func Test_CSRF_From_Form(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{Extractor: extractors.FromForm("_csrf")}))

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

	app.Use(New(Config{Extractor: extractors.FromQuery("_csrf")}))

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

	csrfGroup := app.Group("/:csrf", New(Config{Extractor: extractors.FromParam("csrf")}))

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

func Test_CSRF_From_Custom(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	extractor := extractors.Extractor{
		Extract: func(c fiber.Ctx) (string, error) {
			body := string(c.Body())
			// Generate the correct extractor to get the token from the correct location
			selectors := strings.Split(body, "=")

			if len(selectors) != 2 || selectors[1] == "" {
				return "", extractors.ErrNotFound
			}
			return selectors[1], nil
		},
		Source: extractors.SourceCustom,
		Key:    "_csrf",
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

	extractor := extractors.Extractor{
		Extract: func(_ fiber.Ctx) (string, error) {
			return "", nil
		},
		Source: extractors.SourceCustom,
		Key:    "_csrf",
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

func Test_CSRF_SecFetchSite(t *testing.T) {
	t.Parallel()

	errorHandler := func(c fiber.Ctx, err error) error {
		return c.Status(fiber.StatusForbidden).SendString(err.Error())
	}

	app := newTrustedApp()

	app.Use(New(Config{ErrorHandler: errorHandler}))

	app.All("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := newTrustedRequestCtx()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetHost("example.com")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	tests := []struct {
		name                   string
		method                 string
		secFetchSite           string
		origin                 string
		expectedStatus         int16
		https                  bool
		expectFetchSiteInvalid bool
	}{
		{
			name:           "same-origin allowed",
			method:         fiber.MethodPost,
			secFetchSite:   "same-origin",
			origin:         "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "none allowed",
			method:         fiber.MethodPost,
			secFetchSite:   "none",
			origin:         "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "cross-site with origin allowed",
			method:         fiber.MethodPost,
			secFetchSite:   "cross-site",
			origin:         "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "same-site with origin allowed",
			method:         fiber.MethodPost,
			secFetchSite:   "same-site",
			origin:         "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "cross-site with mismatched origin blocked",
			method:         fiber.MethodPost,
			secFetchSite:   "cross-site",
			origin:         "https://attacker.example",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "same-site with null origin blocked",
			method:         fiber.MethodPost,
			secFetchSite:   "same-site",
			origin:         "null",
			expectedStatus: http.StatusForbidden,
			https:          true,
		},
		{
			name:                   "invalid header blocked",
			method:                 fiber.MethodPost,
			secFetchSite:           "weird",
			origin:                 "http://example.com",
			expectedStatus:         http.StatusForbidden,
			expectFetchSiteInvalid: true,
		},
		{
			name:           "no header with no origin",
			method:         fiber.MethodPost,
			origin:         "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "no header with matching origin",
			method:         fiber.MethodPost,
			origin:         "http://example.com",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "no header with mismatched origin",
			method:         fiber.MethodPost,
			origin:         "https://attacker.example",
			expectedStatus: http.StatusForbidden,
		},
		{
			// Origin comparison is case-insensitive end-to-end: the raw
			// mixed-case header must be accepted on the same-origin fast path.
			name:           "no header with mixed-case matching origin",
			method:         fiber.MethodPost,
			origin:         "HTTP://EXAMPLE.COM",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "no header with null origin",
			method:         fiber.MethodPost,
			origin:         "null",
			expectedStatus: http.StatusForbidden,
			https:          true,
		},
		{
			// "null" detection is case-insensitive too. Over plain HTTP an
			// absent/null origin clears the error (200); a case-sensitive
			// regression would instead parse "NULL" as a URL and reject with
			// ErrOriginNoMatch (403), so the outcomes are distinguishable.
			name:           "no header with uppercase null origin over http",
			method:         fiber.MethodPost,
			origin:         "NULL",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET allowed",
			method:         fiber.MethodGet,
			secFetchSite:   "cross-site",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "HEAD allowed",
			method:         fiber.MethodHead,
			secFetchSite:   "cross-site",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS allowed",
			method:         fiber.MethodOptions,
			secFetchSite:   "cross-site",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PUT with mismatched origin blocked",
			method:         fiber.MethodPut,
			secFetchSite:   "cross-site",
			origin:         "https://attacker.example",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &fasthttp.RequestCtx{}
			scheme := "http"
			if tt.https {
				scheme = "https"
			}
			c.Request.Header.SetMethod(tt.method)
			c.Request.URI().SetScheme(scheme)
			c.Request.URI().SetHost("example.com")
			c.Request.Header.SetHost("example.com")
			c.Request.Header.SetProtocol(scheme)
			if scheme == "https" {
				c.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
			}
			if tt.origin != "" {
				c.Request.Header.Set(fiber.HeaderOrigin, tt.origin)
			}
			if tt.secFetchSite != "" {
				c.Request.Header.Set(fiber.HeaderSecFetchSite, tt.secFetchSite)
			}

			safe := tt.method == fiber.MethodGet || tt.method == fiber.MethodHead || tt.method == fiber.MethodOptions || tt.method == fiber.MethodTrace

			if !safe {
				c.Request.Header.Set(HeaderName, token)
				c.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			}

			h(c)
			require.Equal(t, int(tt.expectedStatus), c.Response.StatusCode())
			if tt.expectFetchSiteInvalid {
				require.Equal(t, ErrFetchSiteInvalid.Error(), string(c.Response.Body()))
			}
		})
	}
}

func Test_CSRF_Origin(t *testing.T) {
	t.Parallel()
	app := newTrustedApp()

	app.Use(New(Config{CookieSecure: true}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := newTrustedRequestCtx()
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

	// Test Origin without default HTTP port against host with default port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com:80")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com:80")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Origin with default HTTP port against host without port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com:80")
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

	// Test Origin without default HTTPS port against host with default port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com:443")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.SetHost("example.com:443")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Origin with default HTTPS port against host without port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "https://example.com:443")
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
	app := newTrustedApp()

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
	ctx := newTrustedRequestCtx()
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

	// Test Trusted Origin with mixed-case header: origin comparisons are
	// case-insensitive, and the fallback lowering must keep matching the
	// pre-lowered TrustedOrigins config (exact entry).
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://SAFE.Example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Origin with mixed-case header (wildcard subdomain entry).
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("domain-1.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://SAFE.Domain-1.com")
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

	// Test Trusted Origin malformed subdomain
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("domain-1.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://evil.comdomain-1.com")
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

	// Test same-origin Referer with mixed-case header: accepted on the
	// fold-based Match fast path, no trusted-origin fallback involved.
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "HTTPS://EXAMPLE.COM/Some/Path")
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

	// Test Trusted Referer with mixed-case header: the fallback lowering must
	// keep matching the pre-lowered TrustedOrigins config (wildcard entry).
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("domain-1.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://SAFE.Domain-1.com/Account/Login?Id=3")
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

	// Test Trusted Referer with path — referer URL includes a path component
	// which must not prevent matching against the trusted origin
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://safe.example.com/some/path?q=1")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Trusted Referer Wildcard with path — wildcard subdomain referer
	// that includes a path must still match the trusted sub-origin
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("domain-1.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("domain-1.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://safe.domain-1.com/api/callback?code=abc")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
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
	app := newTrustedApp()

	app.Use(New(Config{CookieSecure: true}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := newTrustedRequestCtx()
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

	// Test Referer without default HTTPS port against host with default port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com:443")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com:443")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Referer with default HTTPS port against host without port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "https://example.com:443")
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

	// Test Referer without default HTTP port against host with default port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "http")
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com:80")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com:80")
	ctx.Request.Header.Set(fiber.HeaderReferer, "http://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test Referer with default HTTP port against host without port
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "http")
	ctx.Request.URI().SetScheme("http")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("http")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderReferer, "http://example.com:80")
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
	ctx.Request.Header.Reset()
	ctx.Request.ResetBody()
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
	ctx.Request.Header.Reset()
	ctx.Request.ResetBody()
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

	ctx.Request.Header.Reset()
	ctx.Request.ResetBody()
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
		Extractor: extractors.FromCookie("_session"),
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
		require.Equal(t, ErrTokenNotFound, err)
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
	app := newTrustedApp()

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
	ctx := newTrustedRequestCtx()
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

// Test_CSRF_UnsafeHeaderValue ensures that unsafe header values, such as those described in https://github.com/gofiber/fiber/issues/2045, are rejected and the bug remains fixed.
// go test -race -run Test_CSRF_UnsafeHeaderValue
func Test_CSRF_UnsafeHeaderValue(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var token string
	for _, c := range resp.Cookies() {
		if c.Name != ConfigDefault.CookieName {
			continue
		}
		token = c.Value
		break
	}

	t.Log("token", token)

	getReq := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	getReq.Header.Set(HeaderName, token)
	resp, err = app.Test(getReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	getReq = httptest.NewRequest(fiber.MethodGet, "/test", http.NoBody)
	getReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	getReq.Header.Set(fiber.HeaderCacheControl, "no")
	getReq.Header.Set(HeaderName, token)
	getReq.AddCookie(&http.Cookie{
		Name:  ConfigDefault.CookieName,
		Value: token,
	})

	resp, err = app.Test(getReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	getReq.Header.Set(fiber.HeaderAccept, "*/*")
	getReq.Header.Del(HeaderName)
	resp, err = app.Test(getReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	postReq := httptest.NewRequest(fiber.MethodPost, "/", http.NoBody)
	postReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	postReq.Header.Set(HeaderName, token)
	postReq.AddCookie(&http.Cookie{
		Name:  ConfigDefault.CookieName,
		Value: token,
	})
	resp, err = app.Test(postReq)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

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

	for b.Loop() {
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

	for b.Loop() {
		h(ctx)
	}

	// Ensure the GET request returns a 418 status code
	require.Equal(b, fiber.StatusTeapot, ctx.Response.Header.StatusCode())
}

func Test_CSRF_InvalidURLHeaders(t *testing.T) {
	t.Parallel()
	app := newTrustedApp()

	errHandler := func(ctx fiber.Ctx, err error) error {
		return ctx.Status(419).Send([]byte(err.Error()))
	}

	app.Use(New(Config{ErrorHandler: errHandler}))

	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := newTrustedRequestCtx()

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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_CSRF_FromContextMethods(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{PassLocalsToContext: true})

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		token := TokenFromContext(c)
		require.NotEmpty(t, token)

		handler := HandlerFromContext(c)
		require.NotNil(t, handler)

		customCtx, ok := c.(fiber.CustomCtx)
		require.True(t, ok)
		require.Equal(t, token, TokenFromContext(customCtx))
		require.Equal(t, handler, HandlerFromContext(customCtx))
		require.Equal(t, token, TokenFromContext(c.RequestCtx()))
		require.Equal(t, token, TokenFromContext(c.Context()))
		require.Equal(t, handler, HandlerFromContext(c.RequestCtx()))
		require.Equal(t, handler, HandlerFromContext(c.Context()))

		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
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

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_deleteTokenFromStorage(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	t.Cleanup(func() { app.ReleaseCtx(ctx) })

	token := "token123"
	dummy := []byte("dummy")

	store := session.NewStore()
	sm := newSessionManager(store)
	stm := newStorageManager(nil, true)

	sm.setRaw(ctx, token, dummy, time.Minute)
	cfg := Config{Session: store}
	require.NoError(t, deleteTokenFromStorage(ctx, token, &cfg, sm, stm))
	raw := sm.getRaw(ctx, token, dummy)
	require.Nil(t, raw)

	sm2 := newSessionManager(nil)
	stm2 := newStorageManager(nil, true)

	require.NoError(t, stm2.setRaw(context.Background(), token, dummy, time.Minute))
	cfg = Config{}
	require.NoError(t, deleteTokenFromStorage(ctx, token, &cfg, sm2, stm2))
	raw, err := stm2.getRaw(context.Background(), token)
	require.NoError(t, err)
	require.Nil(t, raw)
}

func Test_storageManager_logKey(t *testing.T) {
	t.Parallel()

	redacted := newStorageManager(nil, true)
	require.Equal(t, redactedKey, redacted.logKey("secret"))

	plain := newStorageManager(nil, false)
	require.Equal(t, "secret", plain.logKey("secret"))
}

func Test_CSRF_Chain_Extractor(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Chain extractor: try header first, fall back to form
	chainExtractor := extractors.Chain(
		extractors.FromHeader("X-Csrf-Token"),
		extractors.FromForm("_csrf"),
	)

	app.Use(New(Config{Extractor: chainExtractor}))

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

	// Test 1: Token in header (first extractor should succeed)
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set("X-Csrf-Token", token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test 2: Token in form (fallback should succeed)
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	ctx.Request.SetBodyString("_csrf=" + token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test 3: Token in both header and form (header should take precedence)
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	ctx.Request.Header.Set("X-Csrf-Token", token)
	ctx.Request.SetBodyString("_csrf=wrong_token")
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test 4: No token in either location
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())

	// Test 5: Wrong token in both locations
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
	ctx.Request.Header.Set("X-Csrf-Token", "wrong_token")
	ctx.Request.SetBodyString("_csrf=also_wrong")
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_Chain_Extractor_Empty(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Empty chain extractor
	emptyChain := extractors.Chain()

	app.Use(New(Config{Extractor: emptyChain}))

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

	// Test with empty chain - should always fail
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set("X-Csrf-Token", token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_Chain_Extractor_SingleExtractor(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	// Chain with single extractor (should behave like the single extractor)
	singleChain := extractors.Chain(extractors.FromHeader("X-Csrf-Token"))

	app.Use(New(Config{Extractor: singleChain}))

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

	// Test valid token in header
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set("X-Csrf-Token", token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())

	// Test no token
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode())
}

func Test_CSRF_All_Extractors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		setupRequest func(ctx *fasthttp.RequestCtx, token string)
		name         string
		extractor    extractors.Extractor
		expectStatus int
	}{
		{
			name:      "FromHeader",
			extractor: extractors.FromHeader("X-Csrf-Token"),
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.Set("X-Csrf-Token", token)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 200,
		},
		{
			name:      "FromHeader_Missing",
			extractor: extractors.FromHeader("X-Csrf-Token"),
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 403,
		},
		{
			name:      "FromForm",
			extractor: extractors.FromForm("_csrf"),
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
				ctx.Request.SetBodyString("_csrf=" + token)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 200,
		},
		{
			name:      "FromForm_Missing",
			extractor: extractors.FromForm("_csrf"),
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationForm)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 403,
		},
		{
			name:      "FromQuery",
			extractor: extractors.FromQuery("csrf_token"),
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/?csrf_token=" + token)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 200,
		},
		{
			name:      "FromQuery_Missing",
			extractor: extractors.FromQuery("csrf_token"),
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/")
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 403,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()

			app.Use(New(Config{Extractor: tc.extractor}))
			app.Post("/", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			h := app.Handler()
			ctx := &fasthttp.RequestCtx{}

			// Generate CSRF token
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.SetRequestURI("/")
			h(ctx)
			token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
			token = strings.Split(strings.Split(token, ";")[0], "=")[1]

			// Test the extractor
			ctx.Request.Reset()
			ctx.Response.Reset()
			tc.setupRequest(ctx, token)
			h(ctx)
			require.Equal(t, tc.expectStatus, ctx.Response.StatusCode(),
				"Test case %s failed: expected %d, got %d", tc.name, tc.expectStatus, ctx.Response.StatusCode())
		})
	}
}

func Test_CSRF_Param_Extractor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		setupRequest func(ctx *fasthttp.RequestCtx, token string)
		name         string
		expectStatus int
	}{
		{
			name: "FromParam_Valid",
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/" + token)
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 200,
		},
		{
			name: "FromParam_Invalid",
			setupRequest: func(ctx *fasthttp.RequestCtx, token string) {
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.SetRequestURI("/wrong_token")
				ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
			},
			expectStatus: 403,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()

			// Only use param-based routing for param extractor tests
			csrfGroup := app.Group("/:csrf", New(Config{Extractor: extractors.FromParam("csrf")}))
			csrfGroup.Post("/", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			h := app.Handler()
			ctx := &fasthttp.RequestCtx{}

			// Generate CSRF token
			ctx.Request.Header.SetMethod(fiber.MethodGet)
			ctx.Request.SetRequestURI("/" + utils.UUIDv4())
			h(ctx)
			token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
			token = strings.Split(strings.Split(token, ";")[0], "=")[1]

			// Test the extractor
			ctx.Request.Reset()
			ctx.Response.Reset()
			tc.setupRequest(ctx, token)
			h(ctx)
			require.Equal(t, tc.expectStatus, ctx.Response.StatusCode(),
				"Test case %s failed: expected %d, got %d", tc.name, tc.expectStatus, ctx.Response.StatusCode())
		})
	}
}

func Test_CSRF_Param_Extractor_Missing(t *testing.T) {
	t.Parallel()

	// Test the case where no param is provided (should get 403 from CSRF middleware on the catch-all route)
	app := fiber.New()

	// Add a catch-all route with CSRF middleware for missing param case
	app.Use(New(Config{Extractor: extractors.FromParam("csrf")}))
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate CSRF token
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// Test missing param (accessing "/" instead of "/:csrf")
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, 403, ctx.Response.StatusCode(), "Missing param should return 403")
}

func Test_CSRF_Extractors_ErrorTypes(t *testing.T) {
	t.Parallel()

	// Test all extractor error types
	testCases := []struct {
		expected  error
		setupCtx  func(ctx *fasthttp.RequestCtx) // Add setup function
		name      string
		extractor extractors.Extractor
	}{
		{
			name:      "Missing header",
			extractor: extractors.FromHeader("X-Missing-Header"),
			expected:  extractors.ErrNotFound,
			setupCtx:  func(_ *fasthttp.RequestCtx) {}, // No setup needed for headers
		},
		{
			name:      "Missing query",
			extractor: extractors.FromQuery("missing_param"),
			expected:  extractors.ErrNotFound,
			setupCtx: func(ctx *fasthttp.RequestCtx) {
				ctx.Request.SetRequestURI("/") // Set URI for query parsing
			},
		},
		{
			name:      "Missing param",
			extractor: extractors.FromParam("missing_param"),
			expected:  extractors.ErrNotFound,
			setupCtx:  func(_ *fasthttp.RequestCtx) {}, // Params are handled by router
		},
		{
			name:      "Missing form",
			extractor: extractors.FromForm("missing_field"),
			expected:  extractors.ErrNotFound,
			setupCtx: func(ctx *fasthttp.RequestCtx) {
				// Properly initialize request for form parsing
				ctx.Request.Header.SetMethod(fiber.MethodPost)
				ctx.Request.Header.SetContentType(fiber.MIMEApplicationForm)
				ctx.Request.SetBodyString("") // Empty form body
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New()
			requestCtx := &fasthttp.RequestCtx{}
			tc.setupCtx(requestCtx) // Set up the context properly

			ctx := app.AcquireCtx(requestCtx)
			defer app.ReleaseCtx(ctx)

			token, err := tc.extractor.Extract(ctx)
			require.Empty(t, token)
			require.Equal(t, tc.expected, err)
		})
	}
}

// flakySessionStorage is a fiber.Storage whose Get/Set/Delete operations can be
// configured to fail, so the session-backed manager error paths can be
// exercised deterministically.
type flakySessionStorage struct {
	data    map[string][]byte
	mu      sync.Mutex
	failGet bool
	failSet bool
	failDel bool
}

func newFlakySessionStorage() *flakySessionStorage {
	return &flakySessionStorage{data: make(map[string][]byte)}
}

func (s *flakySessionStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failGet {
		return nil, errors.New("get failed")
	}
	if val, ok := s.data[key]; ok {
		return append([]byte(nil), val...), nil
	}
	return nil, nil
}

func (s *flakySessionStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *flakySessionStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failSet {
		return errors.New("set failed")
	}
	s.data[key] = append([]byte(nil), val...)
	return nil
}

func (s *flakySessionStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (s *flakySessionStorage) DeleteWithContext(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failDel {
		return errors.New("delete failed")
	}
	delete(s.data, key)
	return nil
}

func (s *flakySessionStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (s *flakySessionStorage) ResetWithContext(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string][]byte)
	return nil
}

func (s *flakySessionStorage) Reset() error { return s.ResetWithContext(context.Background()) }

func (*flakySessionStorage) Close() error { return nil }

// Test_CSRF_validateExtractorSecurity_NilConfig ensures the nil guard returns
// without panicking.

// Test_CSRF_DisableValueRedaction_TrustedOrigin verifies that the raw origin
// value is surfaced in the panic message when redaction is disabled.
func Test_CSRF_DisableValueRedaction_TrustedOrigin(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "[CSRF] Invalid origin format in configuration:http://", func() {
		New(Config{
			TrustedOrigins:        []string{"http://"},
			DisableValueRedaction: true,
		})
	})
}

// Test_CSRF_DisableValueRedaction_TrustedOrigin_Wildcard exercises the same path
// for the wildcard subdomain branch.
func Test_CSRF_DisableValueRedaction_TrustedOrigin_Wildcard(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "[CSRF] Invalid origin format in configuration:http://*.", func() {
		New(Config{
			TrustedOrigins:        []string{"http://*."},
			DisableValueRedaction: true,
		})
	})
}

// Test_CSRF_Extractor_NonNotFoundError ensures that an extractor error which is
// not ErrNotFound is forwarded to the error handler verbatim.
func Test_CSRF_Extractor_NonNotFoundError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("extractor boom")

	var captured error
	app := fiber.New()
	app.Use(New(Config{
		Extractor: extractors.Extractor{
			Extract: func(fiber.Ctx) (string, error) {
				return "", sentinel
			},
			Source: extractors.SourceCustom,
			Key:    "_csrf",
		},
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return fiber.ErrTeapot
		},
	}))
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.ErrorIs(t, captured, sentinel)
}

// Test_CSRF_StorageGetError_OnValidation covers the storage fetch error path
// that runs after the double-submit cookie comparison succeeds.
func Test_CSRF_StorageGetError_OnValidation(t *testing.T) {
	t.Parallel()

	storage := newFailingCSRFStorage()
	storage.errs["get|token"] = errors.New("boom")

	var captured error
	app := fiber.New()
	app.Use(New(Config{
		Storage: storage,
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return fiber.ErrTeapot
		},
	}))
	app.Post("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodPost, "/", http.NoBody)
	req.Header.Set(HeaderName, "token")
	req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: "token"})

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusTeapot, resp.StatusCode)
	require.ErrorContains(t, captured, "csrf: failed to fetch token from storage")
}

// Test_CSRF_DeleteToken_NoCookie covers the early return in DeleteToken when no
// CSRF cookie is present on the request.
func Test_CSRF_DeleteToken_NoCookie(t *testing.T) {
	t.Parallel()

	var captured error
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return err
		},
	}))

	var deleteErr error
	app.Get("/", func(c fiber.Ctx) error {
		handler := HandlerFromContext(c)
		require.NotNil(t, handler)
		deleteErr = handler.DeleteToken(c)
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.ErrorIs(t, deleteErr, ErrTokenNotFound)
	require.ErrorIs(t, captured, ErrTokenNotFound)
}

// Test_CSRF_DeleteToken_StorageError covers the storage delete failure path in
// DeleteToken, where the cookie is present but removing the token from storage
// returns an error.
func Test_CSRF_DeleteToken_StorageError(t *testing.T) {
	t.Parallel()

	storage := newFailingCSRFStorage()
	storage.data["token"] = []byte("value")
	storage.errs["del|token"] = errors.New("boom")

	var captured error
	app := fiber.New()
	app.Use(New(Config{
		Storage: storage,
		ErrorHandler: func(_ fiber.Ctx, err error) error {
			captured = err
			return err
		},
	}))

	var deleteErr error
	app.Get("/", func(c fiber.Ctx) error {
		handler := HandlerFromContext(c)
		require.NotNil(t, handler)
		deleteErr = handler.DeleteToken(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: ConfigDefault.CookieName, Value: "token"})

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.ErrorContains(t, deleteErr, "csrf: failed to delete token from storage")
	require.ErrorContains(t, captured, "csrf: failed to delete token from storage")
}

// Test_CSRF_DeleteToken_WithSessionMiddleware exercises DeleteToken while the
// session is loaded into the context by the session middleware, covering the
// in-context branch of the session manager's delRaw.
func Test_CSRF_DeleteToken_WithSessionMiddleware(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	smh, sstore := session.NewWithStore()
	app.Use(smh)
	app.Use(New(Config{Session: sstore}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Post("/delete", func(c fiber.Ctx) error {
		handler := HandlerFromContext(c)
		require.NotNil(t, handler)
		if err := handler.DeleteToken(c); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}

	// Generate the CSRF token and the session id.
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)

	csrfCookie := fasthttp.AcquireCookie()
	csrfCookie.SetKey(ConfigDefault.CookieName)
	require.True(t, ctx.Response.Header.Cookie(csrfCookie))
	csrfToken := string(csrfCookie.Value())
	fasthttp.ReleaseCookie(csrfCookie)

	sessionCookie := fasthttp.AcquireCookie()
	sessionCookie.SetKey("session_id")
	require.True(t, ctx.Response.Header.Cookie(sessionCookie))
	sessionID := string(sessionCookie.Value())
	fasthttp.ReleaseCookie(sessionCookie)

	// Delete the token with the session loaded in the context.
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/delete")
	ctx.Request.Header.Set(HeaderName, csrfToken)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, csrfToken)
	ctx.Request.Header.SetCookie("session_id", sessionID)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
}

// Test_StorageManager_Memory_UnexpectedType covers the defensive type assertion
// in the memory-backed getRaw.
func Test_StorageManager_Memory_UnexpectedType(t *testing.T) {
	t.Parallel()

	m := newStorageManager(nil, false)
	m.memory.Set("key", "not-bytes", 0)

	raw, err := m.getRaw(context.Background(), "key")
	require.Nil(t, raw)
	require.ErrorContains(t, err, "unexpected value type")
}

// Test_StorageManager_Storage_Success covers the success return paths of the
// storage-backed setRaw and delRaw.
func Test_StorageManager_Storage_Success(t *testing.T) {
	t.Parallel()

	storage := newFailingCSRFStorage()
	m := newStorageManager(storage, false)

	require.NoError(t, m.setRaw(context.Background(), "key", []byte("value"), time.Minute))

	raw, err := m.getRaw(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, []byte("value"), raw)

	require.NoError(t, m.delRaw(context.Background(), "key"))

	raw, err = m.getRaw(context.Background(), "key")
	require.NoError(t, err)
	require.Nil(t, raw)
}

// newSessionCtx builds a fiber.Ctx carrying the given session_id cookie. The
// session middleware is intentionally not run, so the session manager falls back
// to loading the session from the store (the else branch).
func newSessionCtx(app *fiber.App, sessionID string) fiber.Ctx {
	reqCtx := &fasthttp.RequestCtx{}
	if sessionID != "" {
		reqCtx.Request.Header.SetCookie("session_id", sessionID)
	}
	return app.AcquireCtx(reqCtx)
}

// Test_SessionManager_GetRaw_StoreError covers the error branch when loading the
// session from the store fails.
func Test_SessionManager_GetRaw_StoreError(t *testing.T) {
	t.Parallel()

	storage := newFlakySessionStorage()
	storage.failGet = true
	store := session.NewStore(session.Config{
		Storage:   storage,
		Extractor: extractors.FromCookie("session_id"),
	})
	m := newSessionManager(store)

	app := fiber.New()
	c := newSessionCtx(app, "abc")
	defer app.ReleaseCtx(c)

	require.Nil(t, m.getRaw(c, "key", dummyValue))
}

// Test_SessionManager_SetRaw_StoreError covers the error branch when the session
// store cannot be loaded during setRaw.
func Test_SessionManager_SetRaw_StoreError(t *testing.T) {
	t.Parallel()

	storage := newFlakySessionStorage()
	storage.failGet = true
	store := session.NewStore(session.Config{
		Storage:   storage,
		Extractor: extractors.FromCookie("session_id"),
	})
	m := newSessionManager(store)

	app := fiber.New()
	c := newSessionCtx(app, "abc")
	defer app.ReleaseCtx(c)

	require.NotPanics(t, func() {
		m.setRaw(c, "key", dummyValue, time.Minute)
	})
}

// Test_SessionManager_SetRaw_SaveError covers the save-failure warning branch of
// setRaw, where the store loads successfully but persisting it fails.
func Test_SessionManager_SetRaw_SaveError(t *testing.T) {
	var buf bytes.Buffer
	fiberlog.SetOutput(&buf)
	t.Cleanup(func() { fiberlog.SetOutput(os.Stderr) })

	storage := newFlakySessionStorage()
	storage.failSet = true
	store := session.NewStore(session.Config{
		Storage:   storage,
		Extractor: extractors.FromCookie("session_id"),
	})
	m := newSessionManager(store)

	app := fiber.New()
	c := newSessionCtx(app, "")
	defer app.ReleaseCtx(c)

	m.setRaw(c, "key", dummyValue, time.Minute)
	require.Contains(t, buf.String(), "failed to save session")
}

// Test_SessionManager_DelRaw_StoreError covers the error branch when the session
// store cannot be loaded during delRaw.
func Test_SessionManager_DelRaw_StoreError(t *testing.T) {
	t.Parallel()

	storage := newFlakySessionStorage()
	storage.failGet = true
	store := session.NewStore(session.Config{
		Storage:   storage,
		Extractor: extractors.FromCookie("session_id"),
	})
	m := newSessionManager(store)

	app := fiber.New()
	c := newSessionCtx(app, "abc")
	defer app.ReleaseCtx(c)

	require.NotPanics(t, func() {
		m.delRaw(c)
	})
}

// Test_SessionManager_DelRaw_SaveError covers the save-failure warning branch of
// delRaw.
func Test_SessionManager_DelRaw_SaveError(t *testing.T) {
	var buf bytes.Buffer
	fiberlog.SetOutput(&buf)
	t.Cleanup(func() { fiberlog.SetOutput(os.Stderr) })

	storage := newFlakySessionStorage()
	storage.failSet = true
	store := session.NewStore(session.Config{
		Storage:   storage,
		Extractor: extractors.FromCookie("session_id"),
	})
	m := newSessionManager(store)

	app := fiber.New()
	c := newSessionCtx(app, "")
	defer app.ReleaseCtx(c)

	m.delRaw(c)
	require.Contains(t, buf.String(), "failed to save session")
}

// Test_CSRF_Security_CompareConstantTime verifies the logical behavior of the
// constant-time comparison helpers used to validate tokens. The functions must
// only report equality for byte/string-identical inputs.

// Test_CSRF_Security_SecFetchSite_Normalization ensures the Sec-Fetch-Site
// validation is case-insensitive and tolerant of surrounding whitespace, while
// still rejecting genuinely unknown values.
func Test_CSRF_Security_SecFetchSite_Normalization(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	cases := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "empty header is ignored", value: "", wantErr: nil},
		{name: "same-origin", value: "same-origin", wantErr: nil},
		{name: "mixed case same-origin", value: "Same-Origin", wantErr: nil},
		{name: "upper case cross-site", value: "CROSS-SITE", wantErr: nil},
		{name: "leading/trailing spaces", value: "  same-site  ", wantErr: nil},
		{name: "none", value: "none", wantErr: nil},
		{name: "unknown value rejected", value: "totally-bogus", wantErr: ErrFetchSiteInvalid},
		{name: "embedded space rejected", value: "same origin", wantErr: ErrFetchSiteInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := app.AcquireCtx(&fasthttp.RequestCtx{})
			defer app.ReleaseCtx(c)
			if tc.value != "" {
				c.Request().Header.Set(fiber.HeaderSecFetchSite, tc.value)
			}
			require.Equal(t, tc.wantErr, validateSecFetchSite(c))
		})
	}
}

// Test_CSRF_Security_DoubleSubmitMismatch ensures that a request presenting a
// valid token in the configured extractor location but a different (also valid)
// token in the cookie is rejected with ErrTokenInvalid. This is the core
// double-submit-cookie protection.
func Test_CSRF_Security_DoubleSubmitMismatch(t *testing.T) {
	t.Parallel()

	var captured error
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.SendStatus(fiber.StatusForbidden)
		},
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	// Generate two distinct, individually valid tokens.
	genToken := func() string {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetMethod(fiber.MethodGet)
		h(ctx)
		setCookie := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
		return strings.Split(strings.Split(setCookie, ";")[0], "=")[1]
	}
	tokenA := genToken()
	tokenB := genToken()
	require.NotEqual(t, tokenA, tokenB)

	// Submit tokenA in the header but tokenB in the cookie.
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, tokenA)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, tokenB)
	h(ctx)

	require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
	require.ErrorIs(t, captured, ErrTokenInvalid)
}

// Test_CSRF_Security_ForgedTokenNotInStorage ensures that a token which is
// consistent across the header and cookie (passing the double-submit check) but
// absent from storage is rejected and the stale cookie is expired.
func Test_CSRF_Security_ForgedTokenNotInStorage(t *testing.T) {
	t.Parallel()

	var captured error
	app := fiber.New()
	app.Use(New(Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			captured = err
			return c.SendStatus(fiber.StatusForbidden)
		},
	}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	const forged = "this-token-was-never-issued"
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.Set(HeaderName, forged)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, forged)
	h(ctx)

	require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
	require.ErrorIs(t, captured, ErrTokenNotFound)

	// The stale cookie must be expired by the middleware.
	expired := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(expired)
	expired.SetKey(ConfigDefault.CookieName)
	require.True(t, ctx.Response.Header.Cookie(expired), "expected the cookie to be reset")
	require.Empty(t, string(expired.Value()), "expected the cookie value to be cleared")
}

// Test_CSRF_Security_CookieAttributes verifies that the security-relevant cookie
// attributes from the configuration are reflected on the Set-Cookie response
// header. Missing HttpOnly/Secure/SameSite flags would weaken the protection.
func Test_CSRF_Security_CookieAttributes(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Use(New(Config{
		CookieName:     "__Host-csrf",
		CookiePath:     "/",
		CookieSecure:   true,
		CookieHTTPOnly: true,
		CookieSameSite: "Strict",
	}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)

	var cookie *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "__Host-csrf" {
			cookie = ck
			break
		}
	}
	require.NotNil(t, cookie, "CSRF cookie should be set")
	require.True(t, cookie.HttpOnly, "cookie must be HttpOnly")
	require.True(t, cookie.Secure, "cookie must be Secure")
	require.Equal(t, http.SameSiteStrictMode, cookie.SameSite, "cookie must be SameSite=Strict")
	require.Equal(t, "/", cookie.Path)
	// A "__Host-" prefixed cookie must stay host-only: setCSRFCookie copies
	// cfg.CookieDomain onto the response cookie, so an empty Domain guards
	// against a regression that would start scoping the cookie to a domain.
	require.Empty(t, cookie.Domain, "__Host- cookie must not set a Domain")
	require.NotEmpty(t, cookie.Value)
}

// Test_CSRF_Security_SchemeDowngradeRejected ensures that an HTTPS request whose
// Origin downgrades to HTTP for the same host is rejected: the scheme is part of
// the origin and must match.
func Test_CSRF_Security_SchemeDowngradeRejected(t *testing.T) {
	t.Parallel()

	app := newTrustedApp()
	app.Use(New(Config{CookieSecure: true}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	// Acquire a valid token over HTTPS.
	ctx := newTrustedRequestCtx()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	// HTTPS host, but the Origin claims plain HTTP for the same host.
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.URI().SetScheme("https")
	ctx.Request.URI().SetHost("example.com")
	ctx.Request.Header.SetProtocol("https")
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	ctx.Request.Header.SetHost("example.com")
	ctx.Request.Header.Set(fiber.HeaderOrigin, "http://example.com")
	ctx.Request.Header.Set(HeaderName, token)
	ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
	h(ctx)
	require.Equal(t, fiber.StatusForbidden, ctx.Response.StatusCode())
}

// Test_CSRF_Security_TrustedOriginSchemeIsolation ensures that trusting an
// origin under one scheme does not implicitly trust it under another scheme.
func Test_CSRF_Security_TrustedOriginSchemeIsolation(t *testing.T) {
	t.Parallel()

	app := newTrustedApp()
	app.Use(New(Config{
		CookieSecure:   true,
		TrustedOrigins: []string{"https://trusted.example.com"},
	}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	ctx := newTrustedRequestCtx()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
	h(ctx)
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]

	post := func(origin string) int {
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(fiber.MethodPost)
		ctx.Request.URI().SetScheme("https")
		ctx.Request.URI().SetHost("example.com")
		ctx.Request.Header.SetProtocol("https")
		ctx.Request.Header.Set(fiber.HeaderXForwardedProto, "https")
		ctx.Request.Header.SetHost("example.com")
		ctx.Request.Header.Set(fiber.HeaderOrigin, origin)
		ctx.Request.Header.Set(HeaderName, token)
		ctx.Request.Header.SetCookie(ConfigDefault.CookieName, token)
		h(ctx)
		return ctx.Response.StatusCode()
	}

	// The exact trusted origin (https) is accepted.
	require.Equal(t, fiber.StatusOK, post("https://trusted.example.com"))
	// The same host over http is NOT trusted.
	require.Equal(t, fiber.StatusForbidden, post("http://trusted.example.com"))
}
