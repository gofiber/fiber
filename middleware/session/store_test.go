package session

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// go test -run TestStore_getSessionID
func TestStore_getSessionID(t *testing.T) {
	t.Parallel()
	expectedID := "test-session-id"

	// fiber instance
	app := fiber.New()

	t.Run("from cookie", func(t *testing.T) {
		t.Parallel()
		// session store
		store := New()
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, expectedID)

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})

	t.Run("from header", func(t *testing.T) {
		t.Parallel()
		// session store
		store := New(Config{
			KeyLookup: "header:session_id",
		})
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// set header
		ctx.Request().Header.Set(store.sessionName, expectedID)

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})

	t.Run("from url query", func(t *testing.T) {
		t.Parallel()
		// session store
		store := New(Config{
			KeyLookup: "query:session_id",
		})
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// set url parameter
		ctx.Request().SetRequestURI(fmt.Sprintf("/path?%s=%s", store.sessionName, expectedID))

		require.Equal(t, expectedID, store.getSessionID(ctx))
	})
}

// go test -run TestStore_Get
// Regression: https://github.com/gofiber/fiber/issues/1408
func TestStore_Get(t *testing.T) {
	t.Parallel()
	unexpectedID := "test-session-id"
	// fiber instance
	app := fiber.New()
	t.Run("session should persisted even session is invalid", func(t *testing.T) {
		t.Parallel()
		// session store
		store := New()
		// fiber context
		ctx := app.NewCtx(&fasthttp.RequestCtx{})

		// set cookie
		ctx.Request().Header.SetCookie(store.sessionName, unexpectedID)

		acquiredSession, err := store.Get(ctx)
		require.NoError(t, err)

		if acquiredSession.ID() != unexpectedID {
			t.Fatal("server should not accept the unexpectedID which is not in the store")
		}
	})
}
