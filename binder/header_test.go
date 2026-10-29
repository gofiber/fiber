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
		Name  string   `header:"Name"`
		Names []string `header:"Names"`
		Posts []string `header:"Posts"`
		Age   int      `header:"Age"`
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

	b.Reset()
	require.False(t, b.EnableSplitting)
}

func Benchmark_HeaderBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &HeaderBinding{
		EnableSplitting: true,
	}

	type User struct {
		Name  string   `header:"Name"`
		Posts []string `header:"Posts"`
		Age   int      `header:"Age"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	b.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	req.Header.Set("name", "john")
	req.Header.Set("age", "42")
	req.Header.Set("posts", "post1,post2,post3")

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

func Test_HeaderBinder_Bind_ParseError(t *testing.T) {
	b := &HeaderBinding{}
	type User struct {
		Age int `header:"Age"`
	}
	var user User
	req := fasthttp.AcquireRequest()
	req.Header.Set("age", "invalid")
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	err := b.Bind(req, &user)
	require.Error(t, err)
}

func Test_HeaderBinder_Bind_Splitting_StringMapNotSplit(t *testing.T) {
	t.Parallel()

	b := &HeaderBinding{
		EnableSplitting: true,
	}

	const userAgent = "Mozilla/5.0 (X11; Linux) AppleWebKit/537.36 (KHTML, like Gecko)"

	req := fasthttp.AcquireRequest()
	req.Header.Set("User-Agent", userAgent)

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// A map[string]string keeps one value per key, so nothing is split.
	stringMap := make(map[string]string)
	require.NoError(t, b.Bind(req, &stringMap))
	require.Equal(t, userAgent, stringMap["User-Agent"])

	// A map[string][]string holds every piece.
	sliceMap := make(map[string][]string)
	require.NoError(t, b.Bind(req, &sliceMap))
	require.Equal(t, []string{"Mozilla/5.0 (X11; Linux) AppleWebKit/537.36 (KHTML", "like Gecko)"}, sliceMap["User-Agent"])
}

func Test_HeaderBinder_Bind_Splitting_TrimsListWhitespace(t *testing.T) {
	t.Parallel()

	b := &HeaderBinding{
		EnableSplitting: true,
	}

	type User struct {
		Roles []string `header:"X-Roles"`
		Tags  []string `header:"X-Tags"`
		IDs   []int    `header:"X-Ids"`
	}

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// Clients and proxies join repeated lines with ", " (RFC 9110 Section 5.3),
	// and the space around a list element is not part of it.
	req.Header.Set("X-Roles", "admin, editor")
	req.Header.Set("X-Ids", "1, 2,\t3")
	// Only SP and HTAB are OWS: another control byte stays in the element.
	req.Header.Set("X-Tags", "a,\vb")

	var user User
	require.NoError(t, b.Bind(req, &user))
	require.Equal(t, []string{"admin", "editor"}, user.Roles)
	require.Equal(t, []string{"a", "\vb"}, user.Tags)
	require.Equal(t, []int{1, 2, 3}, user.IDs)
}

func Test_HeaderBinder_Bind_Splitting_SkipsEmptyListElements(t *testing.T) {
	t.Parallel()

	b := &HeaderBinding{
		EnableSplitting: true,
	}

	type User struct {
		Roles  []string `header:"X-Roles"`
		IDs    []int    `header:"X-Ids"`
		Counts []int    `header:"X-Counts"`
	}

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// Empty elements are ignored (RFC 9110 Section 5.6.1), so a trailing or
	// doubled comma adds no zero value.
	req.Header.Set("X-Ids", "1, 2,")
	req.Header.Set("X-Counts", "3,, 4")
	// A present header still replaces what was bound: one made only of empty
	// elements binds as an empty value.
	req.Header.Set("X-Roles", " , ")

	user := User{Roles: []string{"stale"}}
	require.NoError(t, b.Bind(req, &user))
	require.Equal(t, []int{1, 2}, user.IDs)
	require.Equal(t, []int{3, 4}, user.Counts)
	require.Equal(t, []string{""}, user.Roles)

	values := map[string][]string{"X-Roles": {"stale"}}
	require.NoError(t, b.Bind(req, &values))
	require.Equal(t, []string{"1", "2"}, values["X-Ids"])
	require.Equal(t, []string{""}, values["X-Roles"])
}
