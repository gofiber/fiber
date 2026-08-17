package binder

import (
	"bytes"
	"mime/multipart"
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

// multipartRequest re-encodes a urlencoded input as a multipart body, so the
// same table can be sent through bindMultipart without being written out twice.
func multipartRequest(t *testing.T, input string) *fasthttp.Request {
	t.Helper()

	var args fasthttp.Args
	args.Parse(input)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	for key, value := range args.All() {
		require.NoError(t, writer.WriteField(string(key), string(value)))
	}
	require.NoError(t, writer.Close())

	req := fasthttp.AcquireRequest()
	req.SetBody(body.Bytes())
	req.Header.SetContentType(writer.FormDataContentType())

	return req
}

// Test_Notation_FormAndQuery pins the key notations the binding docs promise.
// Form, multipart and query share one schema decoder, so all three run against
// the same table: a divergence between them is a bug in one of them.
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

			// Multipart takes its own branch through bindMultipart before it
			// reaches the shared decoder, so the docs only get to claim both.
			multipartReq := multipartRequest(t, tc.input)
			defer fasthttp.ReleaseRequest(multipartReq)

			var multipartOut notationTarget
			require.NoError(t, (&FormBinding{}).Bind(multipartReq, &multipartOut))
			require.Equal(t, tc.want, multipartOut)

			queryReq := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(queryReq)
			queryReq.URI().SetQueryString(tc.input)

			var query notationTarget
			require.NoError(t, (&QueryBinding{}).Bind(queryReq, &query))
			require.Equal(t, tc.want, query)
		})
	}
}
