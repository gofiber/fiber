package healthcheck

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/shamaton/msgpack/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func shouldGiveStatus(t *testing.T, app *fiber.App, path string, expectedStatus int) {
	t.Helper()
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, path, http.NoBody))
	require.NoError(t, err)
	require.Equal(t, expectedStatus, req.StatusCode, "path: "+path+" should match "+strconv.Itoa(expectedStatus))
}

func shouldGiveOK(t *testing.T, app *fiber.App, path string) {
	t.Helper()
	shouldGiveStatus(t, app, path, fiber.StatusOK)
}

func shouldGiveNotFound(t *testing.T, app *fiber.App, path string) {
	t.Helper()
	shouldGiveStatus(t, app, path, fiber.StatusNotFound)
}

func Test_HealthCheck_Strict_Routing_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		StrictRouting: true,
	})

	app.Get(LivenessEndpoint, New())
	app.Get(ReadinessEndpoint, New())
	app.Get(StartupEndpoint, New())

	shouldGiveOK(t, app, "/readyz")
	shouldGiveOK(t, app, "/livez")
	shouldGiveOK(t, app, "/startupz")
	shouldGiveNotFound(t, app, "/readyz/")
	shouldGiveNotFound(t, app, "/livez/")
	shouldGiveNotFound(t, app, "/startupz/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
	shouldGiveNotFound(t, app, "/notDefined/startupz")
}

func Test_HealthCheck_Default(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	app.Get(LivenessEndpoint, New())
	app.Get(ReadinessEndpoint, New())
	app.Get(StartupEndpoint, New())

	shouldGiveOK(t, app, "/readyz")
	shouldGiveOK(t, app, "/livez")
	shouldGiveOK(t, app, "/startupz")
	shouldGiveOK(t, app, "/readyz/")
	shouldGiveOK(t, app, "/livez/")
	shouldGiveOK(t, app, "/startupz/")
	shouldGiveNotFound(t, app, "/notDefined/readyz")
	shouldGiveNotFound(t, app, "/notDefined/livez")
	shouldGiveNotFound(t, app, "/notDefined/startupz")
}

func Test_HealthCheck_Custom(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c1 := make(chan struct{}, 1)
	app.Get("/live", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			return true
		},
	}))
	app.Get("/ready", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
	}))
	app.Get(StartupEndpoint, New(Config{
		Probe: func(_ fiber.Ctx) bool {
			return false
		},
	}))

	// Setup custom liveness and readiness probes to simulate application health status
	// Live should return 200 with GET request
	shouldGiveOK(t, app, "/live")
	// Live should return 404 with POST request
	req, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/live", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusMethodNotAllowed, req.StatusCode)

	// Ready should return 404 with POST request
	req, err = app.Test(httptest.NewRequest(fiber.MethodPost, "/ready", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusMethodNotAllowed, req.StatusCode)

	// Ready should return 503 with GET request before the channel is closed
	shouldGiveStatus(t, app, "/ready", fiber.StatusServiceUnavailable)

	shouldGiveStatus(t, app, "/startupz", fiber.StatusServiceUnavailable)

	// Ready should return 200 with GET request after the channel is closed
	c1 <- struct{}{}
	shouldGiveOK(t, app, "/ready")
}

func Test_HealthCheck_Custom_Nested(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	c1 := make(chan struct{}, 1)
	app.Get("/probe/live", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			return true
		},
	}))
	app.Get("/probe/ready", New(Config{
		Probe: func(_ fiber.Ctx) bool {
			select {
			case <-c1:
				return true
			default:
				return false
			}
		},
	}))

	// Testing custom health check endpoints with nested paths
	shouldGiveOK(t, app, "/probe/live")
	shouldGiveStatus(t, app, "/probe/ready", fiber.StatusServiceUnavailable)
	shouldGiveOK(t, app, "/probe/live/")
	shouldGiveStatus(t, app, "/probe/ready/", fiber.StatusServiceUnavailable)
	shouldGiveNotFound(t, app, "/probe/livez")
	shouldGiveNotFound(t, app, "/probe/readyz")
	shouldGiveNotFound(t, app, "/probe/livez/")
	shouldGiveNotFound(t, app, "/probe/readyz/")
	shouldGiveNotFound(t, app, "/livez")
	shouldGiveNotFound(t, app, "/readyz")
	shouldGiveNotFound(t, app, "/readyz/")
	shouldGiveNotFound(t, app, "/livez/")

	c1 <- struct{}{}
	shouldGiveOK(t, app, "/probe/ready")
	c1 <- struct{}{}
	shouldGiveOK(t, app, "/probe/ready/")
}

func Test_HealthCheck_Next(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	checker := New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	})

	app.Get(LivenessEndpoint, checker)
	app.Get(ReadinessEndpoint, checker)
	app.Get(StartupEndpoint, checker)

	// This should give not found since there are no other handlers to execute
	// so it's like the route isn't defined at all
	shouldGiveNotFound(t, app, "/readyz")
	shouldGiveNotFound(t, app, "/livez")
	shouldGiveNotFound(t, app, "/startupz")
}

func Benchmark_HealthCheck(b *testing.B) {
	app := fiber.New()

	app.Get(LivenessEndpoint, New())
	app.Get(ReadinessEndpoint, New())
	app.Get(StartupEndpoint, New())

	h := app.Handler()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/livez")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, fiber.StatusOK, fctx.Response.Header.StatusCode())
}

