package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Proxy_DoRedirects_KeepsTheEscapesOfTheTarget(t *testing.T) {
	t.Parallel()
	final := echoTarget(t)
	// a canonical-host redirect: the Location repeats the request URI
	first := fiber.New()
	first.Use(func(c fiber.Ctx) error {
		c.Set(fiber.HeaderLocation, "http://"+final+c.OriginalURL())
		return c.SendStatus(fiber.StatusPermanentRedirect)
	})
	firstAddr := listenUpstream(t, first)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { return DoRedirects(c, "http://"+firstAddr+c.Path(), 3) })

	for target, want := range map[string]string{
		"/public/..%2Fadmin/x":  "/public/..%2Fadmin/x",
		`/pub"lic/..%2Fadmin/x`: "/pub%22lic/..%2Fadmin/x",
		"/pub|lic/..%2Fadmin/x": "/pub%7Clic/..%2Fadmin/x",
		"/a//b/%2e%2e%2Fc":      "/a//b/..%2Fc",
		"/a/%2Fb":               "/a/%2Fb",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			resp := rawRequest(t, app, target, "example.com")
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, want, string(resp.Body()))
		})
	}
}

func Test_Proxy_DoRedirects_ResolvesRelativeLocations(t *testing.T) {
	t.Parallel()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location := map[string]string{
			"/dir/sub/up":    "../x",
			"/dir/sub/abs":   "/abs",
			"/dir/sub/query": "?q=1",
			"/dir/sub/dot":   "./y",
			"/dir/sub/net":   "//" + strings.TrimPrefix(server.URL, "http://") + "/net",
			"/dir/sub/full":  server.URL + "/full?a=%2F",
			"/dir/sub/pct":   "/discount/100%",
		}[r.URL.Path]
		if location == "" || r.URL.RawQuery != "" && r.URL.Path == "/dir/sub/query" {
			_, err := io.WriteString(w, r.RequestURI)
			assert.NoError(t, err)
			return
		}
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { return DoRedirects(c, server.URL+c.Path(), 3) })

	for target, want := range map[string]string{
		"/dir/sub/up":    "/dir/x",
		"/dir/sub/abs":   "/abs",
		"/dir/sub/query": "/dir/sub/query?q=1",
		"/dir/sub/dot":   "/dir/sub/y",
		"/dir/sub/net":   "/net",
		"/dir/sub/full":  "/full?a=%2F",
		"/dir/sub/pct":   "/discount/100%25",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			apptest.ExpectBody(t, app, target, fiber.StatusOK, want)
		})
	}
}

