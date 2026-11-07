package deprecation

import (
	"errors"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var (
	deprecatedAt = time.Date(2023, time.June, 30, 23, 59, 59, 0, time.UTC)
	sunsetAt     = time.Date(2024, time.June, 30, 23, 59, 59, 0, time.UTC)
)

func Test_Deprecation_Headers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		config      Config
		deprecation string
		sunset      string
		link        string
	}{
		{name: "deprecation only", config: Config{Deprecation: deprecatedAt}, deprecation: "@1688169599"},
		{name: "sunset only", config: Config{Sunset: sunsetAt}, sunset: "Sun, 30 Jun 2024 23:59:59 GMT"},
		{
			name:        "deprecation and sunset",
			config:      Config{Deprecation: deprecatedAt, Sunset: sunsetAt},
			deprecation: "@1688169599", sunset: "Sun, 30 Jun 2024 23:59:59 GMT",
		},
		{
			name:   "sunset in another time zone",
			config: Config{Sunset: sunsetAt.In(time.FixedZone("UTC+3", 3*60*60))},
			sunset: "Sun, 30 Jun 2024 23:59:59 GMT",
		},
		{
			name:        "deprecation link",
			config:      Config{Deprecation: deprecatedAt, Link: "https://example.com/deprecation"},
			deprecation: "@1688169599", link: `<https://example.com/deprecation>; rel="deprecation"`,
		},
		{
			name: "both links",
			config: Config{
				Deprecation: deprecatedAt, Sunset: sunsetAt,
				Link: "https://example.com/d", SunsetLink: "https://example.com/s",
			},
			deprecation: "@1688169599", sunset: "Sun, 30 Jun 2024 23:59:59 GMT",
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
			name: "sub-second dates in the same second",
			config: Config{
				Deprecation: sunsetAt.Add(900 * time.Millisecond), Sunset: sunsetAt.Add(100 * time.Millisecond),
			},
			deprecation: "@1719791999", sunset: "Sun, 30 Jun 2024 23:59:59 GMT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(tt.config))
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

			resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "ok")
			require.Equal(t, tt.deprecation, resp.Header.Get(fiber.HeaderDeprecation))
			require.Equal(t, tt.sunset, resp.Header.Get(fiber.HeaderSunset))
			require.Equal(t, tt.link, resp.Header.Get(fiber.HeaderLink))
		})
	}
}

func Test_Deprecation_KeepsExistingLinks(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Links("https://example.com/next", "next")
		return c.Next()
	})
	app.Use(New(Config{Deprecation: deprecatedAt, Link: "https://example.com/deprecation"}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	resp := apptest.Expect(t, app, "/", fiber.StatusNoContent)
	require.Equal(t,
		`<https://example.com/next>; rel="next",<https://example.com/deprecation>; rel="deprecation"`,
		resp.Header.Get(fiber.HeaderLink))
}

func Test_Deprecation_ErrorResponse(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Deprecation: deprecatedAt, Sunset: sunsetAt}))
	app.Get("/", func(fiber.Ctx) error { return errors.New("boom") })

	resp := apptest.Expect(t, app, "/", fiber.StatusInternalServerError)
	require.Equal(t, "@1688169599", resp.Header.Get(fiber.HeaderDeprecation))
	require.Equal(t, "Sun, 30 Jun 2024 23:59:59 GMT", resp.Header.Get(fiber.HeaderSunset))
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

	resp := apptest.ExpectBody(t, app, "/skip", fiber.StatusOK, "ok")
	require.Empty(t, resp.Header.Get(fiber.HeaderDeprecation))
	require.Empty(t, resp.Header.Get(fiber.HeaderLink))
}

func Test_Deprecation_InvalidConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		msg  string
		cfg  []Config
	}{
		{name: "no config", msg: "deprecation: Deprecation, Sunset, Link or SunsetLink is required"},
		{
			name: "empty config", msg: "deprecation: Deprecation, Sunset, Link or SunsetLink is required",
			cfg: []Config{{}},
		},
		{
			name: "sunset before deprecation", msg: "deprecation: Sunset must not be earlier than Deprecation",
			cfg: []Config{{Deprecation: sunsetAt, Sunset: deprecatedAt}},
		},
		{
			name: "relative link", msg: "deprecation: Link must be an absolute URI",
			cfg: []Config{{Link: "/docs/deprecation"}},
		},
		{
			name: "link that ends the target early", msg: "deprecation: Link must be an absolute URI",
			cfg: []Config{{Link: `https://x>; rel="preload"`}},
		},
		{
			name: "link with a space", msg: "deprecation: Link must be an absolute URI",
			cfg: []Config{{Link: "https://x/a b"}},
		},
		{
			name: "relative sunset link", msg: "deprecation: SunsetLink must be an absolute URI",
			cfg: []Config{{Sunset: sunsetAt, SunsetLink: "sunset"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, tt.msg, func() { New(tt.cfg...) })
		})
	}
}

func Test_Deprecation_SunsetEqualsDeprecation(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Deprecation: sunsetAt, Sunset: sunsetAt}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })

	resp := apptest.ExpectBody(t, app, "/", fiber.StatusOK, "ok")
	require.Equal(t, "@1719791999", resp.Header.Get(fiber.HeaderDeprecation))
	require.Equal(t, "Sun, 30 Jun 2024 23:59:59 GMT", resp.Header.Get(fiber.HeaderSunset))
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
		// Links appends to the Link header, so each run starts from a fresh response.
		fctx.Response.Reset()
		handler(fctx)
	}

	require.Equal(b, "@1688169599", string(fctx.Response.Header.Peek(fiber.HeaderDeprecation)))
}
