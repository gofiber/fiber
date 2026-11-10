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
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/storage/memory"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// sessionTestOption changes the request that sessionTestRequest sends.
type sessionTestOption func(*fasthttp.RequestCtx)

// sessionTestCookie makes the request carry the cookie name=value, as a client
// that has none does when value is empty.
func sessionTestCookie(name, value string) sessionTestOption {
	return func(ctx *fasthttp.RequestCtx) {
		if value != "" {
			ctx.Request.Header.SetCookie(name, value)
		}
	}
}

// sessionTestSend runs one request through h on a fresh fasthttp context and
// returns the context to inspect the response. The request URI is left unset
// when uri is empty. It asserts nothing, so a goroutine may call it.
func sessionTestSend(h fasthttp.RequestHandler, method, uri string, opts ...sessionTestOption) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	if uri != "" {
		ctx.Request.SetRequestURI(uri)
	}
	for _, opt := range opts {
		opt(ctx)
	}
	h(ctx)
	return ctx
}

// sessionTestRequest is sessionTestSend that requires the response to have the
// given status.
func sessionTestRequest(t *testing.T, h fasthttp.RequestHandler, method, uri string, status int, opts ...sessionTestOption) *fasthttp.RequestCtx {
	t.Helper()

	ctx := sessionTestSend(h, method, uri, opts...)
	require.Equal(t, status, ctx.Response.StatusCode())
	return ctx
}

// sessionTestToken returns the session ID in the session cookie that the
// response in ctx sets.
func sessionTestToken(t *testing.T, ctx *fasthttp.RequestCtx) string {
	t.Helper()

	setCookie := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	require.NotEmpty(t, setCookie, "Expected Set-Cookie header to be present")
	nameValue, _, found := strings.Cut(setCookie, ";")
	require.True(t, found, "Expected Set-Cookie header to carry attributes: %q", setCookie)
	_, token, found := strings.Cut(nameValue, "=")
	require.True(t, found, "Expected Set-Cookie header to contain a token: %q", setCookie)
	return token
}

