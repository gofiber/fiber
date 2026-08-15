package binder

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type notationInner struct {
	Name string `form:"name" query:"name"`
	Age  int    `form:"age" query:"age"`
}

type notationDeep struct {
	Sub notationInner `form:"sub" query:"sub"`
}

type notationTarget struct {
	Inner  notationInner   `form:"inner" query:"inner"`
	Deep   notationDeep    `form:"deep" query:"deep"`
	Colors []string        `form:"colors" query:"colors"`
	Items  []notationInner `form:"items" query:"items"`
}

// Test_Notation_FormAndQuery pins the key notations the binding docs promise.
// Form and query share one schema decoder, so both run against the same table:
// a divergence between them is a bug in either.
func Test_Notation_FormAndQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  notationTarget
	}{
		{
			name:  "repeated key",
			input: "colors=red&colors=blue",
			want:  notationTarget{Colors: []string{"red", "blue"}},
		},
		{
			name:  "empty brackets",
			input: "colors[]=red&colors[]=blue",
			want:  notationTarget{Colors: []string{"red", "blue"}},
		},
		{
			// An index addresses a struct field, so it never reaches a []string
			// element. The value is dropped, and without an error.
			name:  "index on a scalar slice is dropped",
			input: "colors[0]=red&colors[1]=blue",
			want:  notationTarget{},
		},
		{
			name:  "dot index on a scalar slice is dropped",
			input: "colors.0=red&colors.1=blue",
			want:  notationTarget{},
		},
		{
			name:  "bracket struct field",
			input: "inner[name]=bob&inner[age]=7",
			want:  notationTarget{Inner: notationInner{Name: "bob", Age: 7}},
		},
		{
			name:  "dot struct field",
			input: "inner.name=bob&inner.age=7",
			want:  notationTarget{Inner: notationInner{Name: "bob", Age: 7}},
		},
		{
			name:  "indexed brackets on a struct slice",
			input: "items[0][name]=a&items[1][name]=b",
			want:  notationTarget{Items: []notationInner{{Name: "a"}, {Name: "b"}}},
		},
		{
			name:  "dot index on a struct slice",
			input: "items.0.name=a&items.1.name=b",
			want:  notationTarget{Items: []notationInner{{Name: "a"}, {Name: "b"}}},
		},
		{
			name:  "bracket index mixed with a dot field",
			input: "items[0].name=a&items[1].name=b",
			want:  notationTarget{Items: []notationInner{{Name: "a"}, {Name: "b"}}},
		},
		{
			name:  "brackets two levels deep",
			input: "deep[sub][name]=x&deep[sub][age]=3",
			want:  notationTarget{Deep: notationDeep{Sub: notationInner{Name: "x", Age: 3}}},
		},
		{
			name:  "dots two levels deep",
			input: "deep.sub.name=x&deep.sub.age=3",
			want:  notationTarget{Deep: notationDeep{Sub: notationInner{Name: "x", Age: 3}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			formReq := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(formReq)
			formReq.SetBodyString(tc.input)
			formReq.Header.SetContentType("application/x-www-form-urlencoded")

			var form notationTarget
			require.NoError(t, (&FormBinding{}).Bind(formReq, &form))
			require.Equal(t, tc.want, form)

			queryReq := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(queryReq)
			queryReq.URI().SetQueryString(tc.input)

			var query notationTarget
			require.NoError(t, (&QueryBinding{}).Bind(queryReq, &query))
			require.Equal(t, tc.want, query)
		})
	}
}
