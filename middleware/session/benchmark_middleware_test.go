package session

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// go test -v -run=^$ -bench=Benchmark_Middleware_ReadOnly -benchmem -count=4
func Benchmark_Middleware_ReadOnly(b *testing.B) {
	app := fiber.New()
	app.Use(New())
	app.Post("/login", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return fiber.ErrInternalServerError
		}
		sess.Set("uid", "1337")
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Get("/me", func(c fiber.Ctx) error {
		sess := FromContext(c)
		if sess == nil {
			return fiber.ErrInternalServerError
		}
		uid, ok := sess.Get("uid").(string)
		if !ok || uid != "1337" {
			return fiber.ErrUnauthorized
		}
		return c.SendString(uid)
	})

	h := app.Handler()

	// log in once and reuse the returned session cookie
	login := &fasthttp.RequestCtx{}
	login.Request.Header.SetMethod(fiber.MethodPost)
	login.Request.SetRequestURI("/login")
	h(login)
	if login.Response.StatusCode() != fiber.StatusNoContent {
		b.Fatalf("login status %d", login.Response.StatusCode())
	}
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey("session_id")
	if !login.Response.Header.Cookie(cookie) {
		b.Fatal("login response carries no session cookie")
	}

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/me")
	fctx.Request.Header.SetCookie("session_id", string(cookie.Value()))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// fasthttp clears user values between requests on a reused ctx
		fctx.ResetUserValues()
		fctx.Response.Reset()
		h(fctx)
		if fctx.Response.StatusCode() != fiber.StatusOK {
			b.Fatalf("status %d", fctx.Response.StatusCode())
		}
	}
}
