package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_QueryBinder_Bind(t *testing.T) {
	t.Parallel()

	b := &QueryBinding{
		EnableSplitting: true,
	}
	require.Equal(t, "query", b.Name())

	type Post struct {
		Title string `query:"title"`
	}

	type User struct {
		Name  string   `query:"name"`
		Names []string `query:"names"`
		Age   int      `query:"age"`

		Posts []Post `query:"posts"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("name=john&names=john,doe&age=42&posts[0][title]=post1&posts[1][title]=post2&posts[2][title]=post3")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	err := b.Bind(req, &user)

	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Len(t, user.Posts, 3)
	require.Equal(t, "post1", user.Posts[0].Title)
	require.Equal(t, "post2", user.Posts[1].Title)
	require.Equal(t, "post3", user.Posts[2].Title)
	require.Contains(t, user.Names, "john")
	require.Contains(t, user.Names, "doe")
}

func Benchmark_QueryBinder_Bind(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &QueryBinding{
		EnableSplitting: true,
	}

	type User struct {
		Name string `query:"name"`
		Age  int    `query:"age"`

		Posts []string `query:"posts"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("name=john&age=42&posts=post1,post2,post3")

	for i := 0; i < b.N; i++ {
		_ = binder.Bind(req, &user)
	}
}
