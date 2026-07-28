package session

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// The middleware saves after the handler returns, so a handler's own deferred
// cancel must not reach storage through Ctx.Err().
func Test_Session_Save_After_Handler_Cancel(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	app.Get("/set", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), time.Hour)
		defer cancel()
		c.SetContext(ctx)

		FromContext(c).Set("hello", "world")
		return nil
	})
	app.Get("/get", func(c fiber.Ctx) error {
		v, ok := FromContext(c).Get("hello").(string)
		if !ok {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendString(v)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/set", http.NoBody))
	require.NoError(t, err)
	cookie := resp.Header.Get("Set-Cookie")
	require.NotEmpty(t, cookie)

	req := httptest.NewRequest(fiber.MethodGet, "/get", http.NoBody)
	req.Header.Set("Cookie", cookie)
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "world", string(body))
}
