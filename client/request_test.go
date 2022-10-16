package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
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

	client := AcquireClient()
	req := AcquireRequest()

	req.SetClient(client)
	require.Equal(t, client, req.Client())
}

func Test_Request_Context(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	ctx := req.Context()
	key := struct{}{}

	require.Nil(t, ctx.Value(key))

	ctx = context.WithValue(ctx, key, "string")
	req.SetContext(ctx)
	ctx = req.Context()

	require.Equal(t, "string", ctx.Value(key).(string))
}

func Test_Request_Header(t *testing.T) {
	t.Parallel()

	t.Run("add header", func(t *testing.T) {
		req := AcquireRequest()
		req.AddHeader("foo", "bar").AddHeader("foo", "fiber")

		res := req.Header("foo")
		require.Equal(t, 2, len(res))
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set header", func(t *testing.T) {
		req := AcquireRequest()
		req.AddHeader("foo", "bar").SetHeader("foo", "fiber")

		res := req.Header("foo")
		require.Equal(t, 1, len(res))
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetHeader("foo", "bar").
			AddHeaders(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Header("foo")
		require.Equal(t, 3, len(res))
		require.Equal(t, "bar", res[0])
		require.Equal(t, "buaa", res[1])
		require.Equal(t, "fiber", res[2])

		res = req.Header("bar")
		require.Equal(t, 1, len(res))
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetHeader("foo", "bar").
			SetHeaders(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Header("foo")
		require.Equal(t, 1, len(res))
		require.Equal(t, "fiber", res[0])

		res = req.Header("bar")
		require.Equal(t, 1, len(res))
		require.Equal(t, "foo", res[0])
	})
}

func Test_Request_QueryParam(t *testing.T) {
	t.Parallel()

	t.Run("add param", func(t *testing.T) {
		req := AcquireRequest()
		req.AddParam("foo", "bar").AddParam("foo", "fiber")

		res := req.Param("foo")
		require.Equal(t, 2, len(res))
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		req := AcquireRequest()
		req.AddParam("foo", "bar").SetParam("foo", "fiber")

		res := req.Param("foo")
		require.Equal(t, 1, len(res))
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			AddParams(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Param("foo")
		require.Equal(t, 3, len(res))
		require.Equal(t, "bar", res[0])
		require.Equal(t, "buaa", res[1])
		require.Equal(t, "fiber", res[2])

		res = req.Param("bar")
		require.Equal(t, 1, len(res))
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Param("foo")
		require.Equal(t, 1, len(res))
		require.Equal(t, "fiber", res[0])

		res = req.Param("bar")
		require.Equal(t, 1, len(res))
		require.Equal(t, "foo", res[0])
	})

	t.Run("set params with struct", func(t *testing.T) {
		t.Parallel()

		type args struct {
			TInt      int
			TString   string
			TFloat    float64
			TBool     bool
			TSlice    []string
			TIntSlice []int `param:"int_slice"`
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

		require.Equal(t, 0, len(p.Param("unexport")))

		require.Equal(t, 1, len(p.Param("TInt")))
		require.Equal(t, "5", p.Param("TInt")[0])

		require.Equal(t, 1, len(p.Param("TString")))
		require.Equal(t, "string", p.Param("TString")[0])

		require.Equal(t, 1, len(p.Param("TFloat")))
		require.Equal(t, "3.1", p.Param("TFloat")[0])

		require.Equal(t, 1, len(p.Param("TBool")))

		tslice := p.Param("TSlice")
		require.Equal(t, 2, len(tslice))
		require.Equal(t, "bar", tslice[0])
		require.Equal(t, "foo", tslice[1])

		tint := p.Param("TSlice")
		require.Equal(t, 2, len(tint))
		require.Equal(t, "bar", tint[0])
		require.Equal(t, "foo", tint[1])
	})

	t.Run("del params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			}).DelParams("foo", "bar")

		res := req.Param("foo")
		require.Equal(t, 0, len(res))

		res = req.Param("bar")
		require.Equal(t, 0, len(res))
	})
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
		req := AcquireRequest().
			SetCookie("foo", "bar")
		require.Equal(t, "bar", req.Cookie("foo"))

		req.SetCookie("foo", "bar1")
		require.Equal(t, "bar1", req.Cookie("foo"))
	})

	t.Run("set cookies", func(t *testing.T) {
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
		type args struct {
			CookieInt    int    `cookie:"int"`
			CookieString string `cookie:"string"`
		}

		req := AcquireRequest().SetCookiesWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.Cookie("int"))
		require.Equal(t, "foo", req.Cookie("string"))
	})

	t.Run("del cookies", func(t *testing.T) {
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

func Test_Request_PathParam(t *testing.T) {
	t.Parallel()

	t.Run("set path param", func(t *testing.T) {
		req := AcquireRequest().
			SetPathParam("foo", "bar")
		require.Equal(t, "bar", req.PathParam("foo"))

		req.SetPathParam("foo", "bar1")
		require.Equal(t, "bar1", req.PathParam("foo"))
	})

	t.Run("set path params", func(t *testing.T) {
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
		type args struct {
			CookieInt    int    `path:"int"`
			CookieString string `path:"string"`
		}

		req := AcquireRequest().SetPathParamsWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.PathParam("int"))
		require.Equal(t, "foo", req.PathParam("string"))
	})

	t.Run("del path params", func(t *testing.T) {
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
}

func Test_Request_FormData(t *testing.T) {
	t.Parallel()

	t.Run("add form data", func(t *testing.T) {
		req := AcquireRequest()
		req.AddFormData("foo", "bar").AddFormData("foo", "fiber")

		res := req.FormData("foo")
		require.Equal(t, 2, len(res))
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		req := AcquireRequest()
		req.AddFormData("foo", "bar").SetFormData("foo", "fiber")

		res := req.FormData("foo")
		require.Equal(t, 1, len(res))
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetFormData("foo", "bar").
			AddFormDatas(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.FormData("foo")
		require.Equal(t, 3, len(res))
		require.Equal(t, "bar", res[0])
		require.Equal(t, "buaa", res[1])
		require.Equal(t, "fiber", res[2])

		res = req.FormData("bar")
		require.Equal(t, 1, len(res))
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetFormData("foo", "bar").
			SetFormDatas(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.FormData("foo")
		require.Equal(t, 1, len(res))
		require.Equal(t, "fiber", res[0])

		res = req.FormData("bar")
		require.Equal(t, 1, len(res))
		require.Equal(t, "foo", res[0])
	})

	t.Run("set params with struct", func(t *testing.T) {
		t.Parallel()

		type args struct {
			TInt      int
			TString   string
			TFloat    float64
			TBool     bool
			TSlice    []string
			TIntSlice []int `form:"int_slice"`
		}

		p := AcquireRequest()
		p.SetFormDatasWithStruct(&args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     true,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		require.Equal(t, 0, len(p.FormData("unexport")))

		require.Equal(t, 1, len(p.FormData("TInt")))
		require.Equal(t, "5", p.FormData("TInt")[0])

		require.Equal(t, 1, len(p.FormData("TString")))
		require.Equal(t, "string", p.FormData("TString")[0])

		require.Equal(t, 1, len(p.FormData("TFloat")))
		require.Equal(t, "3.1", p.FormData("TFloat")[0])

		require.Equal(t, 1, len(p.FormData("TBool")))

		tslice := p.FormData("TSlice")
		require.Equal(t, 2, len(tslice))
		require.Equal(t, "bar", tslice[0])
		require.Equal(t, "foo", tslice[1])

		tint := p.FormData("TSlice")
		require.Equal(t, 2, len(tint))
		require.Equal(t, "bar", tint[0])
		require.Equal(t, "foo", tint[1])

	})

	t.Run("del params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetFormData("foo", "bar").
			SetFormDatas(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			}).DelFormDatas("foo", "bar")

		res := req.FormData("foo")
		require.Equal(t, 0, len(res))

		res = req.FormData("bar")
		require.Equal(t, 0, len(res))
	})
}

func Test_Request_File(t *testing.T) {
	t.Parallel()

	t.Run("add file", func(t *testing.T) {
		req := AcquireRequest().
			AddFile("../.github/index.html").
			AddFiles(AcquireFile(SetFileName("tmp.txt")))

		require.Equal(t, "../.github/index.html", req.File("index.html").path)
		require.Equal(t, "../.github/index.html", req.FileByPath("../.github/index.html").path)
		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)
	})

	t.Run("add file by reader", func(t *testing.T) {
		req := AcquireRequest().
			AddFileWithReader("tmp.txt", io.NopCloser(strings.NewReader("world")))

		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)

		content, err := io.ReadAll(req.File("tmp.txt").reader)
		require.NoError(t, err)
		require.Equal(t, "world", string(content))
	})

	t.Run("add files", func(t *testing.T) {
		req := AcquireRequest().
			AddFiles(AcquireFile(SetFileName("tmp.txt")), AcquireFile(SetFileName("foo.txt")))

		require.Equal(t, "tmp.txt", req.File("tmp.txt").name)
		require.Equal(t, "foo.txt", req.File("foo.txt").name)
	})
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

	require.Equal(t, ErrURLForamt, err)
	require.Equal(t, (*Response)(nil), resp)
}

func Test_Request_Unsupport_Protocol(t *testing.T) {
	t.Parallel()

	resp, err := AcquireRequest().
		Get("ftp://example.com")
	require.Equal(t, ErrURLForamt, err)
	require.Equal(t, (*Response)(nil), resp)
}

func Test_Request_Get(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})
	go start()

	for i := 0; i < 5; i++ {
		req := AcquireRequest().SetDial(ln)

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

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetDial(ln).
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

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetDial(ln).
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

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetDial(ln).
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

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetDial(ln).
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

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetDial(ln).
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

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetDial(ln).
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

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetDial(ln).
			SetFormData("foo", "bar").
			Patch("http://example.com")

		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "bar", resp.String())

		resp.Close()
	}
}

