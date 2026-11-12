package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Proxy_DoRedirects_KeepsTheEscapesOfTheTarget(t *testing.T) {
	t.Parallel()
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, r.RequestURI)
		assert.NoError(t, err)
	}))
	t.Cleanup(final.Close)
	// a canonical-host redirect: the Location repeats the request URI
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", final.URL+r.RequestURI)
		w.WriteHeader(http.StatusPermanentRedirect)
	}))
	t.Cleanup(first.Close)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error { return DoRedirects(c, first.URL+c.Path(), 3) })
	ln := apptest.Serve(t, app, fiber.ListenConfig{DisableStartupMessage: true})

	for target, want := range map[string]string{
		"/public/..%2Fadmin/x":  "/public/..%2Fadmin/x",
		`/pub"lic/..%2Fadmin/x`: "/pub%22lic/..%2Fadmin/x",
		"/pub|lic/..%2Fadmin/x": "/pub%7Clic/..%2Fadmin/x",
		"/a//b/%2e%2e%2Fc":      "/a//b/..%2Fc",
		"/a/%2Fb":               "/a/%2Fb",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			head, body := getRawTarget(t, ln, target)
			require.Contains(t, head, "200 OK")
			require.Equal(t, want, body)
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
