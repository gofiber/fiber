package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Benchmark_QueryBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &QueryBinding{EnableSplitting: true}

	var user benchUser

	req := newQueryRequest(b, "name=john&age=42&posts=post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}

func Test_QueryBinder_Bind_PointerSlices(t *testing.T) {
	t.Parallel()
	binder := &QueryBinding{EnableSplitting: true}

	type Preferences struct {
		Tags *[]string `query:"tags"`
	}

	type Profile struct {
		Emails *[]string    `query:"emails"`
		Prefs  *Preferences `query:"preferences"`
	}

	var profile Profile

	req := newQueryRequest(t, "emails=work,personal&preferences[tags]=golang,api")

	require.NoError(t, binder.Bind(req, &profile))

	require.NotNil(t, profile.Emails)
	require.ElementsMatch(t, []string{"work", "personal"}, *profile.Emails)

	require.NotNil(t, profile.Prefs)
	require.NotNil(t, profile.Prefs.Tags)
	require.ElementsMatch(t, []string{"golang", "api"}, *profile.Prefs.Tags)
}

func Test_QueryBinder_Bind_Splitting_ScalarNotSplit(t *testing.T) {
	t.Parallel()
	b := &QueryBinding{EnableSplitting: true}

	type Filter struct {
		IDs []int `query:"ids"`
	}
	type Request struct {
		Name   string `query:"name"`
		Filter Filter `query:"filter"`
	}

	req := newQueryRequest(t, "name=Smith,+John&filter[ids]=1,2")

	var out Request
	require.NoError(t, b.Bind(req, &out))
	require.Equal(t, "Smith, John", out.Name)
	require.Equal(t, []int{1, 2}, out.Filter.IDs)
}

func Test_QueryBinder_Bind_Splitting_EmbeddedStruct(t *testing.T) {
	t.Parallel()
	b := &QueryBinding{EnableSplitting: true}

	type Embedded struct {
		Title string   `query:"title"`
		Names []string `query:"names"`
	}
	type Request struct {
		Name string `query:"name"`
		Embedded
	}

	req := newQueryRequest(t, "names=a,b&title=x,y&name=p,q")

	var out Request
	require.NoError(t, b.Bind(req, &out))
	require.Equal(t, []string{"a", "b"}, out.Names)
	require.Equal(t, "x,y", out.Title)
	require.Equal(t, "p,q", out.Name)
}

func Test_QueryBinder_Bind_Splitting_NestedDepth(t *testing.T) {
	t.Parallel()
	type Inner struct {
		Name string   `query:"name"`
		Tags []string `query:"tags"`
	}
	type Mid struct {
		Inner Inner `query:"inner"`
	}
	type Item struct {
		Name string   `query:"name"`
		Tags []string `query:"tags"`
	}
	type Request struct {
		Mid   Mid    `query:"mid"`
		Items []Item `query:"items"`
	}

	want := Request{
		Mid:   Mid{Inner: Inner{Name: "c,d", Tags: []string{"a", "b"}}},
		Items: []Item{{Name: "g,h", Tags: []string{"e", "f"}}, {Tags: []string{"i", "j"}}},
	}

	cases := []struct {
		name  string
		query string
	}{
		{
			name:  "bracket notation",
			query: "mid[inner][tags]=a,b&mid[inner][name]=c,d&items[0][tags]=e,f&items[0][name]=g,h&items[1][tags]=i,j",
		},
		{
			name:  "dot notation",
			query: "mid.inner.tags=a,b&mid.inner.name=c,d&items.0.tags=e,f&items.0.name=g,h&items.1.tags=i,j",
		},
		{
			name:  "mixed notation",
			query: "mid[inner].tags=a,b&mid.inner[name]=c,d&items[0].tags=e,f&items.0[name]=g,h&items[1].tags=i,j",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := &QueryBinding{EnableSplitting: true}

			req := newQueryRequest(t, tc.query)

			var out Request
			require.NoError(t, b.Bind(req, &out))
			require.Equal(t, want, out)
		})
	}
}

func Test_QueryBinder_Bind_Splitting_EmptyAliasWithOptions(t *testing.T) {
	t.Parallel()
	b := &QueryBinding{EnableSplitting: true}

	type Filter struct {
		Tags []string `query:",default:x|y"`
	}

	req := newQueryRequest(t, "tags=a,b")

	var filter Filter
	require.NoError(t, b.Bind(req, &filter))
	require.Equal(t, []string{"a", "b"}, filter.Tags)

	// The default only fills the field when the key is absent.
	empty := acquireRequest(t)

	var defaulted Filter
	require.NoError(t, b.Bind(empty, &defaulted))
	require.Equal(t, []string{"x", "y"}, defaulted.Tags)
}

func Test_QueryBinder_Bind_Splitting_NilSliceMap(t *testing.T) {
	t.Parallel()
	b := &QueryBinding{EnableSplitting: true}

	req := newQueryRequest(t, "ids=1,2")

	// The decoder allocates the map, so a nil one still splits.
	var sliceMap map[string][]string
	require.NoError(t, b.Bind(req, &sliceMap))
	require.Equal(t, []string{"1", "2"}, sliceMap["ids"])

	var stringMap map[string]string
	require.NoError(t, b.Bind(req, &stringMap))
	require.Equal(t, "1,2", stringMap["ids"])
}

func Test_QueryBinder_Bind_Splitting_InvalidDestination(t *testing.T) {
	t.Parallel()
	b := &QueryBinding{EnableSplitting: true}

	type User struct {
		Names []string `query:"names"`
	}

	req := newQueryRequest(t, "names=john,doe")

	// The decoder reports these destinations; splitting must not panic first.
	require.NotPanics(t, func() {
		require.Error(t, b.Bind(req, User{}))
		require.Error(t, b.Bind(req, nil))
	})
}
