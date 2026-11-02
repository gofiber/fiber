package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
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
