//nolint:bodyclose // Much easier to just ignore memory leaks in tests
package idempotency_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/idempotency"
	"github.com/valyala/fasthttp"

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
		isIdempotent := idempotency.IsFromCache(c) || idempotency.WasPutToCache(c)
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
			} else {
				// No request header
				if isIdempotent {
					return errors.New("request with unsafe HTTP method should not be idempotent if X-Idempotency-Key request header is not set")
				}
			}
		}

		return nil
	})

	// Needs to be at least a second as the memory storage doesn't support shorter durations.
	const lifetime = 1 * time.Second

	app.Use(idempotency.New(idempotency.Config{
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
		time.Sleep(2 * lifetime)

		return c.SendString(strconv.Itoa(nextCount()))
	})

	doReq := func(method, route, idempotencyKey string) string {
		req := httptest.NewRequest(method, route, http.NoBody)
		if idempotencyKey != "" {
			req.Header.Set("X-Idempotency-Key", idempotencyKey)
		}
		resp, err := app.Test(req, 3*int(lifetime.Milliseconds()))
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
	time.Sleep(2 * lifetime)
	require.Equal(t, "10", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))
	require.Equal(t, "10", doReq(fiber.MethodPost, "/", "00000000-0000-0000-0000-000000000000"))

	// Test raciness
	{
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				require.Equal(t, "11", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
			}()
		}
		wg.Wait()
		require.Equal(t, "11", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
	}
	time.Sleep(2 * lifetime)
	require.Equal(t, "12", doReq(fiber.MethodPost, "/slow", "22222222-2222-2222-2222-222222222222"))
}

// go test -v -run=^$ -bench=Benchmark_Idempotency -benchmem -count=4
func Benchmark_Idempotency(b *testing.B) {
	app := fiber.New()

	// Needs to be at least a second as the memory storage doesn't support shorter durations.
	const lifetime = 1 * time.Second

	app.Use(idempotency.New(idempotency.Config{
		Lifetime: lifetime,
	}))

	app.Post("/", func(c fiber.Ctx) error {
		return nil
	})

	h := app.Handler()

	b.Run("hit", func(b *testing.B) {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(fiber.MethodPost)
		c.Request.SetRequestURI("/")
		c.Request.Header.Set("X-Idempotency-Key", "00000000-0000-0000-0000-000000000000")

		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			h(c)
		}
	})

	b.Run("skip", func(b *testing.B) {
		c := &fasthttp.RequestCtx{}
		c.Request.Header.SetMethod(fiber.MethodPost)
		c.Request.SetRequestURI("/")

		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			h(c)
		}
	})
}
