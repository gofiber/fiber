package unsent

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Mark(t *testing.T) {
	t.Parallel()
	ctx := &fasthttp.RequestCtx{}
	require.False(t, Is(ctx))

	Mark(ctx)
	require.True(t, Dropped(ctx))
	require.True(t, Is(ctx))
}

func Test_IsHijacked(t *testing.T) {
	t.Parallel()
	ctx := &fasthttp.RequestCtx{}
	ctx.Hijack(func(net.Conn) {})
	require.True(t, Is(ctx))
	require.False(t, Dropped(ctx))
}
