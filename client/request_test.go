package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func Test_Request_Method(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.SetMethod("GET")
	require.Equal(t, "GET", req.Method())

	req.SetMethod("POST")
	require.Equal(t, "POST", req.Method())

	req.SetMethod("PUT")
	require.Equal(t, "PUT", req.Method())

	req.SetMethod("DELETE")
	require.Equal(t, "DELETE", req.Method())

	req.SetMethod("PATCH")
	require.Equal(t, "PATCH", req.Method())

	req.SetMethod("OPTIONS")
	require.Equal(t, "OPTIONS", req.Method())

	req.SetMethod("HEAD")
	require.Equal(t, "HEAD", req.Method())

	req.SetMethod("TRACE")
	require.Equal(t, "TRACE", req.Method())

	req.SetMethod("CUSTOM")
	require.Equal(t, "CUSTOM", req.Method())
}

func Test_Request_URL(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()

	req.SetURL("http://example.com/normal")
	require.Equal(t, "http://example.com/normal", req.URL())

	req.SetURL("https://example.com/normal")
	require.Equal(t, "https://example.com/normal", req.URL())
}

func Test_Request_Client(t *testing.T) {
	t.Parallel()

	client := New()
	req := AcquireRequest()

	req.SetClient(client)
	require.Equal(t, client, req.Client())
}

func Test_Request_Context(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	ctx := req.Context()
	type ctxKey struct{}
	var key ctxKey = struct{}{}

	require.Nil(t, ctx.Value(key))

	ctx = context.WithValue(ctx, key, "string")
	req.SetContext(ctx)
	ctx = req.Context()

	v, ok := ctx.Value(key).(string)
	require.True(t, ok)
	require.Equal(t, "string", v)
}

