package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func Test_Request_Method(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	req.SetMethod("GET")
	utils.AssertEqual(t, "GET", req.Method())

	req.SetMethod("POST")
	utils.AssertEqual(t, "POST", req.Method())

	req.SetMethod("PUT")
	utils.AssertEqual(t, "PUT", req.Method())

	req.SetMethod("DELETE")
	utils.AssertEqual(t, "DELETE", req.Method())
}

func Test_Request_URL(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()

	req.SetURL("http://example.com/normal")
	utils.AssertEqual(t, "http://example.com/normal", req.URL())

	req.SetURL("https://example.com/normal")
	utils.AssertEqual(t, "https://example.com/normal", req.URL())
}

func Test_Request_Client(t *testing.T) {
	t.Parallel()

	client := AcquireClient()
	req := AcquireRequest()

	req.SetClient(client)
	utils.AssertEqual(t, client, req.Client())
}

func Test_Request_Context(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	ctx := req.Context()
	key := struct{}{}

	utils.AssertEqual(t, nil, ctx.Value(key))

	ctx = context.WithValue(ctx, key, "string")
	req.SetContext(ctx)
	ctx = req.Context()

	utils.AssertEqual(t, "string", ctx.Value(key).(string))
}

func Test_Request_Header(t *testing.T) {
	t.Parallel()

	t.Run("add header", func(t *testing.T) {
		req := AcquireRequest()
		req.AddHeader("foo", "bar").AddHeader("foo", "fiber")

		res := req.Header("foo")
		utils.AssertEqual(t, 2, len(res))
		utils.AssertEqual(t, "bar", res[0])
		utils.AssertEqual(t, "fiber", res[1])
	})

	t.Run("set header", func(t *testing.T) {
		req := AcquireRequest()
		req.AddHeader("foo", "bar").SetHeader("foo", "fiber")

		res := req.Header("foo")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "fiber", res[0])
	})

	t.Run("add headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetHeader("foo", "bar").
			AddHeaders(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Header("foo")
		utils.AssertEqual(t, 3, len(res))
		utils.AssertEqual(t, "bar", res[0])
		utils.AssertEqual(t, "buaa", res[1])
		utils.AssertEqual(t, "fiber", res[2])

		res = req.Header("bar")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetHeader("foo", "bar").
			SetHeaders(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Header("foo")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "fiber", res[0])

		res = req.Header("bar")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "foo", res[0])
	})
}

func Test_Request_QueryParam(t *testing.T) {
	t.Parallel()

	t.Run("add param", func(t *testing.T) {
		req := AcquireRequest()
		req.AddParam("foo", "bar").AddParam("foo", "fiber")

		res := req.Param("foo")
		utils.AssertEqual(t, 2, len(res))
		utils.AssertEqual(t, "bar", res[0])
		utils.AssertEqual(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		req := AcquireRequest()
		req.AddParam("foo", "bar").SetParam("foo", "fiber")

		res := req.Param("foo")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			AddParams(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Param("foo")
		utils.AssertEqual(t, 3, len(res))
		utils.AssertEqual(t, "bar", res[0])
		utils.AssertEqual(t, "buaa", res[1])
		utils.AssertEqual(t, "fiber", res[2])

		res = req.Param("bar")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Param("foo")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "fiber", res[0])

		res = req.Param("bar")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "foo", res[0])
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

		utils.AssertEqual(t, 0, len(p.Param("unexport")))

		utils.AssertEqual(t, 1, len(p.Param("TInt")))
		utils.AssertEqual(t, "5", p.Param("TInt")[0])

		utils.AssertEqual(t, 1, len(p.Param("TString")))
		utils.AssertEqual(t, "string", p.Param("TString")[0])

		utils.AssertEqual(t, 1, len(p.Param("TFloat")))
		utils.AssertEqual(t, "3.1", p.Param("TFloat")[0])

		utils.AssertEqual(t, 1, len(p.Param("TBool")))

		tslice := p.Param("TSlice")
		utils.AssertEqual(t, 2, len(tslice))
		utils.AssertEqual(t, "bar", tslice[0])
		utils.AssertEqual(t, "foo", tslice[1])

		tint := p.Param("TSlice")
		utils.AssertEqual(t, 2, len(tint))
		utils.AssertEqual(t, "bar", tint[0])
		utils.AssertEqual(t, "foo", tint[1])
	})

	t.Run("del params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			}).DelParams("foo", "bar")

		res := req.Param("foo")
		utils.AssertEqual(t, 0, len(res))

		res = req.Param("bar")
		utils.AssertEqual(t, 0, len(res))
	})
}

