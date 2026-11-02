package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// uriKeys and uriVals are the route parameters the URI binder tests bind.
var (
	uriKeys = []string{"name", "age", "posts"}
	uriVals = []string{"john", "42", "post1,post2,post3"}
)

// uriLookup answers for a route parameter of uriKeys the way a router does.
func uriLookup(key string, _ ...string) string {
	for i, k := range uriKeys {
		if k == key {
			return uriVals[i]
		}
	}

	return ""
}

type uriUser struct {
	Name  string   `uri:"name"`
	Posts []string `uri:"posts"`
	Age   int      `uri:"age"`
}

func Test_URIBinding_Bind(t *testing.T) {
	t.Parallel()
	b := &URIBinding{}
	require.Equal(t, "uri", b.Name())

	var user uriUser

	require.NoError(t, b.Bind(uriKeys, uriLookup, &user))
	require.Equal(t, uriUser{Name: "john", Posts: []string{"post1,post2,post3"}, Age: 42}, user)

	b.Reset()
}

func Benchmark_URIBinding_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &URIBinding{}

	var user uriUser

	var err error
	for b.Loop() {
		err = binder.Bind(uriKeys, uriLookup, &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Equal(b, []string{"post1,post2,post3"}, user.Posts)
}
