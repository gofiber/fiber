package expvar

import (
	"bytes"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

// expvarTestApp returns an app that serves "escaped" at / behind the middleware.
func expvarTestApp() *fiber.App {
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("escaped") })
	return app
}

func Test_Non_Expvar_Path(t *testing.T) {
	t.Parallel()
	apptest.ExpectBody(t, expvarTestApp(), "/", 200, "escaped")
}

// Test_Expvar_Vars covers the full index and the index narrowed by the r query.
func Test_Expvar_Vars(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		target  string
		memstat bool // whether memstat is part of the output
	}{
		{name: "index", target: "/debug/vars", memstat: true},
		{name: "filter", target: "/debug/vars?r=cmd", memstat: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := apptest.Expect(t, expvarTestApp(), tc.target, 200)
			require.Equal(t, fiber.MIMEApplicationJSONCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

			b := apptest.BodyBytes(t, resp)
			require.True(t, bytes.Contains(b, []byte("cmdline")))
			require.Equal(t, tc.memstat, bytes.Contains(b, []byte("memstat")))
		})
	}
}

func Test_Expvar_Other_Path(t *testing.T) {
	t.Parallel()
	apptest.Expect(t, expvarTestApp(), "/debug/vars/303", fiber.StatusSeeOther)
}

func Test_Expvar_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))
	apptest.Expect(t, app, "/debug/vars", 404)
}

// Test_Expvar_Sibling_Prefix_Path ensures a route that merely starts with the
// same characters as /debug/vars is not swallowed by the middleware.
func Test_Expvar_Sibling_Prefix_Path(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/debug/varsdump", func(c fiber.Ctx) error { return c.SendString("app route") })

	apptest.ExpectBody(t, app, "/debug/varsdump", fiber.StatusOK, "app route")
}