func Test_Request_UA(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetUserAgent("fiber")
	utils.AssertEqual(t, "fiber", req.UserAgent())

	req.SetUserAgent("foo")
	utils.AssertEqual(t, "foo", req.UserAgent())
}

func Test_Request_Referer(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetReferer("http://example.com")
	utils.AssertEqual(t, "http://example.com", req.Referer())

	req.SetReferer("https://example.com")
	utils.AssertEqual(t, "https://example.com", req.Referer())
}

func Test_Request_Cookie(t *testing.T) {
	t.Parallel()

	t.Run("set cookie", func(t *testing.T) {
		req := AcquireRequest().
			SetCookie("foo", "bar")
		utils.AssertEqual(t, "bar", req.Cookie("foo"))

		req.SetCookie("foo", "bar1")
		utils.AssertEqual(t, "bar1", req.Cookie("foo"))
	})

	t.Run("set cookies", func(t *testing.T) {
		req := AcquireRequest().
			SetCookies(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		utils.AssertEqual(t, "bar", req.Cookie("foo"))
		utils.AssertEqual(t, "foo", req.Cookie("bar"))

		req.SetCookies(map[string]string{
			"foo": "bar1",
		})
		utils.AssertEqual(t, "bar1", req.Cookie("foo"))
		utils.AssertEqual(t, "foo", req.Cookie("bar"))
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

		utils.AssertEqual(t, "5", req.Cookie("int"))
		utils.AssertEqual(t, "foo", req.Cookie("string"))
	})

	t.Run("del cookies", func(t *testing.T) {
		req := AcquireRequest().
			SetCookies(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		utils.AssertEqual(t, "bar", req.Cookie("foo"))
		utils.AssertEqual(t, "foo", req.Cookie("bar"))

		req.DelCookies("foo")
		utils.AssertEqual(t, "", req.Cookie("foo"))
		utils.AssertEqual(t, "foo", req.Cookie("bar"))
	})
}

func Test_Request_PathParam(t *testing.T) {
	t.Parallel()

	t.Run("set path param", func(t *testing.T) {
		req := AcquireRequest().
			SetPathParam("foo", "bar")
		utils.AssertEqual(t, "bar", req.PathParam("foo"))

		req.SetPathParam("foo", "bar1")
		utils.AssertEqual(t, "bar1", req.PathParam("foo"))
	})

	t.Run("set path params", func(t *testing.T) {
		req := AcquireRequest().
			SetPathParams(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		utils.AssertEqual(t, "bar", req.PathParam("foo"))
		utils.AssertEqual(t, "foo", req.PathParam("bar"))

		req.SetPathParams(map[string]string{
			"foo": "bar1",
		})
		utils.AssertEqual(t, "bar1", req.PathParam("foo"))
		utils.AssertEqual(t, "foo", req.PathParam("bar"))
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

		utils.AssertEqual(t, "5", req.PathParam("int"))
		utils.AssertEqual(t, "foo", req.PathParam("string"))
	})

	t.Run("del path params", func(t *testing.T) {
		req := AcquireRequest().
			SetPathParams(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		utils.AssertEqual(t, "bar", req.PathParam("foo"))
		utils.AssertEqual(t, "foo", req.PathParam("bar"))

		req.DelPathParams("foo")
		utils.AssertEqual(t, "", req.PathParam("foo"))
		utils.AssertEqual(t, "foo", req.PathParam("bar"))
	})
}

func Test_Request_FormData(t *testing.T) {
	t.Parallel()

	t.Run("add form data", func(t *testing.T) {
		req := AcquireRequest()
		req.AddFormData("foo", "bar").AddFormData("foo", "fiber")

		res := req.FormData("foo")
		utils.AssertEqual(t, 2, len(res))
		utils.AssertEqual(t, "bar", res[0])
		utils.AssertEqual(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		req := AcquireRequest()
		req.AddFormData("foo", "bar").SetFormData("foo", "fiber")

		res := req.FormData("foo")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetFormData("foo", "bar").
			AddFormDatas(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.FormData("foo")
		utils.AssertEqual(t, 3, len(res))
		utils.AssertEqual(t, "bar", res[0])
		utils.AssertEqual(t, "buaa", res[1])
		utils.AssertEqual(t, "fiber", res[2])

		res = req.FormData("bar")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		req := AcquireRequest()
		req.SetFormData("foo", "bar").
			SetFormDatas(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.FormData("foo")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "fiber", res[0])

		res = req.FormData("bar")
		utils.AssertEqual(t, 1, len(res))
		utils.AssertEqual(t, "foo", res[0])
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

		utils.AssertEqual(t, 0, len(p.FormData("unexport")))

		utils.AssertEqual(t, 1, len(p.FormData("TInt")))
		utils.AssertEqual(t, "5", p.FormData("TInt")[0])

		utils.AssertEqual(t, 1, len(p.FormData("TString")))
		utils.AssertEqual(t, "string", p.FormData("TString")[0])

		utils.AssertEqual(t, 1, len(p.FormData("TFloat")))
		utils.AssertEqual(t, "3.1", p.FormData("TFloat")[0])

		utils.AssertEqual(t, 1, len(p.FormData("TBool")))

		tslice := p.FormData("TSlice")
		utils.AssertEqual(t, 2, len(tslice))
		utils.AssertEqual(t, "bar", tslice[0])
		utils.AssertEqual(t, "foo", tslice[1])

		tint := p.FormData("TSlice")
		utils.AssertEqual(t, 2, len(tint))
		utils.AssertEqual(t, "bar", tint[0])
		utils.AssertEqual(t, "foo", tint[1])

	})

	t.Run("del params", func(t *testing.T) {
		req := AcquireRequest()
		req.SetFormData("foo", "bar").
			SetFormDatas(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			}).DelFormDatas("foo", "bar")

		res := req.FormData("foo")
		utils.AssertEqual(t, 0, len(res))

		res = req.FormData("bar")
		utils.AssertEqual(t, 0, len(res))
	})
}

func Test_Request_File(t *testing.T) {
	t.Parallel()

	t.Run("add file", func(t *testing.T) {
		req := AcquireRequest().
			AddFile("../.github/index.html").
			AddFiles(AcquireFile(SetFileName("tmp.txt")))

		utils.AssertEqual(t, "../.github/index.html", req.File("index.html").path)
		utils.AssertEqual(t, "../.github/index.html", req.FileByPath("../.github/index.html").path)
		utils.AssertEqual(t, "tmp.txt", req.File("tmp.txt").name)
	})

	t.Run("add file by reader", func(t *testing.T) {
		req := AcquireRequest().
			AddFileWithReader("tmp.txt", io.NopCloser(strings.NewReader("world")))

		utils.AssertEqual(t, "tmp.txt", req.File("tmp.txt").name)

		content, err := ioutil.ReadAll(req.File("tmp.txt").reader)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "world", string(content))
	})

	t.Run("add files", func(t *testing.T) {
		req := AcquireRequest().
			AddFiles(AcquireFile(SetFileName("tmp.txt")), AcquireFile(SetFileName("foo.txt")))

		utils.AssertEqual(t, "tmp.txt", req.File("tmp.txt").name)
		utils.AssertEqual(t, "foo.txt", req.File("foo.txt").name)
	})
}

func Test_Request_Timeout(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetTimeout(5 * time.Second)

	utils.AssertEqual(t, 5*time.Second, req.Timeout())
}

func createHelperServer(t *testing.T) (*fiber.App, *Client, func()) {
	t.Helper()

	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	client := AcquireClient().SetDial(func(addr string) (net.Conn, error) {
		return ln.Dial()
	})

	return app, client, func() {
		utils.AssertEqual(t, nil, app.Listener(ln))
	}
}

func Test_Request_Invalid_URL(t *testing.T) {
	t.Parallel()

	resp, err := AcquireRequest().
		Get("http://example.com\r\n\r\nGET /\r\n\r\n")

	utils.AssertEqual(t, ErrURLForamt, err)
	utils.AssertEqual(t, (*Response)(nil), resp)
}

func Test_Request_Unsupport_Protocol(t *testing.T) {
	t.Parallel()

	resp, err := AcquireRequest().
		Get("ftp://example.com")
	utils.AssertEqual(t, ErrURLForamt, err)
	utils.AssertEqual(t, (*Response)(nil), resp)
}

func Test_Request_Get(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})
	go start()

	for i := 0; i < 5; i++ {
		req := AcquireRequest().SetClient(client)

		resp, err := req.Get("http://example.com")
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, "example.com", resp.String())
		resp.Close()
	}
}

func Test_Request_Post(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)
	app.Post("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).
			SendString(c.FormValue("foo"))
	})
	go start()

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			Post("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusCreated, resp.StatusCode())
		utils.AssertEqual(t, "bar", resp.String())
		resp.Close()
	}
}

