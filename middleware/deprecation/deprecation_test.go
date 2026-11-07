package deprecation

import (
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/middleware/proxy"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/timeout"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var (
	deprecatedAt = time.Date(2023, time.June, 30, 23, 59, 59, 0, time.UTC)
	sunsetAt     = time.Date(2024, time.June, 30, 23, 59, 59, 0, time.UTC)
)

const (
	deprecatedValue = "@1688169599"
	sunsetValue     = "Sun, 30 Jun 2024 23:59:59 GMT"
)

// requireHeader checks that the response carries name with value, or no field
// line of that name at all when value is empty: Header.Get answers "" for both.
func requireHeader(t *testing.T, resp *http.Response, name, value string) {
	t.Helper()
	if value == "" {
		require.NotContains(t, resp.Header, name)
		return
	}
	require.Equal(t, []string{value}, resp.Header.Values(name))
}

func Test_Deprecation_Headers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		config      Config
		deprecation string
		sunset      string
		link        string
	}{
		{name: "deprecation only", config: Config{Deprecation: deprecatedAt}, deprecation: deprecatedValue},
		{name: "sunset only", config: Config{Sunset: sunsetAt}, sunset: sunsetValue},
		{
			name:        "deprecation and sunset",
			config:      Config{Deprecation: deprecatedAt, Sunset: sunsetAt},
			deprecation: deprecatedValue, sunset: sunsetValue,
		},
		{
			name:        "sunset equals deprecation",
			config:      Config{Deprecation: sunsetAt, Sunset: sunsetAt},
			deprecation: "@1719791999", sunset: sunsetValue,
		},
		{
			name: "sub-second dates in the same second",
			config: Config{
				Deprecation: sunsetAt.Add(900 * time.Millisecond), Sunset: sunsetAt.Add(100 * time.Millisecond),
			},
			deprecation: "@1719791999", sunset: sunsetValue,
		},
		{
			name:   "sunset in another time zone",
			config: Config{Sunset: sunsetAt.In(time.FixedZone("UTC+3", 3*60*60))},
			sunset: sunsetValue,
		},
		{
			name:        "deprecation link",
			config:      Config{Deprecation: deprecatedAt, Link: "https://example.com/deprecation"},
			deprecation: deprecatedValue, link: `<https://example.com/deprecation>; rel="deprecation"`,
		},
		{
			name: "both links",
			config: Config{
				Deprecation: deprecatedAt, Sunset: sunsetAt,
				Link: "https://example.com/d", SunsetLink: "https://example.com/s",
			},
			deprecation: deprecatedValue, sunset: sunsetValue,
			link: `<https://example.com/d>; rel="deprecation",<https://example.com/s>; rel="sunset"`,
		},
		{
			name:   "deprecation policy link only",
			config: Config{Link: "https://example.com/policy"},
			link:   `<https://example.com/policy>; rel="deprecation"`,
		},
		{
			name:   "sunset policy link only",
			config: Config{SunsetLink: "https://example.com/sunset"},
			link:   `<https://example.com/sunset>; rel="sunset"`,
		},
		{
			name:   "relative link",
			config: Config{Link: "/docs/deprecation?v=1#guide"},
			link:   `</docs/deprecation?v=1#guide>; rel="deprecation"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tt.config))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "ok")
			requireHeader(t, resp, fiber.HeaderDeprecation, tt.deprecation)
			requireHeader(t, resp, fiber.HeaderSunset, tt.sunset)
			requireHeader(t, resp, fiber.HeaderLink, tt.link)
		})
	}
}

func Test_Deprecation_KeepsExistingLinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		appConfig fiber.Config
		existing  func(c fiber.Ctx)
		name      string
	}{
		{name: "set with Links", existing: func(c fiber.Ctx) { c.Links("https://example.com/next", "next") }},
		{
			name:      "lower-case name without normalizing",
			appConfig: fiber.Config{DisableHeaderNormalizing: true},
			existing:  func(c fiber.Ctx) { c.Set("link", `<https://example.com/next>; rel="next"`) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(tt.appConfig)
			app.Use(func(c fiber.Ctx) error {
				tt.existing(c)
				return c.Next()
			})
			app.Use(New(Config{
				Deprecation: deprecatedAt, Link: "https://example.com/d", SunsetLink: "https://example.com/s",
			}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

			resp := apptest.Expect(t, app, "/", fiber.StatusNoContent)
			requireHeader(t, resp, fiber.HeaderLink, `<https://example.com/next>; rel="next",`+
				`<https://example.com/d>; rel="deprecation",<https://example.com/s>; rel="sunset"`)
		})
	}
}

// The value written when no Link header is set must match what c.Links would
// write, since the middleware takes either path.
func Test_Deprecation_LinkMatchesLinks(t *testing.T) {
	t.Parallel()
	cfgs := []Config{
		{Link: "https://example.com/d"},
		{SunsetLink: "https://example.com/s"},
		{Link: "https://example.com/d", SunsetLink: "https://example.com/s"},
	}
	app := fiber.New()
	for _, cfg := range cfgs {
		link, links := formatLinks(cfg.Link, cfg.SunsetLink)
		c := apptest.Ctx(t, app)
		c.Links(links...)
		require.Equal(t, string(c.Response().Header.Peek(fiber.HeaderLink)), link)
	}
}

