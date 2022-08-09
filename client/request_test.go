package client

import (
	"context"
	"net"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestRequestMethod(t *testing.T) {
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

func TestRequestURL(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()

	req.SetURL("http://example.com/normal")
	utils.AssertEqual(t, "http://example.com/normal", req.URL())

	req.SetURL("https://example.com/normal")
	utils.AssertEqual(t, "https://example.com/normal", req.URL())
}

func TestRequestClient(t *testing.T) {
	t.Parallel()

	client := AcquireClient()
	req := AcquireRequest()

	req.SetClient(client)
	utils.AssertEqual(t, client, req.Client())
}

func TestRequestContext(t *testing.T) {
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

func TestRequestHeader(t *testing.T) {
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

func TestRequestQueryParam(t *testing.T) {
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

func TestRequestUA(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetUserAgent("fiber")
	utils.AssertEqual(t, "fiber", req.UserAgent())

	req.SetUserAgent("foo")
	utils.AssertEqual(t, "foo", req.UserAgent())
}

func TestReferer(t *testing.T) {
	t.Parallel()

	req := AcquireRequest().SetReferer("http://example.com")
	utils.AssertEqual(t, "http://example.com", req.Referer())

	req.SetReferer("https://example.com")
	utils.AssertEqual(t, "https://example.com", req.Referer())
}

func TestRequestCookie(t *testing.T) {
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

func TestRequestPathParam(t *testing.T) {
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

func TestRequestFormData(t *testing.T) {
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

func TestRequestFile(t *testing.T) {
	t.Parallel()

	t.Run("add file", func(t *testing.T) {

	})
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

func TestRequestInvalidURL(t *testing.T) {
	t.Parallel()

	resp, err := AcquireRequest().
		Get("http://example.com\r\n\r\nGET /\r\n\r\n")

	utils.AssertEqual(t, ErrURLForamt, err)
	utils.AssertEqual(t, (*Response)(nil), resp)
}

func TestRequestUnsupportProtocol(t *testing.T) {
	t.Parallel()

	resp, err := AcquireRequest().
		Get("ftp://example.com")
	utils.AssertEqual(t, ErrURLForamt, err)
	utils.AssertEqual(t, (*Response)(nil), resp)
}

func TestRequestGet(t *testing.T) {
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

func TestRequestPost(t *testing.T) {
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

func TestRequestHead(t *testing.T) {
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

func TestRequestPut(t *testing.T) {
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

func TestRequestPatch(t *testing.T) {
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

func TestRequestDelete(t *testing.T) {
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

// func Test_Client_UserAgent(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Header.UserAgent())
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	t.Run("default", func(t *testing.T) {
// 		for i := 0; i < 5; i++ {
// 			a := Get("http://example.com")

// 			a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 			code, body, errs := a.String()

// 			utils.AssertEqual(t, fiber.StatusOK, code)
// 			utils.AssertEqual(t, defaultUserAgent, body)
// 			utils.AssertEqual(t, 0, len(errs))
// 		}
// 	})

// 	t.Run("custom", func(t *testing.T) {
// 		for i := 0; i < 5; i++ {
// 			c := AcquireClient()
// 			c.UserAgent = "ua"

// 			a := c.Get("http://example.com")

// 			a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 			code, body, errs := a.String()

// 			utils.AssertEqual(t, fiber.StatusOK, code)
// 			utils.AssertEqual(t, "ua", body)
// 			utils.AssertEqual(t, 0, len(errs))
// 			ReleaseClient(c)
// 		}
// 	})
// }

// func Test_Client_Agent_Set_Or_Add_Headers(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		c.Request().Header.VisitAll(func(key, value []byte) {
// 			if k := string(key); k == "K1" || k == "K2" {
// 				_, _ = c.Write(key)
// 				_, _ = c.Write(value)
// 			}
// 		})
// 		return nil
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.Set("k1", "v1").
// 			SetBytesK([]byte("k1"), "v1").
// 			SetBytesV("k1", []byte("v1")).
// 			AddBytesK([]byte("k1"), "v11").
// 			AddBytesV("k1", []byte("v22")).
// 			AddBytesKV([]byte("k1"), []byte("v33")).
// 			SetBytesKV([]byte("k2"), []byte("v2")).
// 			Add("k2", "v22")
// 	}

// 	testAgent(t, handler, wrapAgent, "K1v1K1v11K1v22K1v33K2v2K2v22")
// }

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

// func Test_Client_Agent_UserAgent(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Header.UserAgent())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.UserAgent("ua").
// 			UserAgentBytes([]byte("ua"))
// 	}

// 	testAgent(t, handler, wrapAgent, "ua")
// }

// func Test_Client_Agent_Cookie(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.SendString(
// 			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3") + c.Cookies("k4"))
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.Cookie("k1", "v1").
// 			CookieBytesK([]byte("k2"), "v2").
// 			CookieBytesKV([]byte("k2"), []byte("v2")).
// 			Cookies("k3", "v3", "k4", "v4").
// 			CookiesBytesKV([]byte("k3"), []byte("v3"), []byte("k4"), []byte("v4"))
// 	}

// 	testAgent(t, handler, wrapAgent, "v1v2v3v4")
// }

// func Test_Client_Agent_Referer(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Header.Referer())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.Referer("http://referer.com").
// 			RefererBytes([]byte("http://referer.com"))
// 	}

// 	testAgent(t, handler, wrapAgent, "http://referer.com")
// }

// func Test_Client_Agent_ContentType(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Header.ContentType())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.ContentType("custom-type").
// 			ContentTypeBytes([]byte("custom-type"))
// 	}

// 	testAgent(t, handler, wrapAgent, "custom-type")
// }

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

// func Test_Client_Agent_QueryString(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().URI().QueryString())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.QueryString("foo=bar&bar=baz").
// 			QueryStringBytes([]byte("foo=bar&bar=baz"))
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
// }

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

// func Test_Client_Agent_BodyString(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.BodyString("foo=bar&bar=baz")
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
// }

// func Test_Client_Agent_Body(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.Body([]byte("foo=bar&bar=baz"))
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar&bar=baz")
// }

// func Test_Client_Agent_BodyStream(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.BodyStream(strings.NewReader("body stream"), -1)
// 	}

// 	testAgent(t, handler, wrapAgent, "body stream")
// }

// func Test_Client_Agent_Custom_Response(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("custom")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	for i := 0; i < 5; i++ {
// 		a := AcquireAgent()
// 		resp := AcquireResponse()

// 		req := a.Request()
// 		req.Header.SetMethod(fiber.MethodGet)
// 		req.SetRequestURI("http://example.com")

// 		utils.AssertEqual(t, nil, a.Parse())

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.SetResponse(resp).
// 			String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "custom", body)
// 		utils.AssertEqual(t, "custom", string(resp.Body()))
// 		utils.AssertEqual(t, 0, len(errs))

// 		ReleaseResponse(resp)
// 	}
// }

// func Test_Client_Agent_Dest(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("dest")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	t.Run("small dest", func(t *testing.T) {
// 		dest := []byte("de")

// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.Dest(dest[:0]).String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "dest", body)
// 		utils.AssertEqual(t, "de", string(dest))
// 		utils.AssertEqual(t, 0, len(errs))
// 	})

// 	t.Run("enough dest", func(t *testing.T) {
// 		dest := []byte("foobar")

// 		a := Get("http://example.com")

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.Dest(dest[:0]).String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "dest", body)
// 		utils.AssertEqual(t, "destar", string(dest))
// 		utils.AssertEqual(t, 0, len(errs))
// 	})
// }

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

// func Test_Client_Agent_Json(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, fiber.MIMEApplicationJSON, string(c.Request().Header.ContentType()))

// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.JSON(data{Success: true})
// 	}

// 	testAgent(t, handler, wrapAgent, `{"success":true}`)
// }

// func Test_Client_Agent_Json_Error(t *testing.T) {
// 	a := Get("http://example.com").
// 		JSONEncoder(json.Marshal).
// 		JSON(complex(1, 1))

// 	_, body, errs := a.String()

// 	utils.AssertEqual(t, "", body)
// 	utils.AssertEqual(t, 1, len(errs))
// 	utils.AssertEqual(t, "json: unsupported type: complex128", errs[0].Error())
// }

// func Test_Client_Agent_XML(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, fiber.MIMEApplicationXML, string(c.Request().Header.ContentType()))

// 		return c.Send(c.Request().Body())
// 	}

// 	wrapAgent := func(a *Agent) {
// 		a.XML(data{Success: true})
// 	}

// 	testAgent(t, handler, wrapAgent, "<data><success>true</success></data>")
// }

// func Test_Client_Agent_XML_Error(t *testing.T) {
// 	a := Get("http://example.com").
// 		XML(complex(1, 1))

// 	_, body, errs := a.String()

// 	utils.AssertEqual(t, "", body)
// 	utils.AssertEqual(t, 1, len(errs))
// 	utils.AssertEqual(t, "xml: unsupported type: complex128", errs[0].Error())
// }

// func Test_Client_Agent_Form(t *testing.T) {
// 	handler := func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, fiber.MIMEApplicationForm, string(c.Request().Header.ContentType()))

// 		return c.Send(c.Request().Body())
// 	}

// 	args := AcquireArgs()

// 	args.Set("foo", "bar")

// 	wrapAgent := func(a *Agent) {
// 		a.Form(args)
// 	}

// 	testAgent(t, handler, wrapAgent, "foo=bar")

// 	ReleaseArgs(args)
// }

// func Test_Client_Agent_MultipartForm(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Post("/", func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

// 		mf, err := c.MultipartForm()
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, "bar", mf.Value["foo"][0])

// 		return c.Send(c.Request().Body())
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	args := AcquireArgs()

// 	args.Set("foo", "bar")

// 	a := Post("http://example.com").
// 		Boundary("myBoundary").
// 		MultipartForm(args)

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	code, body, errs := a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "--myBoundary\r\nContent-Disposition: form-data; name=\"foo\"\r\n\r\nbar\r\n--myBoundary--\r\n", body)
// 	utils.AssertEqual(t, 0, len(errs))
// 	ReleaseArgs(args)
// }

// func Test_Client_Agent_MultipartForm_Errors(t *testing.T) {
// 	t.Parallel()

// 	a := AcquireAgent()
// 	a.mw = &errorMultipartWriter{}

// 	args := AcquireArgs()
// 	args.Set("foo", "bar")

// 	ff1 := &FormFile{"", "name1", []byte("content"), false}
// 	ff2 := &FormFile{"", "name2", []byte("content"), false}
// 	a.FileData(ff1, ff2).
// 		MultipartForm(args)

// 	utils.AssertEqual(t, 4, len(a.errs))
// 	ReleaseArgs(args)
// }

// func Test_Client_Agent_MultipartForm_SendFiles(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Post("/", func(c fiber.Ctx) error {
// 		utils.AssertEqual(t, "multipart/form-data; boundary=myBoundary", c.Get(fiber.HeaderContentType))

// 		fh1, err := c.FormFile("field1")
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, fh1.Filename, "name")
// 		buf := make([]byte, fh1.Size)
// 		f, err := fh1.Open()
// 		utils.AssertEqual(t, nil, err)
// 		defer func() { _ = f.Close() }()
// 		_, err = f.Read(buf)
// 		utils.AssertEqual(t, nil, err)
// 		utils.AssertEqual(t, "form file", string(buf))

// 		fh2, err := c.FormFile("index")
// 		utils.AssertEqual(t, nil, err)
// 		checkFormFile(t, fh2, ".github/testdata/index.html")

// 		fh3, err := c.FormFile("file3")
// 		utils.AssertEqual(t, nil, err)
// 		checkFormFile(t, fh3, ".github/testdata/index.tmpl")

// 		return c.SendString("multipart form files")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	for i := 0; i < 5; i++ {
// 		ff := AcquireFormFile()
// 		ff.Fieldname = "field1"
// 		ff.Name = "name"
// 		ff.Content = []byte("form file")

// 		a := Post("http://example.com").
// 			Boundary("myBoundary").
// 			FileData(ff).
// 			SendFiles(".github/testdata/index.html", "index", ".github/testdata/index.tmpl").
// 			MultipartForm(nil)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, "multipart form files", body)
// 		utils.AssertEqual(t, 0, len(errs))

// 		ReleaseFormFile(ff)
// 	}
// }

// func checkFormFile(t *testing.T, fh *multipart.FileHeader, filename string) {
// 	t.Helper()

// 	basename := filepath.Base(filename)
// 	utils.AssertEqual(t, fh.Filename, basename)

// 	b1, err := os.ReadFile(filename)
// 	utils.AssertEqual(t, nil, err)

// 	b2 := make([]byte, fh.Size)
// 	f, err := fh.Open()
// 	utils.AssertEqual(t, nil, err)
// 	defer func() { _ = f.Close() }()
// 	_, err = f.Read(b2)
// 	utils.AssertEqual(t, nil, err)
// 	utils.AssertEqual(t, b1, b2)
// }

// func Test_Client_Agent_Multipart_Random_Boundary(t *testing.T) {
// 	t.Parallel()

// 	a := Post("http://example.com").
// 		MultipartForm(nil)

// 	reg := regexp.MustCompile(`multipart/form-data; boundary=\w{30}`)

// 	utils.AssertEqual(t, true, reg.Match(a.req.Header.Peek(fiber.HeaderContentType)))
// }

// func Test_Client_Agent_Multipart_Invalid_Boundary(t *testing.T) {
// 	t.Parallel()

// 	a := Post("http://example.com").
// 		Boundary("*").
// 		MultipartForm(nil)

// 	utils.AssertEqual(t, 1, len(a.errs))
// 	utils.AssertEqual(t, "mime: invalid boundary character", a.errs[0].Error())
// }

// func Test_Client_Agent_SendFile_Error(t *testing.T) {
// 	t.Parallel()

// 	a := Post("http://example.com").
// 		SendFile("non-exist-file!", "")

// 	utils.AssertEqual(t, 1, len(a.errs))
// 	utils.AssertEqual(t, true, strings.Contains(a.errs[0].Error(), "open non-exist-file!"))
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

// func Test_Client_Agent_Timeout(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		time.Sleep(time.Millisecond * 200)
// 		return c.SendString("timeout")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Get("http://example.com").
// 		Timeout(time.Millisecond * 50)

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	_, body, errs := a.String()

// 	utils.AssertEqual(t, "", body)
// 	utils.AssertEqual(t, 1, len(errs))
// 	utils.AssertEqual(t, "timeout", errs[0].Error())
// }

// func Test_Client_Agent_Reuse(t *testing.T) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", func(c fiber.Ctx) error {
// 		return c.SendString("reuse")
// 	})

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	a := Get("http://example.com").
// 		Reuse()

// 	a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 	code, body, errs := a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "reuse", body)
// 	utils.AssertEqual(t, 0, len(errs))

// 	code, body, errs = a.String()

// 	utils.AssertEqual(t, fiber.StatusOK, code)
// 	utils.AssertEqual(t, "reuse", body)
// 	utils.AssertEqual(t, 0, len(errs))
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

// func testAgent(t *testing.T, handler fiber.Handler, wrapAgent func(agent *Agent), excepted string, count ...int) {
// 	t.Parallel()

// 	ln := fasthttputil.NewInmemoryListener()

// 	app := fiber.New(fiber.Config{DisableStartupMessage: true})

// 	app.Get("/", handler)

// 	go func() { utils.AssertEqual(t, nil, app.Listener(ln)) }()

// 	c := 1
// 	if len(count) > 0 {
// 		c = count[0]
// 	}

// 	for i := 0; i < c; i++ {
// 		a := Get("http://example.com")

// 		wrapAgent(a)

// 		a.HostClient.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }

// 		code, body, errs := a.String()

// 		utils.AssertEqual(t, fiber.StatusOK, code)
// 		utils.AssertEqual(t, excepted, body)
// 		utils.AssertEqual(t, 0, len(errs))
// 	}
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

func TestSetValWithStruct(t *testing.T) {
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
