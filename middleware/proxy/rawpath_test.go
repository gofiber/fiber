package proxy

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

func Test_Proxy_Do_KeepsEscapedSlashBesideBytesAPathCannotHold(t *testing.T) {
	t.Parallel()
	base := "http://" + echoTarget(t)

	app := fiber.New()
	app.Use("/admin", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) })
	app.Use("/do", func(c fiber.Ctx) error { return Do(c, base+strings.TrimPrefix(c.Path(), "/do")) })
	app.Use("/timeout", func(c fiber.Ctx) error { return DoTimeout(c, base+strings.TrimPrefix(c.Path(), "/timeout"), 5e9) })
	app.Use("/redirects", func(c fiber.Ctx) error { return DoRedirects(c, base+strings.TrimPrefix(c.Path(), "/redirects"), 3) })

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
				resp := rawRequest(t, app, mount+"/pub"+tc.raw+"lic/..%2Fadmin/x", "example.com")
				require.Equal(t, fiber.StatusOK, resp.StatusCode())
				require.Equal(t, "/pub"+tc.want+"lic/..%2Fadmin/x", string(resp.Body()))
			})
		}
	}
}

func Test_Proxy_BalancerForward_KeepsEscapedSlashInTheBasePath(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(BalancerForward([]string{"http://" + echoTarget(t) + `/pre|fix%2Fx`}))
	apptest.ExpectBody(t, app, "/y", fiber.StatusOK, "/pre%7Cfix%2Fx/y")
}
