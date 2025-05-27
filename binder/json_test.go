package binder

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_JSON_Binding_Bind(t *testing.T) {
	t.Parallel()

	b := &JSONBinding{
		JSONDecoder: json.Unmarshal,
	}
	require.Equal(t, "json", b.Name())

	type Post struct {
		Title string `json:"title"`
	}

	type User struct {
		Name  string `json:"name"`
		Posts []Post `json:"posts"`
		Age   int    `json:"age"`
	}
	var user User

	err := b.Bind([]byte(`{"name":"john","age":42,"posts":[{"title":"post1"},{"title":"post2"},{"title":"post3"}]}`), &user)
	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Len(t, user.Posts, 3)
	require.Equal(t, "post1", user.Posts[0].Title)
	require.Equal(t, "post2", user.Posts[1].Title)
	require.Equal(t, "post3", user.Posts[2].Title)

	b.Reset()
	require.Nil(t, b.JSONDecoder)
}

func Benchmark_JSON_Binding_Bind(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &JSONBinding{
		JSONDecoder: json.Unmarshal,
	}

	type User struct {
		Name  string   `json:"name"`
		Posts []string `json:"posts"`
		Age   int      `json:"age"`
	}

	var user User
	var err error
	for b.Loop() {
		err = binder.Bind([]byte(`{"name":"john","age":42,"posts":["post1","post2","post3"]}`), &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Len(b, user.Posts, 3)
	require.Equal(b, "post1", user.Posts[0])
	require.Equal(b, "post2", user.Posts[1])
	require.Equal(b, "post3", user.Posts[2])
}
