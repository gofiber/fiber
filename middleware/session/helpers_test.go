package session

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// getSession is loadSession for the tests that only want the session, as they did
// before the store learned to say whose session ID a request caches.
func (s *Store) getSession(c fiber.Ctx) (*Session, error) {
	sess, _, err := s.loadSession(c, currentMiddleware(c))
	return sess, err
}

// newRequest returns the context of a request of app, which goes back to the app when
// the test ends.
func newRequest(tb testing.TB, app *fiber.App) fiber.Ctx {
	tb.Helper()
	c := app.AcquireCtx(&fasthttp.RequestCtx{})
	tb.Cleanup(func() { app.ReleaseCtx(c) })
	return c
}

// mustGet asks store for its session in the request c, and releases it when the test ends.
func mustGet(tb testing.TB, store *Store, c fiber.Ctx) *Session {
	tb.Helper()
	sess, err := store.Get(c)
	require.NoError(tb, err)
	tb.Cleanup(sess.Release)
	return sess
}