func Test_Request_Header(t *testing.T) {
	t.Parallel()

	t.Run("add header", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.AddHeader("foo", "bar").AddHeader("foo", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set header", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.AddHeader("foo", "bar").SetHeader("foo", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add headers", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.SetHeader("foo", "bar").
			AddHeaders(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Header("foo")
		require.Len(t, res, 3)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
		require.Equal(t, "buaa", res[2])

		res = req.Header("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.SetHeader("foo", "bar").
			SetHeaders(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Header("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])

		res = req.Header("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})
}

func Test_Request_Headers(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.AddHeaders(map[string][]string{
		"foo": {"bar", "fiber"},
		"bar": {"foo"},
	})

	headers := maps.Collect(req.Headers())

	require.Contains(t, headers["Foo"], "fiber")
	require.Contains(t, headers["Foo"], "bar")
	require.Contains(t, headers["Bar"], "foo")

	require.Len(t, headers, 2)
}

func Benchmark_Request_Headers(b *testing.B) {
	req := AcquireRequest()
	req.AddHeaders(map[string][]string{
		"foo": {"bar", "fiber"},
		"bar": {"foo"},
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for range req.Headers() {
		}
	}
}

func Test_Request_QueryParam(t *testing.T) {
	t.Parallel()

	t.Run("add param", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.AddParam("foo", "bar").AddParam("foo", "fiber")

		res := req.Param("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.AddParam("foo", "bar").SetParam("foo", "fiber")

		res := req.Param("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			AddParams(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Param("foo")
		require.Len(t, res, 3)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
		require.Equal(t, "buaa", res[2])

		res = req.Param("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Param("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])

		res = req.Param("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set params with struct", func(t *testing.T) {
		t.Parallel()

		type args struct {
			TString   string
			TSlice    []string
			TIntSlice []int `param:"int_slice"`
			TInt      int
			TFloat    float64
			TBool     bool
		}

		p := AcquireRequest()
		p.SetParamsWithStruct(&args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     true,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		require.Empty(t, p.Param("unexport"))

		require.Len(t, p.Param("TInt"), 1)
		require.Equal(t, "5", p.Param("TInt")[0])

		require.Len(t, p.Param("TString"), 1)
		require.Equal(t, "string", p.Param("TString")[0])

		require.Len(t, p.Param("TFloat"), 1)
		require.Equal(t, "3.1", p.Param("TFloat")[0])

		require.Len(t, p.Param("TBool"), 1)

		tslice := p.Param("TSlice")
		require.Len(t, tslice, 2)
		require.Equal(t, "foo", tslice[0])
		require.Equal(t, "bar", tslice[1])

		tint := p.Param("TSlice")
		require.Len(t, tint, 2)
		require.Equal(t, "foo", tint[0])
		require.Equal(t, "bar", tint[1])
	})

	t.Run("del params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			}).DelParams("foo", "bar")

		res := req.Param("foo")
		require.Empty(t, res)

		res = req.Param("bar")
		require.Empty(t, res)
	})
}

func Test_Request_Params(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.AddParams(map[string][]string{
		"foo": {"bar", "fiber"},
		"bar": {"foo"},
	})

	pathParams := maps.Collect(req.Params())

	require.Contains(t, pathParams["foo"], "bar")
	require.Contains(t, pathParams["foo"], "fiber")
	require.Contains(t, pathParams["bar"], "foo")

	require.Len(t, pathParams, 2)
}

func Benchmark_Request_Params(b *testing.B) {
	req := AcquireRequest()
	req.AddParams(map[string][]string{
		"foo": {"bar", "fiber"},
		"bar": {"foo"},
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for range req.Params() {
		}
	}
}

func Test_Request_UA(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetUserAgent("fiber")
	require.Equal(t, "fiber", req.UserAgent())

	req.SetUserAgent("foo")
	require.Equal(t, "foo", req.UserAgent())
}

func Test_Request_Referer(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetReferer("http://example.com")
	require.Equal(t, "http://example.com", req.Referer())

	req.SetReferer("https://example.com")
	require.Equal(t, "https://example.com", req.Referer())
}

func Test_Request_Cookie(t *testing.T) {
	t.Parallel()

	t.Run("set cookie", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			SetCookie("foo", "bar")
		require.Equal(t, "bar", req.Cookie("foo"))

		req.SetCookie("foo", "bar1")
		require.Equal(t, "bar1", req.Cookie("foo"))
	})

	t.Run("set cookies", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			SetCookies(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))

		req.SetCookies(map[string]string{
			"foo": "bar1",
		})
		require.Equal(t, "bar1", req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))
	})

	t.Run("set cookies with struct", func(t *testing.T) {
		t.Parallel()
		type args struct {
			CookieString string `cookie:"string"`
			CookieInt    int    `cookie:"int"`
		}

		req := AcquireRequest().SetCookiesWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.Cookie("int"))
		require.Equal(t, "foo", req.Cookie("string"))
	})

	t.Run("del cookies", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			SetCookies(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))

		req.DelCookies("foo")
		require.Equal(t, "", req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))
	})
}

func Test_Request_Cookies(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.SetCookies(map[string]string{
		"foo": "bar",
		"bar": "foo",
	})

	cookies := maps.Collect(req.Cookies())

	require.Equal(t, "bar", cookies["foo"])
	require.Equal(t, "foo", cookies["bar"])

	require.Len(t, cookies, 2)
}

func Benchmark_Request_Cookies(b *testing.B) {
	req := AcquireRequest()
	req.SetCookies(map[string]string{
		"foo": "bar",
		"bar": "foo",
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for range req.Cookies() {
		}
	}
}

func Test_Request_PathParam(t *testing.T) {
	t.Parallel()

	t.Run("set path param", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			SetPathParam("foo", "bar")
		require.Equal(t, "bar", req.PathParam("foo"))

		req.SetPathParam("foo", "bar1")
		require.Equal(t, "bar1", req.PathParam("foo"))
	})

	t.Run("set path params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			SetPathParams(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))

		req.SetPathParams(map[string]string{
			"foo": "bar1",
		})
		require.Equal(t, "bar1", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))
	})

	t.Run("set path params with struct", func(t *testing.T) {
		t.Parallel()
		type args struct {
			CookieString string `path:"string"`
			CookieInt    int    `path:"int"`
		}

		req := AcquireRequest().SetPathParamsWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.PathParam("int"))
		require.Equal(t, "foo", req.PathParam("string"))
	})

	t.Run("del path params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			SetPathParams(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))

		req.DelPathParams("foo")
		require.Equal(t, "", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))
	})

	t.Run("clear path params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			SetPathParams(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))

		req.ResetPathParams()
		require.Equal(t, "", req.PathParam("foo"))
		require.Equal(t, "", req.PathParam("bar"))
	})
}

