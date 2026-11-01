package session

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/stretchr/testify/require"
)

// countingCtx counts how often the session middleware asks for the Go context of the
// request and replaces it.
type countingCtx struct {
	contexts *atomic.Int64
	sets     *atomic.Int64
	fiber.DefaultCtx
}

func (c *countingCtx) Context() context.Context {
	c.contexts.Add(1)
	return c.DefaultCtx.Context()
}

func (c *countingCtx) SetContext(ctx context.Context) {
	c.sets.Add(1)
	c.DefaultCtx.SetContext(ctx)
}

// Without PassLocalsToContext the request context never holds the session, so the
// middleware has nothing to store in it, ask it for, or hide in it at the end: it
// leaves the context alone, which spares Fiber the context it would create for the
// request and the wrapping of it, and with PassLocalsToContext it does all of that.
func Test_Session_Middleware_ContextIsLeftAloneWithoutPassLocalsToContext(t *testing.T) {
	t.Parallel()

	for _, pass := range []bool{false, true} {
		t.Run(fmt.Sprintf("pass=%t", pass), func(t *testing.T) {
			t.Parallel()

			var contexts, sets atomic.Int64
			app := fiber.NewWithCustomCtx(func(app *fiber.App) fiber.CustomCtx {
				return &countingCtx{DefaultCtx: *fiber.NewDefaultCtx(app), contexts: &contexts, sets: &sets}
			}, fiber.Config{PassLocalsToContext: pass})
			app.Use(New())
			app.Get("/", func(c fiber.Ctx) error {
				FromContext(c).Set("k", "v")
				return nil
			})

			resp, _ := sendRequest(t, app, "/", nil)
			require.Equal(t, fiber.StatusOK, resp.StatusCode)

			if pass {
				require.Positive(t, sets.Load(), "the middleware stores itself in the context, and hides itself in it at the end")
				require.Positive(t, contexts.Load())
			} else {
				require.Zero(t, sets.Load(), "the context was replaced though nothing is stored in it")
				require.Zero(t, contexts.Load(), "the context was asked for though nothing is stored in it")
			}
		})
	}
}

// With PassLocalsToContext a middleware that finished hides the session ID and
// itself behind empty values in the context of the request, for whoever asks for
// the context afterwards: an outer middleware that logs the session of the request,
// say. Both values are hidden, and nothing else.
func Test_Session_Middleware_FinishedMiddlewareIsHiddenInTheContext(t *testing.T) {
	t.Parallel()

	type otherKey struct{}

	var after context.Context
	app := fiber.New(fiber.Config{PassLocalsToContext: true})
	app.Use(func(c fiber.Ctx) error {
		c.SetContext(context.WithValue(c.Context(), otherKey{}, "kept")) //nolint:contextcheck // a middleware before the session one stores a value in the context
		err := c.Next()
		after = c.Context()
		return err
	})
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		require.NotEmpty(t, FromContext(c).ID())
		held, ok := c.Context().Value(middlewareContextKey).(*Middleware)
		require.True(t, ok)
		require.NotNil(t, held, "the middleware is in the context while it runs")
		return nil
	})

	resp, _ := sendRequest(t, app, "/", nil)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.NotNil(t, after)
	hiddenID, ok := after.Value(sessionIDContextKey).(string)
	require.True(t, ok, "the session ID is hidden behind an empty value, not removed")
	require.Empty(t, hiddenID)
	held, ok := after.Value(middlewareContextKey).(*Middleware)
	require.True(t, ok, "the middleware is hidden behind an empty value, not removed")
	require.Nil(t, held)
	require.Equal(t, "kept", after.Value(otherKey{}), "other values stay")
	require.Nil(t, FromContext(after))
}

// A middleware that finished holds neither the store nor the error handler it ran
// with: a handler that outlives the request finds none, and a recycled one does not
// keep a closure of the application alive until its next request.
func Test_Session_Middleware_ReleaseForgetsTheStoreAndTheErrorHandler(t *testing.T) {
	t.Parallel()

	c := newRequest(t, fiber.New())
	store := NewStore(Config{Extractor: extractors.FromCookie("a_sid"), ErrorHandler: func(fiber.Ctx, error) {}})

	m := startMiddleware(t, c, store)
	require.Same(t, store, m.Store())
	require.NotNil(t, m.errorHandler)

	// Dropped, not recycled, so that the test can look at it afterwards.
	m.mark(lifeEscaped)
	releaseMiddleware(m)

	require.Nil(t, m.Store())
	require.Nil(t, m.errorHandler)
}
