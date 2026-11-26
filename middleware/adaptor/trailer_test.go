package adaptor

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/stretchr/testify/require"
)

// trailerNames returns "prefix0, prefix1, ..." naming n fields.
func trailerNames(prefix string, n int) string {
	names := make([]string, n)
	for i := range names {
		names[i] = prefix + strconv.Itoa(i)
	}
	return strings.Join(names, ", ")
}

// Test_FiberHandler_TrailerNames pins that a Trailer field naming more than fieldname.MaxTrailers fields is
// refused before the handler runs, as the router refuses it, and that one naming up to that many is dropped, as
// the router drops it. net/http keeps the Trailer field of a request that is not chunked in its header map.
func Test_FiberHandler_TrailerNames(t *testing.T) {
	t.Parallel()
	maxNames := trailerNames("X-T", fieldname.MaxTrailers)
	for _, tt := range []struct {
		name   string
		lines  []string
		status int
	}{
		{name: "none", status: fiber.StatusNoContent},
		{name: "at the cap", lines: []string{maxNames}, status: fiber.StatusNoContent},
		{name: "at the cap over two lines", lines: []string{trailerNames("X-T", 8), trailerNames("X-U", 8)}, status: fiber.StatusNoContent},
		{name: "over the cap", lines: []string{trailerNames("X-T", fieldname.MaxTrailers+1)}, status: fiber.StatusRequestHeaderFieldsTooLarge},
		{name: "over the cap over two lines", lines: []string{maxNames, "X-Extra"}, status: fiber.StatusRequestHeaderFieldsTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := make(chan int, 1)
			h := FiberHandler(func(c fiber.Ctx) error {
				got <- len(c.Request().Header.PeekTrailerKeys())
				return c.SendStatus(fiber.StatusNoContent)
			})
			r := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
			r.Header["Trailer"] = tt.lines
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			require.Equal(t, tt.status, rec.Code)
			if tt.status != fiber.StatusNoContent {
				require.Empty(t, got, "the handler ran for a refused request")
				return
			}
			require.Zero(t, apptest.Recv(t, got, time.Second), "the names were announced to the handler")
		})
	}
}

// Test_FiberApp_TrailerNamesAreCapped pins that a request that names as many trailer fields as it has headers,
// which kept the header copy quadratic (about 6 s for these, 40 s for what fits in net/http's header limit), is
// refused at once instead of copied.
func Test_FiberApp_TrailerNamesAreCapped(t *testing.T) {
	t.Parallel()
	const n = 40_000
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	r := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	for i := range n {
		r.Header.Set("X-H"+strconv.Itoa(i), "v")
	}
	r.Header.Set("Trailer", trailerNames("X-T", n))
	rec := httptest.NewRecorder()
	start := time.Now()
	FiberApp(app).ServeHTTP(rec, r)
	require.Equal(t, fiber.StatusRequestHeaderFieldsTooLarge, rec.Code)
	require.Less(t, time.Since(start), 2*time.Second, "the names a Trailer field announces made the header copy quadratic")
}

// Test_FiberApp_TrailerFieldKeepsHeaders pins that the Trailer field of a request that is not chunked does not file
// the headers it names as trailers: the handler reads what it reads behind Fiber's own server.
func Test_FiberApp_TrailerFieldKeepsHeaders(t *testing.T) {
	t.Parallel()
	const raw = "GET / HTTP/1.1\r\nHost: example.com\r\nX-Client-Address: 203.0.113.7\r\n" +
		"Trailer: X-Client-Address, User-Agent, X-Late\r\nUser-Agent: agent/1\r\nX-Late: 1\r\n\r\n"
	describe := func(c fiber.Ctx) error {
		return c.SendString(c.Get("X-Client-Address") + "|" + c.Get(fiber.HeaderUserAgent) + "|" + c.Get("X-Late") +
			"|" + c.Get(fiber.HeaderTrailer) + "|" + strconv.Itoa(len(c.Request().Header.PeekTrailerKeys())))
	}
	app := fiber.New()
	app.Use(describe)

	want := string(rawLine(t, app, raw).Response.Body())
	require.Equal(t, "203.0.113.7|agent/1|1||0", want)
	for name, h := range map[string]http.Handler{
		"FiberApp": FiberApp(app), "FiberHandler": FiberHandler(describe), "FiberHandlerFunc": FiberHandlerFunc(describe),
	} {
		req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw)))
		require.NoError(t, err)
		require.NotEmpty(t, req.Header.Values(fiber.HeaderTrailer), "net/http keeps the field")
		require.Equal(t, want, adaptorTestServe(h, req).Body.String(), name)
	}
}

// Test_FiberApp_TrailerFieldKeepsProxyHeader pins that naming ProxyHeader in a Trailer field does not hide the
// address a trusted proxy reports there, whichever spelling of the field's name the header map holds.
func Test_FiberApp_TrailerFieldKeepsProxyHeader(t *testing.T) {
	t.Parallel()
	const proxyHeader = "CF-Connecting-IP"
	for _, key := range []string{"Trailer", "trailer"} {
		for _, validate := range []bool{false, true} {
			t.Run(key+" "+strconv.FormatBool(validate), func(t *testing.T) {
				t.Parallel()
				app := fiber.New(fiber.Config{
					ProxyHeader:        proxyHeader,
					TrustProxy:         true,
					EnableIPValidation: validate,
					TrustProxyConfig:   fiber.TrustProxyConfig{Proxies: []string{"127.0.0.1"}},
				})
				app.Get("/", func(c fiber.Ctx) error { return c.SendString(c.IP()) })
				h := FiberApp(app)
				// The map order decides which of the field and the header it names is stored first.
				for range 32 {
					r := apptest.Req(http.MethodGet, "/", apptest.WithHeader(proxyHeader, "203.0.113.7"))
					r.RemoteAddr = "127.0.0.1:40000"
					r.Header[key] = []string{proxyHeader}
					require.Equal(t, "203.0.113.7", adaptorTestServe(h, r).Body.String())
				}
			})
		}
	}
}
