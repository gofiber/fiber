package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Session_Middleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/get", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		value, ok := sess.Get("key").(string)
		if !ok {
			return c.Status(fiber.StatusNotFound).SendString("key not found")
		}
		return c.SendString("value=" + value)
	})

	app.Post("/set", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// get a value from the body
		value := c.FormValue("value")
		sess.Set("key", value)
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/delete", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		sess.Delete("key")
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/reset", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		// Set a value to ensure it is cleared after reset
		sess.Set("key", "value")

		if err := sess.Reset(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Ensure value is cleared
		value, ok := sess.Get("key").(string)
		if ok || value != "" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/regenerate", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Set a value to ensure it is preserved after regeneration
		sess.Set("key", "value")

		// Regenerate the session ID
		if err := sess.Regenerate(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Ensure the session ID has changed but session data is preserved
		newID := sess.ID()
		if newID == "" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Check if the session data is still accessible
		value, ok := sess.Get("key").(string)
		if !ok || value != "value" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/destroy", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if err := sess.Destroy(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/fresh", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Reset the session to make it fresh
		if err := sess.Reset(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if sess.Fresh() {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	app.Post("/keys", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// get a value from the body
		value := c.FormValue("keys")
		for rawKey := range strings.SplitSeq(value, ",") {
			key := utils.TrimSpace(rawKey)
			if key == "" {
				continue
			}
			// Set each key in the session
			sess.Set(key, "value_"+key)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/keys", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		keys := sess.Keys()
		if len(keys) == 0 {
			return c.SendStatus(fiber.StatusNotFound)
		}
		// Keys may be of any type, so convert to string for display
		strKeys := []string{}
		for _, key := range keys {
			strKeys = append(strKeys, fmt.Sprintf("%v", key))
		}
		return c.SendString("keys=" + strings.Join(strKeys, ","))
	})

	// Test GET, SET, DELETE, RESET, REGENERATE, DESTROY by sending requests to the respective routes
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	h := app.Handler()
	h(ctx)
	require.Equal(t, fiber.StatusNotFound, ctx.Response.StatusCode())
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "key not found", string(ctx.Response.Body()))

	// Test POST /set
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/set")
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded") // Set the Content-Type
	ctx.Request.SetBodyString("value=hello")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test GET /get to check if the value was set
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	require.Equal(t, "value=hello", string(ctx.Response.Body()))

	// Test POST /delete to delete the value
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/delete")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test GET /get to check if the value was deleted
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusNotFound, ctx.Response.StatusCode())
	require.Equal(t, "key not found", string(ctx.Response.Body()))

	// Test POST /reset to reset the session
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/reset")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// verify we have a new session token
	newToken := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	newTokenParts := strings.SplitN(strings.SplitN(newToken, ";", 2)[0], "=", 2)
	require.Len(t, newTokenParts, 2, "Expected Set-Cookie header to contain a token")
	newToken = newTokenParts[1]
	require.NotEqual(t, token, newToken)
	token = newToken

	// Test POST /regenerate to regenerate the session ID
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/regenerate")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// verify we have a new session token
	newToken = string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	newTokenParts = strings.SplitN(strings.SplitN(newToken, ";", 2)[0], "=", 2)
	require.Len(t, newTokenParts, 2, "Expected Set-Cookie header to contain a token")
	newToken = newTokenParts[1]
	require.NotEqual(t, token, newToken)
	token = newToken

	// Test POST /destroy to destroy the session
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/destroy")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Verify the session cookie has expired
	setCookieHeader := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.Contains(t, setCookieHeader, "max-age=0")

	// Sleep so that the session expires
	time.Sleep(1 * time.Second)

	// Test GET /get to check if the session was destroyed
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/get")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusNotFound, ctx.Response.StatusCode())
	// check that we have a new session token
	newToken = string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	parts := strings.Split(newToken, ";")
	require.Greater(t, len(parts), 1)
	valueParts := strings.Split(parts[0], "=")
	require.Greater(t, len(valueParts), 1)
	newToken = valueParts[1]
	require.NotEqual(t, token, newToken)
	token = newToken

	// Test POST /fresh to check if the session is fresh
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/fresh")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// check that we have a new session token
	newToken = string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, newToken, "Expected Set-Cookie header to be present")
	newTokenParts = strings.SplitN(strings.SplitN(newToken, ";", 2)[0], "=", 2)
	require.Len(t, newTokenParts, 2, "Expected Set-Cookie header to contain a token")
	newToken = newTokenParts[1]
	require.NotEqual(t, token, newToken)

	token = newToken

	// Test POST /keys to set multiple keys
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.SetRequestURI("/keys")
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded") // Set the Content-Type
	ctx.Request.SetBodyString("keys=key1,key2")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test GET /keys to check if the session has the keys
	ctx.Request.Reset()
	ctx.Response.Reset()
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/keys")
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	require.True(t, strings.HasPrefix(body, "keys="))
	parts = strings.Split(strings.TrimPrefix(body, "keys="), ",")
	require.Len(t, parts, 2, "Expected two keys in the session")
	sort.Strings(parts)
	require.Equal(t, []string{"key1", "key2"}, parts)
}

func Test_SessionLoggerTagRedactsSessionID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New())
	app.Use(logger.New(logger.Config{
		Format: "${session-id}",
		Stream: &buf,
	}))
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, buf.String(), "****")
}

