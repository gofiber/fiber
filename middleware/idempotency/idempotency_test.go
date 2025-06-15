package idempotency

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// go test -run Test_Idempotency
func Test_Idempotency(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(func(c fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}

		isMethodSafe := fiber.IsMethodSafe(c.Method())
		isIdempotent := IsFromCache(c) || WasPutToCache(c)
		hasReqHeader := c.Get("X-Idempotency-Key") != ""

		if isMethodSafe {
			if isIdempotent {
				return errors.New("request with safe HTTP method should not be idempotent")
			}
		} else {
			// Unsafe
			if hasReqHeader {
				if !isIdempotent {
					return errors.New("request with unsafe HTTP method should be idempotent if X-Idempotency-Key request header is set")
				}
			} else if isIdempotent {
				return errors.New("request with unsafe HTTP method should not be idempotent if X-Idempotency-Key request header is not set")
			}
		}

		return nil
	})

	// Needs to be at least a second as the memory storage doesn't support shorter durations.
	const lifetime = 2 * time.Second

	app.Use(New(Config{
		Lifetime: lifetime,
	}))

	nextCount := func() func() int {
		var count int32
		return func() int {
			return int(atomic.AddInt32(&count, 1))
		}
	}()

	app.Add([]string{
		fiber.MethodGet,
		fiber.MethodPost,
	}, "/", func(c fiber.Ctx) error {
		return c.SendString(strconv.Itoa(nextCount()))
	})

	app.Post("/slow", func(c fiber.Ctx) error {
		time.Sleep(3 * lifetime)

		return c.SendString(strconv.Itoa(nextCount()))
	})

	doReq := func(method, route, idempotencyKey string) string {
		req := httptest.NewRequest(method, route, nil)
		if idempotencyKey != "" {
			req.Header.Set("X-Idempotency-Key", idempotencyKey)
		}
		resp, err := app.Test(req, fiber.TestConfig{
			Timeout:       15 * time.Second,
			FailOnTimeout: true,
		})
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode, string(body))
		return string(body)
	}

	require.Equal(t, "1", doReq(fiber.MethodGet, "/", ""))
	require.Equal(t, "2", doReq(fiber.MethodGet, "/", ""))

	require.Equal(t, "3", doReq(fiber.MethodPost, "/", ""))
	require.Equal(t, "4", doReq(fiber.MethodPost, "/", ""))

	require.Equal(t, "5", doReq(fiber.MethodGet, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "6", doReq(fiber.MethodGet, "/", "00000000-0000-0000-0000-000000000000"))

	require.Equal(t, "7", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "7", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "8", doReq(fiber.MethodPost, "/", ""))
	require.Equal(t, "9", doReq(fiber.MethodPost, "/", "11111111-1111-1111-1111-111111111111"))

	require.Equal(t, "7", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	time.Sleep(4 * lifetime)
	require.Equal(t, "10", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "10", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))

	// Test raciness
	{
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.Equal(t, "11", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
			}()
		}
		wg.Wait()
		require.Equal(t, "11", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
	}
	time.Sleep(3 * lifetime)
	require.Equal(t, "12", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
}

// go test -v -run=^$ -bench=Benchmark_Idempotency -benchmem -count=4
func Benchmark_Idempotency(b *testing.B) {
	app := fiber.New()

	// Needs to be at least a second as the memory storage doesn't support shorter durations.
	const lifetime = 1 * time.Second

	app.Use(New(Config{
		Lifetime: lifetime,
	}))

	app.Post("/", func(_ fiber.Ctx) error {
		return nil
	})

	h := app.Handler()

	b.Run("hit", func(b *testing.B) {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(fiber.MethodPost)
		c.Request.SetRequestURI("/")
		c.Request.Header.Set("X-Idempotency-Key", "00000000-0000-0000-0000-000000000000")

		b.ReportAllocs()
		for b.Loop() {
			h(c)
		}
	})

	b.Run("skip", func(b *testing.B) {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(fiber.MethodPost)
		c.Request.SetRequestURI("/")

		b.ReportAllocs()
		for b.Loop() {
			h(c)
		}
	})
}

// ---------- additional tests (moved from config_extra_test.go and idempotency_additional_test.go) ----------

const validKey = "00000000-0000-0000-0000-000000000000"

func Test_configDefault_defaults(t *testing.T) {
	t.Parallel()

	cfg := configDefault()
	require.NotNil(t, cfg.Lock)
	require.NotNil(t, cfg.Storage)
	require.Equal(t, ConfigDefault.Lifetime, cfg.Lifetime)
	require.Equal(t, ConfigDefault.KeyHeader, cfg.KeyHeader)
	require.Nil(t, cfg.KeepResponseHeaders)

	app := fiber.New()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx := app.AcquireCtx(fctx)
	assert.True(t, cfg.Next(ctx))
	app.ReleaseCtx(ctx)

	fctx = &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodPost)
	ctx = app.AcquireCtx(fctx)
	assert.False(t, cfg.Next(ctx))
	app.ReleaseCtx(ctx)

	assert.NoError(t, cfg.KeyHeaderValidate(validKey))
	assert.Error(t, cfg.KeyHeaderValidate("short"))
}

