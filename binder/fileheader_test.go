package binder

import (
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_FileHeaders_UnchangedHeaderIsNotWritten pins that a bind writes a request's own *multipart.FileHeader only
// when schema changed it: the header is what c.FormFile and c.MultipartForm return, and an application may be
// reading it from another goroutine while the handler binds again.
func Test_FileHeaders_UnchangedHeaderIsNotWritten(t *testing.T) {
	t.Parallel()
	type upload struct {
		Avatar *multipart.FileHeader `form:"avatar" query:"avatar"`
		Name   string                `form:"name" query:"name"`
	}
	req := newMultipartRequestWith(t, func(mw *multipart.Writer) {
		require.NoError(t, mw.WriteField("name", "john"))
		writeFormFile(t, mw, "avatar", "avatar.txt", "avatar")
	})

	var first upload
	require.NoError(t, (&FormBinding{}).Bind(req, &first))
	require.NotNil(t, first.Avatar)
	header := reflect.ValueOf(first.Avatar.Header).Pointer()

	var second upload
	require.NoError(t, (&FormBinding{}).Bind(req, &second))
	require.Same(t, first.Avatar, second.Avatar)
	require.NoError(t, (&QueryBinding{}).Bind(req, &first))
	require.Equal(t, header, reflect.ValueOf(first.Avatar.Header).Pointer(), "a header schema did not change is not written back")

	// A header schema did change is put back as it was.
	req.URI().SetQueryString("avatar.filename=x&avatar.size=-1&avatar.header.X-Extra=y")
	require.NoError(t, (&QueryBinding{}).Bind(req, &first))
	requireFile(t, first.Avatar, "avatar.txt", "avatar")
	require.Equal(t, int64(len("avatar")), first.Avatar.Size)
	require.Empty(t, first.Avatar.Header.Get("X-Extra"))
}

// Test_HeaderFields pins the fields a walk visits: the ones schema can fill, an embedded struct of an unexported
// type included, through which a *multipart.FileHeader can be reached.
func Test_HeaderFields(t *testing.T) {
	t.Parallel()
	type base struct{ Avatar *multipart.FileHeader }
	type node struct {
		Next *node
		File *multipart.FileHeader
	}
	type mixed struct {
		Meta   map[string]string
		Node   *node
		Docs   map[string][]*multipart.FileHeader
		hidden *multipart.FileHeader
		Avatar *multipart.FileHeader
		base
		Name    string
		Avatars []*multipart.FileHeader
	}
	require.Nil(t, mixed{}.hidden) // hidden is there to be skipped; reading it keeps the linter from calling it unused
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		want []int
	}{
		{"mixed", reflect.TypeFor[mixed](), []int{1, 2, 4, 5, 7}},
		{"a type that reaches itself", reflect.TypeFor[node](), []int{0, 1}},
		{"no header", reflect.TypeFor[struct{ Name string }](), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, headerFields(tc.typ))
			require.Equal(t, tc.want, headerFields(tc.typ), "the cached answer is the same")
		})
	}
}

// Test_GuardFileHeaders_NothingToRecord pins that a guard with no file part and no header held yet allocates the
// guard alone: the record of the headers is made when there is one to put in it.
// Sequential: testing.AllocsPerRun panics during a parallel test.
func Test_GuardFileHeaders_NothingToRecord(t *testing.T) {
	var out struct{ Avatar *multipart.FileHeader }
	require.LessOrEqual(t, testing.AllocsPerRun(100, func() { guardFileHeaders(&out, nil).restore(&out) }), 1.0)
}

// benchUpload is benchUser with a file to bind, which makes a bind walk its fields.
type benchUpload struct {
	Avatar *multipart.FileHeader `form:"avatar" query:"avatar"`
	Name   string                `form:"name" query:"name"`
	Posts  []string              `form:"posts" query:"posts"`
	Age    int                   `form:"age" query:"age"`
}

func Benchmark_QueryBinder_Bind_FileHeader(b *testing.B) {
	b.ReportAllocs()

	binder := &QueryBinding{EnableSplitting: true}

	var user benchUpload

	req := newQueryRequest(b, "name=john&age=42&posts=post1,post2,post3")

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
}

func Benchmark_FormBinder_BindMultipart_FileHeader(b *testing.B) {
	b.ReportAllocs()

	binder := &FormBinding{EnableSplitting: true}

	var user benchUpload

	req := newMultipartRequestWith(b, func(mw *multipart.Writer) {
		require.NoError(b, mw.WriteField("name", "john"))
		require.NoError(b, mw.WriteField("age", "42"))
		require.NoError(b, mw.WriteField("posts", "post1"))
		require.NoError(b, mw.WriteField("posts", "post2"))
		require.NoError(b, mw.WriteField("posts", "post3"))
		writeFormFile(b, mw, "avatar", "avatar.txt", "avatar")
	})

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	requireJohn42(b, err, user.Name, user.Age, user.Posts)
	require.NotNil(b, user.Avatar)
}