func Test_Session_Middleware(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	// withSession serves a route with the session that the middleware attached to
	// the request, and fails the request when there is none.
	withSession := func(handle func(c fiber.Ctx, sess *Middleware) error) fiber.Handler {
		return func(c fiber.Ctx) error {
			sess := FromContext(c)
			if sess == nil {
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			return handle(c, sess)
		}
	}

	app.Get("/get", withSession(func(c fiber.Ctx, sess *Middleware) error {
		value, ok := sess.Get("key").(string)
		if !ok {
			return c.Status(fiber.StatusNotFound).SendString("key not found")
		}
		return c.SendString("value=" + value)
	}))

	app.Post("/set", withSession(func(c fiber.Ctx, sess *Middleware) error {
		sess.Set("key", c.FormValue("value"))
		return c.SendStatus(fiber.StatusOK)
	}))

	app.Post("/delete", withSession(func(c fiber.Ctx, sess *Middleware) error {
		sess.Delete("key")
		return c.SendStatus(fiber.StatusOK)
	}))

	app.Post("/reset", withSession(func(c fiber.Ctx, sess *Middleware) error {
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
	}))

	app.Post("/regenerate", withSession(func(c fiber.Ctx, sess *Middleware) error {
		// Set a value to ensure it is preserved after regeneration
		sess.Set("key", "value")
		if err := sess.Regenerate(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		// Ensure the session ID has changed but session data is preserved
		if sess.ID() == "" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		value, ok := sess.Get("key").(string)
		if !ok || value != "value" {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	}))

	app.Post("/destroy", withSession(func(c fiber.Ctx, sess *Middleware) error {
		if err := sess.Destroy(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	}))

	app.Post("/fresh", withSession(func(c fiber.Ctx, sess *Middleware) error {
		// Reset the session to make it fresh
		if err := sess.Reset(); err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		if sess.Fresh() {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}))

	app.Post("/keys", withSession(func(c fiber.Ctx, sess *Middleware) error {
		for rawKey := range strings.SplitSeq(c.FormValue("keys"), ",") {
			key := utils.TrimSpace(rawKey)
			if key == "" {
				continue
			}
			sess.Set(key, "value_"+key)
		}
		return c.SendStatus(fiber.StatusOK)
	}))

	app.Get("/keys", withSession(func(c fiber.Ctx, sess *Middleware) error {
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
	}))

	h := app.Handler()
	ctx := &fasthttp.RequestCtx{}
	var token string

	// send runs a request on the one shared context, with the current session
	// token as the session cookie and form, when not empty, as a urlencoded body,
	// and requires status.
	send := func(method, uri, form string, status int) {
		t.Helper()
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(method)
		ctx.Request.SetRequestURI(uri)
		if form != "" {
			ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded") // Set the Content-Type
			ctx.Request.SetBodyString(form)
		}
		if token != "" {
			ctx.Request.Header.SetCookie("session_id", token)
		}
		h(ctx)
		require.Equal(t, status, ctx.Response.StatusCode())
	}
	// rotate requires the response to set a new session token and moves on to it.
	rotate := func() {
		t.Helper()
		newToken := sessionTestToken(t, ctx)
		require.NotEqual(t, token, newToken)
		token = newToken
	}

	send(fiber.MethodGet, "/get", "", fiber.StatusNotFound)
	token = sessionTestToken(t, ctx)
	require.Equal(t, "key not found", string(ctx.Response.Body()))

	send(fiber.MethodPost, "/set", "value=hello", fiber.StatusOK)
	send(fiber.MethodGet, "/get", "", fiber.StatusOK)
	require.Equal(t, "value=hello", string(ctx.Response.Body()))

	send(fiber.MethodPost, "/delete", "", fiber.StatusOK)
	send(fiber.MethodGet, "/get", "", fiber.StatusNotFound)
	require.Equal(t, "key not found", string(ctx.Response.Body()))

	send(fiber.MethodPost, "/reset", "", fiber.StatusOK)
	rotate()

	send(fiber.MethodPost, "/regenerate", "", fiber.StatusOK)
	rotate()

	send(fiber.MethodPost, "/destroy", "", fiber.StatusOK)
	require.Contains(t, string(ctx.Response.Header.Peek(fiber.HeaderSetCookie)), "max-age=0")

	// Sleep so that the session expires
	time.Sleep(1 * time.Second)

	// the destroyed session is gone: a new session token is issued
	send(fiber.MethodGet, "/get", "", fiber.StatusNotFound)
	rotate()

	send(fiber.MethodPost, "/fresh", "", fiber.StatusOK)
	rotate()

	send(fiber.MethodPost, "/keys", "keys=key1,key2", fiber.StatusOK)
	send(fiber.MethodGet, "/keys", "", fiber.StatusOK)
	body := string(ctx.Response.Body())
	require.True(t, strings.HasPrefix(body, "keys="))
	parts := strings.Split(strings.TrimPrefix(body, "keys="), ",")
	require.Len(t, parts, 2, "Expected two keys in the session")
	sort.Strings(parts)
	require.Equal(t, []string{"key1", "key2"}, parts)
}

func Test_SessionLoggerTagRedactsSessionID(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	app := fiber.New()
	app.Use(New())
	app.Use(logger.New(logger.Config{Format: "${session-id}", Stream: &buf}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Contains(t, buf.String(), "****")
}

func Test_SessionLoggerTagWithOuterLoggerDoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	app := fiber.New()
	app.Use(logger.New(logger.Config{Format: "${session-id}", Stream: &buf})) // outside the session middleware
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Empty(t, buf.String())
}

// Test_SessionLogContextTagRedactsSessionID runs serially because it mutates
// package-global default logger output and context format.
// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_SessionLogContextTagRedactsSessionID(t *testing.T) {
	buf := loggertest.CaptureContextLog(t, "session-id=${session-id} ")

	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		fiberlog.WithContext(c).Info("start")
		return c.SendStatus(fiber.StatusOK)
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Contains(t, buf.String(), "[Info] session-id=")
	require.Contains(t, buf.String(), "**** start")
}

func Test_Session_NewWithStore(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("value=" + FromContext(c).ID()) })

	h := app.Handler()

	ctx := sessionTestRequest(t, h, fiber.MethodGet, "", fiber.StatusOK)
	token := sessionTestToken(t, ctx)
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	ctx = sessionTestRequest(t, h, fiber.MethodGet, "", fiber.StatusOK, sessionTestCookie("session_id", token))
	require.Equal(t, "value="+token, string(ctx.Response.Body()))
}

func Test_Session_FromSession(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	sess := FromContext(apptest.Ctx(t, app))
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

	apptest.Expect(t, app, "/", fiber.StatusOK)
}

func Test_Session_WithConfig(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	const idleTimeout = 1 * time.Second

	app.Use(New(Config{
		Next:         func(c fiber.Ctx) bool { return c.Get("key") == "value" },
		IdleTimeout:  idleTimeout,
		Extractor:    extractors.FromCookie("session_id_test"),
		KeyGenerator: func() string { return "test" },
	}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendString("value=" + FromContext(c).ID()) })
	app.Get("/isFresh", func(c fiber.Ctx) error {
		if FromContext(c).Fresh() {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	})
	app.Post("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{Name: "session_id_test", Value: FromContext(c).ID()})
		return nil
	})

	h := app.Handler()

	ctx := sessionTestRequest(t, h, fiber.MethodGet, "", fiber.StatusOK)
	token := sessionTestToken(t, ctx)
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	ctx = sessionTestRequest(t, h, fiber.MethodGet, "", fiber.StatusOK, sessionTestCookie("session_id_test", token))
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	sessionTestRequest(t, h, fiber.MethodPost, "", fiber.StatusOK, sessionTestCookie("session_id_test", token))
	sessionTestRequest(t, h, fiber.MethodPost, "", fiber.StatusOK)
	// a cookie under the wrong key, then the right key with the wrong value
	sessionTestRequest(t, h, fiber.MethodPost, "", fiber.StatusOK, sessionTestCookie("session_id", token))
	sessionTestRequest(t, h, fiber.MethodPost, "", fiber.StatusOK, sessionTestCookie("session_id_test", "wrong"))

	// Check idle timeout not expired
	sessionTestRequest(t, h, fiber.MethodGet, "/isFresh", fiber.StatusInternalServerError, sessionTestCookie("session_id_test", token))

	// Test idle timeout, waiting on the cached clock the storage compares against
	clocktest.SleepPast(t, idleTimeout)
	sessionTestRequest(t, h, fiber.MethodGet, "/isFresh", fiber.StatusOK, sessionTestCookie("session_id_test", token))
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
		return c.SendString("value=" + sess.ID())
	})

	h := app.Handler()

	// Test with Next returning false
	ctx := sessionTestRequest(t, h, fiber.MethodGet, "", fiber.StatusOK)
	token := sessionTestToken(t, ctx)
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test with Next returning true
	muNext.Lock()
	doNext = true
	muNext.Unlock()

	sessionTestRequest(t, h, fiber.MethodGet, "", fiber.StatusInternalServerError)
}

func Test_Session_Middleware_Store(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	handler, sessionStore := NewWithStore()

	app.Use(handler)

	app.Get("/", func(c fiber.Ctx) error {
		if FromContext(c).Store() != sessionStore {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})

	sessionTestRequest(t, app.Handler(), fiber.MethodGet, "", fiber.StatusOK)
}

// Regression: https://github.com/gofiber/fiber/issues/4348
// A failing session store must result in an error response, not a panic.
func Test_Session_Middleware_StoreError(t *testing.T) {
	t.Parallel()
	storage := &storagetest.Failing{Err: errors.New("storage unavailable")}
	app := fiber.New()
	app.Use(New(Config{Storage: storage}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	h := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	// Provide a session ID so the middleware attempts a storage lookup.
	ctx.Request.Header.SetCookie("session_id", "some-session-id")
	require.NotPanics(t, func() { h(ctx) })
	require.Equal(t, fiber.StatusInternalServerError, ctx.Response.StatusCode())
	require.Equal(t, "Internal Server Error", string(ctx.Response.Body()))
	require.NotContains(t, string(ctx.Response.Body()), storage.Err.Error())
}

func Test_Session_Middleware_StoreError_CustomErrorHandler(t *testing.T) {
	t.Parallel()
	storageErr := errors.New("redis dial tcp redis.internal.corp:6379: connect: connection refused")
	storage := &storagetest.Failing{Err: storageErr}
	app := fiber.New()
	var handledErr error
	app.Use(New(Config{
		Storage: storage,
		ErrorHandler: func(c fiber.Ctx, err error) {
			handledErr = err
			require.NoError(t, c.Status(fiber.StatusInternalServerError).SendString("session unavailable"))
		},
	}))

	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	ctx := sessionTestRequest(t, app.Handler(), fiber.MethodGet, "", fiber.StatusInternalServerError,
		sessionTestCookie("session_id", "some-session-id"))

	require.ErrorIs(t, handledErr, storageErr)
	require.Equal(t, "session unavailable", string(ctx.Response.Body()))
}

// sessionTestServe serves route at path behind the session middleware on a fresh
// app and requires a GET for path to be answered 200.
func sessionTestServe(t *testing.T, path string, route fiber.Handler) {
	t.Helper()

	handler, _ := NewWithStore()
	app := fiber.New()
	app.Use(handler)
	app.Get(path, route)
	sessionTestRequest(t, app.Handler(), fiber.MethodGet, path, fiber.StatusOK)
}

func Test_Middleware_DestroyWithContext(t *testing.T) {
	t.Parallel()
	sessionTestServe(t, "/destroy", func(c fiber.Ctx) error {
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
}

func Test_Middleware_ResetWithContext(t *testing.T) {
	t.Parallel()
	sessionTestServe(t, "/reset", func(c fiber.Ctx) error {
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
}

func Test_Middleware_RegenerateWithContext(t *testing.T) {
	t.Parallel()
	sessionTestServe(t, "/regenerate", func(c fiber.Ctx) error {
		sess := FromContext(c)
		originalID := sess.ID()
		if err := sess.RegenerateWithContext(t.Context()); err != nil {
			return err
		}
		if sess.ID() == originalID {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusOK)
	})
}

func Test_Middleware_WithContext_ErrorPropagation(t *testing.T) {
	t.Parallel()
	t.Run("DestroyWithContext propagates canceled context and marks destroyed", func(t *testing.T) {
		t.Parallel()
		m := &Middleware{Session: newSessionWithStorage(t, &storagetest.Memory{HonorContext: true})}
		require.ErrorIs(t, m.DestroyWithContext(sessionTestCanceled()), context.Canceled)
		require.True(t, m.is(lifeDestroyed))
	})

	for _, tc := range []struct {
		call func(*Middleware, context.Context) error
		name string
	}{
		{name: "ResetWithContext", call: (*Middleware).ResetWithContext},
		{name: "RegenerateWithContext", call: (*Middleware).RegenerateWithContext},
	} {
		t.Run(tc.name+" propagates storage error", func(t *testing.T) {
			t.Parallel()
			storageErr := errors.New("storage unavailable")
			m := &Middleware{Session: newSessionWithStorage(t, &storagetest.Failing{Err: storageErr})}
			require.ErrorIs(t, tc.call(m, t.Context()), storageErr)
		})
	}
}

func Test_Middleware_ResolveContext(t *testing.T) {
	t.Parallel()
	t.Run("resolve context returns fiber ctx when not nil", func(t *testing.T) {
		t.Parallel()
		fctx := apptest.Ctx(t, fiber.New())
		m := &Middleware{ctx: fctx}
		require.Equal(t, fctx, m.resolveContext())
	})

	t.Run("resolve context returns background when ctx is nil", func(t *testing.T) {
		t.Parallel()
		m := &Middleware{ctx: nil}
		require.NotNil(t, m.resolveContext())
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

	resp := apptest.Do(t, app, "/set")
	cookie := resp.Header.Get("Set-Cookie")
	require.NotEmpty(t, cookie)

	req := httptest.NewRequest(fiber.MethodGet, "/get", http.NoBody)
	req.Header.Set("Cookie", cookie)
	apptest.ExpectBody(t, app, req, fiber.StatusOK, "world")
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

				apptest.Expect(t, app, "/a", fiber.StatusOK)

				// The request is over: its context no longer leads to a session.
				require.Nil(t, FromContext(retained), "the released middleware is still reachable")

				apptest.Expect(t, app, "/b", fiber.StatusOK)

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

	apptest.Expect(t, app, "/", fiber.StatusOK)
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
	published.mark(lifeEscaped)
	releaseMiddleware(published)
	require.True(t, published.is(lifeReleased))

	local := acquireMiddleware()
	releaseMiddleware(local)
	require.False(t, local.is(lifeReleased), "a middleware only the locals reach goes back to the pool unmarked")

	// sync.Pool gives no promise about what comes back, but it can only return
	// what was put in, so the published one must never appear.
	for range 1000 {
		got := acquireMiddleware()
		require.NotSame(t, published, got)
		releaseMiddleware(got)
	}
}

// Test_Session_Middleware_TimedOutHandler checks that a late write of a handler
// timeout.New gave up on stays in its own session, never in another request's.
func Test_Session_Middleware_TimedOutHandler(t *testing.T) {
	t.Parallel()
	const others = 16

	var (
		held     = make(chan *Middleware, 1)
		release  = make(chan struct{})
		finished = make(chan struct{})
		entered  = make(chan struct{}, others)
		proceed  = make(chan struct{})
	)

	app := fiber.New()
	app.Use(New())
	app.Get("/login", timeout.New(func(c fiber.Ctx) error {
		defer close(finished)
		sess := FromContext(c)
		held <- sess
		<-release
		sess.Set("user", "alice")
		return nil
	}, timeout.Config{Timeout: 10 * time.Millisecond}))
	app.Get("/me", func(c fiber.Ctx) error {
		sess := FromContext(c)
		entered <- struct{}{}
		<-proceed
		return c.SendString(fmt.Sprint(sess.Get("user")))
	})

	apptest.Expect(t, app, "/login", fiber.StatusServiceUnavailable)

	m := apptest.Recv(t, held, 5*time.Second, "the login handler never started")
	m.mu.RLock()
	sess := m.Session
	m.mu.RUnlock()
	require.NotNil(t, sess, "the running handler's middleware was reset")
	id := sess.ID()
	require.NotEmpty(t, id, "the running handler's session was reset")

	// What the requests to /me got, by request: a goroutine cannot assert.
	bodies, errs := make([]string, others), make([]error, others)
	var wg sync.WaitGroup
	for i := range others {
		wg.Go(func() {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/me", http.NoBody), fiber.TestConfig{Timeout: 0})
			if err == nil {
				var body []byte
				// conventions:ignore body-read: the read runs in a goroutine, where FailNow is not allowed
				body, err = io.ReadAll(resp.Body)
				bodies[i] = string(body)
			}
			errs[i] = err
		})
	}
	for range others {
		apptest.Recv(t, entered, 5*time.Second, "a request never reached /me")
	}

	close(release)
	apptest.Recv(t, finished, 5*time.Second, "the login handler never finished")
	close(proceed)
	wg.Wait()
	for i := range others {
		require.NoError(t, errs[i])
		require.Equal(t, "<nil>", bodies[i], "the timed-out handler wrote to another request's session")
	}

	require.Equal(t, id, m.ID())
	require.Equal(t, "alice", m.Get("user"), "the late write must reach the handler's own session")
}

// Test_Session_Middleware_FinishesOnTimedOutGoroutine mounts the session below a
// timeout.New that runs c.Next, so its check runs on the abandoned goroutine.
func Test_Session_Middleware_FinishesOnTimedOutGoroutine(t *testing.T) {
	t.Parallel()
	finished := make(chan struct{})
	app := fiber.New()
	app.Use(timeout.New(func(c fiber.Ctx) error { return c.Next() }, timeout.Config{Timeout: 10 * time.Millisecond}))
	app.Use(func(c fiber.Ctx) error {
		defer close(finished)
		return c.Next()
	})
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		<-c.Context().Done()
		FromContext(c).Set("user", "alice")
		return nil
	})

	apptest.Expect(t, app, "/", fiber.StatusServiceUnavailable)
	apptest.Recv(t, finished, 5*time.Second, "the session middleware never returned on the timed-out goroutine")
}

// Test_Session_Middleware_TimedOutRequestLetsGoOfItsContext checks that, once the timeout
// middleware answered the request, neither the middleware nor the session of the handler it gave up
// on refers to the request any more. The context of the request is reused for another one when the
// handler is done, and whoever still holds the session then must not write into that request. Both
// stay as they are for the handler itself.
func Test_Session_Middleware_TimedOutRequestLetsGoOfItsContext(t *testing.T) {
	t.Parallel()
	var (
		held    = make(chan *Middleware, 1)
		release = make(chan struct{})
		done    = make(chan struct{})
	)
	store := NewStore()
	app := fiber.New()
	app.Use(New(Config{Store: store}))
	app.Get("/slow", timeout.New(func(c fiber.Ctx) error {
		defer close(done)
		held <- FromContext(c)
		<-release
		return nil
	}, timeout.Config{Timeout: 10 * time.Millisecond}))

	apptest.Expect(t, app, "/slow", fiber.StatusServiceUnavailable)

	m := apptest.Recv(t, held, 5*time.Second)
	m.mu.RLock()
	sess, ctx := m.Session, m.ctx
	m.mu.RUnlock()
	require.NotNil(t, sess, "the middleware of the running handler was reset")
	require.Nil(t, ctx, "the middleware still refers to the request that was answered")
	require.Nil(t, sess.requestCtx(), "and so does its session")

	m.Set("late", "x")
	require.Equal(t, "x", m.Get("late"), "the handler goes on with its own session")

	// nothing saved it, since the response was fixed; a save of its own writes the record, and no cookie
	require.Empty(t, storedFor(t, store, sess.ID()))
	require.NoError(t, sess.Save())
	require.NotEmpty(t, storedFor(t, store, sess.ID()))

	close(release)
	apptest.Recv(t, done, 5*time.Second)
}

// slowStorage makes every write and delete take a while, as a storage under load does.
type slowStorage struct {
	fiber.Storage
	delay time.Duration
}

func (s slowStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	time.Sleep(s.delay)
	return s.Storage.SetWithContext(ctx, key, val, exp)
}

func (s slowStorage) DeleteWithContext(ctx context.Context, key string) error {
	time.Sleep(s.delay)
	return s.Storage.DeleteWithContext(ctx, key)
}

// Test_Session_Middleware_TimedOutAnswerDoesNotWaitForTheHandlerInTheStorage checks that the answer
// of the timeout middleware does not wait for the handler it gave up on. The handler is inside a
// call that holds a lock across the storage, the one of its middleware (Destroy) or the one of its
// session (a save), and the cut from the request, which needs that lock, follows when the call is
// done.
func Test_Session_Middleware_TimedOutAnswerDoesNotWaitForTheHandlerInTheStorage(t *testing.T) {
	t.Parallel()
	const (
		storageDelay = 600 * time.Millisecond
		answerWithin = 400 * time.Millisecond
	)
	for name, call := range map[string]func(m *Middleware) error{
		"Destroy holds the lock of the middleware": func(m *Middleware) error { return m.Destroy() },
		"a save holds the lock of the session": func(m *Middleware) error {
			m.Set("k", "v")
			return m.Session.saveSessionWithContext(context.Background())
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var (
				held = make(chan *Middleware, 1)
				done = make(chan struct{})
			)
			app := fiber.New()
			app.Use(New(Config{Storage: slowStorage{Storage: memory.New(), delay: storageDelay}}))
			app.Get("/slow", timeout.New(func(c fiber.Ctx) error {
				defer close(done)
				m := FromContext(c)
				held <- m
				return call(m)
			}, timeout.Config{Timeout: 20 * time.Millisecond}))

			start := time.Now()
			resp := apptest.Do(t, app, "/slow", fiber.TestConfig{Timeout: 5 * time.Second})
			require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
			require.Less(t, time.Since(start), answerWithin, "the answer waited for the handler in the storage")

			m := apptest.Recv(t, held, 5*time.Second)
			apptest.Recv(t, done, 5*time.Second)
			requireDetached(t, m)
		})
	}
}

// Test_Session_Middleware_AbandonedWithoutTimeoutKeepsItsObjects abandons the request the way the
// sse middleware does, with no timeout response. The session is saved as usual, and then neither
// the middleware nor the session goes back to the pool, where the next request would take them
// while the handler still holds them.
func Test_Session_Middleware_AbandonedWithoutTimeoutKeepsItsObjects(t *testing.T) {
	t.Parallel()
	var (
		m    *Middleware
		sess *Session
	)
	store := NewStore()
	app := fiber.New()
	app.Use(New(Config{Store: store}))
	app.Get("/stream", func(c fiber.Ctx) error {
		m = FromContext(c)
		sess = m.Session
		m.Set("user", "alice")
		c.Abandon()
		return nil
	})

	resp := apptest.Expect(t, app, "/stream", fiber.StatusOK)
	require.NotEmpty(t, resp.Header.Get(fiber.HeaderSetCookie), "an abandoned request is still answered with its cookie")
	require.NotEmpty(t, storedFor(t, store, sess.ID()), "and its session is saved")

	require.True(t, m.is(lifeReleased), "the middleware is finished, and not in the pool")
	require.Equal(t, "alice", sess.Get("user"), "its session is left as it was, not reset for the pool")
}

// Benchmark_Middleware drives the whole handler, which the store benchmarks do
// not: it is where the middleware object is acquired, published to the request
// context and released again.
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
			app.Get("/whoami", func(c fiber.Ctx) error { return c.SendString(fmt.Sprint(FromContext(c).Get("user"))) })
			h := app.Handler()

			send := func(method, path, id string) *fasthttp.RequestCtx {
				return sessionTestSend(h, method, path, sessionTestCookie("session_id", id))
			}

			cookie := fasthttp.AcquireCookie()
			defer fasthttp.ReleaseCookie(cookie)
			cookie.SetKey("session_id")
			require.True(t, send(fiber.MethodPost, "/login", "").Response.Header.Cookie(cookie))
			id := string(cookie.Value())
			require.Equal(t, "alice", string(send(fiber.MethodGet, "/whoami", id).Response.Body()))

			reported := make(chan *fasthttp.RequestCtx, 1)
			go func() { reported <- send(fiber.MethodGet, "/report", id) }()
			apptest.Recv(t, loaded, 5*time.Second, "the request never reached its handler")
			require.Equal(t, fiber.StatusOK, send(fiber.MethodPost, logout, id).Response.StatusCode())
			close(loggedOut)

			report := apptest.Recv(t, reported, 5*time.Second, "the request never finished")
			require.Equal(t, fiber.StatusOK, report.Response.StatusCode())
			require.Empty(t, report.Response.Header.Peek(fiber.HeaderSetCookie))
			require.Equal(t, "<nil>", string(send(fiber.MethodGet, "/whoami", id).Response.Body()))
		})
	}
}

// addSessionRoutes registers the routes that take a session through its life: /login stores a user
// and answers with the ID, /read answers with the user, /regenerate answers with the new ID and
// /destroy ends the session.
func addSessionRoutes(app *fiber.App) {
	app.Get("/login", func(c fiber.Ctx) error {
		FromContext(c).Set("user", "alice")
		return c.SendString(FromContext(c).ID())
	})
	app.Get("/read", func(c fiber.Ctx) error { return c.SendString(fmt.Sprint(FromContext(c).Get("user"))) })
	app.Get("/regenerate", func(c fiber.Ctx) error {
		if err := FromContext(c).Regenerate(); err != nil {
			return err
		}
		return c.SendString(FromContext(c).ID())
	})
	app.Get("/destroy", func(c fiber.Ctx) error { return FromContext(c).Destroy() })
}

// skipEmptyApp builds an app whose routes exercise every way a request can leave a session behind.
func skipEmptyApp(t *testing.T, cfg *Config) (*fiber.App, *storagetest.Memory) {
	t.Helper()

	mem := storagetest.New()
	cfg.Storage = mem

	app := fiber.New()
	app.Use(New(*cfg))
	addSessionRoutes(app)
	app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/touch", func(c fiber.Ctx) error { return c.SendString(strconv.FormatBool(FromContext(c).Fresh())) })
	app.Get("/logout", func(c fiber.Ctx) error { return FromContext(c).Reset() })
	app.Get("/fail-empty", func(_ fiber.Ctx) error { return fiber.ErrTeapot })
	app.Get("/fail-with-data", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		return fiber.ErrTeapot
	})
	return app, mem
}

func Test_Session_SkipEmptySessions(t *testing.T) {
	t.Parallel()
	t.Run("off by default: every anonymous request stores a session and sets a cookie", func(t *testing.T) {
		t.Parallel()
		app, mem := skipEmptyApp(t, &Config{})

		for range 5 {
			resp, _ := sendRequest(t, app, "/health", nil)
			require.Contains(t, cookiesByName(resp), "session_id")
		}
		require.Len(t, mem.Keys(), 5)
	})

	t.Run("a visitor who never uses the session leaves nothing behind", func(t *testing.T) {
		t.Parallel()
		app, mem := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		for range 20 {
			resp, body := sendRequest(t, app, "/health", nil)
			require.Equal(t, "ok", body)
			require.Empty(t, resp.Cookies(), "no cookie for a session that is not kept")
		}
		require.Empty(t, mem.Keys())
		require.Empty(t, mem.Calls(storagetest.OpSet))

		// Looking at the session is not using it.
		resp, body := sendRequest(t, app, "/touch", nil)
		require.Equal(t, "true", body, "the session is still new, as always")
		require.Empty(t, resp.Cookies())
		require.Empty(t, mem.Keys())

		// Nor is a cookie for a session the storage no longer has.
		resp, _ = sendRequest(t, app, "/health", map[string]string{"session_id": "expired-or-forged"})
		require.Empty(t, resp.Cookies(), "an ID the visitor made up is not turned into a session")
		require.Empty(t, mem.Keys())
	})

	t.Run("a session that has data is kept: stored, refreshed on every request, and regenerated with its data", func(t *testing.T) {
		t.Parallel()
		app, mem := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		resp, id := sendRequest(t, app, "/login", nil)
		cookies := cookiesByName(resp)
		require.Contains(t, cookies, "session_id")
		require.Equal(t, id, cookies["session_id"].Value)
		require.Len(t, mem.Keys(), 1)
		require.Len(t, mem.Calls(storagetest.OpSet), 1)

		resp, _ = sendRequest(t, app, "/health", map[string]string{"session_id": id})
		require.Len(t, mem.Calls(storagetest.OpSet), 2, "the idle timeout must keep sliding for a session that is in use")
		require.Contains(t, cookiesByName(resp), "session_id")
		require.Len(t, mem.Keys(), 1)

		_, body := sendRequest(t, app, "/read", map[string]string{"session_id": id})
		require.Equal(t, "alice", body)

		resp, newID := sendRequest(t, app, "/regenerate", map[string]string{"session_id": id})
		require.NotEqual(t, id, newID)
		require.Equal(t, newID, cookiesByName(resp)["session_id"].Value)
		require.Len(t, mem.Keys(), 1)

		_, body = sendRequest(t, app, "/read", map[string]string{"session_id": newID})
		require.Equal(t, "alice", body)
		_, body = sendRequest(t, app, "/read", map[string]string{"session_id": id})
		require.Equal(t, "<nil>", body, "the old ID no longer resolves")
	})

	t.Run("a logout expires the cookie and a destroy removes the record, and neither stores a replacement", func(t *testing.T) {
		t.Parallel()
		app, mem := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		_, id := sendRequest(t, app, "/login", nil)
		require.Len(t, mem.Keys(), 1)

		resp, _ := sendRequest(t, app, "/logout", map[string]string{"session_id": id})
		require.Empty(t, mem.Keys(), "the old record is gone and no empty one took its place")
		cookies := cookiesByName(resp)
		require.Contains(t, cookies, "session_id")
		require.Negative(t, cookies["session_id"].MaxAge, "the only cookie sent expires the old one")
		require.Empty(t, cookies["session_id"].Value)

		resp, _ = sendRequest(t, app, "/logout", map[string]string{"session_id": id})
		require.Empty(t, mem.Keys())
		require.NotEmpty(t, resp.Cookies())

		_, id = sendRequest(t, app, "/login", nil)
		_, _ = sendRequest(t, app, "/destroy", map[string]string{"session_id": id})
		require.Empty(t, mem.Keys())
	})

	t.Run("without the option a logout leaves an empty session behind", func(t *testing.T) {
		t.Parallel()
		app, mem := skipEmptyApp(t, &Config{})

		_, id := sendRequest(t, app, "/login", nil)
		resp, _ := sendRequest(t, app, "/logout", map[string]string{"session_id": id})
		require.Len(t, mem.Keys(), 1)
		require.Positive(t, cookiesByName(resp)["session_id"].MaxAge)
	})

	t.Run("a failing handler decides nothing: data is saved, an empty session is not", func(t *testing.T) {
		t.Parallel()
		app, mem := skipEmptyApp(t, &Config{SkipEmptySessions: true})

		resp, _ := sendExpecting(t, app, "/fail-empty", nil, fiber.StatusTeapot)
		require.Empty(t, resp.Cookies())
		require.Empty(t, mem.Keys())

		resp, _ = sendExpecting(t, app, "/fail-with-data", nil, fiber.StatusTeapot)
		require.Contains(t, cookiesByName(resp), "session_id")
		require.Len(t, mem.Keys(), 1)
	})

	t.Run("the absolute timeout bookkeeping is not data", func(t *testing.T) {
		t.Parallel()
		app, mem := skipEmptyApp(t, &Config{SkipEmptySessions: true, AbsoluteTimeout: time.Hour})

		resp, _ := sendRequest(t, app, "/health", nil)
		require.Empty(t, resp.Cookies())
		require.Empty(t, mem.Keys())

		resp, id := sendRequest(t, app, "/login", nil)
		require.Equal(t, id, cookiesByName(resp)["session_id"].Value)
		require.Len(t, mem.Keys(), 1)
	})

	t.Run("the store pattern saves what the handler tells it to", func(t *testing.T) {
		t.Parallel()
		mem := storagetest.New()
		store := NewStore(Config{Storage: mem, SkipEmptySessions: true})
		seedSession(t, fiber.New(), store, func(*Session) {}) // gets the session, stores nothing in it and saves it
		require.Len(t, mem.Keys(), 1, "an explicit Save is the caller's decision")
	})
}

// Benchmark_Middleware_NewVisitor is a request from a client that sends no cookie and never uses
// the session, such as a health check. With SkipEmptySessions it stores nothing and sets no cookie.
func Benchmark_Middleware_NewVisitor(b *testing.B) {
	for _, skip := range []bool{false, true} {
		b.Run("SkipEmptySessions="+strconv.FormatBool(skip), func(b *testing.B) {
			app := fiber.New()
			app.Use(New(Config{SkipEmptySessions: skip}))
			app.Get("/", func(_ fiber.Ctx) error { return nil })
			handler := app.Handler()

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")

			b.ReportAllocs()
			for b.Loop() {
				fctx.ResetUserValues()
				fctx.Response.Reset()
				handler(fctx)
			}
		})
	}
}

// countingCtx counts how often the session middleware asks for the Go context of the request and
// replaces it.
type countingCtx struct {
	contexts *atomic.Int64
	sets     *atomic.Int64
	fiber.DefaultCtx
}

func (c *countingCtx) Context() context.Context {
	c.contexts.Add(1)
	return c.DefaultCtx.Context()
}

func (c *countingCtx) SetContext(ctx context.Context) {
	c.sets.Add(1)
	c.DefaultCtx.SetContext(ctx)
}

// Without PassLocalsToContext the request context never holds the session, so the middleware has
// nothing to store in it, ask it for, or hide in it at the end: it leaves the context alone, which
// spares Fiber the context it would create for the request and the wrapping of it, and with
// PassLocalsToContext it does all of that.
func Test_Session_Middleware_ContextIsLeftAloneWithoutPassLocalsToContext(t *testing.T) {
	t.Parallel()
	for _, pass := range []bool{false, true} {
		t.Run(fmt.Sprintf("pass=%t", pass), func(t *testing.T) {
			t.Parallel()
			var contexts, sets atomic.Int64
			app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
				return &countingCtx{DefaultCtx: *fiber.NewDefaultCtx(app), contexts: &contexts, sets: &sets}
			}, fiber.Config{PassLocalsToContext: pass})
			app.Use(New())
			app.Get("/", func(c fiber.Ctx) error {
				FromContext(c).Set("k", "v")
				return nil
			})

			apptest.Expect(t, app, "/", fiber.StatusOK)

			if pass {
				require.Positive(t, sets.Load(), "the middleware stores itself in the context, and hides itself in it at the end")
				require.Positive(t, contexts.Load())
			} else {
				require.Zero(t, sets.Load(), "the context was replaced though nothing is stored in it")
				require.Zero(t, contexts.Load(), "the context was asked for though nothing is stored in it")
			}
		})
	}
}

