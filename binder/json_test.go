package binder

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_JSON_Binding_Bind(t *testing.T) {
	t.Parallel()
	b := &JSONBinding{JSONDecoder: json.Unmarshal}
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

	require.NoError(t, b.Bind([]byte(`{"name":"john","age":42,"posts":[{"title":"post1"},{"title":"post2"},{"title":"post3"}]}`), &user))
	posts := []Post{{Title: "post1"}, {Title: "post2"}, {Title: "post3"}}
	require.Equal(t, User{Name: "john", Age: 42, Posts: posts}, user)

	b.Reset()
	require.Nil(t, b.JSONDecoder)
}

func Test_JSON_Binding_Bind_OptionalIDParam(t *testing.T) {
	t.Parallel()
	b := &JSONBinding{JSONDecoder: json.Unmarshal}

	// Use case from original issue
	type OptionalIDParam struct {
		IDPtr *int64 `json:"id"`
	}

	tests := []struct {
		want *int64
		name string
		body string
	}{
		{name: "id provided", body: `{"id":123}`, want: new(int64(123))},
		{name: "id not provided", body: `{}`, want: nil},
		{name: "id zero", body: `{"id":0}`, want: new(int64(0))},
		{name: "id null", body: `{"id":null}`, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var param OptionalIDParam
			require.NoError(t, b.Bind([]byte(tc.body), &param))
			require.Equal(t, tc.want, param.IDPtr)
		})
	}
}

func Benchmark_JSON_Binding_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &JSONBinding{JSONDecoder: json.Unmarshal}

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

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}
