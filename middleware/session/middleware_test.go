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
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/clocktest"
	"github.com/gofiber/fiber/v3/internal/loggertest"
	"github.com/gofiber/fiber/v3/internal/storagetest"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// sessionTestOption changes the request that sessionTestRequest sends.
type sessionTestOption func(*fasthttp.RequestCtx)

// sessionTestCookie makes the request carry the cookie name=value.
func sessionTestCookie(name, value string) sessionTestOption {
	return func(ctx *fasthttp.RequestCtx) { ctx.Request.Header.SetCookie(name, value) }
}

// sessionTestRequest runs one request through h on a fresh fasthttp context,
// requires the response to have the given status and returns the context to
// inspect it. The request URI is left unset when uri is empty.
func sessionTestRequest(t *testing.T, h fasthttp.RequestHandler, method, uri string, status int, opts ...sessionTestOption) *fasthttp.RequestCtx {
	t.Helper()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	if uri != "" {
		ctx.Request.SetRequestURI(uri)
	}
	for _, opt := range opts {
		opt(ctx)
	}
	h(ctx)
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
	canceledCtx := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}

	t.Run("DestroyWithContext propagates canceled context and marks destroyed", func(t *testing.T) {
		t.Parallel()
		m := &Middleware{Session: newSessionWithStorage(t, &storagetest.Memory{HonorContext: true})}
		require.ErrorIs(t, m.DestroyWithContext(canceledCtx()), context.Canceled)
		require.True(t, m.isDestroyed)
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

	m := <-held
	m.mu.RLock()
	sess := m.Session
	m.mu.RUnlock()
	require.NotNil(t, sess, "the running handler's middleware was reset")
	id := sess.ID()
	require.NotEmpty(t, id, "the running handler's session was reset")

	type result struct {
		err  error
		body string
	}
	results := make(chan result, others)
	var wg sync.WaitGroup
	for range others {
		wg.Go(func() {
			resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/me", http.NoBody), fiber.TestConfig{Timeout: 0})
			if err != nil {
				results <- result{err: err}
				return
			}
			body, err := io.ReadAll(resp.Body)
			results <- result{err: err, body: string(body)}
		})
	}
	for range others {
		<-entered
	}

	close(release)
	<-finished
	close(proceed)
	wg.Wait()
	close(results)
	for r := range results {
		require.NoError(t, r.err)
		require.Equal(t, "<nil>", r.body, "the timed-out handler wrote to another request's session")
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
	<-finished
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