func Test_Request_PathParams(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.SetPathParams(map[string]string{
		"foo": "bar",
		"bar": "foo",
	})

	pathParams := maps.Collect(req.PathParams())

	require.Equal(t, "bar", pathParams["foo"])
	require.Equal(t, "foo", pathParams["bar"])

	require.Len(t, pathParams, 2)
}

func Benchmark_Request_PathParams(b *testing.B) {
	req := AcquireRequest()
	req.SetPathParams(map[string]string{
		"foo": "bar",
		"bar": "foo",
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for range req.PathParams() {
		}
	}
}

func Test_Request_FormData(t *testing.T) {
	t.Parallel()

	t.Run("add form data", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		defer ReleaseRequest(req)
		req.AddFormData("foo", "bar").AddFormData("foo", "fiber")

		res := req.FormData("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		defer ReleaseRequest(req)
		req.AddFormData("foo", "bar").SetFormData("foo", "fiber")

		res := req.FormData("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		defer ReleaseRequest(req)
		req.SetFormData("foo", "bar").
			AddFormDatas(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.FormData("foo")
		require.Len(t, res, 3)
		require.Contains(t, res, "bar")
		require.Contains(t, res, "buaa")
		require.Contains(t, res, "fiber")

		res = req.FormData("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		defer ReleaseRequest(req)
		req.SetFormData("foo", "bar").
			SetFormDatas(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.FormData("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])

		res = req.FormData("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set params with struct", func(t *testing.T) {
		t.Parallel()

		type args struct {
			TString   string
			TSlice    []string
			TIntSlice []int `form:"int_slice"`
			TInt      int
			TFloat    float64
			TBool     bool
		}

		p := AcquireRequest()
		defer ReleaseRequest(p)
		p.SetFormDatasWithStruct(&args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     true,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		require.Empty(t, p.FormData("unexport"))

		require.Len(t, p.FormData("TInt"), 1)
		require.Equal(t, "5", p.FormData("TInt")[0])

		require.Len(t, p.FormData("TString"), 1)
		require.Equal(t, "string", p.FormData("TString")[0])

		require.Len(t, p.FormData("TFloat"), 1)
		require.Equal(t, "3.1", p.FormData("TFloat")[0])

		require.Len(t, p.FormData("TBool"), 1)

		tslice := p.FormData("TSlice")
		require.Len(t, tslice, 2)
		require.Contains(t, tslice, "bar")
		require.Contains(t, tslice, "foo")

		tint := p.FormData("TSlice")
		require.Len(t, tint, 2)
		require.Contains(t, tint, "bar")
		require.Contains(t, tint, "foo")
	})

	t.Run("del params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()
		defer ReleaseRequest(req)
		req.SetFormData("foo", "bar").
			SetFormDatas(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			}).DelFormDatas("foo", "bar")

		res := req.FormData("foo")
		require.Empty(t, res)

		res = req.FormData("bar")
		require.Empty(t, res)
	})
}

func Test_Request_File(t *testing.T) {
	t.Parallel()

	t.Run("add file", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			AddFile("../.github/index.html").
			AddFiles(AcquireFile(SetFileName("tmp.txt")))

		require.Equal(t, "../.github/index.html", req.File("index.html").path)
		require.Equal(t, "../.github/index.html", req.FileByPath("../.github/index.html").path)
		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)
		require.Nil(t, req.File("tmp2.txt"))
		require.Nil(t, req.FileByPath("tmp2.txt"))
	})

	t.Run("add file by reader", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			AddFileWithReader("tmp.txt", io.NopCloser(strings.NewReader("world")))

		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)

		content, err := io.ReadAll(req.File("tmp.txt").reader)
		require.NoError(t, err)
		require.Equal(t, "world", string(content))
	})

	t.Run("add files", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest().
			AddFiles(AcquireFile(SetFileName("tmp.txt")), AcquireFile(SetFileName("foo.txt")))

		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)
		require.Equal(t, "foo.txt", req.File("foo.txt").name)
	})
}

func Test_Request_Files(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.AddFile("../.github/index.html")
	req.AddFiles(AcquireFile(SetFileName("tmp.txt")))

	files := req.Files()

	require.Equal(t, "../.github/index.html", files[0].path)
	require.Nil(t, files[0].reader)

	require.Equal(t, "tmp.txt", files[1].name)
	require.Nil(t, files[1].reader)

	require.Len(t, files, 2)
}

func Benchmark_Request_Files(b *testing.B) {
	req := AcquireRequest()
	req.AddFile("../.github/index.html")
	req.AddFiles(AcquireFile(SetFileName("tmp.txt")))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for k, v := range req.Files() {
			_ = k
			_ = v
		}
	}
}

