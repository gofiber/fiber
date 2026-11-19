package deprecation

import (
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
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

// The links are appended to the Link lines already set, and a link one of them
// already holds, as an earlier run of the instance wrote it, is not added again.
func Test_Deprecation_ExistingLinks(t *testing.T) {
	t.Parallel()
	next := `<https://example.com/next>; rel="next"`
	d, s := `<https://example.com/d>; rel="deprecation"`, `<https://example.com/s>; rel="sunset"`
	tests := []struct {
		appConfig fiber.Config
		existing  func(c fiber.Ctx)
		name      string
		want      string
	}{
		{
			name: "set with Links", want: next + "," + d + "," + s,
			existing: func(c fiber.Ctx) { c.Links("https://example.com/next", "next") },
		},
		{
			name: "lower-case name without normalizing", want: next + "," + d + "," + s,
			appConfig: fiber.Config{DisableHeaderNormalizing: true}, existing: func(c fiber.Ctx) { c.Set("link", next) },
		},
		{name: "deprecation link held", want: d + "," + s, existing: func(c fiber.Ctx) { c.Set(fiber.HeaderLink, d) }},
		{name: "sunset link held", want: s + "," + d, existing: func(c fiber.Ctx) { c.Set(fiber.HeaderLink, s) }},
		{name: "both held", want: d + "," + s, existing: func(c fiber.Ctx) { c.Set(fiber.HeaderLink, d+","+s) }},
		{
			name: "held in a later line", want: next + "," + d + "," + s,
			existing: func(c fiber.Ctx) {
				c.Response().Header.Add(fiber.HeaderLink, next)
				c.Response().Header.Add(fiber.HeaderLink, d)
			},
		},
		{
			name: "held under a lower-case name without normalizing", want: d + "," + s,
			appConfig: fiber.Config{DisableHeaderNormalizing: true}, existing: func(c fiber.Ctx) { c.Set("link", d) },
		},
		{
			name: "same link for another context", want: d + `; anchor="#a",` + d + "," + s,
			existing: func(c fiber.Ctx) { c.Set(fiber.HeaderLink, d+`; anchor="#a"`) },
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
			requireHeader(t, resp, fiber.HeaderLink, tt.want)
		})
	}
}

// At two levels the inner instance replaces the dates and both add their links.
func Test_Deprecation_TwoLevels(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Link: "https://example.com/policy"}))
	v1 := app.Group("/v1", New(Config{Deprecation: sunsetAt, Sunset: sunsetAt, Link: "https://example.com/v1"}))
	v1.Get("/", func(fiber.Ctx) error { return nil })

	resp := apptest.Expect(t, app, "/v1", fiber.StatusOK)
	requireHeader(t, resp, fiber.HeaderDeprecation, "@1719791999")
	requireHeader(t, resp, fiber.HeaderSunset, sunsetValue)
	requireHeader(t, resp, fiber.HeaderLink,
		`<https://example.com/policy>; rel="deprecation",<https://example.com/v1>; rel="deprecation"`)
}

// An earlier writer's lines of the field, every one and under any spelling, give
// way to the configured value. The spelling is read from the response, which an
// earlier middleware may stop normalizing in a default app.
func Test_Deprecation_ReplacesEarlierLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		appConfig fiber.Config
		existing  func(c fiber.Ctx)
		name      string
	}{
		{
			name: "every earlier line",
			existing: func(c fiber.Ctx) {
				c.Response().Header.Add(fiber.HeaderDeprecation, "@1")
				c.Response().Header.Add(fiber.HeaderDeprecation, "@2")
			},
		},
		{
			name: "app without normalizing", appConfig: fiber.Config{DisableHeaderNormalizing: true},
			existing: func(c fiber.Ctx) { c.Set("deprecation", "@5") },
		},
		{
			name: "response without normalizing",
			existing: func(c fiber.Ctx) {
				c.Response().Header.DisableNormalizing()
				c.Set("deprecation", "@5")
			},
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
			app.Use(New(Config{Deprecation: deprecatedAt}))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "ok")
			requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
		})
	}
}