// With PassLocalsToContext a middleware that finished hides the session ID and itself behind empty
// values in the context of the request, for whoever asks for the context afterwards: an outer
// middleware that logs the session of the request, say. Both values are hidden, and nothing else.
func Test_Session_Middleware_FinishedMiddlewareIsHiddenInTheContext(t *testing.T) {
	t.Parallel()
	type otherKey struct{}

	var after context.Context
	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(func(c fiber.Ctx) error {
		c.SetContext(context.WithValue(c.Context(), otherKey{}, "kept")) //nolint:contextcheck // a middleware before the session one stores a value in the context
		err := c.Next()
		after = c.Context()
		return err
	})
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		require.NotEmpty(t, FromContext(c).ID())
		held, ok := c.Context().Value(middlewareContextKey).(*Middleware)
		require.True(t, ok)
		require.NotNil(t, held, "the middleware is in the context while it runs")
		return nil
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)

	require.NotNil(t, after)
	hiddenID, ok := after.Value(sessionIDContextKey).(string)
	require.True(t, ok, "the session ID is hidden behind an empty value, not removed")
	require.Empty(t, hiddenID)
	held, ok := after.Value(middlewareContextKey).(*Middleware)
	require.True(t, ok, "the middleware is hidden behind an empty value, not removed")
	require.Nil(t, held)
	require.Equal(t, "kept", after.Value(otherKey{}), "other values stay")
	require.Nil(t, FromContext(after))
}

