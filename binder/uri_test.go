package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_URIBinding_Bind(t *testing.T) {
	t.Parallel()

	b := &URIBinding{}
	require.Equal(t, "uri", b.Name())

	type User struct {
		Name  string   `uri:"name"`
		Posts []string `uri:"posts"`
		Age   int      `uri:"age"`
	}
	var user User

	paramsKey := []string{"name", "age", "posts"}
	paramsVals := []string{"john", "42", "post1,post2,post3"}
	paramsFunc := func(key string, _ ...string) string {
		for i, k := range paramsKey {
			if k == key {
				return paramsVals[i]
			}
		}

		return ""
	}

	err := b.Bind(paramsKey, paramsFunc, &user)
	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Equal(t, []string{"post1,post2,post3"}, user.Posts)

	b.Reset()
}

func Benchmark_URIBinding_Bind(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &URIBinding{}

	type User struct {
		Name  string   `uri:"name"`
		Posts []string `uri:"posts"`
		Age   int      `uri:"age"`
	}
	var user User

	paramsKey := []string{"name", "age", "posts"}
	paramsVals := []string{"john", "42", "post1,post2,post3"}
	paramsFunc := func(key string, _ ...string) string {
		for i, k := range paramsKey {
			if k == key {
				return paramsVals[i]
			}
		}

		return ""
	}

	var err error
	for i := 0; i < b.N; i++ {
		err = binder.Bind(paramsKey, paramsFunc, &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Equal(b, []string{"post1,post2,post3"}, user.Posts)
}
