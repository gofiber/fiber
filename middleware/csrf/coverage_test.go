package csrf

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

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
func Test_CSRF_validateExtractorSecurity_NilConfig(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		validateExtractorSecurity(nil)
	})
}

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