func Test_Request_Timeout(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetTimeout(5 * time.Second)

	require.Equal(t, 5*time.Second, req.Timeout())
}

func Test_Request_Invalid_URL(t *testing.T) {
	t.Parallel()

	resp, err := AcquireRequest().
		Get("http://example.com\r\n\r\nGET /\r\n\r\n")

	require.Equal(t, ErrURLFormat, err)
	require.Equal(t, (*Response)(nil), resp)
}

func Test_Request_Unsupport_Protocol(t *testing.T) {
	t.Parallel()

	resp, err := AcquireRequest().
		Get("ftp://example.com")
	require.Equal(t, ErrURLFormat, err)
	require.Equal(t, (*Response)(nil), resp)
}

func Test_Request_Get(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		req := AcquireRequest().SetClient(client)

		resp, err := req.Get("http://example.com")
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "example.com", resp.String())
		resp.Close()
	}
}

func Test_Request_Post(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Post("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).
			SendString(c.FormValue("foo"))
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			Post("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusCreated, resp.StatusCode())
		require.Equal(t, "bar", resp.String())
		resp.Close()
	}
}

func Test_Request_Head(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Head("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			Head("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "", resp.String())
		resp.Close()
	}
}

func Test_Request_Put(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Put("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			Put("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "bar", resp.String())

		resp.Close()
	}
}

func Test_Request_Delete(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)

	app.Delete("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNoContent).
			SendString("deleted")
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			Delete("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
		require.Equal(t, "", resp.String())

		resp.Close()
	}
}

func Test_Request_Options(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)

	app.Options("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).
			SendString("options")
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			Options("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "options", resp.String())

		resp.Close()
	}
}

func Test_Request_Send(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)

	app.Post("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).
			SendString("post")
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetURL("http://example.com").
			SetMethod(fiber.MethodPost).
			Send()

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "post", resp.String())

		resp.Close()
	}
}

func Test_Request_Patch(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)

	app.Patch("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()
	time.Sleep(100 * time.Millisecond)

	client := New().SetDial(ln)

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			Patch("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "bar", resp.String())

		resp.Close()
	}
}

func Test_Request_Header_With_Server(t *testing.T) {
	t.Parallel()
	handler := func(c fiber.Ctx) error {
		c.Request().Header.VisitAll(func(key, value []byte) {
			if k := string(key); k == "K1" || k == "K2" {
				_, err := c.Write(key)
				require.NoError(t, err)
				_, err = c.Write(value)
				require.NoError(t, err)
			}
		})
		return nil
	}

	wrapAgent := func(r *Request) {
		r.SetHeader("k1", "v1").
			AddHeader("k1", "v11").
			AddHeaders(map[string][]string{
				"k1": {"v22", "v33"},
			}).
			SetHeaders(map[string]string{
				"k2": "v2",
			}).
			AddHeader("k2", "v22")
	}

	testRequest(t, handler, wrapAgent, "K1v1K1v11K1v22K1v33K2v2K2v22")
}

func Test_Request_UserAgent_With_Server(t *testing.T) {
	t.Parallel()

	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	}

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		testRequest(t, handler, func(_ *Request) {}, defaultUserAgent, 5)
	})

	t.Run("custom", func(t *testing.T) {
		t.Parallel()
		testRequest(t, handler, func(agent *Request) {
			agent.SetUserAgent("ua")
		}, "ua", 5)
	})
}

