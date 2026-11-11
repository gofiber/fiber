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
	"github.com/valyala/fasthttp/fasthttputil"
)

// requestURIUpstream answers 200 with the request target it received, exactly as it arrived.
func requestURIUpstream(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, r.RequestURI)
		assert.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func Test_Proxy_Do_KeepsEscapedSlashBesideBytesAPathCannotHold(t *testing.T) {
	t.Parallel()
	base := requestURIUpstream(t)

	app := fiber.New()
	app.Use("/admin", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) })
	app.Use("/do", func(c fiber.Ctx) error { return Do(c, base+strings.TrimPrefix(c.Path(), "/do")) })
	app.Use("/timeout", func(c fiber.Ctx) error { return DoTimeout(c, base+strings.TrimPrefix(c.Path(), "/timeout"), 5e9) })
	app.Use("/redirects", func(c fiber.Ctx) error { return DoRedirects(c, base+strings.TrimPrefix(c.Path(), "/redirects"), 3) })

	ln := apptest.Serve(t, app, fiber.ListenConfig{DisableStartupMessage: true})
	for _, mount := range []string{"/do", "/timeout", "/redirects"} {
		for _, tc := range []struct{ raw, want string }{
			{`"`, "%22"},
			{"|", "%7C"},
			{"<", "%3C"},
			{"^", "%5E"},
			{"`", "%60"},
			{"{", "%7B"},
			{"}", "%7D"},
			{"\xc3\xa9", "%C3%A9"},
			{"", ""}, // the control: nothing to escape
		} {
			t.Run(mount+"/"+tc.want, func(t *testing.T) {
				t.Parallel()
				head, body := getRawTarget(t, ln, mount+"/pub"+tc.raw+"lic/..%2Fadmin/x")
				require.Contains(t, head, "200 OK")
				require.Equal(t, "/pub"+tc.want+"lic/..%2Fadmin/x", body)
			})
		}
	}
}

// getRawTarget sends a GET for target as written to ln and returns the head and the body of the answer.
func getRawTarget(t *testing.T, ln *fasthttputil.InmemoryListener, target string) (head, body string) { //nolint:nonamedreturns // names tell the two results apart
	t.Helper()
	conn := apptest.Dial(t, ln)
	_, err := io.WriteString(conn, "GET "+target+" HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)
	raw, err := io.ReadAll(conn)
	require.NoError(t, err)
	head, body, _ = strings.Cut(string(raw), "\r\n\r\n")
	return head, body
}

func Test_Proxy_BalancerForward_KeepsEscapedSlashInTheBasePath(t *testing.T) {
	t.Parallel()
	base := requestURIUpstream(t)

	app := fiber.New()
	app.Use(BalancerForward([]string{base + `/pre|fix%2Fx`}))
	apptest.ExpectBody(t, app, "/y", fiber.StatusOK, "/pre%7Cfix%2Fx/y")
}