// A middleware that finished holds neither the store nor the error handler it ran with: a handler
// that outlives the request finds none, and a recycled one does not keep a closure of the
// application alive until its next request.
func Test_Session_Middleware_ReleaseForgetsTheStoreAndTheErrorHandler(t *testing.T) {
	t.Parallel()
	c := apptest.Ctx(t, fiber.New())
	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid"), ErrorHandler: func(fiber.Ctx, error) {}})

	m := startMiddleware(t, c, store)
	require.Same(t, store, m.Store())
	require.NotNil(t, m.errorHandler)

	// Dropped, not recycled, so that the test can look at it afterwards.
	m.mark(lifeEscaped)
	releaseMiddleware(m)

	require.Nil(t, m.Store())
	require.Nil(t, m.errorHandler)
}

// startMiddleware runs the beginning of the session handler for store on c: the middleware is
// acquired, loads its session and becomes the current middleware of the request. releaseMiddleware
// is its end.
func startMiddleware(t *testing.T, c fiber.Ctx, store *Store) *Middleware {
	t.Helper()

	cfg := store.Config
	cfg.Store = store

	m := acquireMiddleware()
	require.NoError(t, m.initialize(c, &cfg))
	return m
}

// middlewarePair is a session middleware over b_sid started inside one over a_sid on one request.
type middlewarePair struct {
	app            *fiber.App
	c              fiber.Ctx
	storeA, storeB *Store
	outer, inner   *Middleware
}

// startPair starts a session middleware of one store inside one of another on the same request,
// as a nested pair of middleware would be.
func startPair(t *testing.T) middlewarePair {
	t.Helper()

	p := middlewarePair{
		app:    fiber.New(),
		storeA: NewStore(Config{Extractor: extractors.FromCookie("a_sid")}),
		storeB: NewStore(Config{Extractor: extractors.FromCookie("b_sid")}),
	}
	p.c = apptest.Ctx(t, p.app)
	p.outer = startMiddleware(t, p.c, p.storeA)
	p.inner = startMiddleware(t, p.c, p.storeB)
	return p
}

// requireDetached waits until m and its session let go of the fiber.Ctx of the request, which goes
// back to the pool while a handler that outlives the request (one the timeout middleware gave up
// on, or a panic unwound) still holds them.
func requireDetached(t *testing.T, m *Middleware) {
	t.Helper()

	require.Eventually(t, func() bool {
		m.mu.RLock()
		sess, ctx := m.Session, m.ctx
		m.mu.RUnlock()
		return sess != nil && ctx == nil && sess.requestCtx() == nil
	}, 5*time.Second, 5*time.Millisecond, "the middleware and its session still refer to the request")
}

// holdsNoRequest reports whether the session writes to no request any more.
func (s *Session) holdsNoRequest() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.holdsNoRequestLocked()
}

// holdsNoRequestLocked is holdsNoRequest for a caller that holds the lock of the session.
func (s *Session) holdsNoRequestLocked() bool {
	s.reqMu.Lock()
	defer s.reqMu.Unlock()
	_, response := s.requestResponse()
	return response == nil
}

// storedFor returns what the storage of store holds for the session id.
func storedFor(t *testing.T, store *Store, id string) []byte {
	t.Helper()

	stored, err := store.Storage.GetWithContext(context.Background(), string(store.storageKey(id)))
	require.NoError(t, err)
	return stored
}

// A middleware that runs inside another links to it and lets go when it finishes, and the outer one
// is the current middleware of the request again.
func Test_Session_Middleware_NestedLinkIsBalanced(t *testing.T) {
	t.Parallel()
	p := startPair(t)
	c, outer, inner := p.c, p.outer, p.inner
	require.Same(t, outer, inner.outer)
	require.Equal(t, int32(1), outer.links.Load())
	require.Same(t, inner, c.Locals(middlewareContextKey))

	// Dropped instead of recycled, so that the test can look at it afterwards without a parallel
	// test having taken it from the pool in the meantime.
	inner.mark(lifeEscaped)
	releaseMiddleware(inner)
	require.True(t, inner.is(lifeReleased))
	require.Nil(t, inner.outer, "a finished middleware keeps no link, so one the request context still holds cannot lead to an object that went back to the pool")
	require.Nil(t, inner.Session)
	require.Zero(t, outer.links.Load(), "and the outer one is not counted as having it inside any more")
	require.Same(t, outer, c.Locals(middlewareContextKey), "the outer middleware is current again")
	require.False(t, outer.is(lifeReleased))

	releaseMiddleware(outer)
	require.Nil(t, c.Locals(middlewareContextKey))
}

// A middleware links only to one that serves its own request: a Middleware is recycled, and an
// abandoned handler can come back to it after another request has taken it over.
func Test_Session_Middleware_LinkOnlyWithinItsRequest(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	c1 := apptest.Ctx(t, app)
	c2 := apptest.Ctx(t, app)

	outer := startMiddleware(t, c1, NewStore(Config{Extractor: extractors.FromCookie("a_sid")}))

	require.False(t, outer.link(c2), "a middleware of another request is no outer one")
	require.Zero(t, outer.links.Load(), "and nothing was counted for the refusal")

	require.True(t, outer.link(c1))
	require.Equal(t, int32(1), outer.links.Load())
	outer.unlink()
	require.Zero(t, outer.links.Load())

	// Dropped instead of recycled, so that the test can look at it once it finished.
	outer.mark(lifeEscaped)
	releaseMiddleware(outer)
	require.True(t, outer.is(lifeReleased))
	require.False(t, outer.link(c1), "a middleware that finished is no outer one either")
	require.Zero(t, outer.links.Load())
}

