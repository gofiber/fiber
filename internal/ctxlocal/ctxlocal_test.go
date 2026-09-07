package ctxlocal

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// recordingCtx is a custom context whose Locals is observable, so the interface
// fallback in Set can be told apart from the concrete fast path.
type recordingCtx struct {
	fiber.DefaultCtx
	calls int
}

func (c *recordingCtx) Locals(key any, value ...any) any {
	c.calls++
	return c.DefaultCtx.Locals(key, value...)
}

// Test_Set_UsesCustomCtxLocals pins the fallback: the concrete *DefaultCtx path
// exists only to keep the variadic slice off the heap and must never take
// precedence over a custom context's own Locals.
func Test_Set_UsesCustomCtxLocals(t *testing.T) {
	t.Parallel()

	app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
		return &recordingCtx{DefaultCtx: *fiber.NewDefaultCtx(app)}
	})
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	custom, ok := c.(*recordingCtx)
	require.True(t, ok, "the app must hand out the custom context")

	require.Equal(t, "v", Set(c, "k", "v"), "Set returns what Locals returned")
	require.Equal(t, 1, custom.calls, "the overridden Locals must be the one that ran")
	require.Equal(t, "v", c.Locals("k"), "and the value must actually be stored")
}

// Test_Set_UsesDefaultCtxDirectly is the other half: the default context takes
// the concrete path, and the value is stored and returned unchanged.
func Test_Set_UsesDefaultCtxDirectly(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	_, isDefault := c.(*fiber.DefaultCtx)
	require.True(t, isDefault, "the default app must hand out *DefaultCtx")

	require.Equal(t, 42, Set(c, "n", 42))
	require.Equal(t, 42, c.Locals("n"))
	require.Nil(t, Set(c, "n", nil), "a nil value clears the local")
	require.Nil(t, c.Locals("n"))
}

func Benchmark_Set(b *testing.B) {
	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	b.ReportAllocs()
	for b.Loop() {
		Set(c, "k", true)
	}
}
