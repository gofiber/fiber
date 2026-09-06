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

func Test_QueryBinder_Bind_OptionalIDParam(t *testing.T) {
	t.Parallel()

	binder := &QueryBinding{
		EnableSplitting: false,
	}

	// Use case from original issue
	type OptionalIDParam struct {
		IDPtr *int64 `query:"id"`
	}

	t.Run("id provided", func(t *testing.T) {
		t.Parallel()

		var param OptionalIDParam
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("id=123")

		t.Cleanup(func() {
			fasthttp.ReleaseRequest(req)
		})

		err := binder.Bind(req, &param)
		require.NoError(t, err)

		require.NotNil(t, param.IDPtr)
		require.Equal(t, int64(123), *param.IDPtr)
	})

	t.Run("id not provided", func(t *testing.T) {
		t.Parallel()

		var param OptionalIDParam
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("")

		t.Cleanup(func() {
			fasthttp.ReleaseRequest(req)
		})

		err := binder.Bind(req, &param)
		require.NoError(t, err)

		require.Nil(t, param.IDPtr)
	})

	t.Run("id zero", func(t *testing.T) {
		t.Parallel()

		var param OptionalIDParam
		req := fasthttp.AcquireRequest()
		req.URI().SetQueryString("id=0")

		t.Cleanup(func() {
			fasthttp.ReleaseRequest(req)
		})

		err := binder.Bind(req, &param)
		require.NoError(t, err)

		require.NotNil(t, param.IDPtr)
		require.Equal(t, int64(0), *param.IDPtr)
	})
}

func Test_QueryBinder_Bind_Splitting_ScalarNotSplit(t *testing.T) {
	t.Parallel()

	b := &QueryBinding{
		EnableSplitting: true,
	}

	type Filter struct {
		IDs []int `query:"ids"`
	}
	type Request struct {
		Name   string `query:"name"`
		Filter Filter `query:"filter"`
	}

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("name=Smith,+John&filter[ids]=1,2")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	var out Request
	require.NoError(t, b.Bind(req, &out))
	require.Equal(t, "Smith, John", out.Name)
	require.Equal(t, []int{1, 2}, out.Filter.IDs)
}

func Test_QueryBinder_Bind_Splitting_EmbeddedStruct(t *testing.T) {
	t.Parallel()

	b := &QueryBinding{
		EnableSplitting: true,
	}

	type Embedded struct {
		Title string   `query:"title"`
		Names []string `query:"names"`
	}
	type Request struct {
		Name string `query:"name"`
		Embedded
	}

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("names=a,b&title=x,y&name=p,q")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	var out Request
	require.NoError(t, b.Bind(req, &out))
	require.Equal(t, []string{"a", "b"}, out.Names)
	require.Equal(t, "x,y", out.Title)
	require.Equal(t, "p,q", out.Name)
}

func Test_QueryBinder_Bind_Splitting_NestedDepth(t *testing.T) {
	t.Parallel()

	type Inner struct {
		Name string   `query:"name"`
		Tags []string `query:"tags"`
	}
	type Mid struct {
		Inner Inner `query:"inner"`
	}
	type Item struct {
		Name string   `query:"name"`
		Tags []string `query:"tags"`
	}
	type Request struct {
		Mid   Mid    `query:"mid"`
		Items []Item `query:"items"`
	}

	want := Request{
		Mid:   Mid{Inner: Inner{Name: "c,d", Tags: []string{"a", "b"}}},
		Items: []Item{{Name: "g,h", Tags: []string{"e", "f"}}, {Tags: []string{"i", "j"}}},
	}

	cases := []struct {
		name  string
		query string
	}{
		{
			name:  "bracket notation",
			query: "mid[inner][tags]=a,b&mid[inner][name]=c,d&items[0][tags]=e,f&items[0][name]=g,h&items[1][tags]=i,j",
		},
		{
			name:  "dot notation",
			query: "mid.inner.tags=a,b&mid.inner.name=c,d&items.0.tags=e,f&items.0.name=g,h&items.1.tags=i,j",
		},
		{
			name:  "mixed notation",
			query: "mid[inner].tags=a,b&mid.inner[name]=c,d&items[0].tags=e,f&items.0[name]=g,h&items[1].tags=i,j",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := &QueryBinding{
				EnableSplitting: true,
			}

			req := fasthttp.AcquireRequest()
			req.URI().SetQueryString(tc.query)

			t.Cleanup(func() {
				fasthttp.ReleaseRequest(req)
			})

			var out Request
			require.NoError(t, b.Bind(req, &out))
			require.Equal(t, want, out)
		})
	}
}

func Test_QueryBinder_Bind_Splitting_EmptyAliasWithOptions(t *testing.T) {
	t.Parallel()

	b := &QueryBinding{
		EnableSplitting: true,
	}

	type Filter struct {
		Tags []string `query:",default:x|y"`
	}

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("tags=a,b")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	var filter Filter
	require.NoError(t, b.Bind(req, &filter))
	require.Equal(t, []string{"a", "b"}, filter.Tags)

	// The default only fills the field when the key is absent.
	empty := fasthttp.AcquireRequest()

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(empty)
	})

	var defaulted Filter
	require.NoError(t, b.Bind(empty, &defaulted))
	require.Equal(t, []string{"x", "y"}, defaulted.Tags)
}

func Test_QueryBinder_Bind_Splitting_NilSliceMap(t *testing.T) {
	t.Parallel()

	b := &QueryBinding{EnableSplitting: true}

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	req.URI().SetQueryString("ids=1,2")

	// The decoder allocates the map, so a nil one still splits.
	var sliceMap map[string][]string
	require.NoError(t, b.Bind(req, &sliceMap))
	require.Equal(t, []string{"1", "2"}, sliceMap["ids"])

	var stringMap map[string]string
	require.NoError(t, b.Bind(req, &stringMap))
	require.Equal(t, "1,2", stringMap["ids"])
}

func Test_QueryBinder_Bind_Splitting_InvalidDestination(t *testing.T) {
	t.Parallel()

	b := &QueryBinding{
		EnableSplitting: true,
	}

	type User struct {
		Names []string `query:"names"`
	}

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("names=john,doe")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// The decoder reports these destinations; splitting must not panic first.
	require.NotPanics(t, func() {
		require.Error(t, b.Bind(req, User{}))
		require.Error(t, b.Bind(req, nil))
	})
}
