package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_RespHeaderBinder_Bind(t *testing.T) {
	t.Parallel()

	b := &RespHeaderBinding{
		EnableSplitting: true,
	}
	require.Equal(t, "respHeader", b.Name())

	type User struct {
		Name string `respHeader:"name"`
		Age  int    `respHeader:"age"`

		Posts []string `respHeader:"posts"`
	}
	var user User

	resp := fasthttp.AcquireResponse()
	resp.Header.Set("name", "john")
	resp.Header.Set("age", "42")
	resp.Header.Set("posts", "post1,post2,post3")

	t.Cleanup(func() {
		fasthttp.ReleaseResponse(resp)
	})

	err := b.Bind(resp, &user)

	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Equal(t, []string{"post1", "post2", "post3"}, user.Posts)
}

func Benchmark_RespHeaderBinder_Bind(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &RespHeaderBinding{
		EnableSplitting: true,
	}

	type User struct {
		Name string `respHeader:"name"`
		Age  int    `respHeader:"age"`

		Posts []string `respHeader:"posts"`
	}
	var user User

	resp := fasthttp.AcquireResponse()
	resp.Header.Set("name", "john")
	resp.Header.Set("age", "42")
	resp.Header.Set("posts", "post1,post2,post3")

	b.Cleanup(func() {
		fasthttp.ReleaseResponse(resp)
	})

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_ = binder.Bind(resp, &user)
	}

	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Equal(b, []string{"post1", "post2", "post3"}, user.Posts)
}