// The shape of a request that a middleware abandoned, as sse does: the outer middleware finishes
// and its request is answered while the middleware inside it still runs. A walk from the inner
// middleware stops at the finished one, even if nothing but its flag says so. The outer one must
// not be recycled then, for the inner one holds it, and when the inner one finishes it must not
// hand the request an outer middleware that now belongs to somebody else.
func Test_Session_Middleware_OuterFinishesBeforeInner(t *testing.T) {
	t.Parallel()
	p := startPair(t)
	c1, outer, inner := p.c, p.outer, p.inner
	c2 := apptest.Ctx(t, p.app)

	// Nothing but its flag says that the outer middleware finished, and a walk stops there.
	require.Same(t, outer, p.storeA.FromContext(c1))
	outer.mark(lifeReleased)
	require.Nil(t, p.storeA.FromContext(c1), "the walk went on to a middleware that finished")
	require.Same(t, inner, p.storeB.FromContext(c1), "the middleware in front of it is still found")
	require.Nil(t, inner.find(func(m *Middleware) bool { return m == outer }), "even one a caller asks for by name")
	outer.unmark(lifeReleased)

	// The outer middleware finishes first.
	releaseMiddleware(outer)
	require.True(t, outer.is(lifeReleased), "an outer middleware with one still inside it is not recycled")
	require.Nil(t, c1.Locals(middlewareContextKey))
	require.Nil(t, p.storeA.FromContext(c1))
	require.Same(t, outer, inner.outer, "the inner middleware still holds what it was linked to")

	// Another request starts. Nothing the pool hands out is the old outer middleware.
	for range 100 {
		got := acquireMiddleware()
		require.NotSame(t, outer, got, "the outer middleware went back to the pool with a middleware inside it")
		releaseMiddleware(got)
	}
	other := startMiddleware(t, c2, p.storeA)
	require.NotSame(t, outer, other)
	otherSession := other.Session

	// The inner middleware finishes last.
	releaseMiddleware(inner)
	require.Nil(t, c1.Locals(middlewareContextKey), "the request got a finished middleware back")
	require.Empty(t, c1.Locals(sessionIDContextKey))
	require.Zero(t, outer.links.Load())

	require.Same(t, other, c2.Locals(middlewareContextKey), "the other request kept its own")
	require.Same(t, otherSession, other.Session)
	require.NotEmpty(t, other.Session.ID())
	releaseMiddleware(other)
}

// A timeout middleware around a session middleware, behind another one. The handlers inside the
// timeout outlive the answer. The outer middleware is not recycled while they may still use it, so
// the next request is served by another one before they finish. What the abandoned handler finds in
// the request afterwards may be the middleware of its own request, but must not be the session of
// the request that came after.
//
// The answer waits until the inner middleware has started, so that what the timeout leaves running
// is ordered after what it left behind: the race detector would otherwise trip over the shared
// request locals, which the timeout middleware leaves to the application.
func Test_Session_MultipleStores_AbandonedByTimeout(t *testing.T) {
	t.Parallel()
	type attempt struct {
		slowMayFinish chan struct{} // closed to let the abandoned handler finish
		innerStarted  chan struct{} // closed once the abandoned handler runs inside the inner middleware
		otherHolds    chan struct{} // closed once the next request has stored its secret
		abandonedDone chan struct{} // closed once the abandoned handler looked around
		seen          atomic.Value  // what the abandoned handler found after its inner middleware finished
		injected      atomic.Value  // what the next request found in its own session at the end
	}
	var current atomic.Pointer[attempt]

	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, _ := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})

	app := fiber.New()
	app.Use(handlerA)
	app.Get("/slow", timeout.New(func(c fiber.Ctx) error {
		att := current.Load()
		err := c.Next() // handlerB starts and finishes in here, after handlerA did
		if m := FromContext(c); m != nil {
			att.seen.Store(fmt.Sprint(m.Get("marker")))
			m.Set("injected", "from-the-abandoned-handler")
		} else {
			att.seen.Store("nil")
		}
		close(att.abandonedDone)
		return err
	}, timeout.Config{
		Timeout: 10 * time.Millisecond,
		OnTimeout: func(fiber.Ctx) error {
			select {
			case <-current.Load().innerStarted:
			case <-time.After(5 * time.Second):
			}
			return fiber.ErrServiceUnavailable
		},
	}), handlerB, func(c fiber.Ctx) error {
		att := current.Load()
		close(att.innerStarted)
		<-att.slowMayFinish
		return c.SendString("slow")
	})
	app.Get("/next", func(c fiber.Ctx) error {
		att := current.Load()
		storeA.FromContext(c).Set("marker", "secret-of-the-next-request")
		close(att.otherHolds)
		select {
		case <-att.abandonedDone:
		case <-time.After(5 * time.Second):
		}
		att.injected.Store(fmt.Sprint(storeA.FromContext(c).Get("injected")))
		return c.SendString("next")
	})

	for range 25 {
		att := &attempt{
			slowMayFinish: make(chan struct{}),
			innerStarted:  make(chan struct{}),
			otherHolds:    make(chan struct{}),
			abandonedDone: make(chan struct{}),
		}
		current.Store(att)

		apptest.Expect(t, app, "/slow", fiber.StatusServiceUnavailable)

		// The middleware of the answered request stays out of the pool, so the next request gets another one.
		nextDone := make(chan error, 1)
		go func() {
			_, _, err := doRequest(app, "/next", nil)
			nextDone <- err
		}()
		apptest.Recv(t, att.otherHolds, 5*time.Second)
		close(att.slowMayFinish)
		apptest.Recv(t, att.abandonedDone, 5*time.Second)
		require.NoError(t, apptest.Recv(t, nextDone, 5*time.Second))

		require.NotContains(t, att.seen.Load(), "secret-of-the-next-request", "the abandoned handler reached the session of the next request")
		require.Equal(t, "<nil>", att.injected.Load(), "the abandoned handler wrote into another request's session")
	}
}

// Finding a store's middleware, and telling that a session belongs to one, cost a request nothing:
// they walk the middleware without allocating.
//
// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_Session_Middleware_FindDoesNotAllocate(t *testing.T) {
	p := startPair(t)
	defer releaseMiddleware(p.outer)
	defer releaseMiddleware(p.inner)

	require.Same(t, p.inner, p.storeB.FromContext(p.c))
	require.Same(t, p.outer, p.storeA.FromContext(p.c))

	save := func(s *Session) func() {
		return func() {
			if err := s.Save(); err != nil {
				t.Error(err)
			}
		}
	}

	require.Zero(t, testing.AllocsPerRun(100, func() { _ = p.storeB.FromContext(p.c) }), "the innermost middleware")
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = p.storeA.FromContext(p.c) }), "the one it runs inside")
	require.Zero(t, testing.AllocsPerRun(100, save(p.inner.Session)), "the session of the innermost middleware")
	require.Zero(t, testing.AllocsPerRun(100, save(p.outer.Session)), "the session of the one it runs inside")
}

// A middleware that nothing refers to is recycled when it finishes, with its session reset, and the
// outer middleware is not held by the one that ran inside it any more.
//
// conventions:ignore parallel: it reads the middleware and the sessions after they went back to the
// shared pools, where a parallel test could take them in the meantime.
func Test_Session_ReleaseMiddleware_RecyclesWhatNothingRefersTo(t *testing.T) {
	p := startPair(t)
	outer, inner := p.outer, p.inner
	outerSession, innerSession := outer.Session, inner.Session
	require.NotEmpty(t, outerSession.ID())
	require.NotEmpty(t, innerSession.ID())

	releaseMiddleware(inner)
	require.False(t, inner.is(lifeReleased), "nothing refers to the inner middleware, so it is recycled")
	require.Nil(t, inner.Session)
	require.Empty(t, innerSession.ID(), "the inner session was reset and went back to the pool with its middleware")
	require.NotEmpty(t, outerSession.ID(), "and the outer one is still the outer middleware's")
	require.Zero(t, outer.links.Load())

	releaseMiddleware(outer)
	require.False(t, outer.is(lifeReleased), "nothing refers to the outer one either, since the inner one let go of it")
	require.Nil(t, outer.Session)
	require.Empty(t, outerSession.ID())
}

// A handler that a middleware abandoned, as sse does, can hold the middleware and the session of
// its request after the request is answered, the middleware in front of it finished and the next
// request started. Neither is recycled then, or the next request would find itself in the handler's
// hands. The context of the request is reused once the handlers that outlive it are done, so the
// session lets go of it: a goroutine that still holds the session neither writes to the response of
// that request nor runs its storage calls under its context, but can still save what it holds. And
// a lookup the handler began before the middleware finished must not find the middleware of the
// next request.
func Test_Session_ReleaseMiddleware_AbandonedRequestKeepsItsObjects(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer c.ForceRelease()

	store := NewStore()
	first := startMiddleware(t, c, store)
	sess := first.Session
	sess.Set("k", "v")
	held := FromContext(c) // the abandoned handler begins a lookup
	require.Same(t, first, held)

	c.Abandon()
	releaseMiddleware(first)
	require.True(t, first.is(lifeReleased), "the middleware is marked finished, so lookups stop returning it")
	require.Nil(t, first.Session)

	for range 100 {
		got := acquireMiddleware()
		require.NotSame(t, first, got, "the middleware of an abandoned request went back to the pool")
		releaseMiddleware(got)

		next := acquireSession()
		require.NotSame(t, sess, next, "the session of an abandoned request went back to the pool")
		releaseSession(next)
	}
	require.Equal(t, "v", sess.Get("k"), "and what the abandoned handler holds is left as it was")

	require.Nil(t, sess.requestCtx(), "the session still points at the context of its request")
	require.NoError(t, sess.Save())
	require.Empty(t, c.Response().Header.Peek(fiber.HeaderSetCookie), "a save by a goroutine that still holds the session wrote to the response")
	require.NotEmpty(t, storedFor(t, store, sess.ID()), "but the session was saved")

	second := startMiddleware(t, apptest.Ctx(t, app), store) // the next request starts
	second.Set("req", "secret-of-the-second-request")
	require.NotSame(t, first, second)

	found := held.find(func(m *Middleware) bool { return m.store == store })
	require.Nil(t, found, "the abandoned handler found a middleware after its request ended")
	require.Nil(t, store.FromContext(c))
	require.Equal(t, "secret-of-the-second-request", second.Get("req"))

	releaseMiddleware(second)
}

// The session of an abandoned request is cut off from its context while a goroutine that outlives
// the request may be reading it. Run with -race.
func Test_Session_ReleaseMiddleware_AbandonedSessionIsRaceFree(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore()

	for range 200 {
		c := app.AcquireCtx(&fasthttp.RequestCtx{})
		m := startMiddleware(t, c, store)
		sess := m.Session
		c.Abandon()

		var wg sync.WaitGroup
		wg.Go(func() { _ = sess.resolveContext() })
		wg.Go(func() { _ = m.requestContext() })
		releaseMiddleware(m)
		wg.Wait()

		c.ForceRelease()
	}
}

// Save and SaveWithContext read the context of the session under its lock, while the end of a
// request forgets it. Run with -race: it fails if either reads it bare. Whether the detector sees
// the bare read depends on how the goroutines interleave: on several CPUs it always does, on a
// single one it does in most runs, not in all.
func Test_Session_Save_EntryPointsReadTheContextUnderItsLock(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	c := apptest.Ctx(t, app)

	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	sess := mustGet(t, store, c)
	sess.Set("k", "v")

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			sess.detach()
			sess.mu.Lock()
			sess.ctx = c
			sess.mu.Unlock()
		}
	})

	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for range 200 {
				assert.NoError(t, sess.Save())
				assert.NoError(t, sess.SaveWithContext(context.Background()))
			}
		})
	}
	readers.Wait()
	close(stop)
	writer.Wait()
}

