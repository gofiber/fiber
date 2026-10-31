package binder

import (
	"testing"

	"github.com/shamaton/msgpack/v3"
	"github.com/stretchr/testify/require"
)

func Test_Msgpack_Binding_Bind(t *testing.T) {
	t.Parallel()
	b := &MsgPackBinding{MsgPackDecoder: msgpack.Unmarshal}
	require.Equal(t, "msgpack", b.Name())

	type Post struct {
		Title string `msgpack:"title"`
	}

	type User struct {
		Name  string `msgpack:"name"`
		Posts []Post `msgpack:"posts"`
		Age   int    `msgpack:"age"`
	}
	var user User

	// Prepare msgpack data
	input := map[string]any{
		"name":  "john",
		"age":   42,
		"posts": []map[string]any{{"title": "post1"}, {"title": "post2"}, {"title": "post3"}},
	}
	data, err := msgpack.Marshal(input)
	require.NoError(t, err)

	require.NoError(t, b.Bind(data, &user))
	posts := []Post{{Title: "post1"}, {Title: "post2"}, {Title: "post3"}}
	require.Equal(t, User{Name: "john", Age: 42, Posts: posts}, user)

	b.Reset()
	require.Nil(t, b.MsgPackDecoder)
}

func Benchmark_Msgpack_Binding_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &MsgPackBinding{MsgPackDecoder: msgpack.Unmarshal}

	type User struct {
		Name  string   `msgpack:"name"`
		Posts []string `msgpack:"posts"`
		Age   int      `msgpack:"age"`
	}

	var user User
	var err error
	for b.Loop() {
		// {"name":"john","age":42,"posts":[{"title":"post1"},{"title":"post2"},{"title":"post3"}]}
		err = binder.Bind([]byte{
			0x83, 0xa4, 0x6e, 0x61, 0x6d, 0x65, 0xa4, 0x6a, 0x6f, 0x68, 0x6e, 0xa3, 0x61, 0x67, 0x65, 0x2a,
			0xa5, 0x70, 0x6f, 0x73, 0x74, 0x73, 0x93, 0xa5, 0x70, 0x6f, 0x73, 0x74, 0x31, 0xa5, 0x70, 0x6f,
			0x73, 0x74, 0x32, 0xa5, 0x70, 0x6f, 0x73, 0x74, 0x33,
		},
			&user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}
