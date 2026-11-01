package session

import (
	"testing"

	"github.com/gofiber/fiber/v3/extractors"
	"github.com/stretchr/testify/require"
)

// sinkNames names the sinks as "source:key", in order; nil for none.
func sinkNames(sinks []extractors.Extractor) []string {
	var names []string
	for _, sink := range sinks {
		switch sink.Source {
		case extractors.SourceCookie:
			names = append(names, "cookie:"+sink.Key)
		case extractors.SourceHeader:
			names = append(names, "header:"+sink.Key)
		default:
			names = append(names, "unexpected:"+sink.Key)
		}
	}
	return names
}

// A store works out once where a session ID can be written back to: the cookies and headers of its
// extractor, in order, and nothing a request only reads from. A store built without NewStore finds
// the same by walking the extractor.
func Test_Store_WritableSinks(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		want      []string
		extractor extractors.Extractor
	}{
		{name: "cookie", extractor: extractors.FromCookie("sid"), want: []string{"cookie:sid"}},
		{name: "header", extractor: extractors.FromHeader("X-Sid"), want: []string{"header:X-Sid"}},
		{name: "query is read-only", extractor: extractors.FromQuery("sid")},
		{name: "authorization header is read-only", extractor: extractors.FromAuthHeader("Bearer")},
		{
			name: "chain keeps the order and drops what is read-only",
			extractor: extractors.Chain(
				extractors.FromQuery("q"),
				extractors.FromCookie("a"),
				extractors.FromAuthHeader("Bearer"),
				extractors.FromHeader("X-B"),
			),
			want: []string{"cookie:a", "header:X-B"},
		},
		{
			name: "chain inside a chain",
			extractor: extractors.Chain(
				extractors.FromCookie("a"),
				extractors.Chain(extractors.FromHeader("X-B"), extractors.FromCookie("c")),
			),
			want: []string{"cookie:a", "header:X-B", "cookie:c"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewStore(Config{Extractor: tc.extractor})
			require.NotNil(t, store.sinks, "NewStore works the sinks out, even when there are none")
			require.Equal(t, tc.want, sinkNames(store.writableSinks()))

			literal := &Store{Config: store.Config}
			require.Nil(t, literal.sinks)
			require.Equal(t, tc.want, sinkNames(literal.writableSinks()), "a store built without NewStore walks the extractor")
		})
	}
}

// Finding the sinks of a session costs a save nothing: with the store's own slice, or the
// session's, there is nothing to allocate.
//
// testing.AllocsPerRun panics in a parallel test, so this one stays sequential.
func Test_Session_getExtractorInfo_DoesNotAllocate(t *testing.T) {
	store := NewStore(Config{Extractor: extractors.Chain(extractors.FromCookie("a"), extractors.FromHeader("X-B"))})
	provenance := &Session{
		config:    store,
		extractor: extractors.Result{Value: "v", Key: "a", Source: extractors.SourceCookie, Resolved: true},
	}
	fresh := &Session{config: store, isFresh: true}
	refused := &Session{config: store, extractor: extractors.Result{Value: "v", Key: "q", Source: extractors.SourceQuery, Resolved: true}}

	require.Zero(t, testing.AllocsPerRun(100, func() { _ = provenance.getExtractorInfo() }), "the sink the ID came from")
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = fresh.getExtractorInfo() }), "the sinks of a new session")
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = refused.getExtractorInfo() }), "an ID that must not be written back")
}
