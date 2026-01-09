package binder

import (
	"bytes"
	"io"
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

func Test_FormBinder_Bind_ParseError(t *testing.T) {
	b := &FormBinding{}
	type User struct {
		Age int `form:"age"`
	}
	var user User
	req := fasthttp.AcquireRequest()
	req.SetBodyString("age=invalid")
	req.Header.SetContentType("application/x-www-form-urlencoded")
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	err := b.Bind(req, &user)
	require.Error(t, err)
}

func Benchmark_FormBinder_Bind(b *testing.B) {
	b.ReportAllocs()

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
	req.SetBodyString("name=john&age=42&posts=post1,post2,post3")
	req.Header.SetContentType("application/x-www-form-urlencoded")

	var err error
	for b.Loop() {
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
		Avatar  *multipart.FileHeader   `form:"avatar"`
		Name    string                  `form:"name"`
		Names   []string                `form:"names"`
		Posts   []Post                  `form:"posts"`
		Avatars []*multipart.FileHeader `form:"avatars"`
		Age     int                     `form:"age"`
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

	writer, err := mw.CreateFormFile("avatar", "avatar.txt")
	require.NoError(t, err)

	_, err = writer.Write([]byte("avatar"))
	require.NoError(t, err)

	writer, err = mw.CreateFormFile("avatars", "avatar1.txt")
	require.NoError(t, err)

	_, err = writer.Write([]byte("avatar1"))
	require.NoError(t, err)

	writer, err = mw.CreateFormFile("avatars", "avatar2.txt")
	require.NoError(t, err)

	_, err = writer.Write([]byte("avatar2"))
	require.NoError(t, err)

	require.NoError(t, mw.Close())

	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())

	t.Cleanup(func() {
		fasthttp.ReleaseRequest(req)
	})

	err = b.Bind(req, &user)

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

	require.NotNil(t, user.Avatar)
	require.Equal(t, "avatar.txt", user.Avatar.Filename)
	require.Equal(t, "application/octet-stream", user.Avatar.Header.Get("Content-Type"))

	file, err := user.Avatar.Open()
	require.NoError(t, err)

	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "avatar", string(content))

	require.Len(t, user.Avatars, 2)
	require.Equal(t, "avatar1.txt", user.Avatars[0].Filename)
	require.Equal(t, "application/octet-stream", user.Avatars[0].Header.Get("Content-Type"))

	file, err = user.Avatars[0].Open()
	require.NoError(t, err)

	content, err = io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "avatar1", string(content))

	require.Equal(t, "avatar2.txt", user.Avatars[1].Filename)
	require.Equal(t, "application/octet-stream", user.Avatars[1].Header.Get("Content-Type"))

	file, err = user.Avatars[1].Open()
	require.NoError(t, err)

	content, err = io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "avatar2", string(content))
}

func Test_FormBinder_BindMultipart_ValueError(t *testing.T) {
	b := &FormBinding{}
	req := fasthttp.AcquireRequest()
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	require.NoError(t, mw.WriteField("invalid[", "val"))
	require.NoError(t, mw.Close())
	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	err := b.Bind(req, &struct{}{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unmatched brackets")
}

func Test_FormBinder_BindMultipart_FileError(t *testing.T) {
	b := &FormBinding{}
	req := fasthttp.AcquireRequest()
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	writer, err := mw.CreateFormFile("invalid[", "file.txt")
	require.NoError(t, err)
	_, err = writer.Write([]byte("content"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req.Header.SetContentType(mw.FormDataContentType())
	req.SetBody(buf.Bytes())
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	err = b.Bind(req, &struct{}{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unmatched brackets")
}

func Test_FormBinder_Bind_MapClearedBetweenRequests(t *testing.T) {
	t.Parallel()

	b := &FormBinding{}

	type payload struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}

	firstReq := fasthttp.AcquireRequest()
	firstReq.SetBodyString("name=john&age=21")
	firstReq.Header.SetContentType("application/x-www-form-urlencoded")
	t.Cleanup(func() { fasthttp.ReleaseRequest(firstReq) })

	var first payload
	require.NoError(t, b.Bind(firstReq, &first))
	require.Equal(t, "john", first.Name)
	require.Equal(t, 21, first.Age)

	secondReq := fasthttp.AcquireRequest()
	secondReq.SetBodyString("age=42")
	secondReq.Header.SetContentType("application/x-www-form-urlencoded")
	t.Cleanup(func() { fasthttp.ReleaseRequest(secondReq) })

	var second payload
	require.NoError(t, b.Bind(secondReq, &second))
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

	firstReq := fasthttp.AcquireRequest()
	firstBuffer := &bytes.Buffer{}
	firstWriter := multipart.NewWriter(firstBuffer)

	require.NoError(t, firstWriter.WriteField("name", "john"))
	require.NoError(t, firstWriter.WriteField("age", "21"))

	firstFile, err := firstWriter.CreateFormFile("avatar", "avatar.txt")
	require.NoError(t, err)
	_, err = firstFile.Write([]byte("avatar-content"))
	require.NoError(t, err)
	require.NoError(t, firstWriter.Close())

	firstReq.Header.SetContentType(firstWriter.FormDataContentType())
	firstReq.SetBody(firstBuffer.Bytes())
	t.Cleanup(func() { fasthttp.ReleaseRequest(firstReq) })

	var first payload
	require.NoError(t, b.Bind(firstReq, &first))
	require.Equal(t, "john", first.Name)
	require.Equal(t, 21, first.Age)
	require.NotNil(t, first.Avatar)
	require.Equal(t, "avatar.txt", first.Avatar.Filename)

	secondReq := fasthttp.AcquireRequest()
	secondBuffer := &bytes.Buffer{}
	secondWriter := multipart.NewWriter(secondBuffer)
	require.NoError(t, secondWriter.WriteField("age", "42"))
	require.NoError(t, secondWriter.Close())

	secondReq.Header.SetContentType(secondWriter.FormDataContentType())
	secondReq.SetBody(secondBuffer.Bytes())
	t.Cleanup(func() { fasthttp.ReleaseRequest(secondReq) })

	var second payload
	require.NoError(t, b.Bind(secondReq, &second))
	require.Empty(t, second.Name)
	require.Equal(t, 42, second.Age)
	require.Nil(t, second.Avatar)
}

func Benchmark_FormBinder_BindMultipart(b *testing.B) {
	b.ReportAllocs()

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

	var err error
	for b.Loop() {
		err = binder.Bind(req, &user)
	}

	require.NoError(b, err)
	require.Equal(b, "john", user.Name)
	require.Equal(b, 42, user.Age)
	require.Len(b, user.Posts, 3)
}
