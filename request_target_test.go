package fiber

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// serveRaw runs app on the request raw, read off the wire, and returns the answer's body.
func serveRaw(t *testing.T, app *App, raw string) (status int, body string) { //nolint:nonamedreturns // names tell the two results apart
	t.Helper()
	fctx := wireRequestCtx(t, raw, nil)
	app.Handler()(fctx)
	return fctx.Response.StatusCode(), string(fctx.Response.Body())
}

func Test_Ctx_RequestTarget_OriginForm(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error {
		return c.SendString(c.Host() + "|" + c.Hostname() + "|" + c.Path() + "|" + c.OriginalURL() + "|" + c.BaseURL() + "|" + c.Query("x"))
	})

	for _, tc := range []struct{ name, raw, want string }{
		// a target that begins with "/" is a path whatever its query holds (RFC 9112 Section 3.2.1)
		{
			"query with ://", "GET //internal.example.com/data?x=:// HTTP/1.1\r\nHost: other.example\r\n\r\n",
			"other.example|other.example|//internal.example.com/data|//internal.example.com/data?x=://|http://other.example|://",
		},
		{
			"query with a URL", "GET //internal.example.com/data?next=http://a HTTP/1.1\r\nHost: other.example\r\n\r\n",
			"other.example|other.example|//internal.example.com/data|//internal.example.com/data?next=http://a|http://other.example|",
		},
		{
			"fragment with ://", "GET //internal.example.com/data#:// HTTP/1.1\r\nHost: other.example\r\n\r\n",
			"other.example|other.example|//internal.example.com/data|//internal.example.com/data#://|http://other.example|",
		},
		{
			"path with ://", "GET //a/b://c HTTP/1.1\r\nHost: other.example:8080\r\n\r\n",
			"other.example:8080|other.example|//a/b://c|//a/b://c|http://other.example:8080|",
		},
		{"HTTP/1.0 without Host", "GET //admin/x HTTP/1.0\r\n\r\n", "||//admin/x|//admin/x|http://|"},
		// controls
		{
			"plain target", "GET /data HTTP/1.1\r\nHost: other.example\r\n\r\n",
			"other.example|other.example|/data|/data|http://other.example|",
		},
		{
			"plain target with ://", "GET /data?x=:// HTTP/1.1\r\nHost: other.example\r\n\r\n",
			"other.example|other.example|/data|/data?x=://|http://other.example|://",
		},
		{
			"double slash without ://", "GET //a/b HTTP/1.1\r\nHost: other.example\r\n\r\n",
			"other.example|other.example|//a/b|//a/b|http://other.example|",
		},
		{
			"absolute-form", "GET http://internal.example.com/data?x=1 HTTP/1.1\r\nHost: other.example\r\n\r\n",
			"internal.example.com|internal.example.com|/data|http://internal.example.com/data?x=1|http://internal.example.com|1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, body := serveRaw(t, app, tc.raw)
			require.Equal(t, StatusOK, status)
			require.Equal(t, tc.want, body)
		})
	}
}

func Test_Redirect_Back_RequestTargetWithAuthorityLookalike(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error { return c.Redirect().Back("/home") })

	for _, tc := range []struct{ name, target, want string }{
		{"lookalike", "//other.example/back?x=://", "/home"},
		{"plain", "/back?x=://", "/home"},
		{"same origin", "/back", "http://other.example/page"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			host := "victim.example"
			if tc.name == "same origin" {
				host = "other.example"
			}
			fctx := wireRequestCtx(t, "GET "+tc.target+" HTTP/1.1\r\nHost: "+host+"\r\nReferer: http://other.example/page\r\n\r\n", nil)
			app.Handler()(fctx)
			require.Equal(t, StatusSeeOther, fctx.Response.StatusCode())
			require.Equal(t, tc.want, string(fctx.Response.Header.Peek(HeaderLocation)))
		})
	}
}

// Test_Ctx_RequestTarget_OriginForm_AfterSendFile pins that SendFile, which puts the request line back once it has
// served the file, leaves it read as the path it is: the host stays the Host header.
func Test_Ctx_RequestTarget_OriginForm_AfterSendFile(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "f.txt")
	require.NoError(t, os.WriteFile(file, []byte("f"), 0o600))

	for _, tc := range []struct{ target, path string }{
		{"//internal.example.com/data?x=://", "//internal.example.com/data"},
		{"//internal.example.com/data#://", "//internal.example.com/data"},
		{"//a/b://c", "//a/b://c"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()
			app := New()
			var before, after string
			app.Use(func(c Ctx) error {
				before = c.Host() + "|" + c.Hostname() + "|" + c.Path() + "|" + c.OriginalURL()
				if err := c.SendFile(file); err != nil {
					return err
				}
				after = c.Host() + "|" + c.Hostname() + "|" + c.Path() + "|" + c.OriginalURL()
				return nil
			})
			status, body := serveRaw(t, app, "GET "+tc.target+" HTTP/1.1\r\nHost: other.example\r\n\r\n")
			require.Equal(t, StatusOK, status)
			require.Equal(t, "f", body)
			require.Equal(t, "other.example|other.example|"+tc.path+"|"+tc.target, before)
			require.Equal(t, before, after)
		})
	}
}

// Test_Ctx_RequestTarget_OriginForm_InvalidHost pins that a Host header fasthttp cannot parse names no host: the
// request has none, as without the header, and the first segment of its path is not taken for one (fasthttp answers
// 400 to a plain target with such a header). The request is not blanked either: it is routed as the path it is.
func Test_Ctx_RequestTarget_OriginForm_InvalidHost(t *testing.T) {
	t.Parallel()
	app := New()
	app.Use(func(c Ctx) error { return c.SendString(c.Host() + "|" + c.Path() + "|" + c.OriginalURL()) })

	for _, host := range []string{"bad[host", "a:b:c", "example.com:abc"} {
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			status, body := serveRaw(t, app, "GET //admin/x?a=:// HTTP/1.1\r\nHost: "+host+"\r\n\r\n")
			require.Equal(t, StatusOK, status)
			require.Equal(t, "|//admin/x|//admin/x?a=://", body)
		})
	}
}