func Test_Request_Cookie_With_Server(t *testing.T) {
	t.Parallel()
	handler := func(c fiber.Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3") + c.Cookies("k4"))
	}

	wrapAgent := func(req *Request) {
		req.SetCookie("k1", "v1").
			SetCookies(map[string]string{
				"k2": "v2",
				"k3": "v3",
				"k4": "v4",
			}).DelCookies("k4")
	}

	testRequest(t, handler, wrapAgent, "v1v2v3")
}

func Test_Request_Referer_With_Server(t *testing.T) {
	t.Parallel()
	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(req *Request) {
		req.SetReferer("http://referer.com")
	}

	testRequest(t, handler, wrapAgent, "http://referer.com")
}

func Test_Request_QueryString_With_Server(t *testing.T) {
	t.Parallel()
	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().URI().QueryString())
	}

	wrapAgent := func(req *Request) {
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"bar": "baz",
			})
	}

	testRequest(t, handler, wrapAgent, "foo=bar&bar=baz")
}

func checkFormFile(t *testing.T, fh *multipart.FileHeader, filename string) {
	t.Helper()

	basename := filepath.Base(filename)
	require.Equal(t, fh.Filename, basename)

	b1, err := os.ReadFile(filepath.Clean(filename))
	require.NoError(t, err)

	b2 := make([]byte, fh.Size)
	f, err := fh.Open()
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	_, err = f.Read(b2)
	require.NoError(t, err)
	require.Equal(t, b1, b2)
}

