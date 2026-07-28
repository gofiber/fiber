package idempotency_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/idempotency"
	"github.com/stretchr/testify/require"
)

// The response is stored after the handler returns, so a handler's own deferred
// cancel must not reach storage through Ctx.Err().
func Test_Idempotency_Store_After_Handler_Cancel(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(idempotency.New())

	var calls int
	app.Post("/", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), time.Hour)
		defer cancel()
		c.SetContext(ctx)

		calls++
		return c.SendString("response")
	})

	do := func() string {
		req := httptest.NewRequest(fiber.MethodPost, "/", http.NoBody)
		req.Header.Set("X-Idempotency-Key", "00000000-0000-0000-0000-000000000000")
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(body)
	}

	require.Equal(t, "response", do())
	require.Equal(t, "response", do())
	require.Equal(t, 1, calls, "second request was not served from the idempotency cache")
}