func Test_Deprecation_ReplacedResponse(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: "https://example.com/d"}))
	app.Get("/", func(c fiber.Ctx) error {
		// What proxy.Forward and fasthttp's NotModified do to the header block:
		// reset it, and here keep an upstream Link under the name it sent.
		c.Response().Reset()
		c.Response().Header.DisableNormalizing()
		c.Response().Header.Set("link", `</next>; rel="next"`)
		return c.SendString("upstream")
	})

	resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "upstream")
	requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
	requireHeader(t, resp, fiber.HeaderSunset, sunsetValue)
	requireHeader(t, resp, fiber.HeaderLink, `</next>; rel="next",<https://example.com/d>; rel="deprecation"`)
}

func Test_Deprecation_ReplacedResponseKeepsLink(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Link: "https://example.com/d"}))
	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Reset()
		c.Set(fiber.HeaderLink, `<https://example.com/d>; rel="deprecation"`)
		return c.SendString("upstream")
	})

	resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "upstream")
	requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
	requireHeader(t, resp, fiber.HeaderLink, `<https://example.com/d>; rel="deprecation"`)
}

func Test_Deprecation_LeavesTimedOutResponse(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt}))
	ctxCh := make(chan fiber.Ctx, 1)
	release := make(chan struct{})
	defer close(release)
	app.Get("/", timeout.New(func(c fiber.Ctx) error {
		c.Response().Reset()
		ctxCh <- c
		<-c.Context().Done()
		<-release
		return nil
	}, timeout.Config{Timeout: 10 * time.Millisecond}))

	apptest.Expect(t, app, "/", fiber.StatusServiceUnavailable)
	c := apptest.Recv(t, ctxCh, time.Second)
	require.Empty(t, c.GetRespHeader(fiber.HeaderDeprecation), "the timed-out handler still owns the response")
}

// The proxy replaces the response with the upstream's, Link included.
func Test_Deprecation_Proxy(t *testing.T) {
	t.Parallel()
	upstream := fiber.New()
	upstream.Get("/", func(c fiber.Ctx) error {
		c.Links("/next", "next")
		return c.SendString("upstream")
	})
	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		//nolint:errcheck // the listener stops with the Shutdown below
		_ = upstream.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	t.Cleanup(func() { require.NoError(t, upstream.Shutdown()) })

	policy := proxy.DefaultSecurityPolicy()
	policy.AllowPrivateIPs = true
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: "https://example.com/d"}))
	app.Get("/", proxy.Balancer(proxy.Config{
		Servers: []string{"http://" + ln.Addr().String()}, SecurityPolicy: &policy,
	}))

	resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "upstream")
	requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
	requireHeader(t, resp, fiber.HeaderSunset, sunsetValue)
	requireHeader(t, resp, fiber.HeaderLink, `</next>; rel="next",<https://example.com/d>; rel="deprecation"`)
}

// fasthttp resets the response for the 304 of a conditional SendFile.
func Test_Deprecation_NotModified(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "doc.txt")
	require.NoError(t, os.WriteFile(file, []byte("doc"), 0o600))
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: "https://example.com/d"}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendFile(file) })

	resp := apptest.Expect(t, app, "/", fiber.StatusOK)
	lastModified := resp.Header.Get(fiber.HeaderLastModified)
	require.NotEmpty(t, lastModified)

	resp = apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/",
		apptest.WithHeader(fiber.HeaderIfModifiedSince, lastModified)), fiber.StatusNotModified)
	requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
	requireHeader(t, resp, fiber.HeaderSunset, sunsetValue)
	requireHeader(t, resp, fiber.HeaderLink, `<https://example.com/d>; rel="deprecation"`)
}

// The headers are written before the chain runs: c.End() sends the response
// from inside the handler, and an outer recover.New() answers a panic.
func Test_Deprecation_BeforeChain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		handler fiber.Handler
		name    string
		status  int
	}{
		{
			name: "End in the handler", status: fiber.StatusOK,
			handler: func(c fiber.Ctx) error {
				if err := c.SendString("ok"); err != nil {
					return err
				}
				return c.End()
			},
		},
		{
			name: "panic recovered outside", status: fiber.StatusInternalServerError,
			handler: func(fiber.Ctx) error { panic("boom") },
		},
		{
			name: "error response", status: fiber.StatusInternalServerError,
			handler: func(fiber.Ctx) error { return errors.New("boom") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(recover.New())
			app.Use(New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt}))
			app.Get("/", tt.handler)

			resp := apptest.Expect(t, app, "/", tt.status)
			requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
			requireHeader(t, resp, fiber.HeaderSunset, sunsetValue)
		})
	}
}