// The response a chain ends with is no more cacheable than the redirects that led to it.
func Test_Proxy_FollowRedirects_CarriesRestrictiveCaching(t *testing.T) {
	t.Parallel()
	type hop struct {
		location string
		fields   [][2]string
	}
	cases := []struct {
		hops         map[string]hop
		name         string
		cacheControl string
		vary         string
		status       int
	}{
		{name: "bare directives and Vary", status: fasthttp.StatusOK, hops: map[string]hop{
			"/start": {location: "/next", fields: [][2]string{
				{"Cache-Control", "No-Store, max-age=0"}, {"Cache-Control", `private="Set-Cookie"`}, {"Vary", "Authorization"},
			}},
			"/next":  {location: "/final", fields: [][2]string{{"Cache-Control", `no-cache="Set-Cookie", private`}, {"Vary", "Accept-Language"}}},
			"/final": {fields: [][2]string{{"Cache-Control", "public, max-age=60"}, {"Vary", "accept-encoding, authorization"}}},
		}, cacheControl: "public, max-age=60, no-store, private", vary: "accept-encoding, authorization, Accept-Language"},
		{name: "a Vary of *", status: fasthttp.StatusOK, hops: map[string]hop{
			"/start": {location: "/final", fields: [][2]string{{"Vary", "*"}}},
			"/final": {fields: [][2]string{{"Vary", "Accept"}}},
		}, vary: "*"},
		{name: "a redirect that goes back", status: fasthttp.StatusFound, hops: map[string]hop{
			"/start": {location: "/next", fields: [][2]string{{"Cache-Control", "no-cache"}}},
			"/next":  {location: "mailto:user@example.com"},
		}, cacheControl: "no-cache"},
		{name: "a directive inside a quoted argument", status: fasthttp.StatusOK, hops: map[string]hop{
			"/start": {location: "/final", fields: [][2]string{{"Cache-Control", "no-store"}}},
			"/final": {fields: [][2]string{{"Cache-Control", `public, max-age=60, ext="a, no-store, b"`}}},
		}, cacheControl: `public, max-age=60, ext="a, no-store, b", no-store`},
		{name: "a quoted argument on a redirect", status: fasthttp.StatusOK, hops: map[string]hop{
			"/start": {location: "/final", fields: [][2]string{{"Cache-Control", `ext="a, no-store, b"`}}},
			"/final": {fields: [][2]string{{"Cache-Control", "max-age=5"}}},
		}, cacheControl: "max-age=5"},
		{name: "nothing restrictive", status: fasthttp.StatusOK, hops: map[string]hop{
			"/start": {location: "/final", fields: [][2]string{{"Cache-Control", "public, max-age=60"}}},
			"/final": {fields: [][2]string{{"Cache-Control", "max-age=5"}}},
		}, cacheControl: "max-age=5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &fasthttp.Client{Transport: roundTripperFunc(func(req *fasthttp.Request, resp *fasthttp.Response) error {
				step := tc.hops[string(req.URI().Path())]
				resp.Reset()
				resp.Header.SetStatusCode(fasthttp.StatusOK)
				if step.location != "" {
					resp.Header.SetStatusCode(fasthttp.StatusFound)
					resp.Header.Set(fiber.HeaderLocation, step.location)
				}
				for _, f := range step.fields {
					resp.Header.Add(f[0], f[1])
				}
				return nil
			})}
			req, resp := proxyTestRequest(t), proxyTestResponse(t)
			req.SetRequestURI("http://example.com/start")

			start := mustParseTestURL(t, "http://example.com/start")
			var carried hopCaching
			require.NoError(t, followRedirects(client, req, resp, 3, start, proxyTestAllowPrivatePolicy(), &carried))
			carried.apply(&resp.Header)
			require.Equal(t, tc.status, resp.StatusCode())
			require.Equal(t, tc.cacheControl, string(resp.Header.Peek(fiber.HeaderCacheControl)))
			require.Equal(t, tc.vary, string(resp.Header.Peek(fiber.HeaderVary)))
			require.Len(t, resp.Header.PeekAll(fiber.HeaderCacheControl), min(1, len(tc.cacheControl)))
		})
	}
}

// The restrictions go on once the upstream's hop-by-hop fields are gone: a final response listing
// Cache-Control and Vary in Connection still goes out with those the redirect carried.
func Test_Proxy_DoRedirects_CarriesRestrictiveCachingPastConnection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, head string
		status     int
	}{
		{name: "a final response", head: "HTTP/1.1 200 OK", status: fiber.StatusOK},
		{name: "a redirect that goes back", head: "HTTP/1.1 302 Found\r\nLocation: mailto:user@example.com", status: fiber.StatusFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Not a Fiber app: the fields go out exactly as written.
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, _, err := http.NewResponseController(w).Hijack()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				defer conn.Close() //nolint:errcheck // It is fine to ignore the error here
				if r.URL.Path == "/start" {
					fmt.Fprint(conn, "HTTP/1.1 302 Found\r\nLocation: /final\r\nCache-Control: no-store\r\nVary: Cookie\r\nConnection: close\r\nContent-Length: 0\r\n\r\n")
					return
				}
				fmt.Fprint(conn, tc.head+"\r\nConnection: close, Cache-Control, Vary\r\nCache-Control: public, max-age=60\r\nVary: Accept\r\nContent-Length: 2\r\n\r\nok")
			}))
			t.Cleanup(upstream.Close)

			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error { return DoRedirects(c, upstream.URL+"/start", 3) })
			resp := apptest.Expect(t, app, "/", tc.status)
			require.Equal(t, []string{"no-store"}, resp.Header.Values(fiber.HeaderCacheControl))
			require.Equal(t, []string{"Cookie"}, resp.Header.Values(fiber.HeaderVary))
		})
	}
}
