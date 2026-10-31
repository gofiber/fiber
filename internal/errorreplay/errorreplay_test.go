package errorreplay

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_MarkUnmark(t *testing.T) {
	t.Parallel()
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("other", true)
	require.False(t, Active(ctx))

	Mark(ctx)
	require.True(t, Active(ctx))

	Unmark(ctx)
	require.False(t, Active(ctx))
	require.Equal(t, true, ctx.UserValue("other"), "other user values are kept")
}
