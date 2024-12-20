package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_HeaderBinder_Bind(t *testing.T) {
	t.Parallel()

	b := &HeaderBinding{
		EnableSplitting: true,
	}
	require.Equal(t, "header", b.Name())

	type User struct {
		Name  string   `header:"name"`
		Names []string `header:"names"`
		Age   int      `header:"age"`

		Posts []string `header:"posts"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	req.Header.Set("name", "john")
	req.Header.Set("names", "john,doe")
	req.Header.Set("age", "42")
	req.Header.Set("posts", "post1,post2,post3")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	err := b.Bind(req, &user)

	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Len(t, user.Posts, 3)
	require.Equal(t, "post1", user.Posts[0])
	require.Equal(t, "post2", user.Posts[1])
	require.Equal(t, "post3", user.Posts[2])
	require.Contains(t, user.Names, "john")
	require.Contains(t, user.Names, "doe")
}

func Benchmark_HeaderBinder_Bind(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &HeaderBinding{
		EnableSplitting: true,
	}

	type User struct {
		Name string `query:"name"`
		Age  int    `query:"age"`

		Posts []string `query:"posts"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	req.Header.Set("name", "john")
	req.Header.Set("age", "42")
	req.Header.Set("posts", "post1,post2,post3")

	for i := 0; i < b.N; i++ {
		_ = binder.Bind(req, &user)
	}
}