// contextSpyStorage records the context each save runs under.
type contextSpyStorage struct {
	fiber.Storage
	sets []context.Context
	mu   sync.Mutex
}

func (s *contextSpyStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	s.mu.Lock()
	s.sets = append(s.sets, ctx)
	s.mu.Unlock()
	return s.Storage.SetWithContext(ctx, key, val, exp)
}

// Save runs its storage call under the context of the request, SaveWithContext under the one it is
// given, and a session that belongs to no request falls back to the background context.
func Test_Session_Save_RunsUnderTheRequestContext(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	c := apptest.Ctx(t, app)

	spy := &contextSpyStorage{Storage: memory.New()}
	store := NewStore(Config{Storage: spy})
	sess := mustGet(t, store, c)
	sess.Set("k", "v")

	require.NoError(t, sess.Save())
	require.Len(t, spy.sets, 1)
	require.Same(t, c, spy.sets[0], "Save runs its storage call under the context of the request")

	type ctxKey struct{}
	given := context.WithValue(context.Background(), ctxKey{}, "given")
	require.NoError(t, sess.SaveWithContext(given))
	require.Len(t, spy.sets, 2)
	require.Same(t, given, spy.sets[1], "SaveWithContext runs it under the context it is given")

	byID, err := store.GetByID(context.Background(), sess.ID())
	require.NoError(t, err)
	defer byID.Release()
	byID.Set("k", "w")
	require.NoError(t, byID.Save())
	require.Len(t, spy.sets, 3)
	require.Equal(t, context.Background(), spy.sets[2], "a session without a request saves under the background context")
}

// The link, the restore and the release of the outer middleware run on different goroutines when a
// handler outlives its request. They must not race on the middleware: run with -race.
func Test_Session_Middleware_LinkProtocolIsRaceFree(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})

	for name, linkFirst := range map[string]bool{
		"linking while the outer middleware finishes": false,
		// The link is made first; from then on the goroutine and the release run on their own.
		"restoring while the outer middleware finishes": true,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			linked := make(chan bool, 1)
			for range 200 {
				c1 := app.AcquireCtx(&fasthttp.RequestCtx{})
				c2 := app.AcquireCtx(&fasthttp.RequestCtx{}) // restore writes the locals of the request it is given
				// outer runs on c1 and the goroutine is about to link to or restore it; it is
				// dropped instead of recycled, so that the test can look at it afterwards.
				outer := startMiddleware(t, c1, store)
				outer.mark(lifeEscaped)

				var wg sync.WaitGroup
				wg.Go(func() {
					ok := outer.link(c1)
					if linkFirst {
						linked <- ok
					}
					if ok {
						restoreMiddlewareContext(c2, outer)
						outer.unlink()
					}
				})
				if linkFirst {
					require.True(t, <-linked, "the outer middleware was still running")
				}
				releaseMiddleware(outer)
				wg.Wait()

				require.Zero(t, outer.links.Load(), "every link that was made was undone")
				app.ReleaseCtx(c1)
				app.ReleaseCtx(c2)
			}
		})
	}
}

// recoverBetween is a middleware that recovers the panic of whatever runs behind it and reports
// what it can still reach of the session middleware at that point.
func recoverBetween(storeA *Store, seen *atomic.Value) fiber.Handler {
	return func(c fiber.Ctx) error {
		var err error
		func() {
			defer func() {
				if recover() != nil {
					seen.Store(fmt.Sprintf("outer=%t current=%t", storeA.FromContext(c) != nil, FromContext(c) != nil))
					err = fiber.ErrInternalServerError
				}
			}()
			err = c.Next()
		}()
		return err
	}
}

// A handler panics behind an inner session middleware and a middleware that recovers it, in front
// of an outer session middleware. As it does without an outer one, the inner middleware stays the
// current one for the recovering middleware, which can still reach the outer one; the outer one
// then saves its session as usual and the inner one, unwound by the panic, does not save.
func Test_Session_MultipleStores_PanicInsideKeepsWhatItShould(t *testing.T) {
	t.Parallel()
	// With PassLocalsToContext the request context holds the inner middleware too; it is still the
	// recovering middleware's to reach through the request.
	for _, pass := range []bool{false, true} {
		t.Run(fmt.Sprintf("PassLocalsToContext=%t", pass), func(t *testing.T) {
			t.Parallel()
			recA, recB := storagetest.New(), storagetest.New()
			handlerA, storeA := NewWithStore(Config{Storage: recA, Extractor: extractors.FromCookie("a_sid")})
			handlerB, _ := NewWithStore(Config{Storage: recB, Extractor: extractors.FromCookie("b_sid")})

			var seen atomic.Value
			var inner atomic.Pointer[Middleware]
			app := fiber.New(fiber.Config{PassLocalsToContext: pass})
			app.Use(handlerA, recoverBetween(storeA, &seen), handlerB)
			app.Get("/", func(c fiber.Ctx) error {
				storeA.FromContext(c).Set("k", "v")
				inner.Store(FromContext(c))
				panic("boom")
			})

			apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
			require.Equal(t, "outer=true current=true", seen.Load())
			require.Len(t, recA.Calls(storagetest.OpSet), 1, "the outer middleware saved its session")
			require.Empty(t, recB.Calls(storagetest.OpSet), "the inner one was unwound before it could")

			// The outer middleware, which finished after it, ended the inner one's request.
			m := inner.Load()
			m.mu.RLock()
			sess := m.Session
			m.mu.RUnlock()
			require.NotNil(t, sess)
			require.True(t, sess.holdsNoRequest(), "the session of the inner middleware still writes to the request")
		})
	}
}

// panickingStorage is a storage that reads but panics when it is asked to save.
type panickingStorage struct {
	fiber.Storage
}

func (panickingStorage) SetWithContext(context.Context, string, []byte, time.Duration) error {
	panic("storage panic")
}

func (panickingStorage) Set(string, []byte, time.Duration) error {
	panic("storage panic")
}

// conventions:ignore parallel: it replaces the New of the shared middleware pool to count what the
// pool creates, and keeps the process on one P without garbage collection so that the pool hands
// back what it was given.
//
// The inner middleware is unwound by a panic and never comes back, with or without an outer one, so
// each request creates one. The outer middleware must not cost another: it is recycled, because
// nothing runs inside it any more. The panic can come from the rest of the stack, from the save of
// the inner middleware, or from its error handler.
func Test_Session_MultipleStores_PanicInsideRecyclesTheOuterMiddleware(t *testing.T) {
	prevGC := debug.SetGCPercent(-1)
	prevProcs := runtime.GOMAXPROCS(1)
	defer debug.SetGCPercent(prevGC)
	defer runtime.GOMAXPROCS(prevProcs)

	var created atomic.Int64
	oldNew := middlewarePool.New
	middlewarePool.New = func() any {
		created.Add(1)
		return &Middleware{}
	}
	defer func() { middlewarePool.New = oldNew }()

	for _, tc := range []struct {
		handler fiber.Handler
		name    string
		inner   Config
	}{
		{
			name:    "in the rest of the stack",
			inner:   Config{Extractor: extractors.FromCookie("b_sid")},
			handler: func(fiber.Ctx) error { panic("boom") },
		},
		{
			name:  "in the save of the inner middleware",
			inner: Config{Extractor: extractors.FromCookie("b_sid"), Storage: panickingStorage{Storage: memory.New()}},
			handler: func(c fiber.Ctx) error {
				FromContext(c).Set("k", "v")
				return nil
			},
		},
		{
			name: "in the error handler of the inner middleware",
			inner: Config{
				Extractor:    extractors.FromCookie("b_sid"),
				Storage:      storagetest.New().SetErr(storagetest.OpSet, "*", errors.New("storage unavailable")),
				ErrorHandler: func(fiber.Ctx, error) { panic("error handler") },
			},
			handler: func(c fiber.Ctx) error {
				FromContext(c).Set("k", "v")
				return nil
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
			handlerB, _ := NewWithStore(tc.inner)

			var seen atomic.Value
			app := fiber.New()
			app.Use(handlerA, recoverBetween(storeA, &seen), handlerB)
			app.Get("/", tc.handler)
			handler := app.Handler()

			do := func() int { return sessionTestSend(handler, fiber.MethodGet, "/").Response.StatusCode() }

			require.Equal(t, fiber.StatusInternalServerError, do(), "warm-up")
			created.Store(0)

			const requests = 100
			for range requests {
				require.Equal(t, fiber.StatusInternalServerError, do())
			}
			// One per request for the inner middleware. The race detector makes a pool drop some of
			// what it is given, so the outer one is allowed a quarter more, but not one more per
			// request.
			require.Less(t, created.Load(), int64(requests)*8/5,
				"the outer middleware was dropped instead of recycled: %d created for %d requests", created.Load(), requests)
		})
	}
}

// A panic that unwinds a middleware inside another skips its release, so the deferred call that
// ends the middleware does what the release would. Once a middleware finished it was released, and
// may already serve another request: the call leaves a finished one alone. One that was unwound
// gives the outer middleware its count back, once, and lets go of the request, and keeps its
// session; the way to the outer middleware stays, for whatever recovers the panic.
func Test_Session_Middleware_EndIfUnwoundUnlinksOnce(t *testing.T) {
	t.Parallel()
	p := startPair(t)
	c, outer, inner := p.c, p.outer, p.inner
	require.Equal(t, int32(1), outer.links.Load())

	finished := true
	inner.endIfUnwound(&finished, c)
	require.Equal(t, int32(1), outer.links.Load(), "the count of a finished middleware was given back")
	require.Same(t, outer, inner.linked)
	require.NotNil(t, inner.ctx, "a finished middleware was cut off")

	finished = false
	inner.endIfUnwound(&finished, c)
	require.Nil(t, inner.linked)
	require.Zero(t, outer.links.Load(), "the count of one that was unwound was not")
	require.Same(t, outer, inner.outer, "the way to the outer middleware stays")
	require.Nil(t, inner.ctx, "the middleware that was unwound still refers to the fiber.Ctx")
	require.NotNil(t, inner.Session, "its session was let go of")
	require.Nil(t, inner.Session.requestCtx(), "its session still refers to the fiber.Ctx")
	require.Same(t, c.RequestCtx(), inner.Session.rctx, "its session keeps the request itself until the request ends")
	require.False(t, inner.is(lifeReleased), "it stays reachable for whatever recovers the panic")
	require.True(t, inner.is(lifeUnwound), "and is marked unwound, for a context kept past the request")
	c.RequestCtx().ResetUserValues() // what fasthttp does when it resets the request: it closes the middleware
	require.Nil(t, inner.Session.rctx, "the end of the request did not cut the session off from it")

	inner.unlinkFromOuter()
	require.Zero(t, outer.links.Load(), "a second call finds nothing to undo")

	// A release after that does not undo the link again.
	inner.mark(lifeEscaped)
	releaseMiddleware(inner)
	require.Zero(t, outer.links.Load())

	releaseMiddleware(outer)
}