func Benchmark_HealthCheck_Parallel(b *testing.B) {
	app := fiber.New()

	app.Get(LivenessEndpoint, New())
	app.Get(ReadinessEndpoint, New())
	app.Get(StartupEndpoint, New())

	h := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		fctx := &fasthttp.RequestCtx{}
		fctx.Request.Header.SetMethod(fiber.MethodGet)
		fctx.Request.SetRequestURI("/livez")

		for pb.Next() {
			h(fctx)
		}
	})
}

func Test_HealthCheck_Text_Format(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	// Test default format (text)
	app.Get("/livez", New())
	app.Get("/readyz", New(Config{
		ResponseFormat: FormatText,
		Probe: func(_ fiber.Ctx) bool {
			return false
		},
	}))

	// Test successful healthcheck with default text format
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)
	require.Equal(t, "text/plain; charset=utf-8", req.Header.Get("Content-Type"))

	// Read body
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "OK", string(body))

	// Test failed healthcheck with text format
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/readyz", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, req.StatusCode)
	require.Equal(t, "text/plain; charset=utf-8", req.Header.Get("Content-Type"))

	// Read body
	body, err = io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "Service Unavailable", string(body))
}

func Test_HealthCheck_JSON_Format(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/livez", New(Config{
		ResponseFormat: FormatJSON,
	}))
	app.Get("/readyz", New(Config{
		ResponseFormat: FormatJSON,
		Probe: func(_ fiber.Ctx) bool {
			return false
		},
	}))

	// Test successful healthcheck with JSON format
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)
	require.Equal(t, "application/json; charset=utf-8", req.Header.Get("Content-Type"))

	// Read and parse JSON body
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"OK"}`, string(body))

	// Test failed healthcheck with JSON format
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/readyz", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, req.StatusCode)
	require.Equal(t, "application/json; charset=utf-8", req.Header.Get("Content-Type"))

	// Read and parse JSON body
	body, err = io.ReadAll(req.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"Service Unavailable"}`, string(body))
}

func Test_HealthCheck_XML_Format(t *testing.T) {
	t.Parallel()

	app := fiber.New()

	app.Get("/livez", New(Config{
		ResponseFormat: FormatXML,
	}))
	app.Get("/readyz", New(Config{
		ResponseFormat: FormatXML,
		Probe: func(_ fiber.Ctx) bool {
			return false
		},
	}))

	// Test successful healthcheck with XML format
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)
	require.Equal(t, "application/xml; charset=utf-8", req.Header.Get("Content-Type"))

	// Read and check XML body
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "<healthResponse>")
	require.Contains(t, string(body), "<status>OK</status>")

	// Test failed healthcheck with XML format
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/readyz", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, req.StatusCode)
	require.Equal(t, "application/xml; charset=utf-8", req.Header.Get("Content-Type"))

	// Read and check XML body
	body, err = io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "<healthResponse>")
	require.Contains(t, string(body), "<status>Service Unavailable</status>")
}

func Test_HealthCheck_MsgPack_Format(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		MsgPackEncoder: msgpack.Marshal,
	})

	app.Get("/livez", New(Config{
		ResponseFormat: FormatMsgPack,
	}))
	app.Get("/readyz", New(Config{
		ResponseFormat: FormatMsgPack,
		Probe: func(_ fiber.Ctx) bool {
			return false
		},
	}))

	// Test successful healthcheck with MsgPack format
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)
	require.Equal(t, "application/vnd.msgpack", req.Header.Get("Content-Type"))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	var livezResponse map[string]string
	require.NoError(t, msgpack.Unmarshal(body, &livezResponse))
	require.Len(t, livezResponse, 1)
	require.Contains(t, livezResponse, "status")
	require.NotContains(t, livezResponse, "Status")
	require.Equal(t, "OK", livezResponse["status"])

	// Test failed healthcheck with MsgPack format
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/readyz", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, req.StatusCode)
	require.Equal(t, "application/vnd.msgpack", req.Header.Get("Content-Type"))

	body, err = io.ReadAll(req.Body)
	require.NoError(t, err)
	var readyzResponse map[string]string
	require.NoError(t, msgpack.Unmarshal(body, &readyzResponse))
	require.Len(t, readyzResponse, 1)
	require.Contains(t, readyzResponse, "status")
	require.NotContains(t, readyzResponse, "Status")
	require.Equal(t, "Service Unavailable", readyzResponse["status"])
}

func Test_HealthCheck_CBOR_Format(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{
		CBOREncoder: cbor.Marshal,
	})

	app.Get("/livez", New(Config{
		ResponseFormat: FormatCBOR,
	}))
	app.Get("/readyz", New(Config{
		ResponseFormat: FormatCBOR,
		Probe: func(_ fiber.Ctx) bool {
			return false
		},
	}))

	// Test successful healthcheck with CBOR format
	req, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/livez", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, req.StatusCode)
	require.Equal(t, "application/cbor", req.Header.Get("Content-Type"))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	var livezResponse map[string]string
	require.NoError(t, cbor.Unmarshal(body, &livezResponse))
	require.Len(t, livezResponse, 1)
	require.Contains(t, livezResponse, "status")
	require.NotContains(t, livezResponse, "Status")
	require.Equal(t, "OK", livezResponse["status"])

	// Test failed healthcheck with CBOR format
	req, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/readyz", http.NoBody))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, req.StatusCode)
	require.Equal(t, "application/cbor", req.Header.Get("Content-Type"))

	body, err = io.ReadAll(req.Body)
	require.NoError(t, err)
	var readyzResponse map[string]string
	require.NoError(t, cbor.Unmarshal(body, &readyzResponse))
	require.Len(t, readyzResponse, 1)
	require.Contains(t, readyzResponse, "status")
	require.NotContains(t, readyzResponse, "Status")
	require.Equal(t, "Service Unavailable", readyzResponse["status"])
}
