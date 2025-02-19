package binder

import (
	"bytes"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func Test_FormBinder_Bind(t *testing.T) {
	t.Parallel()

	b := &FormBinding{
		EnableSplitting: true,
	}
	require.Equal(t, "form", b.Name())

	type Post struct {
		Title string `form:"title"`
	}

	type User struct {
		Name  string   `form:"name"`
		Names []string `form:"names"`
		Posts []Post   `form:"posts"`
		Age   int      `form:"age"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	req.SetBodyString("name=john&names=john,doe&age=42&posts[0][title]=post1&posts[1][title]=post2&posts[2][title]=post3")
	req.Header.SetContentType("application/x-www-form-urlencoded")

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	err := b.Bind(req, &user)

	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Len(t, user.Posts, 3)
	require.Equal(t, "post1", user.Posts[0].Title)
	require.Equal(t, "post2", user.Posts[1].Title)
	require.Equal(t, "post3", user.Posts[2].Title)
	require.Contains(t, user.Names, "john")
	require.Contains(t, user.Names, "doe")

	b.Reset()
	require.False(t, b.EnableSplitting)
}

func Benchmark_FormBinder_Bind(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &QueryBinding{
		EnableSplitting: true,
	}

	type User struct {
		Name  string   `query:"name"`
		Posts []string `query:"posts"`
		Age   int      `query:"age"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	req.URI().SetQueryString("name=john&age=42&posts=post1,post2,post3")
	req.Header.SetContentType("application/x-www-form-urlencoded")

	b.ResetTimer()

	var err error
	for i := 0; i < b.N; i++ {
		err = binder.Bind(req, &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Len(b, user.Posts, 3)
}

func Test_FormBinder_BindMultipart(t *testing.T) {
	t.Parallel()

	b := &FormBinding{
		EnableSplitting: true,
	}
	require.Equal(t, "form", b.Name())

	type Post struct {
		Title string `form:"title"`
	}

	type User struct {
		Name  string   `form:"name"`
		Names []string `form:"names"`
		Posts []Post   `form:"posts"`
		Age   int      `form:"age"`
	}
	var user User

	req := fasthttp.AcquireRequest()

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	require.NoError(t, mw.WriteField("name", "john"))
	require.NoError(t, mw.WriteField("names", "john,eric"))
	require.NoError(t, mw.WriteField("names", "doe"))
	require.NoError(t, mw.WriteField("age", "42"))
	require.NoError(t, mw.WriteField("posts[0][title]", "post1"))
	require.NoError(t, mw.WriteField("posts[1][title]", "post2"))
	require.NoError(t, mw.WriteField("posts[2][title]", "post3"))

	require.NoError(t, mw.Close())

	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	err := b.Bind(req, &user)

	require.NoError(t, err)
	require.Equal(t, "john", user.Name)
	require.Equal(t, 42, user.Age)
	require.Contains(t, user.Names, "john")
	require.Contains(t, user.Names, "doe")
	require.Contains(t, user.Names, "eric")
	require.Len(t, user.Posts, 3)
	require.Equal(t, "post1", user.Posts[0].Title)
	require.Equal(t, "post2", user.Posts[1].Title)
	require.Equal(t, "post3", user.Posts[2].Title)
}

func Test_FormBinder_ShouldBindMultipartFormWithOnlyFiles(t *testing.T) {
	t.Parallel()

	b := &FormBinding{
		EnableSplitting: true,
	}

	type Document struct {
		File1 *multipart.FileHeader `form:"file1"`
		File2 *multipart.FileHeader `form:"file2"`
	}

	var doc Document

	// Create test request
	req := fasthttp.AcquireRequest()
	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// Create multipart form
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	// Add file 1
	file1Writer, err := mw.CreateFormFile("file1", "test1.txt")
	require.NoError(t, err)
	_, err = file1Writer.Write([]byte("test content 1"))
	require.NoError(t, err)

	// Add file 2
	file2Writer, err := mw.CreateFormFile("file2", "test2.txt")
	require.NoError(t, err)
	_, err = file2Writer.Write([]byte("test content 2"))
	require.NoError(t, err)

	require.NoError(t, mw.Close())

	// Setup request
	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	// Test bind operation
	err = b.Bind(req, &doc)
	require.NoError(t, err)

	// Check results
	require.NotNil(t, doc.File1)
	require.Equal(t, "test1.txt", doc.File1.Filename)

	require.NotNil(t, doc.File2)
	require.Equal(t, "test2.txt", doc.File2.Filename)
}

func Test_FormBinder_ShouldBindMultipartFormWithMixedFileAndStringFields(t *testing.T) {
	t.Parallel()

	b := &FormBinding{
		EnableSplitting: true,
	}

	type Person struct {
		File1      *multipart.FileHeader `form:"file1"`
		File2      *multipart.FileHeader `form:"file2"`
		TestString string                `form:"test_string"`
	}

	var person Person

	// Create test request
	req := fasthttp.AcquireRequest()
	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// Create multipart form
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	// Add file 1
	file1Writer, err := mw.CreateFormFile("file1", "test1.txt")
	require.NoError(t, err)
	_, err = file1Writer.Write([]byte("test content 1"))
	require.NoError(t, err)

	// Add file 2
	file2Writer, err := mw.CreateFormFile("file2", "test2.txt")
	require.NoError(t, err)
	_, err = file2Writer.Write([]byte("test content 2"))
	require.NoError(t, err)

	// Add string field
	require.NoError(t, mw.WriteField("test_string", "test string value"))

	require.NoError(t, mw.Close())

	// Setup request
	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	// Test bind operation
	err = b.Bind(req, &person)
	require.NoError(t, err)

	// Check results
	require.NotNil(t, person.File1)
	require.Equal(t, "test1.txt", person.File1.Filename)

	require.NotNil(t, person.File2)
	require.Equal(t, "test2.txt", person.File2.Filename)

	require.Equal(t, "test string value", person.TestString)
}

func Test_FormBinder_ShouldBindMultipartFormWithMixedFileAndNumberFields(t *testing.T) {
	t.Parallel()

	b := &FormBinding{
		EnableSplitting: true,
	}

	type Document struct {
		File1    *multipart.FileHeader `form:"file1"`
		File2    *multipart.FileHeader `form:"file2"`
		FileSize int64                 `form:"file_size"`
	}

	var doc Document

	// Create test request
	req := fasthttp.AcquireRequest()
	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// Create multipart form
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	// Add file 1
	file1Writer, err := mw.CreateFormFile("file1", "test1.txt")
	require.NoError(t, err)
	_, err = file1Writer.Write([]byte("test content 1"))
	require.NoError(t, err)

	// Add file 2
	file2Writer, err := mw.CreateFormFile("file2", "test2.txt")
	require.NoError(t, err)
	_, err = file2Writer.Write([]byte("test content 2"))
	require.NoError(t, err)

	// Add number field
	require.NoError(t, mw.WriteField("file_size", "1024"))

	require.NoError(t, mw.Close())

	// Setup request
	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	// Test bind operation
	err = b.Bind(req, &doc)
	require.NoError(t, err)

	// Check results
	require.NotNil(t, doc.File1)
	require.Equal(t, "test1.txt", doc.File1.Filename)

	require.NotNil(t, doc.File2)
	require.Equal(t, "test2.txt", doc.File2.Filename)

	require.Equal(t, int64(1024), doc.FileSize)
}

func Test_FormBinder_ShouldBindMultipartFormWithMultipleFiles(t *testing.T) {
	t.Parallel()

	b := &FormBinding{
		EnableSplitting: true,
	}

	type Document struct {
		Files []*multipart.FileHeader `form:"files"`
	}

	var doc Document

	// Create test request
	req := fasthttp.AcquireRequest()
	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// Create multipart form
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	// Add multiple files
	filenames := []string{"test1.txt", "test2.txt", "test3.txt"}
	for _, filename := range filenames {
		fileWriter, err := mw.CreateFormFile("files", filename)
		require.NoError(t, err)
		_, err = fileWriter.Write([]byte("test content"))
		require.NoError(t, err)
	}

	require.NoError(t, mw.Close())

	// Setup request
	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	// Test bind operation
	err := b.Bind(req, &doc)
	require.NoError(t, err)

	// Check results
	require.Len(t, doc.Files, 3)
	for i, file := range doc.Files {
		require.Equal(t, filenames[i], file.Filename)
	}
}

func Benchmark_FormBinder_BindMultipart(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &FormBinding{
		EnableSplitting: true,
	}

	type User struct {
		Name  string   `form:"name"`
		Posts []string `form:"posts"`
		Age   int      `form:"age"`
	}
	var user User

	req := fasthttp.AcquireRequest()
	b.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	require.NoError(b, mw.WriteField("name", "john"))
	require.NoError(b, mw.WriteField("age", "42"))
	require.NoError(b, mw.WriteField("posts", "post1"))
	require.NoError(b, mw.WriteField("posts", "post2"))
	require.NoError(b, mw.WriteField("posts", "post3"))
	require.NoError(b, mw.Close())

	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	b.ResetTimer()

	var err error
	for i := 0; i < b.N; i++ {
		err = binder.Bind(req, &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Len(b, user.Posts, 3)
}

func Benchmark_FormBinder_BindMultipartWithMixedTypes(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	binder := &FormBinding{
		EnableSplitting: true,
	}

	type Document struct {
		File1    *multipart.FileHeader `form:"file1"`
		Name     string                `form:"name"`
		FileSize int64                 `form:"file_size"`
	}
	var doc Document

	// Create initial request template
	req := fasthttp.AcquireRequest()
	b.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	// Create form data template
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	// Add files
	file1Writer, err := mw.CreateFormFile("file1", "test1.txt")
	require.NoError(b, err)
	_, err = file1Writer.Write([]byte("test content 1"))
	require.NoError(b, err)

	// Add string and number fields
	require.NoError(b, mw.WriteField("name", "test document"))
	require.NoError(b, mw.WriteField("file_size", "1024"))
	require.NoError(b, mw.Close())

	// Setup request
	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		err = binder.Bind(req, &doc)
	}

	// Verify results
	require.NoError(b, err)
	require.NotNil(b, doc.File1)
	require.Equal(b, "test document", doc.Name)
	require.Equal(b, int64(1024), doc.FileSize)
}
