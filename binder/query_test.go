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
		Posts []Post   `query:"posts"`
		Age   int      `query:"age"`
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

	b.Reset()
	require.False(t, b.EnableSplitting)
}

func Benchmark_QueryBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &QueryBinding{
		EnableSplitting: true,
	}

	type User struct {
		Name  string   `query:"name"`
		Posts []string `query:"posts"`
		Age   int      `query:"age"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	b.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	req.URI().SetQueryString("name=john&age=42&posts=post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Len(b, user.Posts, 3)
	require.Contains(b, user.Posts, "post1")
	require.Contains(b, user.Posts, "post2")
	require.Contains(b, user.Posts, "post3")
}

func Test_QueryBinder_Bind_PointerSlices(t *testing.T) {
	t.Parallel()

	binder := &QueryBinding{
		EnableSplitting: true,
	}

	type Preferences struct {
		Tags *[]string `query:"tags"`
	}

	type Profile struct {
		Emails *[]string    `query:"emails"`
		Prefs  *Preferences `query:"preferences"`
	}

	var profile Profile

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("emails=work,personal&preferences[tags]=golang,api")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	err := binder.Bind(req, &profile)
	require.NoError(t, err)

	require.NotNil(t, profile.Emails)
	require.ElementsMatch(t, []string{"work", "personal"}, *profile.Emails)

	require.NotNil(t, profile.Prefs)
	require.NotNil(t, profile.Prefs.Tags)
	require.ElementsMatch(t, []string{"golang", "api"}, *profile.Prefs.Tags)
}

func Test_QueryBinder_Bind_PointerScalars(t *testing.T) {
	t.Parallel()

	binder := &QueryBinding{
		EnableSplitting: false,
	}

	type Query struct {
		ID     *int64   `query:"id"`
		Name   *string  `query:"name"`
		Active *bool    `query:"active"`
		Score  *float64 `query:"score"`
	}

	t.Run("all fields provided", func(t *testing.T) {
		t.Parallel()

		var q Query
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("id=123&name=test&active=true&score=98.5")

		t.Cleanup(func() {
			fasthttp.ReleaseRequest(req)
		})

		err := binder.Bind(req, &q)
		require.NoError(t, err)

		require.NotNil(t, q.ID)
		require.Equal(t, int64(123), *q.ID)

		require.NotNil(t, q.Name)
		require.Equal(t, "test", *q.Name)

		require.NotNil(t, q.Active)
		require.True(t, *q.Active)

		require.NotNil(t, q.Score)
		require.InDelta(t, 98.5, *q.Score, 0.001)
	})

	t.Run("no fields provided", func(t *testing.T) {
		t.Parallel()

		var q Query
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("")

		t.Cleanup(func() {
			fasthttp.ReleaseRequest(req)
		})

		err := binder.Bind(req, &q)
		require.NoError(t, err)

		require.Nil(t, q.ID)
		require.Nil(t, q.Name)
		require.Nil(t, q.Active)
		require.Nil(t, q.Score)
	})

	t.Run("partial fields provided", func(t *testing.T) {
		t.Parallel()

		var q Query
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("id=456&active=false")

		t.Cleanup(func() {
			fasthttp.ReleaseRequest(req)
		})

		err := binder.Bind(req, &q)
		require.NoError(t, err)

		require.NotNil(t, q.ID)
		require.Equal(t, int64(456), *q.ID)

		require.Nil(t, q.Name)

		require.NotNil(t, q.Active)
		require.False(t, *q.Active)

		require.Nil(t, q.Score)
	})

	t.Run("zero values provided", func(t *testing.T) {
		t.Parallel()

		var q Query
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("id=0&name=&active=false&score=0")

		t.Cleanup(func() {
			fasthttp.ReleaseRequest(req)
		})

		err := binder.Bind(req, &q)
		require.NoError(t, err)

		require.NotNil(t, q.ID)
		require.Equal(t, int64(0), *q.ID)

		require.NotNil(t, q.Name)
		require.Empty(t, *q.Name)

		require.NotNil(t, q.Active)
		require.False(t, *q.Active)

		require.NotNil(t, q.Score)
		require.InDelta(t, 0.0, *q.Score, 0.001)
	})
}
