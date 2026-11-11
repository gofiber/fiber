package proxy

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

// authEchoUpstream answers with the Authorization field of the request, "-" when it has none.
func authEchoUpstream(t *testing.T) string {
	t.Helper()
	upstream := fiber.New()
	upstream.All("/*", func(c fiber.Ctx) error {
		if c.Get(fiber.HeaderAuthorization) == "" {
			return c.SendString("-")
		}
		return c.SendString(c.Get(fiber.HeaderAuthorization))
	})
	return listenUpstream(t, upstream)
}

func Test_Proxy_Do_LeavesAuthorizationAsReceived(t *testing.T) {
	t.Parallel()
	const basic = "Basic Z2F0ZXdheTpwdw==" // gateway:pw
	checkAddr := authEchoUpstream(t)
	nextAddr := authEchoUpstream(t)
	redirectAddr := proxyTestRedirectUpstream(t, "http://other:pw2@"+nextAddr+"/final")

	for _, tc := range []struct {
		name string
		call func(c fiber.Ctx, addr string) error
		addr string
		// seen is what the call itself reaches when the client sent no Authorization
		seen string
	}{
		{"Do", func(c fiber.Ctx, a string) error { return Do(c, a) }, "http://gateway:pw@" + checkAddr + "/", basic},
		{"DoTimeout", func(c fiber.Ctx, a string) error { return DoTimeout(c, a, time.Second) }, "http://gateway:pw@" + checkAddr + "/", basic},
		{"DoDeadline", func(c fiber.Ctx, a string) error { return DoDeadline(c, a, time.Now().Add(time.Second)) }, "http://gateway:pw@" + checkAddr + "/", basic},
		{"DoRedirects", func(c fiber.Ctx, a string) error { return DoRedirects(c, a, 3) }, "http://gateway:pw@" + checkAddr + "/", basic},
		{"DoRedirects_location_user_info", func(c fiber.Ctx, a string) error { return DoRedirects(c, a, 3) }, "http://gateway:pw@" + redirectAddr + "/", "-"},
	} {
		for _, client := range []string{"", "Bearer client-token"} {
			t.Run(tc.name+"/client="+client, func(t *testing.T) {
				t.Parallel()
				app := fiber.New()
				var seen, after string
				app.Use(func(c fiber.Ctx) error {
					if err := tc.call(c, tc.addr); err != nil {
						return err
					}
					seen = string(c.Response().Body())
					after = c.Get(fiber.HeaderAuthorization)
					return c.Next()
				})
				app.Use(Forward("http://" + nextAddr))

				req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeaderIf(fiber.HeaderAuthorization, client))
				resp := apptest.Expect(t, app, req, fiber.StatusOK)

				want := "-"
				if client != "" {
					want = client
				}
				// fasthttp fills the field from user info only when the client sent none.
				wantSeen := tc.seen
				switch {
				case tc.name == "DoRedirects_location_user_info":
					wantSeen = "-" // the redirect leaves the origin: nothing follows it
				case client != "":
					wantSeen = client
				}
				require.Equal(t, wantSeen, seen)
				require.Equal(t, client, after, "the request keeps the Authorization it was received with")
				require.Equal(t, want, apptest.Body(t, resp), "the next upstream gets only the client's own")
			})
		}
	}
}