// reach returns the request the session writes to in place of its fiber.Ctx, if any, and whether
// its request is over for it.
func (s *Session) reach() (*fasthttp.RequestCtx, bool) {
	s.reqMu.Lock()
	defer s.reqMu.Unlock()
	return s.rctx, s.requestOver
}

// parkedStorage is a storage whose deletes wait until release is closed, as a storage under load
// makes them, and says when the first one arrived.
type parkedStorage struct {
	fiber.Storage
	entered, release chan struct{}
	once             sync.Once
}

func (s *parkedStorage) DeleteWithContext(ctx context.Context, key string) error {
	s.once.Do(func() { close(s.entered) })
	<-s.release
	return s.Storage.DeleteWithContext(ctx, key)
}

// A handler leaves a goroutine behind that ends the session, and panics. The call is inside the
// storage, holding the lock of its middleware or of its session, when the request ends, so neither
// can be cut off from the request under its lock; the session is told all the same. What the call
// writes once the storage answers, the cookie that expires the session, goes to no request: the one
// it was for is over, and fasthttp has reset it for the next one of the connection.
func Test_Session_Middleware_CallThatOutlivesAnUnwoundRequestWritesToNoOtherRequest(t *testing.T) {
	t.Parallel()
	for name, call := range map[string]func(m *Middleware) error{
		"Destroy, which holds the lock of the middleware":  (*Middleware).Destroy,
		"Reset, which holds the lock of the middleware":    (*Middleware).Reset,
		"Destroy of the session, which holds its own lock": func(m *Middleware) error { return m.Session.Destroy() },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			storage := &parkedStorage{Storage: memory.New(), entered: make(chan struct{}), release: make(chan struct{})}
			unwound, done := make(chan struct{}), make(chan error, 1)

			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				defer func() {
					if recover() == nil {
						return
					}
					c.Status(fiber.StatusInternalServerError)
					// The call starts now that the middleware was unwound, and is in the storage when the request ends.
					close(unwound)
					apptest.Recv(t, storage.entered, 5*time.Second, "the call never reached the storage")
				}()
				return c.Next()
			})
			app.Use(New(Config{Storage: storage}))
			app.Get("/", func(c fiber.Ctx) error {
				m := FromContext(c)
				go func() {
					<-unwound
					done <- call(m)
				}()
				panic("boom")
			})

			rctx := sessionTestRequest(t, app.Handler(), fiber.MethodGet, "/", fiber.StatusInternalServerError)
			// What fasthttp does once the response is sent: it resets the request, which ends it for
			// the session, and the response, and reads the next request of the connection into both.
			rctx.Request.Reset()
			rctx.Response.Reset()

			close(storage.release)
			require.NoError(t, apptest.Recv(t, done, 5*time.Second, "the call never returned"))
			require.Empty(t, rctx.Response.Header.Peek(fiber.HeaderSetCookie), "the call wrote its cookie into the response of the next request")
		})
	}
}

// The session of a middleware that ends early is told without the lock of the middleware or its
// own, which a call can hold across the storage: from the unwinding on it writes to the request
// itself, and at the end of the request to nothing. Forgetting the fiber.Ctx, which needs the
// locks, follows when they are free.
func Test_Session_Middleware_DetachSoonTellsTheSessionWithoutTheLocks(t *testing.T) {
	t.Parallel()
	for name, lockOf := range map[string]func(m *Middleware) sync.Locker{
		"the lock of the middleware is held": func(m *Middleware) sync.Locker { return &m.mu },
		"the lock of the session is held":    func(m *Middleware) sync.Locker { return &m.Session.mu },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, fiber.New())
			m := startMiddleware(t, c, NewStore())
			sess, held := m.Session, lockOf(m)

			held.Lock()
			m.detachSoon(c.RequestCtx())
			rctx, over := sess.reach()
			require.Same(t, c.RequestCtx(), rctx, "the session was not told to write to the request itself")
			require.False(t, over)

			m.detachSoon(nil)
			rctx, over = sess.reach()
			require.Nil(t, rctx)
			require.True(t, over, "the session was not told that its request is over")
			held.Unlock()

			requireDetached(t, m)
		})
	}
}

// From the moment a panic unwinds its middleware, a session writes to the request itself and no
// longer through the fiber.Ctx: that one goes back to the pool when the handlers return, and serves
// another request while a call that holds the session across the storage still refers to it. Once
// its request is over the session writes to nothing, and a request that reaches it after that, as
// the unwinding does when the session was busy, is not taken up.
func Test_Session_LeaveCtx(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	c, other := apptest.Ctx(t, app), apptest.Ctx(t, app)
	sess := mustGet(t, NewStore(), c)
	setCookie := func(c fiber.Ctx) []byte { return c.Response().Header.Peek(fiber.HeaderSetCookie) }
	// write does what the calls that touch the request do, as a caller that holds the session.
	write := func() {
		sess.setSession(time.Minute)
		sess.delSession()
		sess.refresh()
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.leaveCtx(c.RequestCtx())
	sess.ctx = other // the fiber.Ctx of the session, once it serves another request
	write()
	require.NotEmpty(t, setCookie(c), "the session did not write to its own request")
	require.Empty(t, setCookie(other), "the session wrote its cookie through a fiber.Ctx that is no longer its own")
	require.Nil(t, requestIDs(other), "the session cached its new ID on another request")

	c.Response().Header.Del(fiber.HeaderSetCookie)
	sess.leaveCtx(nil)
	sess.leaveCtx(c.RequestCtx())
	rctx, over := sess.reach()
	require.Nil(t, rctx, "a session that let go of its request took up one that reached it late")
	require.True(t, over)
	write()
	require.Empty(t, setCookie(c), "the session wrote to a request that was over for it")
	require.Empty(t, setCookie(other))
	require.Nil(t, requestIDs(other))
}

// parkedCtx is a fiber.Ctx whose response and locals are handed out only once release is closed:
// whatever writes to the request through it is under way until then, and says so.
type parkedCtx struct {
	fiber.Ctx
	entered, release chan struct{}
	once             *sync.Once
}

func (c parkedCtx) park() {
	c.once.Do(func() { close(c.entered) })
	<-c.release
}

func (c parkedCtx) Response() *fasthttp.Response {
	c.park()
	return c.Ctx.Response()
}

func (c parkedCtx) Locals(key any, value ...any) any {
	c.park()
	return c.Ctx.Locals(key, value...)
}

// The end of the request waits for a write of the session to it that is under way, and for nothing
// else: when it returns none is, and none follows. The cookie, the cookie that expires it and the
// ID a Regenerate caches on the request are such writes.
func Test_Session_LeaveCtx_WaitsForAWriteUnderWay(t *testing.T) {
	t.Parallel()
	for name, write := range map[string]func(sess *Session){
		"the cookie":              func(sess *Session) { sess.setSession(time.Minute) },
		"the cookie that expires": (*Session).delSession,
		"the ID cached for it":    (*Session).refresh,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := apptest.Ctx(t, fiber.New())
			sess := mustGet(t, NewStore(), c)
			parked := parkedCtx{Ctx: c, entered: make(chan struct{}), release: make(chan struct{}), once: new(sync.Once)}

			sess.mu.Lock()
			defer sess.mu.Unlock()
			sess.ctx = parked
			written, left := make(chan struct{}), make(chan struct{})
			go func() {
				defer close(written)
				write(sess)
			}()
			apptest.Recv(t, parked.entered, 5*time.Second, "the write never reached the request")
			go func() {
				defer close(left)
				sess.leaveCtx(nil)
			}()
			select {
			case <-left:
				close(parked.release)
				t.Fatal("the request was over for the session while it was still writing to it")
			case <-time.After(50 * time.Millisecond):
			}
			close(parked.release)
			apptest.Recv(t, written, 5*time.Second, "the write never finished")
			apptest.Recv(t, left, 5*time.Second, "the end of the request never reached the session")
			require.True(t, sess.holdsNoRequestLocked())
		})
	}
}

// panicAtCtx is a custom context that panics at one chosen point of the life of a session
// middleware, as a Ctx of an application could: as the middleware starts (publishing it to the
// request) and as it ends (clearing the request locals, which it does holding its lock; replacing
// the Go context; asking whether the request was abandoned; making the outer middleware current
// again).
type panicAtCtx struct {
	point *atomic.Value // string; unset: nowhere
	fiber.DefaultCtx
}

func (c *panicAtCtx) Locals(key any, value ...any) any {
	if key == middlewareContextKey && len(value) == 1 {
		storing := value[0] != nil
		switch c.point.Load() {
		case "publishing the inner middleware", "restoring the outer middleware":
			if storing {
				panic("custom context: Locals panicked while storing the middleware")
			}
		case "clearing the locals":
			if !storing {
				panic("custom context: Locals panicked while clearing")
			}
		default:
		}
	}
	return c.DefaultCtx.Locals(key, value...)
}

func (c *panicAtCtx) SetContext(ctx context.Context) {
	if c.point.Load() == "replacing the Go context" {
		panic("custom context: SetContext panicked")
	}
	c.DefaultCtx.SetContext(ctx)
}

func (c *panicAtCtx) IsAbandoned() bool {
	if c.point.Load() == "asking whether the request was abandoned" {
		panic("custom context: IsAbandoned panicked")
	}
	return c.DefaultCtx.IsAbandoned()
}

// A panic in the start or in the release of a nested middleware, wherever it comes from, goes on as
// it does for a middleware without an outer one: nothing waits for the lock the release holds, and
// the outer middleware's count is given back, or was never taken, so that it is recycled when it
// finishes.
func Test_Session_MultipleStores_PanicInReleaseGoesOn(t *testing.T) {
	t.Parallel()
	for _, point := range []string{
		"publishing the inner middleware",
		"clearing the locals",
		"replacing the Go context",
		"asking whether the request was abandoned",
		"restoring the outer middleware",
	} {
		for _, nested := range []bool{false, true} {
			for _, pass := range []bool{false, true} {
				// Without an outer middleware there is none to restore or to run inside, and
				// without PassLocalsToContext the release leaves the Go context alone.
				if (!nested && (point == "restoring the outer middleware" || point == "publishing the inner middleware")) ||
					(!pass && point == "replacing the Go context") {
					continue
				}
				t.Run(fmt.Sprintf("%s nested=%t pass=%t", point, nested, pass), func(t *testing.T) {
					t.Parallel()
					armed := &atomic.Value{}
					app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
						return &panicAtCtx{DefaultCtx: *fiber.NewDefaultCtx(app), point: armed}
					}, fiber.Config{PassLocalsToContext: pass})
					handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
					handlerB, _ := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
					if nested {
						app.Use(handlerA)
					}
					var outer atomic.Pointer[Middleware]
					app.Use(func(c fiber.Ctx) error { // between the two session middleware
						if nested {
							outer.Store(storeA.FromContext(c))
						}
						if point == "publishing the inner middleware" {
							armed.Store(point) // the start of the middleware that runs last panics
						}
						return c.Next()
					}, handlerB)
					app.Get("/", func(fiber.Ctx) error {
						if point != "publishing the inner middleware" {
							armed.Store(point) // the release of the middleware that runs last panics
						}
						return nil
					})
					handler := app.Handler()

					recovered := make(chan any, 1)
					go func() {
						defer func() { recovered <- recover() }()
						sessionTestSend(handler, fiber.MethodGet, "/")
					}()

					got := apptest.Recv(t, recovered, 5*time.Second, "the request hung: the panic in the release was swallowed by a deadlock")
					require.NotNil(t, got, "the panic did not come out of the handler")
					if nested {
						require.NotNil(t, outer.Load())
						require.Zero(t, outer.Load().links.Load(), "the outer middleware still counts the one that panicked")
					}
				})
			}
		}
	}
}

