package binder

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Test_FormBinder_Bind_MixedCaseContentType covers callers that drive the
// binder directly, which do not go through Fiber's header normalization. Media
// types and parameter names are case-insensitive (RFC 9110 Sections 8.3.1 and
// 5.6.6), but fasthttp's PostArgs and MultipartForm match them case-sensitively
// — so the binder has to fold them before handing the request over.
func Test_FormBinder_Bind_MixedCaseContentType(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `form:"name"`
	}

	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "urlencoded", contentType: "Application/X-WWW-Form-Urlencoded", body: "name=john"},
		{
			// The boundary parameter *name* is folded, its value is not.
			name:        "multipart",
			contentType: `Multipart/Form-Data; BOUNDARY=AbC`,
			body:        "--AbC\r\nContent-Disposition: form-data; name=\"name\"\r\n\r\njohn\r\n--AbC--\r\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := acquireRequest(t)
			req.Header.SetMethod("POST")
			req.Header.SetContentType(tc.contentType)
			req.SetBodyString(tc.body)

			var user User
			require.NoError(t, (&FormBinding{}).Bind(req, &user))
			require.Equal(t, "john", user.Name)
		})
	}
}

func Test_ReleaseBindData(t *testing.T) {
	t.Parallel()
	small := acquireBindDataMode(bindMap)
	small.add("name", "john")
	releaseBindData(small)

	reused := acquireBindDataMode(bindMap)
	require.Empty(t, reused.values)
	releaseBindData(reused)

	pairs := acquireBindDataMode(bindPairs)
	pairs.add("name", "john")
	require.Equal(t, []string{"name"}, pairs.keys)
	require.Equal(t, []string{"john"}, pairs.pairValues)
	// Checked on reset rather than after release: once pairs is back in the
	// pool, a parallel test may take it and write the same array.
	keys := pairs.keys
	pairs.reset()
	require.Empty(t, keys[:1][0], "release must drop the strings the pairs held")
	releaseBindData(pairs)

	large := &bindData{values: make(map[string][]string, maxPoolableDataMapSize+1), mode: bindMap}
	for i := range maxPoolableDataMapSize + 1 {
		large.add(strings.Repeat("a", i+1), "value")
	}
	firstKey := "a"

	require.Len(t, large.values, maxPoolableDataMapSize+1)
	require.Equal(t, []string{"value"}, large.values[firstKey])

	releaseBindData(large)
	// An oversized one is dropped rather than cleared for reuse.
	require.Equal(t, []string{"value"}, large.values[firstKey])

	after := acquireBindDataMode(bindMap)
	require.Empty(t, after.values)
	require.Empty(t, after.keys)
	releaseBindData(after)
}

// Test_AcquireBindData_ReservesPairs pins that a bind into a struct gets room
// for its pairs up front, so that a large one does not grow them by appending,
// and that a bind into a map, which keeps no pairs, gets none.
func Test_AcquireBindData_ReservesPairs(t *testing.T) {
	t.Parallel()
	type Demo struct {
		Name string `form:"name"`
	}
	const n = 4 * maxPoolablePairs
	pairs := acquireBindData(&Demo{}, n)
	require.Equal(t, bindPairs, pairs.mode)
	require.GreaterOrEqual(t, cap(pairs.keys), n)
	require.GreaterOrEqual(t, cap(pairs.pairValues), n)
	for range n {
		pairs.add("name", "john")
	}
	require.Len(t, pairs.keys, n)
	releaseBindData(pairs)

	m := acquireBindData(&map[string]string{}, n)
	require.Equal(t, bindLast, m.mode)
	require.Less(t, cap(m.keys), n)
	releaseBindData(m)
}

