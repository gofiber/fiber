package fiber

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/stretchr/testify/require"
)

type bindUpload struct {
	Avatar *multipart.FileHeader `form:"avatar" query:"avatar"`
	Name   string                `form:"name" query:"name"`
}

type bindUploadNested struct {
	Doc  struct{ File *multipart.FileHeader }   `form:"doc"`
	Docs []struct{ File *multipart.FileHeader } `form:"docs"`
}

// bindUploadCases returns n spellings in letter case of each value key that continues the name of the field "avatar".
func bindUploadKeys(n int) []string {
	var keys []string
	for i := range n {
		for _, tail := range []string{".filename", "[filename]", ".Filename", ".size", ".header.X-Evil", "[size]", "[header][X-Evil]"} {
			key := []byte("avatar" + tail)
			for j := range key {
				if (i>>(j%6))&1 == 1 && key[j] >= 'a' && key[j] <= 'z' {
					key[j] -= 'a' - 'A'
				}
			}
			keys = append(keys, string(key))
		}
	}
	return keys
}

// bindUploadRequest builds a multipart request holding the file part "avatar" and the value parts in values.
func bindUploadRequest(t *testing.T, fileName string, values []string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if fileName != "" {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="avatar"; filename="`+fileName+`"`)
		h.Set("Content-Type", "image/png")
		part, err := w.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write([]byte("png"))
		require.NoError(t, err)
	}
	require.NoError(t, w.WriteField("name", "john"))
	for _, key := range values {
		value := "../../escaped.txt"
		if strings.Contains(strings.ToLower(key), "size") {
			value = "-1"
		}
		require.NoError(t, w.WriteField(key, value))
	}
	require.NoError(t, w.Close())
	req := httptest.NewRequest(MethodPost, "/", &body)
	req.Header.Set(HeaderContentType, w.FormDataContentType())
	return req
}

func Test_Bind_FileHeader_IgnoresValueKeys(t *testing.T) {
	t.Parallel()
	binders := map[string]func(c Ctx, out any) error{
		"Form": func(c Ctx, out any) error { return c.Bind().Form(out) },
		"Body": func(c Ctx, out any) error { return c.Bind().Body(out) },
		"All":  func(c Ctx, out any) error { return c.Bind().All(out) },
	}
	for name, bind := range binders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := New()
			app.Post("/", func(c Ctx) error {
				var out bindUpload
				if err := bind(c, &out); err != nil {
					return err
				}
				own, err := c.FormFile("avatar")
				require.NoError(t, err)
				require.NotNil(t, out.Avatar)
				require.Equal(t, "cat.png", out.Avatar.Filename)
				require.Equal(t, int64(3), out.Avatar.Size)
				require.Equal(t, own.Header, out.Avatar.Header)
				require.Equal(t, "john", out.Name)
				return c.SendString("ok")
			})
			apptest.ExpectBody(t, app, bindUploadRequest(t, "../../cat.png", bindUploadKeys(64)), StatusOK, "ok")
		})
	}

	t.Run("value keys alone leave the field nil", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Post("/", func(c Ctx) error {
			var out bindUpload
			require.NoError(t, c.Bind().Form(&out))
			require.Nil(t, out.Avatar)
			return c.SendString("ok")
		})
		apptest.ExpectBody(t, app, bindUploadRequest(t, "", bindUploadKeys(64)), StatusOK, "ok")
	})

	t.Run("nested positions", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Post("/", func(c Ctx) error {
			var out bindUploadNested
			require.NoError(t, c.Bind().Form(&out))
			require.Nil(t, out.Doc.File)
			for _, doc := range out.Docs {
				require.Nil(t, doc.File)
			}
			return c.SendString("ok")
		})
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		for _, key := range []string{"doc[file][filename]", "doc.file.filename", "Doc.File.Size", "docs[1][file][filename]", "docs.1.file.size"} {
			require.NoError(t, w.WriteField(key, "1"))
		}
		require.NoError(t, w.Close())
		req := httptest.NewRequest(MethodPost, "/", &body)
		req.Header.Set(HeaderContentType, w.FormDataContentType())
		apptest.ExpectBody(t, app, req, StatusOK, "ok")
	})

	t.Run("a header bound by the form is not changed by the query", func(t *testing.T) {
		t.Parallel()
		app := New()
		app.Post("/", func(c Ctx) error {
			var out bindUpload
			require.NoError(t, c.Bind().Form(&out))
			require.NoError(t, c.Bind().Query(&out))
			require.NotNil(t, out.Avatar)
			require.Equal(t, "cat.png", out.Avatar.Filename)
			require.Equal(t, int64(3), out.Avatar.Size)
			return c.SendString("ok")
		})
		req := bindUploadRequest(t, "../../cat.png", nil)
		req.URL.RawQuery = "avatar.filename=x&avatar.size=-1&Avatar[Filename]=y"
		apptest.ExpectBody(t, app, req, StatusOK, "ok")
	})
}

// Test_Bind_FileHeader_UploadHandlerWritesInside runs the upload handler of the documentation: whatever the value
// parts hold, the file is saved in the upload directory.
func Test_Bind_FileHeader_UploadHandlerWritesInside(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	require.NoError(t, os.Mkdir(uploads, 0o750))

	app := New()
	app.Post("/", func(c Ctx) error {
		p := new(bindUpload)
		if err := c.Bind().Form(p); err != nil {
			return err
		}
		return c.SaveFile(p.Avatar, fmt.Sprintf("%s/%s", uploads, p.Avatar.Filename))
	})
	apptest.Expect(t, app, bindUploadRequest(t, "../../cat.png", bindUploadKeys(64)), StatusOK)

	require.FileExists(t, filepath.Join(uploads, "cat.png"))
	require.NoFileExists(t, filepath.Join(root, "escaped.txt"))
	require.NoFileExists(t, filepath.Join(uploads, "escaped.txt"))
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
