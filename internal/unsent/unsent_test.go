package unsent

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_Is(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		run  func(*fasthttp.RequestCtx)
		name string
		want bool
	}{
		{name: "untouched", run: func(*fasthttp.RequestCtx) {}},
		{name: "marked", run: Mark, want: true},
		{name: "hijacked with no response", want: true, run: func(ctx *fasthttp.RequestCtx) {
			ctx.HijackSetNoResponse(true)
			ctx.Hijack(func(net.Conn) {})
		}},
		// fasthttp does not report HijackSetNoResponse, so a hijack it answers first counts as well.
		{name: "hijacked after the response", want: true, run: func(ctx *fasthttp.RequestCtx) { ctx.Hijack(func(net.Conn) {}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := &fasthttp.RequestCtx{}
			tc.run(ctx)
			require.Equal(t, tc.want, Is(ctx))
		})
	}
}