func Test_Request_Header_With_Server(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		c.Request().Header.VisitAll(func(key, value []byte) {
			if k := string(key); k == "K1" || k == "K2" {
				_, _ = c.Write(key)
				_, _ = c.Write(value)
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
		testRequest(t, handler, func(agent *Request) {}, defaultUserAgent, 5)
	})

	t.Run("custom", func(t *testing.T) {
		testRequest(t, handler, func(agent *Request) {
			agent.SetUserAgent("ua")
		}, "ua", 5)
	})
}

func Test_Request_Cookie_With_Server(t *testing.T) {
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
	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(req *Request) {
		req.SetReferer("http://referer.com")
	}

	testRequest(t, handler, wrapAgent, "http://referer.com")
}

// func Test_Client_Agent_Host(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString(c.Hostname())
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Get("http://1.1.1.1:8080").
// 		Host("example.com").
// 		HostBytes([]byte("example.com"))

// 	utils.AssertEqual(t, "1.1.1.1:8080", a.HostClient.Addr)

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	code, body, errs := a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "example.com", body)
// 	utils.AssertEqual(t, 0, len(errs))
// }

func Test_Request_QueryString_With_Server(t *testing.T) {
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

	b1, err := os.ReadFile(filename)
	require.NoError(t, err)

	b2 := make([]byte, fh.Size)
	f, err := fh.Open()
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	_, err = f.Read(b2)
	require.NoError(t, err)
	require.Equal(t, b1, b2)
}

