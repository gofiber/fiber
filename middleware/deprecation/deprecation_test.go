package deprecation

import (
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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
		l := newLinks(cfg.Link, cfg.SunsetLink)
		c := apptest.Ctx(t, app)
		c.Links(l.pairs...)
		require.Equal(t, string(c.Response().Header.Peek(fiber.HeaderLink)), l.joined)
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
	tests := []struct {
		upstream    http.Header
		name        string
		deprecation string
	}{
		{name: "upstream sends a Link", upstream: http.Header{"Link": {`</next>; rel="next"`}}, deprecation: deprecatedValue},
		{
			name:        "upstream sends its own Deprecation",
			upstream:    http.Header{"Link": {`</next>; rel="next"`}, "Deprecation": {"@1"}},
			deprecation: "@1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				maps.Copy(w.Header(), tt.upstream)
				_, _ = w.Write([]byte("upstream")) //nolint:errcheck // the test reads the body
			}))
			t.Cleanup(upstream.Close)

			policy := proxy.DefaultSecurityPolicy()
			policy.AllowPrivateIPs = true
			app := fiber.New()
			app.Use(New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: "https://example.com/d"}))
			app.Get("/", proxy.Balancer(proxy.Config{Servers: []string{upstream.URL}, SecurityPolicy: &policy}))

			resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "upstream")
			requireHeader(t, resp, fiber.HeaderDeprecation, tt.deprecation)
			requireHeader(t, resp, fiber.HeaderSunset, sunsetValue)
			requireHeader(t, resp, fiber.HeaderLink, `</next>; rel="next",<https://example.com/d>; rel="deprecation"`)
		})
	}
}

// Each configured header and link is checked on its own after the chain, so
// what a handler changes does not depend on which of them are configured.
func Test_Deprecation_AfterChain(t *testing.T) {
	t.Parallel()
	const (
		d     = "https://example.com/d"
		s     = "https://example.com/s"
		dLink = `<https://example.com/d>; rel="deprecation"`
		sLink = `<https://example.com/s>; rel="sunset"`
	)
	all := Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: d, SunsetLink: s}
	tests := []struct {
		handler     fiber.Handler
		appConfig   fiber.Config
		name        string
		deprecation string
		sunset      string
		link        string
		config      Config
	}{
		{
			name: "replaced block with its own Deprecation", config: all,
			handler: func(c fiber.Ctx) error {
				c.Response().Reset()
				c.Set(fiber.HeaderDeprecation, "@1")
				return nil
			},
			deprecation: "@1", sunset: sunsetValue, link: dLink + "," + sLink,
		},
		{
			name: "replaced block with its own Sunset", config: Config{Sunset: sunsetAt, SunsetLink: s},
			handler: func(c fiber.Ctx) error {
				c.Response().Reset()
				c.Set(fiber.HeaderSunset, "Mon, 01 Jan 2024 00:00:00 GMT")
				return nil
			},
			sunset: "Mon, 01 Jan 2024 00:00:00 GMT", link: sLink,
		},
		{
			name: "replaced block holding one of two links", config: all,
			handler: func(c fiber.Ctx) error {
				c.Response().Reset()
				c.Set(fiber.HeaderLink, dLink)
				return nil
			},
			deprecation: deprecatedValue, sunset: sunsetValue, link: dLink + "," + sLink,
		},
		{
			name: "replaced block holding the other link", config: all,
			handler: func(c fiber.Ctx) error {
				c.Response().Reset()
				c.Set(fiber.HeaderLink, sLink)
				return nil
			},
			deprecation: deprecatedValue, sunset: sunsetValue, link: sLink + "," + dLink,
		},
		{
			name: "Link replaced under a policy-only config", config: Config{Link: d},
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderLink, `</next>; rel="next"`)
				return nil
			},
			link: `</next>; rel="next",` + dLink,
		},
		{
			name: "Link replaced under a dated config", config: Config{Deprecation: deprecatedAt, Link: d},
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderLink, `</next>; rel="next"`)
				return nil
			},
			deprecation: deprecatedValue, link: `</next>; rel="next",` + dLink,
		},
		{
			name: "removed headers are written again", config: all,
			handler: func(c fiber.Ctx) error {
				c.Response().Header.Del(fiber.HeaderDeprecation)
				c.Response().Header.Del(fiber.HeaderSunset)
				c.Response().Header.Del(fiber.HeaderLink)
				return nil
			},
			deprecation: deprecatedValue, sunset: sunsetValue, link: dLink + "," + sLink,
		},
		{
			name: "an added Deprecation line replaces ours", config: Config{Deprecation: deprecatedAt, Sunset: sunsetAt},
			handler: func(c fiber.Ctx) error {
				c.Response().Header.Add(fiber.HeaderDeprecation, "@5")
				c.Response().Header.Add(fiber.HeaderSunset, sunsetValue)
				return nil
			},
			deprecation: "@5", sunset: sunsetValue,
		},
		{
			name: "response with a trailer section", config: all,
			handler: func(c fiber.Ctx) error {
				return c.Response().Header.SetTrailer("X-Checksum")
			},
			deprecation: deprecatedValue, sunset: sunsetValue, link: dLink + "," + sLink,
		},
		{
			name: "another spelling without normalizing", config: Config{Deprecation: deprecatedAt},
			appConfig: fiber.Config{DisableHeaderNormalizing: true},
			handler: func(c fiber.Ctx) error {
				c.Set("deprecation", "@5")
				return nil
			},
			deprecation: "@5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(tt.appConfig)
			app.Use(New(tt.config))
			app.Get("/", func(c fiber.Ctx) error {
				if err := tt.handler(c); err != nil {
					return err
				}
				return c.SendString("ok")
			})

			resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "ok")
			requireHeader(t, resp, fiber.HeaderDeprecation, tt.deprecation)
			requireHeader(t, resp, fiber.HeaderSunset, tt.sunset)
			requireHeader(t, resp, fiber.HeaderLink, tt.link)
		})
	}
}

// An earlier middleware's Deprecation under another spelling is replaced, not
// kept beside ours.
func Test_Deprecation_ReplacesOtherSpelling(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
	app.Use(func(c fiber.Ctx) error {
		c.Set("deprecation", "@5")
		return c.Next()
	})
	app.Use(New(Config{Deprecation: deprecatedAt}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "ok")
	requireHeader(t, resp, fiber.HeaderDeprecation, deprecatedValue)
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
	// The Ctx's own rewrite drops the link, so each request folds it in twice.
	require.Len(t, got, 4)
	for _, args := range got {
		require.Equal(t, []string{"https://example.com/d", "deprecation"}, args)
	}
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

// fasthttp resets the response for the 416 of an unsatisfiable range.
func Test_Deprecation_RangeNotSatisfiable(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "doc.txt")
	require.NoError(t, os.WriteFile(file, []byte("doc"), 0o600))
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt, Link: "https://example.com/d"}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendFile(file, fiber.SendFile{ByteRange: true}) })

	resp := apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/",
		apptest.WithHeader(fiber.HeaderRange, "bytes=100-200")), fiber.StatusRequestedRangeNotSatisfiable)
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