func Test_SessionLoggerTagWithOuterLoggerDoesNotPanic(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	sessionHandler := New()
	requestLogger := logger.New(logger.Config{
		Format: "${session-id}",
		Stream: &buf,
	})

	app := fiber.New()
	app.Use(requestLogger)
	app.Use(sessionHandler)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Empty(t, buf.String())
}

// Test_SessionLogContextTagRedactsSessionID runs serially because it mutates
// package-global default logger output and context format.
func Test_SessionLogContextTagRedactsSessionID(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, "session-id=${session-id} ")

	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Contains(t, buf.String(), "[Info] session-id=")
	require.Contains(t, buf.String(), "**** start")
}

func Test_Session_NewWithStore(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		return c.SendString("value=" + id)
	})
	app.Post("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		c.Cookie(&fiber.Cookie{
			Name:  "session_id",
			Value: id,
		})
		return nil
	})

	h := app.Handler()

	// Test GET request without cookie
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// Get session cookie
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test GET request with cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	require.Equal(t, "value="+token, string(ctx.Response.Body()))
}

func Test_Session_FromSession(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	sess := FromContext(app.AcquireCtx(&fasthttp.RequestCtx{}))
	require.Nil(t, sess)

	app.Use(New())
}

func Test_Session_FromContext_Types(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		require.NotNil(t, FromContext(c))
		customCtx, ok := c.(fiber.CustomCtx)
		require.True(t, ok)
		require.NotNil(t, FromContext(customCtx))
		require.NotNil(t, FromContext(c.RequestCtx()))
		require.NotNil(t, FromContext(c.Context()))
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func Test_Session_WithConfig(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	const idleTimeout = 1 * time.Second

	app.Use(New(Config{
		Next: func(c fiber.Ctx) bool {
			return c.Get("key") == "value"
		},
		IdleTimeout: idleTimeout,
		Extractor:   extractors.FromCookie("session_id_test"),
		KeyGenerator: func() string {
			return "test"
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		return c.SendString("value=" + id)
	})

	app.Get("/isFresh", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess.Fresh() {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	})

	app.Post("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		c.Cookie(&fiber.Cookie{
			Name:  "session_id_test",
			Value: id,
		})
		return nil
	})

	h := app.Handler()

	// Test GET request without cookie
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// Get session cookie
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test GET request with cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id_test", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test POST request with cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie("session_id_test", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test POST request without cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test POST request with wrong key
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Test POST request with wrong value
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx.Request.Header.SetCookie("session_id_test", "wrong")
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())

	// Check idle timeout not expired
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id_test", token)
	ctx.Request.SetRequestURI("/isFresh")
	h(ctx)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response.StatusCode())

	// Test idle timeout, waiting on the cached clock the storage compares against
	clocktest.SleepPast(t, idleTimeout)
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id_test", token)
	ctx.Request.SetRequestURI("/isFresh")
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
}

func Test_Session_Next(t *testing.T) {
	t.Parallel()

	var (
		doNext bool
		muNext sync.RWMutex
	)

	app := fiber.New()

	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			muNext.RLock()
			defer muNext.RUnlock()
			return doNext
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		id := sess.ID()
		return c.SendString("value=" + id)
	})

	h := app.Handler()

	// Test with Next returning false
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	// Get session cookie
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, token, "Expected Set-Cookie header to be present")
	tokenParts := strings.SplitN(strings.SplitN(token, ";", 2)[0], "=", 2)
	require.Len(t, tokenParts, 2, "Expected Set-Cookie header to contain a token")
	token = tokenParts[1]
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test with Next returning true
	muNext.Lock()
	doNext = true
	muNext.Unlock()

	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response.StatusCode())
}