func Test_Request_Body_With_Server(t *testing.T) {
	t.Parallel()

	t.Run("json body", func(t *testing.T) {
		t.Parallel()
		testRequest(t,
			func(c fiber.Ctx) error {
				require.Equal(t, "application/json", string(c.Request().Header.ContentType()))
				return c.SendString(string(c.Request().Body()))
			},
			func(agent *Request) {
				agent.SetJSON(map[string]string{
					"success": "hello",
				})
			},
			"{\"success\":\"hello\"}",
		)
	})

	t.Run("xml body", func(t *testing.T) {
		t.Parallel()
		testRequest(t,
			func(c fiber.Ctx) error {
				require.Equal(t, "application/xml", string(c.Request().Header.ContentType()))
				return c.SendString(string(c.Request().Body()))
			},
			func(agent *Request) {
				type args struct {
					Content string `xml:"content"`
				}
				agent.SetXML(args{
					Content: "hello",
				})
			},
			"<args><content>hello</content></args>",
		)
	})

	t.Run("cbor body", func(t *testing.T) {
		t.Parallel()
		testRequest(t,
			func(c fiber.Ctx) error {
				require.Equal(t, "application/cbor", string(c.Request().Header.ContentType()))
				return c.SendString(string(c.Request().Body()))
			},
			func(agent *Request) {
				type args struct {
					Content string `cbor:"content"`
				}
				agent.SetCBOR(args{
					Content: "hello",
				})
			},
			"\xa1gcontentehello",
		)
	})

	t.Run("formdata", func(t *testing.T) {
		t.Parallel()
		testRequest(t,
			func(c fiber.Ctx) error {
				require.Equal(t, fiber.MIMEApplicationForm, string(c.Request().Header.ContentType()))
				return c.Send([]byte("foo=" + c.FormValue("foo") + "&bar=" + c.FormValue("bar") + "&fiber=" + c.FormValue("fiber")))
			},
			func(agent *Request) {
				agent.SetFormData("foo", "bar").
					SetFormDatas(map[string]string{
						"bar":   "baz",
						"fiber": "fast",
					})
			},
			"foo=bar&bar=baz&fiber=fast")
	})

	t.Run("multipart form", func(t *testing.T) {
		t.Parallel()

		app, ln, start := createHelperServer(t)
		app.Post("/", func(c fiber.Ctx) error {
			require.Equal(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

			mf, err := c.MultipartForm()
			require.NoError(t, err)
			require.Equal(t, "bar", mf.Value["foo"][0])

			return c.Send(c.Request().Body())
		})

		go start()

		client := New().SetDial(ln)

		req := AcquireRequest().
			SetClient(client).
			SetBoundary("myBoundary").
			SetFormData("foo", "bar").
			AddFiles(AcquireFile(
				SetFileName("hello.txt"),
				SetFileFieldName("foo"),
				SetFileReader(io.NopCloser(strings.NewReader("world"))),
			))

		require.Equal(t, "myBoundary", req.Boundary())

		resp, err := req.Post("http://exmaple.com")
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())

		form, err := multipart.NewReader(bytes.NewReader(resp.Body()), "myBoundary").ReadForm(1024 * 1024)
		require.NoError(t, err)
		require.Equal(t, "bar", form.Value["foo"][0])
		resp.Close()
	})

	t.Run("multipart form send file", func(t *testing.T) {
		t.Parallel()

		app, ln, start := createHelperServer(t)
		app.Post("/", func(c fiber.Ctx) error {
			require.Equal(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

			fh1, err := c.FormFile("field1")
			require.NoError(t, err)
			require.Equal(t, "name", fh1.Filename)
			buf := make([]byte, fh1.Size)
			f, err := fh1.Open()
			require.NoError(t, err)
			defer func() { require.NoError(t, f.Close()) }()
			_, err = f.Read(buf)
			require.NoError(t, err)
			require.Equal(t, "form file", string(buf))

			fh2, err := c.FormFile("file2")
			require.NoError(t, err)
			checkFormFile(t, fh2, "../.github/testdata/index.html")

			fh3, err := c.FormFile("file3")
			require.NoError(t, err)
			checkFormFile(t, fh3, "../.github/testdata/index.tmpl")

			return c.SendString("multipart form files")
		})

		go start()

		client := New().SetDial(ln)

		for i := 0; i < 5; i++ {
			req := AcquireRequest().
				SetClient(client).
				AddFiles(
					AcquireFile(
						SetFileFieldName("field1"),
						SetFileName("name"),
						SetFileReader(io.NopCloser(bytes.NewReader([]byte("form file")))),
					),
				).
				AddFile("../.github/testdata/index.html").
				AddFile("../.github/testdata/index.tmpl").
				SetBoundary("myBoundary")

			resp, err := req.Post("http://example.com")
			require.NoError(t, err)
			require.Equal(t, "multipart form files", resp.String())

			resp.Close()
		}
	})

	t.Run("multipart random boundary", func(t *testing.T) {
		t.Parallel()

		app, ln, start := createHelperServer(t)
		app.Post("/", func(c fiber.Ctx) error {
			reg := regexp.MustCompile(`multipart/form-data; boundary=[\-\w]{35}`)
			require.True(t, reg.MatchString(c.Get(fiber.HeaderContentType)))

			return c.Send(c.Request().Body())
		})

		go start()

		client := New().SetDial(ln)

		req := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			AddFiles(AcquireFile(
				SetFileName("hello.txt"),
				SetFileFieldName("foo"),
				SetFileReader(io.NopCloser(strings.NewReader("world"))),
			))

		resp, err := req.Post("http://exmaple.com")
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
	})

	t.Run("raw body", func(t *testing.T) {
		t.Parallel()
		testRequest(t,
			func(c fiber.Ctx) error {
				return c.SendString(string(c.Request().Body()))
			},
			func(agent *Request) {
				agent.SetRawBody([]byte("hello"))
			},
			"hello",
		)
	})
}

func Test_Request_FormDatas(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.AddFormDatas(map[string][]string{
		"foo": {"bar", "fiber"},
		"bar": {"foo"},
	})

	pathParams := maps.Collect(req.FormDatas())

	require.Contains(t, pathParams["foo"], "bar")
	require.Contains(t, pathParams["foo"], "fiber")
	require.Contains(t, pathParams["bar"], "foo")

	require.Len(t, pathParams, 2)
}

func Benchmark_Request_FormDatas(b *testing.B) {
	req := AcquireRequest()
	req.AddFormDatas(map[string][]string{
		"foo": {"bar", "fiber"},
		"bar": {"foo"},
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for range req.FormDatas() {
		}
	}
}

func Test_Request_Error_Body_With_Server(t *testing.T) {
	t.Parallel()
	t.Run("json error", func(t *testing.T) {
		t.Parallel()
		testRequestFail(t,
			func(c fiber.Ctx) error {
				return c.SendString("")
			},
			func(agent *Request) {
				agent.SetJSON(complex(1, 1))
			},
			errors.New("json: unsupported type: complex128"),
		)
	})

	t.Run("xml error", func(t *testing.T) {
		t.Parallel()
		testRequestFail(t,
			func(c fiber.Ctx) error {
				return c.SendString("")
			},
			func(agent *Request) {
				agent.SetXML(complex(1, 1))
			},
			errors.New("xml: unsupported type: complex128"),
		)
	})

	t.Run("form body with invalid boundary", func(t *testing.T) {
		t.Parallel()

		_, err := AcquireRequest().
			SetBoundary("*").
			AddFileWithReader("t.txt", io.NopCloser(strings.NewReader("world"))).
			Get("http://example.com")
		require.Equal(t, "set boundary error: mime: invalid boundary character", err.Error())
	})

	t.Run("open non exist file", func(t *testing.T) {
		t.Parallel()

		_, err := AcquireRequest().
			AddFile("non-exist-file!").
			Get("http://example.com")
		require.Contains(t, err.Error(), "open non-exist-file!")
	})
}

func Test_Request_Timeout_With_Server(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		time.Sleep(time.Millisecond * 200)
		return c.SendString("timeout")
	})
	go start()

	client := New().SetDial(ln)

	_, err := AcquireRequest().
		SetClient(client).
		SetTimeout(50 * time.Millisecond).
		Get("http://example.com")

	require.Equal(t, ErrTimeoutOrCancel, err)
}

