package binder

import (
	"testing"

	"github.com/shamaton/msgpack/v2"
	"github.com/stretchr/testify/require"
)

func Test_Msgpack_Binding_Bind(t *testing.T) {
	t.Parallel()

	b := &MsgPackBinding{
		MsgPackDecoder: msgpack.Unmarshal,
	}
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
		"name": "john",
		"age":  42,
		"posts": []map[string]any{
			{"title": "post1"},
			{"title": "post2"},
			{"title": "post3"},
		},
	}
	data, err := msgpack.Marshal(input)
	require.NoError(t, err)

	err = b.Bind(data, &user)
	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Len(t, user.Posts, 3)
	require.Equal(t, "post1", user.Posts[0].Title)
	require.Equal(t, "post2", user.Posts[1].Title)
	require.Equal(t, "post3", user.Posts[2].Title)

	b.Reset()
	require.Nil(t, b.MsgPackDecoder)
}

func Benchmark_Msgpack_Binding_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &MsgPackBinding{
		MsgPackDecoder: msgpack.Unmarshal,
	}

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

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Len(b, user.Posts, 3)
	require.Equal(b, "post1", user.Posts[0])
	require.Equal(b, "post2", user.Posts[1])
	require.Equal(b, "post3", user.Posts[2])
}