func Test_Session_Middleware_Store(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	handler, sessionStore := NewWithStore()

	app.Use(handler)

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		st := sess.Store()
		if st != sessionStore {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	// Test GET request
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
}

// failingStorage simulates a session store whose backend is unavailable.
type failingStorage struct {
	err error
}

func (s *failingStorage) GetWithContext(context.Context, string) ([]byte, error) { return nil, s.err }

func (s *failingStorage) Get(string) ([]byte, error) { return nil, s.err }

func (s *failingStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	return s.err
}
func (s *failingStorage) Set(string, []byte, time.Duration) error         { return s.err }
func (s *failingStorage) DeleteWithContext(context.Context, string) error { return s.err }
func (s *failingStorage) Delete(string) error                             { return s.err }
func (s *failingStorage) ResetWithContext(context.Context) error          { return s.err }
func (s *failingStorage) Reset() error                                    { return s.err }
func (*failingStorage) Close() error                                      { return nil }

// contextStorage is a storage that honors context cancellation by returning
// ctx.Err() from its *WithContext methods. It is used to verify that the
// session *WithContext variants propagate a canceled/deadline context.
type contextStorage struct{}

func (*contextStorage) GetWithContext(ctx context.Context, _ string) ([]byte, error) {
	return nil, ctx.Err() //nolint:wrapcheck // test stub returns ctx.Err() verbatim
}

func (*contextStorage) Get(string) ([]byte, error) { return nil, nil }

func (*contextStorage) SetWithContext(ctx context.Context, _ string, _ []byte, _ time.Duration) error {
	return ctx.Err() //nolint:wrapcheck // test stub returns ctx.Err() verbatim
}

func (*contextStorage) Set(_ string, _ []byte, _ time.Duration) error { return nil }
func (*contextStorage) DeleteWithContext(ctx context.Context, _ string) error {
	return ctx.Err() //nolint:wrapcheck // test stub returns ctx.Err() verbatim
}
func (*contextStorage) Delete(_ string) error { return nil }
func (*contextStorage) ResetWithContext(ctx context.Context) error {
	return ctx.Err() //nolint:wrapcheck // test stub returns ctx.Err() verbatim
}
func (*contextStorage) Reset() error { return nil }
func (*contextStorage) Close() error { return nil }

// Regression: https://github.com/gofiber/fiber/issues/4348
// A failing session store must result in an error response, not a panic.
func Test_Session_Middleware_StoreError(t *testing.T) {
	t.Parallel()

	storage := &failingStorage{err: errors.New("storage unavailable")}
	app := fiber.New()
	app.Use(New(Config{Storage: storage}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	// Provide a session ID so the middleware attempts a storage lookup.
	ctx.Request.Header.SetCookie("session_id", "some-session-id")
	require.NotPanics(t, func() { h(ctx) })
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response.StatusCode())
	require.Equal(t, "Internal Server Error", string(ctx.Response.Body()))
	require.NotContains(t, string(ctx.Response.Body()), storage.err.Error())
}

func Test_Session_Middleware_StoreError_CustomErrorHandler(t *testing.T) {
	t.Parallel()

	storageErr := errors.New("redis dial tcp redis.internal.corp:6379: connect: connection refused")
	storage := &failingStorage{err: storageErr}
	app := fiber.New()
	var handledErr error
	app.Use(New(Config{
		Storage: storage,
		ErrorHandler: func(c fiber.Ctx, err error) {
			handledErr = err
			require.NoError(t, c.Status(fiber.StatusInternalServerError).SendString("session unavailable"))
		},
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id", "some-session-id")
	app.Handler()(ctx)

	require.ErrorIs(t, handledErr, storageErr)
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response.StatusCode())
	require.Equal(t, "session unavailable", string(ctx.Response.Body()))
}

func Test_Middleware_DestroyWithContext(t *testing.T) {
	t.Parallel()

	handler, store := NewWithStore()
	app := fiber.New()
	app.Use(handler)

	app.Get("/destroy", func(c fiber.Ctx) error {
		sess := FromContext(c)
		sess.Set("key", "value")
		if err := sess.DestroyWithContext(t.Context()); err != nil {
			return err
		}
		// Assert the actual effect, not just that no error occurred: the
		// destroyed session must no longer hold its data.
		if sess.Get("key") != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/destroy")
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	_ = store
}

func Test_Middleware_ResetWithContext(t *testing.T) {
	t.Parallel()

	handler, store := NewWithStore()
	app := fiber.New()
	app.Use(handler)

	app.Get("/reset", func(c fiber.Ctx) error {
		sess := FromContext(c)
		originalID := sess.ID()
		sess.Set("key", "value")
		if err := sess.ResetWithContext(t.Context()); err != nil {
			return err
		}
		// Assert the actual effect: data is cleared and a new ID is issued.
		if sess.ID() == originalID || sess.Get("key") != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/reset")
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	_ = store
}

func Test_Middleware_RegenerateWithContext(t *testing.T) {
	t.Parallel()

	handler, store := NewWithStore()
	app := fiber.New()
	app.Use(handler)

	app.Get("/regenerate", func(c fiber.Ctx) error {
		sess := FromContext(c)
		originalID := sess.ID()
		err := sess.RegenerateWithContext(t.Context())
		if err != nil {
			return err
		}
		if sess.ID() == originalID {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.SetRequestURI("/regenerate")
	h(ctx)
	require.Equal(t, fiber.StatusOK, ctx.Response.StatusCode())
	_ = store
}

// go test -run Test_Middleware_WithContext_ErrorPropagation
func Test_Middleware_WithContext_ErrorPropagation(t *testing.T) {
	t.Parallel()

	canceledCtx := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}

	t.Run("DestroyWithContext propagates canceled context and marks destroyed", func(t *testing.T) {
		t.Parallel()
		m := &Middleware{Session: newSessionWithStorage(t, &contextStorage{})}
		require.ErrorIs(t, m.DestroyWithContext(canceledCtx()), context.Canceled)
		require.True(t, m.isDestroyed)
	})

	t.Run("ResetWithContext propagates storage error", func(t *testing.T) {
		t.Parallel()
		storageErr := errors.New("storage unavailable")
		m := &Middleware{Session: newSessionWithStorage(t, &failingStorage{err: storageErr})}
		require.ErrorIs(t, m.ResetWithContext(t.Context()), storageErr)
	})

	t.Run("RegenerateWithContext propagates storage error", func(t *testing.T) {
		t.Parallel()
		storageErr := errors.New("storage unavailable")
		m := &Middleware{Session: newSessionWithStorage(t, &failingStorage{err: storageErr})}
		require.ErrorIs(t, m.RegenerateWithContext(t.Context()), storageErr)
	})
}

func Test_Middleware_ResolveContext(t *testing.T) {
	t.Parallel()

	t.Run("resolve context returns fiber ctx when not nil", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		fctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(fctx)

		m := &Middleware{ctx: fctx}
		ctx := m.resolveContext()
		require.Equal(t, fctx, ctx)
	})

	t.Run("resolve context returns background when ctx is nil", func(t *testing.T) {
		t.Parallel()
		m := &Middleware{ctx: nil}
		ctx := m.resolveContext()
		require.NotNil(t, ctx)
	})
}

// The middleware saves after the handler returns, so a handler's own deferred
// cancel must not reach storage through Ctx.Err().
func Test_Session_Save_After_Handler_Cancel(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/set", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), time.Hour)
		defer cancel()
		c.SetContext(ctx)

		FromContext(c).Set("hello", "world")
		return nil
	})
	app.Get("/get", func(c fiber.Ctx) error {
		v, ok := FromContext(c).Get("hello").(string)
		if !ok {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendString(v)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/set", http.NoBody))
	require.NoError(t, err)
	cookie := resp.Header.Get("Set-Cookie")
	require.NotEmpty(t, cookie)

	req := httptest.NewRequest(fiber.MethodGet, "/get", http.NoBody)
	req.Header.Set("Cookie", cookie)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "world", string(body))
}

// Test_Session_FromContext_RetainedContext covers a context kept past the end of
// its request, which PassLocalsToContext exists to allow: a goroutine started by
// the handler holds c.Context() and looks the session up later. The lookup used
// to return the pooled *Middleware that request had released, and once another
// request acquired it that goroutine read and wrote the other request's session.
func Test_Session_FromContext_RetainedContext(t *testing.T) {
	t.Parallel()

	for _, pass := range []bool{true, false} {
		name := "default"
		if pass {
			name = "PassLocalsToContext"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			app := fiber.New(fiber.Config{PassLocalsToContext: pass})
			app.Use(New())

			// app.Test finishes a request before it returns, so these are written by
			// one handler and read by the next without overlapping.
			var (
				retained context.Context
				stale    *Middleware
				current  *Middleware
				staleID  string
			)
			app.Get("/a", func(c fiber.Ctx) error {
				FromContext(c).Set("user", "alice")
				retained = c.Context()
				return c.SendString("a")
			})
			app.Get("/b", func(c fiber.Ctx) error {
				current = FromContext(c)
				current.Set("user", "bob")

				// What a goroutine that outlived request /a does.
				stale = FromContext(retained)
				if stale != nil {
					staleID = stale.ID()
					stale.Set("user", "mallory")
				}
				return c.SendString("b")
			})

			// Under -race a pool drops one Put in four, so a reused object is only
			// seen on some rounds; repeating makes the old behavior certain to show.
			for range 40 {
				stale, current, staleID = nil, nil, ""

				resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/a", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode)

				// The request is over: its context no longer leads to a session.
				require.Nil(t, FromContext(retained), "the released middleware is still reachable")

				resp, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/b", http.NoBody))
				require.NoError(t, err)
				require.Equal(t, fiber.StatusOK, resp.StatusCode)

				require.Nil(t, stale, "a retained context reached request /b's session")
				require.Empty(t, staleID)
				require.NotNil(t, current)
			}
		})
	}
}

// Test_Session_FromContext_RetainedContext_AfterHandler looks the session up
// from a goroutine that is still running when the handler returns, which is the
// race the pooled pointer made dangerous: it has to see nil once the middleware
// is released, and never a half-reset object.
func Test_Session_FromContext_RetainedContext_AfterHandler(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(New())

	var (
		wg      sync.WaitGroup
		inside  *Middleware
		release = make(chan struct{})
		seen    = make(chan *Middleware, 1)
	)
	app.Get("/", func(c fiber.Ctx) error {
		ctx := c.Context()
		inside = FromContext(ctx)

		wg.Go(func() {
			<-release
			seen <- FromContext(ctx)
		})
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.NotNil(t, inside, "the context must resolve while the request runs")

	close(release)
	wg.Wait()
	require.Nil(t, <-seen)
}

// Test_Session_FromContext_RetainedContext_Concurrent runs many overlapping
// requests, each leaving a goroutine behind that keeps looking its session up
// through the context it kept. Whatever the interleaving, the lookup has to give
// that request's own middleware while it runs and nil once it is over. Comparing
// pointers alone cannot show the old fault, since the pooled object kept its
// address and only changed owner.
func Test_Session_FromContext_RetainedContext_Concurrent(t *testing.T) {
	t.Parallel()

	const (
		requests = 64
		lookups  = 400
	)

	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(New())

	var (
		wg       sync.WaitGroup
		finished [requests]atomic.Bool
		stale    atomic.Int64
		foreign  atomic.Int64
	)
	app.Get("/", func(c fiber.Ctx) error {
		n, err := strconv.Atoi(c.Get("X-Request"))
		if err != nil {
			return fmt.Errorf("bad X-Request header: %w", err)
		}
		over := &finished[n]
		own := FromContext(c)
		ctx := c.Context()

		wg.Go(func() {
			for range lookups {
				// Read before the lookup: a flag already set means the request had
				// ended before the lookup began, so it has to find nothing.
				ended := over.Load()
				switch got := FromContext(ctx); {
				case got == nil:
				case ended:
					stale.Add(1)
				case got != own:
					foreign.Add(1)
				}
				runtime.Gosched()
			}
		})
		return c.SendStatus(fiber.StatusOK)
	})

	var clients sync.WaitGroup
	for n := range requests {
		clients.Go(func() {
			req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			req.Header.Set("X-Request", strconv.Itoa(n))
			resp, err := app.Test(req)
			if assert.NoError(t, err) {
				assert.Equal(t, fiber.StatusOK, resp.StatusCode)
			}
			// app.Test returns after the middleware has released its object.
			finished[n].Store(true)
		})
	}
	clients.Wait()
	wg.Wait()

	require.Zero(t, stale.Load(), "a retained context still resolved a session after its request ended")
	require.Zero(t, foreign.Load(), "a retained context reached another request's middleware")
}

// Test_Session_ReleaseMiddleware_DoesNotRecycleAPublishedOne pins the other half:
// a middleware the request context holds is dropped, not pooled, so a pointer
// that was looked up while the request ran can never be handed to another one.
func Test_Session_ReleaseMiddleware_DoesNotRecycleAPublishedOne(t *testing.T) {
	t.Parallel()

	published := acquireMiddleware()
	published.escaped = true
	releaseMiddleware(published)
	require.True(t, published.released.Load())

	local := acquireMiddleware()
	releaseMiddleware(local)
	require.False(t, local.released.Load(), "a middleware only the locals reach goes back to the pool unmarked")

	// sync.Pool gives no promise about what comes back, but it can only return
	// what was put in, so the published one must never appear.
	for range 1000 {
		got := acquireMiddleware()
		require.NotSame(t, published, got)
		releaseMiddleware(got)
	}
}

// Benchmark_Middleware drives the whole handler, which the store benchmarks do
// not: it is where the middleware object is acquired, published to the request
// context and released again.
//
// go test -v -run=^$ -bench=Benchmark_Middleware -benchmem -count=4
func Benchmark_Middleware(b *testing.B) {
	for _, tc := range []struct {
		name string
		pass bool
	}{
		{name: "default"},
		{name: "PassLocalsToContext", pass: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			app := fiber.New(fiber.Config{PassLocalsToContext: tc.pass})
			app.Use(New())
			app.Get("/", func(c fiber.Ctx) error {
				FromContext(c).Set("john", "doe")
				return nil
			})
			handler := app.Handler()

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")

			// The first request creates the session and the rest load it, so the
			// store does not grow while the benchmark runs.
			handler(fctx)
			setCookie := fasthttp.AcquireCookie()
			defer fasthttp.ReleaseCookie(setCookie)
			require.NoError(b, setCookie.ParseBytes(fctx.Response.Header.Peek(fiber.HeaderSetCookie)))
			sessionID := string(setCookie.Value())

			b.ReportAllocs()
			for b.Loop() {
				fctx.ResetUserValues()
				fctx.Response.Reset()
				fctx.Request.Header.SetCookie("session_id", sessionID)
				handler(fctx)
			}
			require.Equal(b, fiber.StatusOK, fctx.Response.StatusCode())
		})
	}
}

// A request still in its handler when another request of the same session logs
// out must not bring the session back when the middleware saves it.
//
// go test -run Test_Session_Middleware_LogoutDuringRequest
func Test_Session_Middleware_LogoutDuringRequest(t *testing.T) {
	t.Parallel()

	for _, logout := range []string{"/reset", "/destroy"} {
		t.Run(logout, func(t *testing.T) {
			t.Parallel()

			loaded, loggedOut := make(chan struct{}), make(chan struct{})
			app := fiber.New()
			app.Use(New())
			app.Post("/login", func(c fiber.Ctx) error {
				sess := FromContext(c)
				if err := sess.Regenerate(); err != nil {
					return err
				}
				sess.Set("user", "alice")
				return nil
			})
			app.Post("/reset", func(c fiber.Ctx) error { return FromContext(c).Reset() })
			app.Post("/destroy", func(c fiber.Ctx) error { return FromContext(c).Destroy() })
			app.Get("/report", func(_ fiber.Ctx) error {
				close(loaded)
				<-loggedOut
				return nil
			})
			app.Get("/whoami", func(c fiber.Ctx) error {
				return c.SendString(fmt.Sprint(FromContext(c).Get("user")))
			})
			h := app.Handler()

			send := func(method, path, id string) *fasthttp.RequestCtx {
				ctx := &fasthttp.RequestCtx{}
				ctx.Request.Header.SetMethod(method)
				ctx.Request.SetRequestURI(path)
				if id != "" {
					ctx.Request.Header.SetCookie("session_id", id)
				}
				h(ctx)
				return ctx
			}

			cookie := fasthttp.AcquireCookie()
			defer fasthttp.ReleaseCookie(cookie)
			cookie.SetKey("session_id")
			require.True(t, send(fiber.MethodPost, "/login", "").Response.Header.Cookie(cookie))
			id := string(cookie.Value())
			require.Equal(t, "alice", string(send(fiber.MethodGet, "/whoami", id).Response.Body()))

			reported := make(chan *fasthttp.RequestCtx, 1)
			go func() { reported <- send(fiber.MethodGet, "/report", id) }()
			<-loaded
			require.Equal(t, fiber.StatusOK, send(fiber.MethodPost, logout, id).Response.StatusCode())
			close(loggedOut)

			report := <-reported
			require.Equal(t, fiber.StatusOK, report.Response.StatusCode())
			require.Empty(t, report.Response.Header.Peek(fiber.HeaderSetCookie))
			require.Equal(t, "<nil>", string(send(fiber.MethodGet, "/whoami", id).Response.Body()))
		})
	}
}
