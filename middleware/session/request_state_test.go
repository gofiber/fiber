package session

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Whether an app passes the request locals to the request context decides what a
// middleware must hide when it finishes, and whether it may be recycled. A store
// can serve several apps, so the answer it remembers belongs to one app only.
func Test_Store_PassesLocalsToContext_FollowsTheApp(t *testing.T) {
	t.Parallel()

	store := NewStore()
	off := fiber.New()
	on := fiber.New(fiber.Config{PassLocalsToContext: true})
	cOff := off.AcquireCtx(&fasthttp.RequestCtx{})
	defer off.ReleaseCtx(cOff)
	cOn := on.AcquireCtx(&fasthttp.RequestCtx{})
	defer on.ReleaseCtx(cOn)

	for range 3 {
		require.False(t, store.passesLocalsToContext(cOff))
		require.True(t, store.passesLocalsToContext(cOn), "the answer of the other app was reused")
		require.True(t, store.passesLocalsToContext(cOn))
		require.False(t, store.passesLocalsToContext(cOff), "the answer of the other app was reused")
	}
}

// A middleware that finished holds neither the store nor the error handler it ran
// with: a handler that outlives the request finds none, and a recycled one does not
// keep a closure of the application alive until its next request.
func Test_Session_Middleware_ReleaseForgetsTheStoreAndTheErrorHandler(t *testing.T) {
	t.Parallel()

	app := fiber.New()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(c)

	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid")})
	cfg := store.Config
	cfg.Store = store
	cfg.ErrorHandler = func(fiber.Ctx, error) {}

	m := acquireMiddleware()
	require.NoError(t, m.initialize(c, &cfg))
	require.Same(t, store, m.Store())
	require.NotNil(t, m.errorHandler)

	// Dropped, not recycled, so that the test can look at it afterwards.
	m.escaped = true
	releaseMiddleware(m)

	require.Nil(t, m.Store())
	require.Nil(t, m.errorHandler)
}
