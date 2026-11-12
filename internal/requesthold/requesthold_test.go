package requesthold

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// newCtx returns a RequestCtx that holds a plain GET request.
func newCtx() *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetHost("example.com")
	return ctx
}

// streaming flushes, then keeps running until release is closed; started reports that it has flushed.
func streaming(started chan<- struct{}, release <-chan struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first")) //nolint:errcheck // nothing to do with a failing write here
		w.(http.Flusher).Flush()        //nolint:errcheck,forcetypeassert // the adaptor's writer flushes
		started <- struct{}{}
		<-release
	})
}

// buffered answers at once.
func buffered(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }) //nolint:errcheck // see above
}

// Test_NewFastHTTPHandler_HoldsWhileTheHandlerRuns pins that the request stays pending while a flushed handler
// runs, is released by the reset once it returned, and that a handler that answers at once leaves no hold.
func Test_NewFastHTTPHandler_HoldsWhileTheHandlerRuns(t *testing.T) {
	t.Parallel()
	t.Run("buffered", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx()
		NewFastHTTPHandler(buffered("plain"))(ctx)
		require.False(t, Pending(ctx))
		require.Nil(t, ctx.UserValue(holdKey{}))
		require.Equal(t, "plain", string(ctx.Response.Body()))
	})

	t.Run("flushed", func(t *testing.T) {
		t.Parallel()
		ctx := newCtx()
		started, release := make(chan struct{}, 1), make(chan struct{})
		NewFastHTTPHandler(streaming(started, release))(ctx)
		apptest.Recv(t, started, 10*time.Second, "the handler did not flush")
		require.True(t, Pending(ctx))

		reset := make(chan struct{})
		go func() {
			ctx.ResetUserValues() // what fasthttp does when it resets the request: closes the hold, which waits
			close(reset)
		}()
		select {
		case <-reset:
			t.Fatal("the reset did not wait for the running handler")
		case <-time.After(50 * time.Millisecond):
		}
		close(release)
		apptest.Recv(t, reset, 10*time.Second, "the reset did not return once the handler had")
		require.False(t, Pending(ctx))
	})
}

// Test_NewFastHTTPHandler_SecondAdapterLeavesTheHold pins that an adapter that finds the hold of an earlier
// adaptation of the same request registered neither removes nor pools it, although it answers with a buffered
// body (which ends the stream of the first): the handler of the earlier one still runs, and the reset has to
// wait for it.
func Test_NewFastHTTPHandler_SecondAdapterLeavesTheHold(t *testing.T) {
	t.Parallel()
	ctx := newCtx()
	started, release := make(chan struct{}, 1), make(chan struct{})
	NewFastHTTPHandler(streaming(started, release))(ctx)
	apptest.Recv(t, started, 10*time.Second, "the first handler did not flush")
	hold := ctx.UserValue(holdKey{})
	require.NotNil(t, hold)

	NewFastHTTPHandler(buffered("second"))(ctx)
	require.False(t, ctx.Response.IsBodyStream(), "the buffered answer should have ended the stream of the first handler")
	require.True(t, Pending(ctx), "the second adapter let go of the hold of the handler that still runs")
	require.Same(t, hold, ctx.UserValue(holdKey{}))

	reset := make(chan struct{})
	go func() {
		ctx.ResetUserValues()
		close(reset)
	}()
	select {
	case <-reset:
		t.Fatal("the reset did not wait for the running handler")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	apptest.Recv(t, reset, 10*time.Second, "the reset did not return once the handler had")
	require.False(t, Pending(ctx))
}
