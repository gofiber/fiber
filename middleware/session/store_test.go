package session

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Store_getSessionID
func Test_Store_getSessionID(t *testing.T) {
	t.Parallel()
	expectedID := "test-session-id"

	// fiber instance
	app := fiber.New()

	t.Run("from cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set cookie
		ctx.Request().Header.SetCookie(store.Extractor.Key, expectedID)

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})

	t.Run("from header", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore(Config{
			Extractor: extractors.FromHeader("session_id"),
		})
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set header
		ctx.Request().Header.Set(store.Extractor.Key, expectedID)

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})

	t.Run("from url query", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore(Config{
			Extractor: extractors.FromQuery("session_id"),
		})
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set url parameter
		ctx.Request().SetRequestURI(fmt.Sprintf("/path?%s=%s", store.Extractor.Key, expectedID))

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})
}

// go test -run Test_Store_Get
// Regression: https://github.com/gofiber/fiber/issues/1408
// Regression: https://github.com/gofiber/fiber/security/advisories/GHSA-98j2-3j3p-fw2v
func Test_Store_Get(t *testing.T) {
	// Regression: https://github.com/gofiber/fiber/security/advisories/GHSA-98j2-3j3p-fw2v
	t.Parallel()
	unexpectedID := "test-session-id"
	// fiber instance
	app := fiber.New()
	t.Run("session should be re-generated if it is invalid", func(t *testing.T) {
		t.Parallel()
		// session store
		store := NewStore()
		// fiber context
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		// set cookie
		ctx.Request().Header.SetCookie(store.Extractor.Key, unexpectedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)

		require.NotEqual(t, unexpectedID, acquiredSession.ID())
	})
}

// go test -run Test_Store_DeleteSession
func Test_Store_DeleteSession(t *testing.T) {
	t.Parallel()
	// fiber instance
	app := fiber.New()
	// session store
	store := NewStore()

	// fiber context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Create a new session
	session, err := store.Get(ctx)
	require.NoError(t, err)

	// Save the session ID
	sessionID := session.ID()

	// Delete the session
	err = store.Delete(ctx, sessionID)
	require.NoError(t, err)

	// Try to get the session again
	session, err = store.Get(ctx)
	require.NoError(t, err)

	// The session ID should be different now, because the old session was deleted
	require.NotEqual(t, sessionID, session.ID())
}

func TestStore_Get_SessionAlreadyLoaded(t *testing.T) {
	// Create a new Fiber app
	app := fiber.New()

	// Create a new context
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	// Mock middleware and set it in the context
	middleware := &Middleware{}
	ctx.Locals(middlewareContextKey, middleware)

	// Create a new store
	store := &Store{}

	// Call the Get method
	sess, err := store.Get(ctx)

	// Assert that the error is ErrSessionAlreadyLoadedByMiddleware
	require.Nil(t, sess)
	require.Equal(t, ErrSessionAlreadyLoadedByMiddleware, err)
}

func TestStore_Delete(t *testing.T) {
	// Create a new store
	store := NewStore()

	t.Run("delete with empty session ID", func(t *testing.T) {
		err := store.Delete(context.Background(), "")
		require.Error(t, err)
		require.Equal(t, ErrEmptySessionID, err)
	})

	t.Run("delete non-existing session", func(t *testing.T) {
		err := store.Delete(context.Background(), "non-existing-session-id")
		require.NoError(t, err)
	})
}