// observeRestoreCtx is a custom context that tells when a session middleware makes the one it ran
// inside the current middleware of the request again.
type observeRestoreCtx struct {
	onRestore func()
	fiber.DefaultCtx
}

func (c *observeRestoreCtx) Locals(key any, value ...any) any {
	if c.onRestore != nil && key == middlewareContextKey && len(value) == 1 && value[0] != nil {
		c.onRestore()
	}
	return c.DefaultCtx.Locals(key, value...)
}

// The outer middleware stays counted until the inner one is done with it, including the moment it
// makes it current again: that is the moment a handler that a timeout middleware abandoned could
// otherwise find it finished and recycled.
func Test_Session_Middleware_OuterStaysLinkedWhileItIsRestored(t *testing.T) {
	t.Parallel()
	var (
		outer atomic.Pointer[Middleware]
		armed atomic.Bool
		seen  atomic.Int32
		after atomic.Int32
	)
	seen.Store(-1)
	after.Store(-1)

	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &observeRestoreCtx{DefaultCtx: *fiber.NewDefaultCtx(app), onRestore: func() {
			if armed.Load() {
				seen.Store(outer.Load().links.Load())
			}
		}}
	})
	handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
	handlerB, _ := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
	app.Use(handlerA, func(c fiber.Ctx) error {
		err := c.Next() // the inner middleware starts and finishes in here
		// The outer middleware still runs, so it is still the request's own.
		after.Store(storeA.FromContext(c).links.Load())
		return err
	}, handlerB)
	app.Get("/", func(c fiber.Ctx) error {
		outer.Store(storeA.FromContext(c))
		armed.Store(true)
		return nil
	})

	apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Equal(t, int32(1), seen.Load(), "the outer middleware was given back before it was made current again")
	require.Zero(t, after.Load(), "and it was given back by the time the inner middleware was done")
}

// A handler that outlives its request calls Destroy, Reset or Regenerate while the request ends:
// the read of the middleware's context in those methods must not race with the release. Run with
// -race.
func Test_Session_Middleware_DestroyResetRegenerateRaceTheRelease(t *testing.T) {
	t.Parallel()
	for name, call := range map[string]func(m *Middleware) error{
		"Destroy":    (*Middleware).Destroy,
		"Reset":      (*Middleware).Reset,
		"Regenerate": (*Middleware).Regenerate,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
			for range 200 {
				c := app.AcquireCtx(&fasthttp.RequestCtx{})
				m := startMiddleware(t, c, store)
				m.mark(lifeEscaped) // dropped, not recycled: the goroutine below still holds it

				var wg sync.WaitGroup
				wg.Go(func() {
					// A finished middleware has no session: the call panics.
					defer func() { recover() }() //nolint:errcheck,revive // only the race matters here, not how the call ends
					if err := call(m); err != nil {
						t.Error(err)
					}
				})
				releaseMiddleware(m)
				wg.Wait()

				app.ReleaseCtx(c)
			}
		})
	}
}

// A panic that unwinds the session middleware skips its release. The middleware and its session let
// go of the fiber.Ctx, which whatever recovers the panic answers and hands back to the pool, and
// keep the request itself until fasthttp resets it: the recovering middleware still sets cookies
// through them, and a save by a goroutine of the handler that still holds the session afterwards
// writes its record, and no cookie, into no request. A context kept past the request does not find
// the middleware. The middleware does not save the session.
func Test_Session_Middleware_UnwoundByAPanicLetsGoOfTheRequest(t *testing.T) {
	t.Parallel()
	t.Run("recovered in front of the middleware", func(t *testing.T) {
		t.Parallel()
		var (
			held  atomic.Pointer[Middleware]
			kept  atomic.Value // context.Context
			after atomic.Value // string: what the middleware in front of the recovering one could do
		)
		store := NewStore()
		app := fiber.New(fiber.Config{PassLocalsToContext: true})
		app.Use(func(c fiber.Ctx) error {
			err := c.Next()
			// The panic was recovered behind this point: the middleware is still the request's to
			// reach, and a Destroy from here deletes the record and expires the cookie, as before
			// the panic.
			after.Store("unreachable")
			if m := FromContext(c); m != nil {
				after.Store(fmt.Sprintf("reached|destroy=%v", m.Destroy()))
			}
			return err
		}, recoverer.New(), New(Config{Store: store}))
		app.Get("/", func(c fiber.Ctx) error {
			m := FromContext(c)
			m.Set("k", "v")
			held.Store(m)
			kept.Store(c.Context())
			panic("boom")
		})

		resp, _ := sendExpecting(t, app, "/", nil, fiber.StatusInternalServerError)
		require.Equal(t, "reached|destroy=<nil>", after.Load(), "the recovering side could not reach the middleware")
		cookies := cookiesByName(resp)
		require.Len(t, cookies, 1, "the Destroy of the recovering side set no cookie, or the middleware set one")
		require.NotNil(t, cookies["session_id"])
		require.Negative(t, cookies["session_id"].MaxAge, "the Destroy of the recovering side did not expire the cookie")

		require.Nil(t, FromContext(kept.Load()), "a context kept past the request still finds the middleware the panic unwound")
		m := held.Load()
		m.mu.RLock()
		ctx, sess := m.ctx, m.Session
		m.mu.RUnlock()
		require.Nil(t, ctx, "the middleware still refers to the fiber.Ctx")
		require.NotNil(t, sess, "its session was recycled")
		require.Nil(t, sess.requestCtx(), "the session still refers to the fiber.Ctx")
		// fasthttp resets the request once the response is written, and closes the middleware then:
		// the session has nothing to write to from then on.
		require.Eventually(t, sess.holdsNoRequest, 5*time.Second, 5*time.Millisecond, "the session still writes to the request after it ended")

		require.Nil(t, storedFor(t, store, sess.ID()), "the middleware saved the session of a request that panicked")
		require.NoError(t, sess.Save(), "a save by the handler's own goroutine")
		require.NotEmpty(t, storedFor(t, store, sess.ID()), "writes the record")
	})

	t.Run("a save from the recovering side saves the session and sets its cookie", func(t *testing.T) {
		t.Parallel()
		var (
			id    atomic.Value          // string
			saved atomic.Pointer[error] // the result of the save
		)
		store := NewStore()
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			err := c.Next()
			// The middleware does not save the session of a request that panicked, and is not in
			// charge of it any more: a Save from here does what a Save does for a session that no
			// middleware manages, where it would do nothing while the middleware was.
			if m := FromContext(c); m != nil {
				saveErr := m.Session.Save()
				saved.Store(&saveErr)
			}
			return err
		}, recoverer.New(), New(Config{Store: store}))
		app.Get("/", func(c fiber.Ctx) error {
			m := FromContext(c)
			m.Set("k", "v")
			id.Store(m.ID())
			panic("boom")
		})

		resp, _ := sendExpecting(t, app, "/", nil, fiber.StatusInternalServerError)
		result := saved.Load()
		require.NotNil(t, result, "the recovering side could not reach the middleware")
		require.NoError(t, *result)

		cookie := cookiesByName(resp)["session_id"]
		require.NotNil(t, cookie, "the save of the recovering side set no cookie")
		require.Equal(t, id.Load(), cookie.Value)
		require.NotEmpty(t, storedFor(t, store, cookie.Value), "the save of the recovering side wrote no record")
	})

	t.Run("two middleware unwound by one panic", func(t *testing.T) {
		t.Parallel()
		var held [2]atomic.Pointer[Middleware]
		handlerA, storeA := NewWithStore(Config{Extractor: extractors.FromCookie("a_sid")})
		handlerB, storeB := NewWithStore(Config{Extractor: extractors.FromCookie("b_sid")})
		app := fiber.New()
		app.Use(recoverer.New(), handlerA, handlerB)
		app.Get("/", func(c fiber.Ctx) error {
			held[0].Store(storeA.FromContext(c))
			held[1].Store(storeB.FromContext(c))
			panic("boom")
		})

		apptest.Expect(t, app, "/", fiber.StatusInternalServerError)

		// fasthttp resets the request once the response is written, and closes every middleware the
		// panic unwound then, the outer one and the inner one.
		for i := range held {
			m := held[i].Load()
			require.NotNil(t, m)
			m.mu.RLock()
			sess := m.Session
			m.mu.RUnlock()
			require.NotNil(t, sess)
			require.Eventually(t, sess.holdsNoRequest, 5*time.Second, 5*time.Millisecond, "the session of middleware %d still writes to the request after it ended", i)
		}
	})

	t.Run("timed out, then panics", func(t *testing.T) {
		t.Parallel()
		var (
			held    = make(chan *Middleware, 1)
			release = make(chan struct{})
			done    = make(chan struct{})
		)
		app := fiber.New()
		app.Use(timeout.New(func(c fiber.Ctx) error { return c.Next() }, timeout.Config{Timeout: 10 * time.Millisecond}), New())
		app.Get("/slow", func(c fiber.Ctx) error {
			defer close(done)
			held <- FromContext(c)
			<-release
			panic("boom")
		})

		resp := apptest.Do(t, app, "/slow", fiber.TestConfig{Timeout: 5 * time.Second})
		require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

		m := apptest.Recv(t, held, 5*time.Second)
		close(release)
		apptest.Recv(t, done, 5*time.Second)
		requireDetached(t, m)
	})
}

// A custom Ctx that panics while the release clears the locals leaves the middleware in the request
// for whatever recovers the panic, and the lock of the middleware free: the recovering middleware
// can still ask it for the session.
func Test_Session_Middleware_ReleasePanicLeavesTheLockFree(t *testing.T) {
	t.Parallel()
	armed := &atomic.Value{}
	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &panicAtCtx{DefaultCtx: *fiber.NewDefaultCtx(app), point: armed}
	})
	keys := make(chan []any, 1)
	app.Use(func(c fiber.Ctx) error {
		defer func() {
			if recover() == nil {
				return
			}
			if m := FromContext(c); m != nil {
				go func() { keys <- m.Keys() }()
			}
		}()
		return c.Next()
	}, New())
	app.Get("/", func(c fiber.Ctx) error {
		FromContext(c).Set("k", "v")
		armed.Store("clearing the locals")
		return nil
	})

	apptest.Do(t, app, "/")
	got := apptest.Recv(t, keys, 2*time.Second, "the recovering middleware could not read the session: the lock of the middleware is held")
	require.Equal(t, []any{"k"}, got, "the recovering middleware could not read the session: the release left the lock held")
}