func Test_Request_MaxRedirects(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()

	app := fiber.New()

	app.Get("/", func(c fiber.Ctx) error {
		if c.Request().URI().QueryArgs().Has("foo") {
			return c.Redirect().To("/foo")
		}
		return c.Redirect().To("/")
	})
	app.Get("/foo", func(c fiber.Ctx) error {
		return c.SendString("redirect")
	})

	go func() { assert.NoError(t, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})) }()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		client := New().SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })

		resp, err := AcquireRequest().
			SetClient(client).
			SetMaxRedirects(1).
			Get("http://example.com?foo")
		body := resp.String()
		code := resp.StatusCode()

		require.Equal(t, 200, code)
		require.Equal(t, "redirect", body)
		require.NoError(t, err)

		resp.Close()
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		client := New().SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })

		resp, err := AcquireRequest().
			SetClient(client).
			SetMaxRedirects(1).
			Get("http://example.com")

		require.Nil(t, resp)
		require.Equal(t, "too many redirects detected when doing the request", err.Error())
	})

	t.Run("MaxRedirects", func(t *testing.T) {
		t.Parallel()

		client := New().SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })

		req := AcquireRequest().
			SetClient(client).
			SetMaxRedirects(3)

		require.Equal(t, 3, req.MaxRedirects())
	})
}

func Test_SetValWithStruct(t *testing.T) {
	t.Parallel()

	// test SetValWithStruct vai QueryParam struct.
	type args struct {
		TString   string
		TSlice    []string
		TIntSlice []int `param:"int_slice"`
		unexport  int
		TInt      int
		TFloat    float64
		TBool     bool
	}

	t.Run("the struct should be applied", func(t *testing.T) {
		t.Parallel()
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}

		SetValWithStruct(p, "param", args{
			unexport:  5,
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     false,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		require.Equal(t, "", string(p.Peek("unexport")))
		require.Equal(t, []byte("5"), p.Peek("TInt"))
		require.Equal(t, []byte("string"), p.Peek("TString"))
		require.Equal(t, []byte("3.1"), p.Peek("TFloat"))
		require.Equal(t, "", string(p.Peek("TBool")))
		require.True(t, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())

		require.True(t, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" { //nolint:goconst // test
					return true
				}
			}
			return false
		}())

		require.True(t, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())

		require.True(t, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "2" {
					return true
				}
			}
			return false
		}())
	})

	t.Run("the pointer of a struct should be applied", func(t *testing.T) {
		t.Parallel()
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}

		SetValWithStruct(p, "param", &args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     true,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		require.Equal(t, []byte("5"), p.Peek("TInt"))
		require.Equal(t, []byte("string"), p.Peek("TString"))
		require.Equal(t, []byte("3.1"), p.Peek("TFloat"))
		require.Equal(t, "true", string(p.Peek("TBool")))
		require.True(t, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())

		require.True(t, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" {
					return true
				}
			}
			return false
		}())

		require.True(t, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())

		require.True(t, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "2" {
					return true
				}
			}
			return false
		}())
	})

	t.Run("the zero val should be ignore", func(t *testing.T) {
		t.Parallel()
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}
		SetValWithStruct(p, "param", &args{
			TInt:    0,
			TString: "",
			TFloat:  0.0,
		})

		require.Equal(t, "", string(p.Peek("TInt")))
		require.Equal(t, "", string(p.Peek("TString")))
		require.Equal(t, "", string(p.Peek("TFloat")))
		require.Empty(t, p.PeekMulti("TSlice"))
		require.Empty(t, p.PeekMulti("int_slice"))
	})

	t.Run("error type should ignore", func(t *testing.T) {
		t.Parallel()
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}
		SetValWithStruct(p, "param", 5)
		require.Equal(t, 0, p.Len())
	})
}

