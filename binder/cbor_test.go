package binder

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func Test_CBORBinder_Bind(t *testing.T) {
	t.Parallel()
	b := &CBORBinding{CBORDecoder: cbor.Unmarshal}
	require.Equal(t, "cbor", b.Name())

	type Post struct {
		Title string `cbor:"title"`
	}

	type User struct {
		Name  string   `cbor:"name"`
		Posts []Post   `cbor:"posts"`
		Names []string `cbor:"names"`
		Age   int      `cbor:"age"`
	}
	var user User

	wantedUser := User{
		Name:  "john",
		Names: []string{"john", "doe"},
		Age:   42,
		Posts: []Post{{Title: "post1"}, {Title: "post2"}, {Title: "post3"}},
	}

	body, err := cbor.Marshal(wantedUser)
	require.NoError(t, err)

	require.NoError(t, b.Bind(body, &user))
	require.Equal(t, wantedUser, user)

	b.Reset()
	require.Nil(t, b.CBORDecoder)
}

func Benchmark_CBORBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &CBORBinding{CBORDecoder: cbor.Unmarshal}

	type User struct {
		Name string `cbor:"name"`
		Age  int    `cbor:"age"`
	}

	var user User
	wantedUser := User{Name: "john", Age: 42}

	body, err := cbor.Marshal(wantedUser)
	require.NoError(b, err)

	for b.Loop() {
		err = binder.Bind(body, &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
}
