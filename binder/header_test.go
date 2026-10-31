package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Benchmark_HeaderBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &HeaderBinding{EnableSplitting: true}

	var user benchUser

	req := acquireRequest(b)
	req.Header.Set("name", "john")
	req.Header.Set("age", "42")
	req.Header.Set("posts", "post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}

func Test_HeaderBinder_Bind_Splitting_StringMapNotSplit(t *testing.T) {
	t.Parallel()
	b := &HeaderBinding{EnableSplitting: true}

	const userAgent = "Mozilla/5.0 (X11; Linux) AppleWebKit/537.36 (KHTML, like Gecko)"

	req := acquireRequest(t)
	req.Header.Set("User-Agent", userAgent)

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
