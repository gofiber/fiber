package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// pairUser is what the scenarios below bind into. Each binder finds the fields
// through its own tag.
type pairUser struct {
	Name  string   `cookie:"name" form:"name" header:"Name" query:"name" respHeader:"name"`
	Names []string `cookie:"names" form:"names" header:"Names" query:"names" respHeader:"names"`
	Posts []string `cookie:"posts" form:"posts" header:"Posts" query:"posts" respHeader:"posts"`
	Age   int      `cookie:"age" form:"age" header:"Age" query:"age" respHeader:"age"`
}

// benchUser is what the benchmarks of the pair-reading binders bind; each finds
// the fields through its own tag.
type benchUser struct {
	Name  string   `cookie:"name" form:"name" header:"Name" query:"name" respHeader:"name"`
	Posts []string `cookie:"posts" form:"posts" header:"Posts" query:"posts" respHeader:"posts"`
	Age   int      `cookie:"age" form:"age" header:"Age" query:"age" respHeader:"age"`
}

// Test_PairSources_Bind pins that every binder that reads name/value pairs
// fills a struct from them, and splits a comma separated value into a slice
// when splitting is on.
func Test_PairSources_Bind(t *testing.T) {
	t.Parallel()
	for _, src := range pairSources {
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			var user pairUser
			require.NoError(t, src.bind(t, "name=john&names=john,doe&age=42&posts=post1,post2,post3", true, &user))
			posts := []string{"post1", "post2", "post3"}
			require.Equal(t, pairUser{Name: "john", Names: []string{"john", "doe"}, Posts: posts, Age: 42}, user)
		})
	}
}

// Test_PairSources_ParseError pins that a value the destination cannot hold is
// an error from every binder that reads name/value pairs.
func Test_PairSources_ParseError(t *testing.T) {
	t.Parallel()
	for _, src := range pairSources {
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			var user pairUser
			require.Error(t, src.bind(t, "age=invalid", false, &user))
		})
	}
}

// Test_PairSources_Splitting_ElementWhitespace pins that only the header binders
// read a value as an HTTP list and trim its elements; the others keep each
// element as written.
func Test_PairSources_Splitting_ElementWhitespace(t *testing.T) {
	t.Parallel()
	type Data struct {
		IDs []string `cookie:"ids" form:"ids" header:"Ids" query:"ids" respHeader:"ids"`
	}

	for _, src := range pairSources {
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			want := []string{"a", " b"}
			if src.list {
				want = []string{"a", "b"}
			}

			var data Data
			require.NoError(t, src.bind(t, "ids=a,%20b", true, &data))
			require.Equal(t, want, data.IDs)
		})
	}
}