func Test_Store_GetByID(t *testing.T) {
	t.Parallel()
	// Create a new store
	store := NewStore()

	t.Run("empty session ID", func(t *testing.T) {
		t.Parallel()
		sess, err := store.GetByID(context.Background(), "")
		require.Error(t, err)
		require.Nil(t, sess)
		require.Equal(t, ErrEmptySessionID, err)
	})

	t.Run("nonexistent session ID", func(t *testing.T) {
		t.Parallel()
		sess, err := store.GetByID(context.Background(), "nonexistent-session-id")
		require.Error(t, err)
		require.Nil(t, sess)
		require.Equal(t, ErrSessionIDNotFoundInStore, err)
	})

	t.Run("valid session ID", func(t *testing.T) {
		t.Parallel()
		app := fiber.New()
		// Create a new session
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		session, err := store.Get(ctx)
		defer session.Release()
		defer app.ReleaseCtx(ctx)
		require.NoError(t, err)

		// Save the session ID
		sessionID := session.ID()

		// Save the session
		err = session.Save()
		require.NoError(t, err)

		// Retrieve the session by ID
		retrievedSession, err := store.GetByID(context.Background(), sessionID)
		require.NoError(t, err)
		require.NotNil(t, retrievedSession)
		require.Equal(t, sessionID, retrievedSession.ID())

		// Call Save on the retrieved session
		retrievedSession.Set("key", "value")
		err = retrievedSession.Save()
		require.NoError(t, err)

		// Call Other Session methods
		require.Equal(t, "value", retrievedSession.Get("key"))
		require.False(t, retrievedSession.Fresh())

		require.NoError(t, retrievedSession.Reset())
		require.NoError(t, retrievedSession.Destroy())
		require.IsType(t, []any{}, retrievedSession.Keys())
		require.NoError(t, retrievedSession.Regenerate())
		require.NotPanics(t, func() {
			retrievedSession.Release()
		})
	})
}

type trackingStorage struct {
	data        map[string][]byte
	deleteErr   error
	deleteCalls int
}

func newTrackingStorage() *trackingStorage {
	return &trackingStorage{data: make(map[string][]byte)}
}

func (s *trackingStorage) GetWithContext(_ context.Context, key string) ([]byte, error) {
	if v, ok := s.data[key]; ok {
		copied := make([]byte, len(v))
		copy(copied, v)
		return copied, nil
	}
	return nil, nil
}

func (s *trackingStorage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

func (s *trackingStorage) SetWithContext(_ context.Context, key string, val []byte, _ time.Duration) error {
	copied := make([]byte, len(val))
	copy(copied, val)
	s.data[key] = copied
	return nil
}

func (s *trackingStorage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

func (s *trackingStorage) DeleteWithContext(_ context.Context, key string) error {
	s.deleteCalls++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.data, key)
	return nil
}

func (s *trackingStorage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

func (*trackingStorage) ResetWithContext(context.Context) error { return nil }
func (*trackingStorage) Reset() error                           { return nil }
func (*trackingStorage) Close() error                           { return nil }

func seedExpiredSessionInStore(t *testing.T, store *Store, sessionID string) {
	t.Helper()

	sess := acquireSession()
	sess.mu.Lock()
	sess.config = store
	sess.id = sessionID
	sess.fresh = false
	sess.mu.Unlock()
	sess.Set("name", "john")
	sess.Set(absExpirationKey, time.Now().Add(-time.Minute))
	require.NoError(t, sess.Save())
	sess.Release()
}

func Test_Store_getSession_ExpiredResetFailureReleasesSession(t *testing.T) {
	t.Parallel()

	storage := newTrackingStorage()
	store := NewStore(Config{
		Storage:         storage,
		IdleTimeout:     time.Minute,
		AbsoluteTimeout: time.Minute,
	})

	const sessionID = "existing-session-id"
	seedExpiredSessionInStore(t, store, sessionID)
	storage.deleteErr = errors.New("delete failed")

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)
	ctx.Request().Header.SetCookie("session_id", sessionID)

	sess, err := store.Get(ctx)
	require.Nil(t, sess)
	require.ErrorContains(t, err, "failed to reset session")
	require.Equal(t, 1, storage.deleteCalls)

	reused := acquireSession()
	require.Nil(t, reused.ctx)
	require.Nil(t, reused.config)
	require.Empty(t, reused.id)
	reused.Release()
}

func Test_Store_GetByID_ExpiredDestroySuccessReleasesSession(t *testing.T) {
	t.Parallel()

	storage := newTrackingStorage()
	store := NewStore(Config{
		Storage:         storage,
		IdleTimeout:     time.Minute,
		AbsoluteTimeout: time.Minute,
	})

	const sessionID = "expired-session-id"
	seedExpiredSessionInStore(t, store, sessionID)

	sess, err := store.GetByID(context.Background(), sessionID)
	require.Nil(t, sess)
	require.ErrorIs(t, err, ErrSessionIDNotFoundInStore)
	require.Equal(t, 1, storage.deleteCalls)

	reused := acquireSession()
	require.Nil(t, reused.ctx)
	require.Nil(t, reused.config)
	require.Empty(t, reused.id)
	reused.Release()
}