func Benchmark_SetValWithStruct(b *testing.B) {
	// test SetValWithStruct vai QueryParam struct.
	type args struct {
		TString   string
		TSlice    []string
		TIntSlice []int `param:"int_slice"`
		unexport  int
		TInt      int
		TFloat    float64
		TBool     bool
	}

	b.Run("the struct should be applied", func(b *testing.B) {
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}

		b.ReportAllocs()
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			SetValWithStruct(p, "param", args{
				unexport:  5,
				TInt:      5,
				TString:   "string",
				TFloat:    3.1,
				TBool:     false,
				TSlice:    []string{"foo", "bar"},
				TIntSlice: []int{1, 2},
			})
		}

		require.Equal(b, "", string(p.Peek("unexport")))
		require.Equal(b, []byte("5"), p.Peek("TInt"))
		require.Equal(b, []byte("string"), p.Peek("TString"))
		require.Equal(b, []byte("3.1"), p.Peek("TFloat"))
		require.Equal(b, "", string(p.Peek("TBool")))
		require.True(b, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())

		require.True(b, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" {
					return true
				}
			}
			return false
		}())

		require.True(b, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())

		require.True(b, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "2" {
					return true
				}
			}
			return false
		}())
	})

	b.Run("the pointer of a struct should be applied", func(b *testing.B) {
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}

		b.ReportAllocs()
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			SetValWithStruct(p, "param", &args{
				TInt:      5,
				TString:   "string",
				TFloat:    3.1,
				TBool:     true,
				TSlice:    []string{"foo", "bar"},
				TIntSlice: []int{1, 2},
			})
		}

		require.Equal(b, []byte("5"), p.Peek("TInt"))
		require.Equal(b, []byte("string"), p.Peek("TString"))
		require.Equal(b, []byte("3.1"), p.Peek("TFloat"))
		require.Equal(b, "true", string(p.Peek("TBool")))
		require.True(b, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())

		require.True(b, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" {
					return true
				}
			}
			return false
		}())

		require.True(b, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())

		require.True(b, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "2" {
					return true
				}
			}
			return false
		}())
	})

	b.Run("the zero val should be ignore", func(b *testing.B) {
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}

		b.ReportAllocs()
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			SetValWithStruct(p, "param", &args{
				TInt:    0,
				TString: "",
				TFloat:  0.0,
			})
		}

		require.Empty(b, string(p.Peek("TInt")))
		require.Empty(b, string(p.Peek("TString")))
		require.Empty(b, string(p.Peek("TFloat")))
		require.Empty(b, p.PeekMulti("TSlice"))
		require.Empty(b, p.PeekMulti("int_slice"))
	})

	b.Run("error type should ignore", func(b *testing.B) {
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}

		b.ReportAllocs()
		b.StartTimer()

		for i := 0; i < b.N; i++ {
			SetValWithStruct(p, "param", 5)
		}

		require.Equal(b, 0, p.Len())
	})
}
