package binder

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_URIBinding_Bind(t *testing.T) {
	t.Parallel()

	b := &URIBinding{}
	require.Equal(t, "uri", b.Name())

	type User struct {
		Name string `uri:"name"`
		Age  int    `uri:"age"`

		Posts []string `uri:"posts"`
	}
	var user User

	paramsKey := []string{"name", "age", "posts"}
	paramsVals := []string{"john", "42", "post1,post2,post3"}
	paramsFunc := func(key string, defaultValue ...string) string {
		for i, k := range paramsKey {
			if k == key {
				return paramsVals[i]
			}
		}

		return ""
	}

	err := b.Bind(paramsKey, paramsFunc, &user)

	fmt.Println(user)

	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Equal(t, []string{"post1,post2,post3"}, user.Posts)
}

func Benchmark_URIBinding_Bind(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &URIBinding{}

	type User struct {
		Name string `uri:"name"`
		Age  int    `uri:"age"`

		Posts []string `uri:"posts"`
	}
	var user User

	paramsKey := []string{"name", "age", "posts"}
	paramsVals := []string{"john", "42", "post1,post2,post3"}
	paramsFunc := func(key string, defaultValue ...string) string {
		for i, k := range paramsKey {
			if k == key {
				return paramsVals[i]
			}
		}

		return ""
	}

	for i := 0; i < b.N; i++ {
		_ = binder.Bind(paramsKey, paramsFunc, &user)
	}
}
