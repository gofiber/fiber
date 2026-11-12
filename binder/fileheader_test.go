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