// Test_HeaderSources_Splitting_Lists pins how the header binders split a list
// (RFC 9110 Section 5.6.1) into a struct and into a map of slices.
func Test_HeaderSources_Splitting_Lists(t *testing.T) {
	t.Parallel()
	type listTarget struct {
		Roles  []string `header:"X-Roles" respHeader:"X-Roles"`
		Tags   []string `header:"X-Tags" respHeader:"X-Tags"`
		IDs    []int    `header:"X-Ids" respHeader:"X-Ids"`
		Counts []int    `header:"X-Counts" respHeader:"X-Counts"`
	}

	for _, src := range pairSources {
		if !src.list {
			continue
		}
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				wantMap map[string][]string
				name    string
				input   string
				want    listTarget
				stale   bool // the destination holds a roles value before the bind
			}{
				{
					// Clients and proxies join repeated lines with ", " (RFC 9110 Section 5.3), and the
					// space around a list element is not part of it.
					name:    "SP and HTAB around an element are not part of it",
					input:   "x-roles=admin,+editor&x-ids=1,+2,%093",
					want:    listTarget{Roles: []string{"admin", "editor"}, IDs: []int{1, 2, 3}},
					wantMap: map[string][]string{"X-Roles": {"admin", "editor"}, "X-Ids": {"1", "2", "3"}},
				},
				{
					// Only SP and HTAB are OWS: another control byte stays in the element.
					name: "another control byte stays in the element", input: "x-tags=a,%0Bb",
					want:    listTarget{Tags: []string{"a", "\vb"}},
					wantMap: map[string][]string{"X-Tags": {"a", "\vb"}},
				},
				{
					name: "a tab right after the comma and a trailing comma", input: "x-ids=1,%092,",
					want:    listTarget{IDs: []int{1, 2}},
					wantMap: map[string][]string{"X-Ids": {"1", "2"}},
				},
				{
					// Empty elements are ignored (RFC 9110 Section 5.6.1), so a trailing or doubled
					// comma adds no zero value.
					name: "empty elements add no zero value", input: "x-ids=1,+2,&x-counts=3,,+4",
					want:    listTarget{IDs: []int{1, 2}, Counts: []int{3, 4}},
					wantMap: map[string][]string{"X-Ids": {"1", "2"}, "X-Counts": {"3", "4"}},
				},
				{
					// A present header still replaces what was bound: one made only of empty
					// elements binds as an empty value.
					name: "a list of empty elements replaces what was bound", input: "x-roles=+,+", stale: true,
					want:    listTarget{Roles: []string{""}},
					wantMap: map[string][]string{"X-Roles": {""}},
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					var got listTarget
					values := map[string][]string{}
					if tc.stale {
						got.Roles = []string{"stale"}
						values["X-Roles"] = []string{"stale"}
					}

					require.NoError(t, src.bind(t, tc.input, true, &got))
					require.Equal(t, tc.want, got)
					require.NoError(t, src.bind(t, tc.input, true, &values))
					for key, want := range tc.wantMap {
						require.Equal(t, want, values[key], key)
					}
				})
			}
		})
	}
}

func Test_FormSources_Bind(t *testing.T) {
	t.Parallel()
	type Post struct {
		Title string `form:"title" query:"title"`
	}

	type User struct {
		Name  string   `form:"name" query:"name"`
		Names []string `form:"names" query:"names"`
		Posts []Post   `form:"posts" query:"posts"`
		Age   int      `form:"age" query:"age"`
	}

	for _, src := range formSources {
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			var user User
			require.NoError(t, src.bind(t, "name=john&names=john,doe&age=42&posts[0][title]=post1&posts[1][title]=post2&posts[2][title]=post3", true, &user))
			posts := []Post{{Title: "post1"}, {Title: "post2"}, {Title: "post3"}}
			require.Equal(t, User{Name: "john", Names: []string{"john", "doe"}, Posts: posts, Age: 42}, user)
		})
	}
}

func Test_FormSources_PointerScalars(t *testing.T) {
	t.Parallel()
	type Scalars struct {
		ID     *int64   `form:"id" query:"id"`
		Name   *string  `form:"name" query:"name"`
		Active *bool    `form:"active" query:"active"`
		Score  *float64 `form:"score" query:"score"`
	}

	runFormSources(t, []formCase[Scalars]{
		{
			name:  "all fields provided",
			input: "id=123&name=test&active=true&score=98.5",
			want:  Scalars{ID: new(int64(123)), Name: new("test"), Active: new(true), Score: new(98.5)},
		},
		{name: "no fields provided", input: "", want: Scalars{}},
		{
			name:  "partial fields provided",
			input: "id=456&active=false",
			want:  Scalars{ID: new(int64(456)), Active: new(false)},
		},
		{
			name:  "explicit zero and empty values provided",
			input: "id=0&name=&active=false&score=0",
			want:  Scalars{ID: new(int64(0)), Name: new(""), Active: new(false), Score: new(0.0)},
		},
	})
}

func Test_FormSources_OptionalIDParam(t *testing.T) {
	t.Parallel()

	// Use case from original issue
	type OptionalIDParam struct {
		IDPtr *int64 `form:"id" query:"id"`
	}

	runFormSources(t, []formCase[OptionalIDParam]{
		{name: "id provided", input: "id=123", want: OptionalIDParam{IDPtr: new(int64(123))}},
		{name: "id not provided", input: "", want: OptionalIDParam{}},
		{name: "id zero", input: "id=0", want: OptionalIDParam{IDPtr: new(int64(0))}},
	})
}