func Test_Deprecation_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Deprecation: deprecatedAt,
		Link:        "https://example.com/deprecation",
		Next:        func(c fiber.Ctx) bool { return c.Path() == "/skip" },
	}))
	app.Get("/skip", func(c fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/keep", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp := apptest.ExpectBody(t, app, "/skip", fiber.StatusOK, "ok")
	requireHeader(t, resp, fiber.HeaderDeprecation, "")
	requireHeader(t, resp, fiber.HeaderLink, "")

	resp = apptest.ExpectBody(t, app, "/keep", fiber.StatusOK, "ok")
	requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
	requireHeader(t, resp, fiber.HeaderLink, `<https://example.com/deprecation>; rel="deprecation"`)
}

func Test_Deprecation_InvalidConfig(t *testing.T) {
	t.Parallel()
	const (
		required = "deprecation: Deprecation, Sunset, Link or SunsetLink is required"
		order    = "deprecation: Sunset must not be earlier than Deprecation"
		years    = "deprecation: Sunset must fall within the years 0000 to 9999 of an HTTP-date"
		badLink  = "deprecation: Link must be a URI reference"
	)
	tests := []struct {
		name string
		msg  string
		cfg  []Config
	}{
		{name: "no config", msg: required},
		{name: "empty config", msg: required, cfg: []Config{{}}},
		{name: "sunset before deprecation", msg: order, cfg: []Config{{Deprecation: sunsetAt, Sunset: deprecatedAt}}},
		{
			name: "sunset after year 9999", msg: years,
			cfg: []Config{{Sunset: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}},
		},
		{name: "sunset before year 0", msg: years, cfg: []Config{{Sunset: time.Date(-1, 6, 1, 0, 0, 0, 0, time.UTC)}}},
		{
			name: "deprecation beyond an sf-integer",
			msg:  "deprecation: Deprecation is out of the range of a Structured Field Date",
			cfg:  []Config{{Deprecation: time.Unix(1e15, 0)}},
		},
		{name: "link that ends the target early", msg: badLink, cfg: []Config{{Link: `https://x>;rel="preload"`}}},
		{name: "link with a less-than sign", msg: badLink, cfg: []Config{{Link: "https://x/<a"}}},
		{name: "link with a space", msg: badLink, cfg: []Config{{Link: "https://x/a b"}}},
		{name: "link with CR and LF", msg: badLink, cfg: []Config{{Link: "https://x/a#\r\nb"}}},
		{name: "link with NUL", msg: badLink, cfg: []Config{{Link: "https://x/a#\x00b"}}},
		{name: "link with DEL", msg: badLink, cfg: []Config{{Link: "https://x/a#\x7f"}}},
		{name: "link with non-ASCII", msg: badLink, cfg: []Config{{Link: "https://example.com/migración"}}},
		{name: "link with a quote", msg: badLink, cfg: []Config{{Link: `https://x/a"b`}}},
		{name: "link with a backslash", msg: badLink, cfg: []Config{{Link: `https://x/a\b`}}},
		{name: "link that does not parse", msg: badLink, cfg: []Config{{Link: "https://x/%zz"}}},
		{name: "https link without a host", msg: badLink, cfg: []Config{{Link: "https:///x"}}},
		{name: "bare https scheme", msg: badLink, cfg: []Config{{Link: "https:"}}},
		{
			name: "invalid sunset link", msg: "deprecation: SunsetLink must be a URI reference",
			cfg: []Config{{Sunset: sunsetAt, SunsetLink: "https://"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, tt.msg, func() { New(tt.cfg...) })
		})
	}
}

func Benchmark_Deprecation(b *testing.B) {
	app := fiber.New()
	h := New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: "https://example.com/deprecation"})
	app.Get("/", h, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	handler := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()
	for b.Loop() {
		// A reused response would keep the Link header and take the folding path.
		fctx.Response.Reset()
		handler(fctx)
	}

	require.Equal(b, deprecatedValue, string(fctx.Response.Header.Peek(fiber.HeaderDeprecation)))
	require.Equal(b, sunsetValue, string(fctx.Response.Header.Peek(fiber.HeaderSunset)))
	require.Equal(b, `<https://example.com/deprecation>; rel="deprecation"`,
		string(fctx.Response.Header.Peek(fiber.HeaderLink)))
}

func Benchmark_Deprecation_ExistingLink(b *testing.B) {
	app := fiber.New()
	h := New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: "https://example.com/deprecation"})
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderLink, `</next>; rel="next"`)
		return c.Next()
	}, h, func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	handler := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()
	for b.Loop() {
		fctx.Response.Reset()
		handler(fctx)
	}

	require.Equal(b, deprecatedValue, string(fctx.Response.Header.Peek(fiber.HeaderDeprecation)))
	require.Equal(b, sunsetValue, string(fctx.Response.Header.Peek(fiber.HeaderSunset)))
	require.Equal(b, `</next>; rel="next",<https://example.com/deprecation>; rel="deprecation"`,
		string(fctx.Response.Header.Peek(fiber.HeaderLink)))
}
