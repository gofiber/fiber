package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Benchmark_RespHeaderBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &RespHeaderBinding{EnableSplitting: true}

	var user benchUser

	resp := fasthttp.AcquireResponse()
	resp.Header.Set("name", "john")
	resp.Header.Set("age", "42")
	resp.Header.Set("posts", "post1,post2,post3")

	b.Cleanup(func() { fasthttp.ReleaseResponse(resp) })

	var err error
	for b.Loop() {
		err = binder.Bind(resp, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}

func Test_RespHeaderBinder_Bind_Splitting_TrimsListWhitespace(t *testing.T) {
	t.Parallel()
	b := &RespHeaderBinding{EnableSplitting: true}

	type User struct {
		Posts []string `respHeader:"posts"`
		IDs   []int    `respHeader:"ids"`
	}

	resp := fasthttp.AcquireResponse()
	t.Cleanup(func() { fasthttp.ReleaseResponse(resp) })

	resp.Header.Set("posts", "post1, post2")
	resp.Header.Set("ids", "1,	2,")

	var user User
	require.NoError(t, b.Bind(resp, &user))
	require.Equal(t, []string{"post1", "post2"}, user.Posts)
	require.Equal(t, []int{1, 2}, user.IDs)

	values := make(map[string][]string)
	require.NoError(t, b.Bind(resp, &values))
	require.Equal(t, []string{"post1", "post2"}, values["Posts"])
}
