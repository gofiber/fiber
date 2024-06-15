package session

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestNewWithStore(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		return c.SendString("value=" + id)
	})
	app.Post("/", func(c fiber.Ctx) error {
		sess := FromContext(c)
		id := sess.ID()
		c.Cookie(&fiber.Cookie{
			Name:  "session_id",
			Value: id,
		})
		return nil
	})

	h := app.Handler()

	// Test GET request without cookie
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	// Get session cookie
	token := string(ctx.Response.Header.Peek(fiber.HeaderSetCookie))
	token = strings.Split(strings.Split(token, ";")[0], "=")[1]
	require.Equal(t, "value="+token, string(ctx.Response.Body()))

	// Test GET request with cookie
	ctx = &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fiber.MethodGet)
	ctx.Request.Header.SetCookie("session_id", token)
	h(ctx)
	require.Equal(t, 200, ctx.Response.StatusCode())
	require.Equal(t, "value="+token, string(ctx.Response.Body()))
}