func Test_Request_Body_With_Server(t *testing.T) {
	t.Parallel()

	t.Run("json body", func(t *testing.T) {
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

	t.Run("formdata", func(t *testing.T) {
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

		req := AcquireRequest().
			SetDial(ln).
			SetBoundary("myBoundary").
			SetFormData("foo", "bar").
			AddFiles(AcquireFile(
				SetFileName("hello.txt"),
				SetFileFieldName("foo"),
				SetFileReader(io.NopCloser(strings.NewReader("world"))),
			))

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
			require.Equal(t, fh1.Filename, "name")
			buf := make([]byte, fh1.Size)
			f, err := fh1.Open()
			require.NoError(t, err)
			defer func() { _ = f.Close() }()
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

		for i := 0; i < 5; i++ {
			req := AcquireRequest().
				SetDial(ln).
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

		req := AcquireRequest().
			SetDial(ln).
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

func Test_Request_Error_Body_With_Server(t *testing.T) {
	t.Run("json error", func(t *testing.T) {
		testClientFail(t,
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
		testClientFail(t,
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
		require.Equal(t, "mime: invalid boundary character", err.Error())
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

	_, err := AcquireRequest().
		SetDial(ln).
		SetTimeout(50 * time.Millisecond).
		Get("http://example.com")

	require.Equal(t, ErrTimeoutOrCancel, err)
}

// // readErrorConn is a struct for testing retryIf
// type readErrorConn struct {
// 	net.Conn
// }

// func (r *readErrorConn) Read(p []byte) (int, error) {
// 	return 0, fmt.Errorf("error")
// }

// func (r *readErrorConn) Write(p []byte) (int, error) {
// 	return len(p), nil
// }

// func (r *readErrorConn) Close() error {
// 	return nil
// }

// func (r *readErrorConn) LocalAddr() net.Addr {
// 	return nil
// }

// func (r *readErrorConn) RemoteAddr() net.Addr {
// 	return nil
// }
// func Test_Client_Agent_RetryIf(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Post("http://example.com").
// 		RetryIf(func(req *Request) bool {
// 			return true
// 		})
// 	dialsCount := 0
// 	a.HostClient.Dial = func(addr string) (net.Conn, error) {
// 		dialsCount++
// 		switch dialsCount {
// 		case 1:
// 			return &readErrorConn{}, nil
// 		case 2:
// 			return &readErrorConn{}, nil
// 		case 3:
// 			return &readErrorConn{}, nil
// 		case 4:
// 			return ln.Dial()
// 		default:
// 			t.Fatalf("unexpected number of dials: %d", dialsCount)
// 		}
// 		panic("unreachable")
// 	}

// 	_, _, errs := a.String()
// 	utils.AssertEqual(t, dialsCount, 4)
// 	utils.AssertEqual(t, 0, len(errs))
// }

// func Test_Client_Debug(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.SendString("debug")
// 	}

// 	var output bytes.Buffer

// 	wrapAgent := func(a *Agent) {
// 		a.Debug(&output)
// 	}

// 	testAgent(t, handler, wrapAgent, "debug", 1)

// 	str := output.String()

// 	utils.AssertEqual(t, true, strings.Contains(str, "Connected to example.com(pipe)"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "GET / HTTP/1.1"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "User-Agent: fiber"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "Host: example.com\r\n\r\n"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "HTTP/1.1 200 OK"))
// 	utils.AssertEqual(t, true, strings.Contains(str, "Content-Type: text/plain; charset=utf-8\r\nContent-Length: 5\r\n\r\ndebug"))
// }

// func Test_Client_Agent_InsecureSkipVerify(t *testing.T) {
// 	t.Parallel()

// 	cer, err := tls.LoadX509KeyPair("./.github/testdata/ssl.pem", "./.github/testdata/ssl.key")
// 	utils.AssertEqual(t, nil, err)

// 	serverTLSConf := &tls.Config{
// 		Certificates: []tls.Certificate{cer},
// 	}

// 	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
// 	utils.AssertEqual(t, nil, err)

// 	ln = tls.NewListener(ln, serverTLSConf)

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("ignore tls")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	code, body, errs := Get("https://" + ln.Addr().String()).
// 		InsecureSkipVerify().
// 		InsecureSkipVerify().
// 		String()

// 	utils.AssertEqual(t, 0, len(errs))
// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "ignore tls", body)
// }

// func Test_Client_Agent_TLS(t *testing.T) {
// 	t.Parallel()

// 	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
// 	utils.AssertEqual(t, nil, err)

// 	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
// 	utils.AssertEqual(t, nil, err)

// 	ln = tls.NewListener(ln, serverTLSConf)

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("tls")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	code, body, errs := Get("https://" + ln.Addr().String()).
// 		TLSConfig(clientTLSConf).
// 		String()

// 	utils.AssertEqual(t, 0, len(errs))
// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "tls", body)
// }

// func Test_Client_Agent_MaxRedirectsCount(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		if c.Request().URI().QueryArgs().Has("foo") {
// 			return c.Redirect("/foo")
// 		}
// 		return c.Redirect("/")
// 	})
// 	app.Get("/foo", func(c fiber.Ctx) error {
// 		return c.SendString("redirect")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	t.Run("success", func(t *testing.T) {
// 		a := Get("http://example.com?foo").
// 			MaxRedirectsCount(1)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.String()

// 		utils.AssertEqual(t, 200, code)
// 		utils.AssertEqual(t, "redirect", body)
// 		utils.AssertEqual(t, 0, len(errs))
// 	})

// 	t.Run("error", func(t *testing.T) {
// 		a := Get("http://example.com").
// 			MaxRedirectsCount(1)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		_, body, errs := a.String()

// 		utils.AssertEqual(t, "", body)
// 		utils.AssertEqual(t, 1, len(errs))
// 		utils.AssertEqual(t, "too many redirects detected when doing the request", errs[0].Error())
// 	})
// }

// func Test_Client_Agent_Struct(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.JSON(data{true})
// 	})

// 	app.Get("/error", func(c fiber.Ctx) error {
// 		return c.SendString(`{"success"`)
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	t.Run("success", func(t *testing.T) {
// 		t.Parallel()

// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		var d data

// 		code, body, errs := a.Struct(&d)

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, `{"success":true}`, string(body))
// 		utils.AssertEqual(t, 0, len(errs))
// 		utils.AssertEqual(t, true, d.Success)
// 	})

// 	t.Run("pre error", func(t *testing.T) {
// 		t.Parallel()
// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
// 		a.errs = append(a.errs, errors.New("pre errors"))

// 		var d data
// 		_, body, errs := a.Struct(&d)

// 		utils.AssertEqual(t, "", string(body))
// 		utils.AssertEqual(t, 1, len(errs))
// 		utils.AssertEqual(t, "pre errors", errs[0].Error())
// 		utils.AssertEqual(t, false, d.Success)
// 	})

// 	t.Run("error", func(t *testing.T) {
// 		a := Get("http://example.com/error")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		var d data

// 		code, body, errs := a.JSONDecoder(json.Unmarshal).Struct(&d)

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, `{"success"`, string(body))
// 		utils.AssertEqual(t, 1, len(errs))
// 		utils.AssertEqual(t, "unexpected end of JSON input", errs[0].Error())
// 	})
// }

// func Test_Client_Agent_Parse(t *testing.T) {
// 	t.Parallel()

// 	a := Get("https://example.com:10443")

// 	utils.AssertEqual(t, nil, a.Parse())
// }

// func Test_AddMissingPort_TLS(t *testing.T) {
// 	addr := addMissingPort("example.com", true)
// 	utils.AssertEqual(t, "example.com:443", addr)
// }

// type data struct {
// 	Success bool `json:"success" xml:"success"`
// }

// type errorMultipartWriter struct {
// 	count int
// }

// func (e *errorMultipartWriter) Boundary() string           { return "myBoundary" }
// func (e *errorMultipartWriter) SetBoundary(_ string) error { return nil }
// func (e *errorMultipartWriter) CreateFormFile(_, _ string) (io.Writer, error) {
// 	if e.count == 0 {
// 		e.count++
// 		return nil, errors.New("CreateFormFile error")
// 	}
// 	return errorWriter{}, nil
// }
// func (e *errorMultipartWriter) WriteField(_, _ string) error { return errors.New("WriteField error") }
// func (e *errorMultipartWriter) Close() error                 { return errors.New("Close error") }

// type errorWriter struct{}

// func (errorWriter) Write(_ []byte) (int, error) { return 0, errors.New("Write error") }

func Test_SetValWithStruct(t *testing.T) {
	t.Parallel()

	// test SetValWithStruct vai QueryParam struct.
	type args struct {
		unexport  int
		TInt      int
		TString   string
		TFloat    float64
		TBool     bool
		TSlice    []string
		TIntSlice []int `param:"int_slice"`
	}

	t.Run("the struct should be applied", func(t *testing.T) {
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

	t.Run("the pointer of a struct should be applied", func(t *testing.T) {
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
		require.Equal(t, 0, len(p.PeekMulti("TSlice")))
		require.Equal(t, 0, len(p.PeekMulti("int_slice")))
	})

	t.Run("error type should ignore", func(t *testing.T) {
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}
		SetValWithStruct(p, "param", 5)
		require.Equal(t, 0, p.Len())
	})
}