func Test_Request_Head(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			Head("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, "", resp.String())
		resp.Close()
	}
}

func Test_Request_Put(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)
	app.Put("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			Put("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, "bar", resp.String())

		resp.Close()
	}
}
func Test_Request_Delete(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)

	app.Delete("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNoContent).
			SendString("deleted")
	})

	go start()

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			Delete("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusNoContent, resp.StatusCode())
		utils.AssertEqual(t, "", resp.String())

		resp.Close()
	}
}

func Test_Request_Options(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)

	app.Options("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).
			SendString("options")
	})

	go start()

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			Options("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, "options", resp.String())

		resp.Close()
	}
}

func Test_Request_Send(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)

	app.Post("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).
			SendString("post")
	})

	go start()

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetURL("http://example.com").
			SetMethod(fiber.MethodPost).
			Send()

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, "post", resp.String())

		resp.Close()
	}
}

func Test_Request_Patch(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)

	app.Patch("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()

	for i := 0; i < 5; i++ {
		resp, err := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			Patch("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, "bar", resp.String())

		resp.Close()
	}
}

func testAgent(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted string, count ...int) {
	t.Parallel()

	app, client, start := createHelperServer(t)
	app.Get("/", handler)
	go start()

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	for i := 0; i < c; i++ {
		req := AcquireRequest().SetClient(client)
		wrapAgent(req)

		resp, err := req.Get("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
		utils.AssertEqual(t, excepted, resp.String())
		resp.Close()
	}
}

func testAgentFail(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Request), excepted error, count ...int) {
	t.Parallel()

	app, client, start := createHelperServer(t)
	app.Get("/", handler)
	go start()

	c := 1
	if len(count) > 0 {
		c = count[0]
	}

	for i := 0; i < c; i++ {
		req := AcquireRequest().SetClient(client)
		wrapAgent(req)

		_, err := req.Get("http://example.com")

		utils.AssertEqual(t, excepted.Error(), err.Error())
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

	testAgent(t, handler, wrapAgent, "K1v1K1v11K1v22K1v33K2v2K2v22")
}

// func Test_Client_Agent_Connection_Close(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		if c.Request().Header.ConnectionClose() {
// 			return c.SendString("close")
// 		}
// 		return c.SendString("not close")
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.ConnectionClose()
// 	}

// 	testAgent(t, handler, wrapAgent, "close")
// }

func Test_Request_UserAgent_With_Server(t *testing.T) {
	t.Parallel()

	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	}

	t.Run("default", func(t *testing.T) {
		testAgent(t, handler, func(agent *Request) {}, defaultUserAgent, 5)
	})

	t.Run("custom", func(t *testing.T) {
		testAgent(t, handler, func(agent *Request) {
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

	testAgent(t, handler, wrapAgent, "v1v2v3")
}

func Test_Request_Referer_With_Server(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(req *Request) {
		req.SetReferer("http://referer.com")
	}

	testAgent(t, handler, wrapAgent, "http://referer.com")
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

	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
}

// func Test_Client_Agent_BasicAuth(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		// Get authorization header
// 		auth := c.Get(fiber.HeaderAuthorization)
// 		// Decode the header contents
// 		raw, err := base64.StdEncoding.DecodeString(auth[6:])
// 		utils.AssertEqual(t, nil, err)

// 		return c.Send(raw)
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.BasicAuth("foo", "bar").
// 			BasicAuthBytes([]byte("foo"), []byte("bar"))
// 	}

// 	testAgent(t, handler, wrapAgent, "foo:bar")
// }

func checkFormFile(t *testing.T, fh *multipart.FileHeader, filename string) {
	t.Helper()

	basename := filepath.Base(filename)
	utils.AssertEqual(t, fh.Filename, basename)

	b1, err := os.ReadFile(filename)
	utils.AssertEqual(t, nil, err)

	b2 := make([]byte, fh.Size)
	f, err := fh.Open()
	utils.AssertEqual(t, nil, err)
	defer func() { _ = f.Close() }()
	_, err = f.Read(b2)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, b1, b2)
}

func Test_Request_Body_With_Server(t *testing.T) {
	t.Parallel()

	t.Run("json body", func(t *testing.T) {
		testAgent(t,
			func(c fiber.Ctx) error {
				utils.AssertEqual(t, "application/json", string(c.Request().Header.ContentType()))
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
		testAgent(t,
			func(c fiber.Ctx) error {
				utils.AssertEqual(t, "application/xml", string(c.Request().Header.ContentType()))
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
		testAgent(t,
			func(c fiber.Ctx) error {
				utils.AssertEqual(t, fiber.MIMEApplicationForm, string(c.Request().Header.ContentType()))
				return c.Send(c.Request().Body())
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

		app, client, start := createHelperServer(t)
		app.Post("/", func(c fiber.Ctx) error {
			utils.AssertEqual(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

			mf, err := c.MultipartForm()
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, "bar", mf.Value["foo"][0])

			return c.Send(c.Request().Body())
		})

		go start()

		req := AcquireRequest().
			SetClient(client).
			SetBoundary("myBoundary").
			SetFormData("foo", "bar").
			AddFiles(AcquireFile(
				SetFileName("hello.txt"),
				SetFileFieldName("foo"),
				SetFileReader(io.NopCloser(strings.NewReader("world"))),
			))

		resp, err := req.Post("http://exmaple.com")
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())

		form, err := multipart.NewReader(bytes.NewReader(resp.Body()), "myBoundary").ReadForm(1024 * 1024)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "bar", form.Value["foo"][0])
		resp.Close()
	})

	t.Run("multipart form send file", func(t *testing.T) {
		t.Parallel()

		app, client, start := createHelperServer(t)
		app.Post("/", func(c fiber.Ctx) error {
			utils.AssertEqual(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

			fh1, err := c.FormFile("field1")
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, fh1.Filename, "name")
			buf := make([]byte, fh1.Size)
			f, err := fh1.Open()
			utils.AssertEqual(t, nil, err)
			defer func() { _ = f.Close() }()
			_, err = f.Read(buf)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, "form file", string(buf))

			fh2, err := c.FormFile("file2")
			utils.AssertEqual(t, nil, err)
			checkFormFile(t, fh2, "../.github/testdata/index.html")

			fh3, err := c.FormFile("file3")
			utils.AssertEqual(t, nil, err)
			checkFormFile(t, fh3, "../.github/testdata/index.tmpl")

			return c.SendString("multipart form files")
		})

		go start()

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
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, "multipart form files", resp.String())

			resp.Close()
		}
	})

	t.Run("multipart random boundary", func(t *testing.T) {
		t.Parallel()

		app, client, start := createHelperServer(t)
		app.Post("/", func(c fiber.Ctx) error {
			reg := regexp.MustCompile(`multipart/form-data; boundary=[\-\w]{35}`)
			utils.AssertEqual(t, true, reg.MatchString(c.Get(fiber.HeaderContentType)))

			return c.Send(c.Request().Body())
		})

		go start()

		req := AcquireRequest().
			SetClient(client).
			SetFormData("foo", "bar").
			AddFiles(AcquireFile(
				SetFileName("hello.txt"),
				SetFileFieldName("foo"),
				SetFileReader(io.NopCloser(strings.NewReader("world"))),
			))

		resp, err := req.Post("http://exmaple.com")
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode())
	})

	t.Run("raw body", func(t *testing.T) {
		testAgent(t,
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
		testAgentFail(t,
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
		testAgentFail(t,
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
		utils.AssertEqual(t, "mime: invalid boundary character", err.Error())
	})

	t.Run("open non exist file", func(t *testing.T) {
		t.Parallel()

		_, err := AcquireRequest().
			AddFile("non-exist-file!").
			Get("http://example.com")
		utils.AssertEqual(t, "open non-exist-file!: The system cannot find the file specified.", err.Error())
	})
}

func Test_Request_Timeout_With_Server(t *testing.T) {
	t.Parallel()

	app, client, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		time.Sleep(time.Millisecond * 200)
		return c.SendString("timeout")
	})
	go start()

	_, err := AcquireRequest().
		SetClient(client).
		SetTimeout(50 * time.Millisecond).
		Get("http://example.com")

	utils.AssertEqual(t, ErrTimeoutOrCancel, err)
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

		utils.AssertEqual(t, "", string(p.Peek("unexport")))
		utils.AssertEqual(t, []byte("5"), p.Peek("TInt"))
		utils.AssertEqual(t, []byte("string"), p.Peek("TString"))
		utils.AssertEqual(t, []byte("3.1"), p.Peek("TFloat"))
		utils.AssertEqual(t, "", string(p.Peek("TBool")))
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
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

		utils.AssertEqual(t, []byte("5"), p.Peek("TInt"))
		utils.AssertEqual(t, []byte("string"), p.Peek("TString"))
		utils.AssertEqual(t, []byte("3.1"), p.Peek("TFloat"))
		utils.AssertEqual(t, "true", string(p.Peek("TBool")))
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "foo" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("TSlice") {
				if string(v) == "bar" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
			for _, v := range p.PeekMulti("int_slice") {
				if string(v) == "1" {
					return true
				}
			}
			return false
		}())
		utils.AssertEqual(t, true, func() bool {
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

		utils.AssertEqual(t, "", string(p.Peek("TInt")))
		utils.AssertEqual(t, "", string(p.Peek("TString")))
		utils.AssertEqual(t, "", string(p.Peek("TFloat")))
		utils.AssertEqual(t, 0, len(p.PeekMulti("TSlice")))
		utils.AssertEqual(t, 0, len(p.PeekMulti("int_slice")))
	})

	t.Run("error type should ignore", func(t *testing.T) {
		p := &QueryParam{
			Args: fasthttp.AcquireArgs(),
		}
		SetValWithStruct(p, "param", 5)
		utils.AssertEqual(t, 0, p.Len())
	})
}
