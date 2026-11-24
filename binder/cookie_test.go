package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_CookieBinder_Bind_FirstWins pins that a repeated cookie name binds its first value, the one Ctx.Cookies returns.
func Test_CookieBinder_Bind_FirstWins(t *testing.T) {
	t.Parallel()
	req := acquireRequest(t)
	req.Header.Set(fasthttp.HeaderCookie, "tok=a; ids=1,2; tok=b; ids=3")
	b := &CookieBinding{EnableSplitting: true}

	var out struct {
		Tok string   `cookie:"tok"`
		IDs []string `cookie:"ids"`
	}
	require.NoError(t, b.Bind(req, &out))
	require.Equal(t, "a", out.Tok)
	require.Equal(t, []string{"1", "2"}, out.IDs)

	last := map[string]string{}
	require.NoError(t, b.Bind(req, &last))
	require.Equal(t, map[string]string{"tok": "a", "ids": "1,2"}, last)

	all := map[string][]string{}
	require.NoError(t, b.Bind(req, &all))
	require.Equal(t, map[string][]string{"tok": {"a"}, "ids": {"1", "2"}}, all)
}

func Benchmark_CookieBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &CookieBinding{EnableSplitting: true}

	var user benchUser

	req := acquireRequest(b)
	req.Header.SetCookie("name", "john")
	req.Header.SetCookie("age", "42")
	req.Header.SetCookie("posts", "post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}
