package fiber

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestValueFromContext(t *testing.T) {
	t.Parallel()

	t.Run("fiber.Ctx", func(t *testing.T) {
		t.Parallel()

		app := New()
		raw := &fasthttp.RequestCtx{}
		c := app.AcquireCtx(raw)
		defer app.ReleaseCtx(c)

		c.Locals("key", "value")

		value, ok := ValueFromContext[string](c, "key")
		require.True(t, ok)
		require.Equal(t, "value", value)
	})

	t.Run("fiber.CustomCtx", func(t *testing.T) {
		t.Parallel()

		app := NewWithCustomCtx(func(app *App) CustomCtx {
			return &customCtx{DefaultCtx: *NewDefaultCtx(app)}
		})
		raw := &fasthttp.RequestCtx{}
		c := app.AcquireCtx(raw)
		defer app.ReleaseCtx(c)

		c.Locals("key", "value")

		value, ok := ValueFromContext[string](c, "key")
		require.True(t, ok)
		require.Equal(t, "value", value)
	})

	t.Run("fasthttp request ctx", func(t *testing.T) {
		t.Parallel()

		raw := &fasthttp.RequestCtx{}
		raw.SetUserValue("key", "value")

		value, ok := ValueFromContext[string](raw, "key")
		require.True(t, ok)
		require.Equal(t, "value", value)
	})

	t.Run("context.Context", func(t *testing.T) {
		t.Parallel()

		type testContextKey struct{}

		ctx := context.WithValue(context.Background(), testContextKey{}, "value")

		value, ok := ValueFromContext[string](ctx, testContextKey{})
		require.True(t, ok)
		require.Equal(t, "value", value)
	})

	t.Run("unsupported ctx", func(t *testing.T) {
		t.Parallel()

		value, ok := ValueFromContext[string](42, "key")
		require.False(t, ok)
		require.Empty(t, value)
	})
}
