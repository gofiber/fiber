package personalheader

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_New(t *testing.T) {
	t.Parallel()
	require.Nil(t, New(), "no names, no set")
	require.Nil(t, New("", ""), "the empty name is not a header")

	names := New("X-Session-ID", "x-session-id", "X-Other")
	require.Len(t, names.names, 2, "names that differ in case are one name")
	require.True(t, names.Has([]byte("x-SESSION-id")))
	require.True(t, names.Has([]byte("X-Other")))
	require.False(t, names.Has([]byte("X-Session")), "a prefix is not the name")
	require.False(t, names.Has(nil))
	require.False(t, (*Names)(nil).Has([]byte("X-Session-ID")), "the nil set is empty")
}

func Test_Mark(t *testing.T) {
	t.Parallel()
	a, b := New("X-A"), New("X-B")

	t.Run("nothing marked", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, Marked(&fasthttp.RequestCtx{}))
		require.Nil(t, Marked(nil))
	})
	t.Run("the first set is stored as it is", func(t *testing.T) {
		t.Parallel()
		rctx := &fasthttp.RequestCtx{}
		Mark(rctx, a)
		require.Same(t, a, Marked(rctx))
		Mark(rctx, a)
		require.Same(t, a, Marked(rctx), "marking again changes nothing")
		Mark(rctx, nil)
		Mark(nil, a)
		require.Same(t, a, Marked(rctx))
	})
	t.Run("sets add up", func(t *testing.T) {
		t.Parallel()
		rctx := &fasthttp.RequestCtx{}
		Mark(rctx, a)
		Mark(rctx, b)
		marked := Marked(rctx)
		require.True(t, marked.Has([]byte("X-A")))
		require.True(t, marked.Has([]byte("x-b")))
		require.NotSame(t, a, marked, "the sets are not changed: other requests share them")
		require.Len(t, a.names, 1)
		Mark(rctx, a)
		require.Same(t, marked, Marked(rctx), "a set the marked ones cover is not added again")
	})
	t.Run("the marks belong to the request", func(t *testing.T) {
		t.Parallel()
		rctx := &fasthttp.RequestCtx{}
		Mark(rctx, a)
		rctx.ResetUserValues()
		require.Nil(t, Marked(rctx))
	})
}