func Test_configDefault_override(t *testing.T) {
	t.Parallel()

	l := &stubLock{}
	s := &stubStorage{}

	cfg := configDefault(Config{
		Lifetime:            42 * time.Second,
		KeyHeader:           "Foo",
		KeepResponseHeaders: []string{},
		Lock:                l,
		Storage:             s,
	})

	require.Equal(t, 42*time.Second, cfg.Lifetime)
	require.Equal(t, "Foo", cfg.KeyHeader)
	require.Nil(t, cfg.KeepResponseHeaders)
	require.Equal(t, l, cfg.Lock)
	require.Equal(t, s, cfg.Storage)
	require.NotNil(t, cfg.Next)
	require.NotNil(t, cfg.KeyHeaderValidate)
}

// helper to perform request
func do(app *fiber.App, req *http.Request) (*http.Response, string) {
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		panic(err)
	}
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func Test_New_NextSkip(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	var count int

	app.Use(New(Config{Next: func(c fiber.Ctx) bool { return true }}))

	app.Post("/", func(c fiber.Ctx) error {
		count++
		return c.SendString(fmt.Sprintf("%d", count))
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	_, body1 := do(app, req)

	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	req2.Header.Set(ConfigDefault.KeyHeader, validKey)
	_, body2 := do(app, req2)

	require.Equal(t, "1", body1)
	require.Equal(t, "2", body2)
}

func Test_New_InvalidKey(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Post("/", func(c fiber.Ctx) error { return nil })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, "bad")
	resp, body := do(app, req)

	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, body, "invalid length")
}

func Test_New_StorageGetError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{getErr: errors.New("boom")}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)

	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, body, "failed to write cached response at fastpath")
}

func Test_New_UnmarshalError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{data: map[string][]byte{validKey: []byte("bad")}}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)

	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, body, "failed to write cached response at fastpath")
}

func Test_New_StoreRetrieve_FilterHeaders(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	app.Use(New(Config{
		Storage:             s,
		Lock:                &stubLock{},
		KeepResponseHeaders: []string{"Foo"},
	}))

	var count int
	app.Post("/", func(c fiber.Ctx) error {
		count++
		c.Set("Foo", "foo")
		c.Set("Bar", "bar")
		return c.SendString(fmt.Sprintf("resp%d", count))
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, "resp1", body)
	require.Equal(t, "foo", resp.Header.Get("Foo"))
	require.Equal(t, "bar", resp.Header.Get("Bar"))

	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	req2.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp2, body2 := do(app, req2)
	require.Equal(t, "resp1", body2)
	require.Equal(t, "foo", resp2.Header.Get("Foo"))
	require.Empty(t, resp2.Header.Get("Bar"))
	require.Equal(t, 1, count)
	require.Equal(t, 1, s.setCount)
}

func Test_New_HandlerError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(c fiber.Ctx) error { return errors.New("boom") })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "boom", body)
	require.Equal(t, 0, s.setCount)

	resp2, body2 := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp2.StatusCode)
	require.Equal(t, "boom", body2)
	require.Equal(t, 0, s.setCount)
}

func Test_New_LockError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	l := &stubLock{lockErr: errors.New("fail")}
	app.Use(New(Config{Lock: l, Storage: &stubStorage{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, body, "failed to lock")
}

func Test_New_StorageSetError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{setErr: errors.New("nope")}
	app.Use(New(Config{Storage: s, Lock: &stubLock{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, body, "failed to save response")
}

func Test_New_UnlockError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	l := &stubLock{unlockErr: errors.New("u")}
	app.Use(New(Config{Lock: l, Storage: &stubStorage{}}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "ok", body)
}

func Test_New_SecondPassReadError(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	s := &stubStorage{}
	l := &stubLock{afterLock: func() { s.getErr = errors.New("g") }}
	app.Use(New(Config{Lock: l, Storage: s}))
	app.Post("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(ConfigDefault.KeyHeader, validKey)
	resp, body := do(app, req)
	require.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, body, "failed to write cached response while locked")
}
