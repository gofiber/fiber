package adaptor

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// Test_FiberApp_ReadsAnOriginFormTargetAsAPath pins that a request target that begins with "//" is a path
// (RFC 9112 Section 3.2.1) whichever way the request reached Fiber: the net/http adapters build the request
// before they set its Host header, and fasthttp then takes the first segment of such a target for the authority.
// The context must see the path and the host that Fiber's own server gives it.
func Test_FiberApp_ReadsAnOriginFormTargetAsAPath(t *testing.T) {
	t.Parallel()
	describe := func(c fiber.Ctx) error { return c.SendString(c.Path() + "|" + c.Host() + "|" + c.OriginalURL()) }
	app := fiber.New()
	app.Use(describe)
	for _, target := range []string{
		"//x/admin/data", "//x/admin/data?q=a", "//x/admin/data#frag", "//x/admin/data?q=://", "//x/admin/data#a://b",
		"///x/admin/data", "//x", "//u:pw@other.example/admin/data", "//a:b/admin/data",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			want := string(rawCtx(t, app, target).Response.Body())
			require.Contains(t, want, "example.com", "the host of a path target is the Host header")
			for name, h := range map[string]http.Handler{
				"FiberApp": FiberApp(app), "FiberHandler": FiberHandler(describe), "FiberHandlerFunc": FiberHandlerFunc(describe),
			} {
				req, err := http.ReadRequest(bufio.NewReader(strings.NewReader("GET " + target + " HTTP/1.1\r\nHost: example.com\r\n\r\n")))
				require.NoError(t, err)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				require.Equal(t, want, rec.Body.String(), name)
			}
		})
	}
}