func Benchmark_FormBinder_Bind(b *testing.B) {
	b.ReportAllocs()

	binder := &FormBinding{EnableSplitting: true}

	var user benchUser

	req := newFormRequest(b, "name=john&age=42&posts=post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}

func Test_FormBinder_BindMultipart(t *testing.T) {
	t.Parallel()
	b := &FormBinding{EnableSplitting: true}
	require.Equal(t, "form", b.Name())

	type Post struct {
		Title string `form:"title"`
	}

	type User struct {
		Avatar  *multipart.FileHeader   `form:"avatar"`
		Name    string                  `form:"name"`
		Names   []string                `form:"names"`
		Posts   []Post                  `form:"posts"`
		Avatars []*multipart.FileHeader `form:"avatars"`
		Age     int                     `form:"age"`
	}
	var user User

	req := newMultipartRequestWith(t, func(mw *multipart.Writer) {
		require.NoError(t, mw.WriteField("name", "john"))
		require.NoError(t, mw.WriteField("names", "john,eric"))
		require.NoError(t, mw.WriteField("names", "doe"))
		require.NoError(t, mw.WriteField("age", "42"))
		require.NoError(t, mw.WriteField("posts[0][title]", "post1"))
		require.NoError(t, mw.WriteField("posts[1][title]", "post2"))
		require.NoError(t, mw.WriteField("posts[2][title]", "post3"))

		writeFormFile(t, mw, "avatar", "avatar.txt", "avatar")
		writeFormFile(t, mw, "avatars", "avatar1.txt", "avatar1")
		writeFormFile(t, mw, "avatars", "avatar2.txt", "avatar2")
	})

	require.NoError(t, b.Bind(req, &user))
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Equal(t, []string{"john", "eric", "doe"}, user.Names)
	require.Equal(t, []Post{{Title: "post1"}, {Title: "post2"}, {Title: "post3"}}, user.Posts)

	require.NotNil(t, user.Avatar)
	requireFile(t, user.Avatar, "avatar.txt", "avatar")

	require.Len(t, user.Avatars, 2)
	requireFile(t, user.Avatars[0], "avatar1.txt", "avatar1")
	requireFile(t, user.Avatars[1], "avatar2.txt", "avatar2")
}

// requireFile pins the name, content type and content of an uploaded file.
func requireFile(t *testing.T, fh *multipart.FileHeader, filename, content string) {
	t.Helper()

	require.Equal(t, filename, fh.Filename)
	require.Equal(t, "application/octet-stream", fh.Header.Get("Content-Type"))

	file, err := fh.Open()
	require.NoError(t, err)

	got, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}

func Test_FormBinder_BindMultipart_BodyLimitExceeded(t *testing.T) {
	t.Parallel()
	b := &FormBinding{MaxBodySize: 64}

	type User struct {
		Name string `form:"name"`
	}
	var user User

	req := newMultipartRequestWith(t, func(mw *multipart.Writer) {
		require.NoError(t, mw.WriteField("name", strings.Repeat("a", 1024)))
	})

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, err := gz.Write(req.Body())
	require.NoError(t, err)
	require.NoError(t, gz.Flush())
	require.NoError(t, gz.Close())

	req.Header.SetContentEncoding("gzip")
	req.SetBody(compressed.Bytes())

	err = b.Bind(req, &user)
	require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
}

func Test_FormBinder_Bind_StreamedBodyLimitExceeded(t *testing.T) {
	t.Parallel()
	b := &FormBinding{MaxBodySize: 64}

	type User struct {
		Name string `form:"name"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })

	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyStream(strings.NewReader("name=john"), -1)
	require.NoError(t, b.Bind(req, &user))
	require.Equal(t, "john", user.Name)

	req.Reset()
	stream := strings.NewReader("name=" + strings.Repeat("a", 1024))
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyStream(stream, -1)
	require.ErrorIs(t, b.Bind(req, &user), fasthttp.ErrBodyTooLarge)
	require.Equal(t, 5+1024-65, stream.Len(), "the rest of the stream is not read")
}

func Test_FormBinder_Bind_StreamedNonFormBodyUnread(t *testing.T) {
	t.Parallel()
	b := &FormBinding{MaxBodySize: 64}

	type User struct {
		Name string `form:"name"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })

	stream := strings.NewReader("name=" + strings.Repeat("a", 1024))
	req.Header.SetContentType("application/octet-stream")
	req.SetBodyStream(stream, -1)
	require.NoError(t, b.Bind(req, &user))
	require.Empty(t, user.Name)
	require.True(t, req.IsBodyStream())
	require.Equal(t, 5+1024, stream.Len())
}

func Test_FormBinder_Bind_MapClearedBetweenRequests(t *testing.T) {
	t.Parallel()
	b := &FormBinding{}

	type payload struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}

	var first payload
	require.NoError(t, b.Bind(newFormRequest(t, "name=john&age=21"), &first))
	require.Equal(t, "john", first.Name)
	require.Equal(t, 21, first.Age)

	var second payload
	require.NoError(t, b.Bind(newFormRequest(t, "age=42"), &second))
	require.Empty(t, second.Name)
	require.Equal(t, 42, second.Age)
}

func Test_FormBinder_BindMultipart_MapsClearedBetweenRequests(t *testing.T) {
	t.Parallel()
	b := &FormBinding{}

	type payload struct { // betteralign:ignore - test payload prioritizes readability over alignment
		Avatar *multipart.FileHeader `form:"avatar"`
		Name   string                `form:"name"`
		Age    int                   `form:"age"`
	}

	firstReq := newMultipartRequestWith(t, func(mw *multipart.Writer) {
		require.NoError(t, mw.WriteField("name", "john"))
		require.NoError(t, mw.WriteField("age", "21"))
		writeFormFile(t, mw, "avatar", "avatar.txt", "avatar-content")
	})

	var first payload
	require.NoError(t, b.Bind(firstReq, &first))
	require.Equal(t, "john", first.Name)
	require.Equal(t, 21, first.Age)
	require.NotNil(t, first.Avatar)
	require.Equal(t, "avatar.txt", first.Avatar.Filename)

	secondReq := newMultipartRequestWith(t, func(mw *multipart.Writer) {
		require.NoError(t, mw.WriteField("age", "42"))
	})

	var second payload
	require.NoError(t, b.Bind(secondReq, &second))
	require.Empty(t, second.Name)
	require.Equal(t, 42, second.Age)
	require.Nil(t, second.Avatar)
}

func Benchmark_FormBinder_BindMultipart(b *testing.B) {
	b.ReportAllocs()

	binder := &FormBinding{EnableSplitting: true}

	var user benchUser

	req := newMultipartRequestWith(b, func(mw *multipart.Writer) {
		require.NoError(b, mw.WriteField("name", "john"))
		require.NoError(b, mw.WriteField("age", "42"))
		require.NoError(b, mw.WriteField("posts", "post1"))
		require.NoError(b, mw.WriteField("posts", "post2"))
		require.NoError(b, mw.WriteField("posts", "post3"))
	})

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}