// A middleware that restarts routing runs the instance again on one response.
func Test_Deprecation_RestartRouting(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Link: "https://example.com/d", SunsetLink: "https://example.com/s"}))
	app.Get("/old", func(c fiber.Ctx) error {
		c.Path("/new")
		return c.RestartRouting()
	})
	app.Get("/new", func(c fiber.Ctx) error { return c.SendString("new") })

	resp := apptest.ExpectBody(t, app, "/old", fiber.StatusOK, "new")
	requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
	requireHeader(t, resp, fiber.HeaderLink,
		`<https://example.com/d>; rel="deprecation",<https://example.com/s>; rel="sunset"`)
}

// linksMutatingCtx is a custom context whose Links records and then rewrites
// its arguments, which Ctx's contract does not forbid.
type linksMutatingCtx struct {
	got *[][]string
	fiber.DefaultCtx
}

func (c *linksMutatingCtx) Links(link ...string) {
	*c.got = append(*c.got, slices.Clone(link))
	for i := range link {
		link[i] = "X-Mutated"
	}
	c.DefaultCtx.Links(link...)
}

// The link pairs are shared by every request, so a custom Ctx gets a copy.
func Test_Deprecation_LinksNotSharedWithCustomCtx(t *testing.T) {
	t.Parallel()
	var got [][]string
	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &linksMutatingCtx{DefaultCtx: *fiber.NewDefaultCtx(app), got: &got}
	})
	app.Use(func(c fiber.Ctx) error {
		c.Set(fiber.HeaderLink, `</next>; rel="next"`)
		return c.Next()
	})
	app.Use(New(Config{Link: "https://example.com/d"}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	for range 2 {
		apptest.ExpectBody(t, app, "/", fiber.StatusOK, "ok")
	}
	// The Ctx's own rewrite drops the link, so each request folds it in again.
	require.Len(t, got, 2)
	for _, args := range got {
		require.Equal(t, []string{"https://example.com/d", "deprecation"}, args)
	}
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
			app.Use(recoverer.New())
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
		{name: "bare http scheme", msg: badLink, cfg: []Config{{Link: "http:"}}},
		{name: "http link with an empty authority", msg: badLink, cfg: []Config{{Link: "http://"}}},
		{name: "http link without a host", msg: badLink, cfg: []Config{{Link: "http:///x"}}},
		{name: "https link with only a port", msg: badLink, cfg: []Config{{Link: "https://:443/docs"}}},
		{name: "http link with an empty port", msg: badLink, cfg: []Config{{Link: "http://:/x"}}},
		{name: "link with userinfo", msg: badLink, cfg: []Config{{Link: "https://user:token@example.com/x"}}},
		{name: "network-path link without a host", msg: badLink, cfg: []Config{{Link: "///deprecation"}}},
		{name: "bare network-path link", msg: badLink, cfg: []Config{{Link: "//"}}},
		{name: "stray percent in the query", msg: badLink, cfg: []Config{{Link: "https://x/?q=%zz"}}},
		{name: "trailing percent", msg: badLink, cfg: []Config{{Link: "https://example.com/search?q=100%"}}},
		{
			name: "deprecation below an sf-integer",
			msg:  "deprecation: Deprecation is out of the range of a Structured Field Date",
			cfg:  []Config{{Deprecation: time.Unix(-1e15, 0)}},
		},
		{
			name: "sunset link with only a port", msg: "deprecation: SunsetLink must be a URI reference",
			cfg: []Config{{SunsetLink: "https://:443/sunset"}},
		},
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

func Test_Deprecation_ValidConfig(t *testing.T) {
	t.Parallel()
	for _, cfg := range []Config{
		{Sunset: time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Sunset: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)},
		// Year 10000 in UTC+1 is still 9999 in UTC.
		{Sunset: time.Date(10000, 1, 1, 0, 30, 0, 0, time.FixedZone("UTC+1", 60*60))},
		{Deprecation: time.Unix(-999_999_999_999_999, 0)},
		{Deprecation: time.Unix(999_999_999_999_999, 0)},
		{Link: "https://[::1]:8443/docs"},
		{Link: "//example.com/docs"},
		{Link: "/docs/%41%2F%25?q=%20"},
		{Link: "urn:example:deprecation"},
		{Link: "mailto:api@example.com"},
	} {
		require.NotPanics(t, func() { New(cfg) }, "%+v", cfg)
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
