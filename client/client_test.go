package client

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/retry"
	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
)

func startTestServerWithPort(t *testing.T, beforeStarting func(app *fiber.App)) (*fiber.App, string) {
	t.Helper()

	app := fiber.New()

	if beforeStarting != nil {
		beforeStarting(app)
	}

	addrChan := make(chan string)
	errChan := make(chan error, 1)
	go func() {
		err := app.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				addrChan <- addr.String()
			},
		})
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case addr := <-addrChan:
		return app, addr
	case err := <-errChan:
		t.Fatalf("Failed to start test server: %v", err)
	}

	return nil, ""
}

func Test_Client_Add_Hook(t *testing.T) {
	t.Parallel()

	t.Run("add request hooks", func(t *testing.T) {
		t.Parallel()

		buf := bytebufferpool.Get()
		defer bytebufferpool.Put(buf)

		client := New().AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook1")
			return nil
		})

		require.Len(t, client.RequestHook(), 1)

		client.AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook2")
			return nil
		}, func(_ *Client, _ *Request) error {
			buf.WriteString("hook3")
			return nil
		})

		require.Len(t, client.RequestHook(), 3)
	})

	t.Run("add response hooks", func(t *testing.T) {
		t.Parallel()
		client := New().AddResponseHook(func(_ *Client, _ *Response, _ *Request) error {
			return nil
		})

		require.Len(t, client.ResponseHook(), 1)

		client.AddResponseHook(func(_ *Client, _ *Response, _ *Request) error {
			return nil
		}, func(_ *Client, _ *Response, _ *Request) error {
			return nil
		})

		require.Len(t, client.ResponseHook(), 3)
	})
}

func Test_Client_Add_Hook_CheckOrder(t *testing.T) {
	t.Parallel()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	client := New().
		AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook1")
			return nil
		}).
		AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook2")
			return nil
		}).
		AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook3")
			return nil
		})

	for _, hook := range client.RequestHook() {
		require.NoError(t, hook(client, &Request{}))
	}

	require.Equal(t, "hook1hook2hook3", buf.String())
}

func Test_Client_Marshal(t *testing.T) {
	t.Parallel()

	t.Run("set json marshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetJSONMarshal(func(_ any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.JSONMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set json marshal error", func(t *testing.T) {
		t.Parallel()

		emptyErr := errors.New("empty json")
		client := New().
			SetJSONMarshal(func(_ any) ([]byte, error) {
				return nil, emptyErr
			})

		val, err := client.JSONMarshal()(nil)
		require.Nil(t, val)
		require.ErrorIs(t, err, emptyErr)
	})

	t.Run("set json unmarshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetJSONUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty json")
			})

		err := client.JSONUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty json"), err)
	})

	t.Run("set json unmarshal error", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetJSONUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty json")
			})

		err := client.JSONUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty json"), err)
	})

	t.Run("set xml marshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLMarshal(func(_ any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.XMLMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set xml marshal error", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLMarshal(func(_ any) ([]byte, error) {
				return nil, errors.New("empty xml")
			})

		val, err := client.XMLMarshal()(nil)
		require.Nil(t, val)
		require.Equal(t, errors.New("empty xml"), err)
	})

	t.Run("set xml unmarshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty xml")
			})

		err := client.XMLUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty xml"), err)
	})

	t.Run("set xml unmarshal error", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty xml")
			})

		err := client.XMLUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty xml"), err)
	})
}

func Test_Client_SetBaseURL(t *testing.T) {
	t.Parallel()

	client := New().SetBaseURL("http://example.com")

	require.Equal(t, "http://example.com", client.BaseURL())
}

func Test_Client_Invalid_URL(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	_, err := New().SetDial(dial).
		R().
		Get("http//example")

	require.ErrorIs(t, err, ErrURLFormat)
}

func Test_Client_Unsupported_Protocol(t *testing.T) {
	t.Parallel()

	_, err := New().
		R().
		Get("ftp://example.com")

	require.ErrorIs(t, err, ErrURLFormat)
}

func Test_Client_ConcurrencyRequests(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.All("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname() + " " + c.Method())
	})
	go start()

	client := New().SetDial(dial)

	wg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
			wg.Add(1)
			go func(m string) {
				defer wg.Done()
				resp, err := client.Custom("http://example.com", m)
				assert.NoError(t, err)
				assert.Equal(t, "example.com "+m, utils.UnsafeString(resp.RawResponse.Body()))
			}(method)
		}
	}

	wg.Wait()
}

func Test_Get(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString(c.Hostname())
			})
		})

		return app, addr
	}

	t.Run("global get function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := Get("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "0.0.0.0", utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client get", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := New().Get("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "0.0.0.0", utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Head(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Head("/", func(c fiber.Ctx) error {
				return c.SendString(c.Hostname())
			})
		})

		return app, addr
	}

	t.Run("global head function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := Head("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "7", resp.Header(fiber.HeaderContentLength))
		require.Equal(t, "", utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client head", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := New().Head("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "7", resp.Header(fiber.HeaderContentLength))
		require.Equal(t, "", utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Post(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Post("/", func(c fiber.Ctx) error {
				return c.Status(fiber.StatusCreated).
					SendString(c.FormValue("foo"))
			})
		})

		return app, addr
	}

	t.Run("global post function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := Post("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client post", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := New().Post("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Put(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Put("/", func(c fiber.Ctx) error {
				return c.SendString(c.FormValue("foo"))
			})
		})

		return app, addr
	}

	t.Run("global put function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := Put("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client put", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := New().Put("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Delete(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Delete("/", func(c fiber.Ctx) error {
				return c.Status(fiber.StatusNoContent).
					SendString("deleted")
			})
		})

		return app, addr
	}

	t.Run("global delete function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		time.Sleep(1 * time.Second)

		for i := 0; i < 5; i++ {
			resp, err := Delete("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})

	t.Run("client delete", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := New().Delete("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})
}

func Test_Options(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Options("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderAllow, "GET, POST, PUT, DELETE, PATCH")
				return c.Status(fiber.StatusNoContent).SendString("")
			})
		})

		return app, addr
	}

	t.Run("global options function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := Options("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, "GET, POST, PUT, DELETE, PATCH", resp.Header(fiber.HeaderAllow))
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})

	t.Run("client options", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := New().Options("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, "GET, POST, PUT, DELETE, PATCH", resp.Header(fiber.HeaderAllow))
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})
}

func Test_Patch(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Patch("/", func(c fiber.Ctx) error {
				return c.SendString(c.FormValue("foo"))
			})
		})

		return app, addr
	}

	t.Run("global patch function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		time.Sleep(1 * time.Second)

		for i := 0; i < 5; i++ {
			resp, err := Patch("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client patch", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := New().Patch("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Client_UserAgent(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.Send(c.Request().Header.UserAgent())
			})
		})

		return app, addr
	}

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			resp, err := Get("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, defaultUserAgent, resp.String())
		}
	})

	t.Run("custom", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for i := 0; i < 5; i++ {
			c := New().
				SetUserAgent("ua")

			resp, err := c.Get("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "ua", resp.String())
		}
	})
}

func Test_Client_Header(t *testing.T) {
	t.Parallel()

	t.Run("add header", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddHeader("foo", "bar").AddHeader("foo", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set header", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddHeader("foo", "bar").SetHeader("foo", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add headers", func(t *testing.T) {
		t.Parallel()
		req := New()
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
		req := New()
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

	t.Run("set header case insensitive", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.SetHeader("foo", "bar").
			AddHeader("FOO", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})
}

func Test_Client_Header_With_Server(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		c.Request().Header.VisitAll(func(key, value []byte) {
			if k := string(key); k == "K1" || k == "K2" {
				_, _ = c.Write(key)   //nolint:errcheck // It is fine to ignore the error here
				_, _ = c.Write(value) //nolint:errcheck // It is fine to ignore the error here
			}
		})
		return nil
	}

	wrapAgent := func(c *Client) {
		c.SetHeader("k1", "v1").
			AddHeader("k1", "v11").
			AddHeaders(map[string][]string{
				"k1": {"v22", "v33"},
			}).
			SetHeaders(map[string]string{
				"k2": "v2",
			}).
			AddHeader("k2", "v22")
	}

	testClient(t, handler, wrapAgent, "K1v1K1v11K1v22K1v33K2v2K2v22")
}

func Test_Client_Cookie(t *testing.T) {
	t.Parallel()

	t.Run("set cookie", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetCookie("foo", "bar")
		require.Equal(t, "bar", req.Cookie("foo"))

		req.SetCookie("foo", "bar1")
		require.Equal(t, "bar1", req.Cookie("foo"))
	})

	t.Run("set cookies", func(t *testing.T) {
		t.Parallel()
		req := New().
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

		req := New().SetCookiesWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.Cookie("int"))
		require.Equal(t, "foo", req.Cookie("string"))
	})

	t.Run("del cookies", func(t *testing.T) {
		t.Parallel()
		req := New().
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

func Test_Client_Cookie_With_Server(t *testing.T) {
	t.Parallel()

	handler := func(c fiber.Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3") + c.Cookies("k4"))
	}

	wrapAgent := func(c *Client) {
		c.SetCookie("k1", "v1").
			SetCookies(map[string]string{
				"k2": "v2",
				"k3": "v3",
				"k4": "v4",
			}).DelCookies("k4")
	}

	testClient(t, handler, wrapAgent, "v1v2v3")
}

func Test_Client_CookieJar(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"))
	}

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	jar.SetKeyValue("example.com", "k1", "v1")
	jar.SetKeyValue("example.com", "k2", "v2")
	jar.SetKeyValue("example", "k3", "v3")

	wrapAgent := func(c *Client) {
		c.SetCookieJar(jar)
	}
	testClient(t, handler, wrapAgent, "v1v2")
}

func Test_Client_CookieJar_Response(t *testing.T) {
	t.Parallel()

	t.Run("without expiration", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:  "k4",
				Value: "v4",
			})
			return c.SendString(
				c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"))
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")
		jar.SetKeyValue("example.com", "k2", "v2")
		jar.SetKeyValue("example", "k3", "v3")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1v2")

		require.Len(t, jar.getCookiesByHost("example.com"), 3)
	})

	t.Run("with expiration", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:    "k4",
				Value:   "v4",
				Expires: time.Now().Add(1 * time.Nanosecond),
			})
			return c.SendString(
				c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"))
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")
		jar.SetKeyValue("example.com", "k2", "v2")
		jar.SetKeyValue("example", "k3", "v3")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1v2")

		require.Len(t, jar.getCookiesByHost("example.com"), 2)
	})

	t.Run("override cookie value", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:  "k1",
				Value: "v2",
			})
			return c.SendString(
				c.Cookies("k1") + c.Cookies("k2"))
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")
		jar.SetKeyValue("example.com", "k2", "v2")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1v2")

		for _, cookie := range jar.getCookiesByHost("example.com") {
			if string(cookie.Key()) == "k1" {
				require.Equal(t, "v2", string(cookie.Value()))
			}
		}
	})

	t.Run("different domain", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			return c.SendString(c.Cookies("k1"))
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1")

		require.Len(t, jar.getCookiesByHost("example.com"), 1)
		require.Empty(t, jar.getCookiesByHost("example"))
	})
}

func Test_Client_Referer(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(c *Client) {
		c.SetReferer("http://referer.com")
	}

	testClient(t, handler, wrapAgent, "http://referer.com")
}

func Test_Client_QueryParam(t *testing.T) {
	t.Parallel()

	t.Run("add param", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddParam("foo", "bar").AddParam("foo", "fiber")

		res := req.Param("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddParam("foo", "bar").SetParam("foo", "fiber")

		res := req.Param("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		t.Parallel()
		req := New()
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
		req := New()
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

		p := New()
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
		req := New()
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

func Test_Client_QueryParam_With_Server(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		_, _ = c.WriteString(c.Query("k1")) //nolint:errcheck // It is fine to ignore the error here
		_, _ = c.WriteString(c.Query("k2")) //nolint:errcheck // It is fine to ignore the error here

		return nil
	}

	wrapAgent := func(c *Client) {
		c.SetParam("k1", "v1").
			AddParam("k2", "v2")
	}

	testClient(t, handler, wrapAgent, "v1v2")
}

func Test_Client_PathParam(t *testing.T) {
	t.Parallel()

	t.Run("set path param", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetPathParam("foo", "bar")
		require.Equal(t, "bar", req.PathParam("foo"))

		req.SetPathParam("foo", "bar1")
		require.Equal(t, "bar1", req.PathParam("foo"))
	})

	t.Run("set path params", func(t *testing.T) {
		t.Parallel()
		req := New().
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

		req := New().SetPathParamsWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.PathParam("int"))
		require.Equal(t, "foo", req.PathParam("string"))
	})

	t.Run("del path params", func(t *testing.T) {
		t.Parallel()
		req := New().
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

func Test_Client_PathParam_With_Server(t *testing.T) {
	app, dial, start := createHelperServer(t)

	app.Get("/:test", func(c fiber.Ctx) error {
		return c.SendString(c.Params("test"))
	})

	go start()

	resp, err := New().SetDial(dial).
		SetPathParam("path", "test").
		Get("http://example.com/:path")

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "test", resp.String())
}

func Test_Client_TLS(t *testing.T) {
	t.Parallel()

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("tls")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		}))
	}()
	time.Sleep(1 * time.Second)

	client := New()
	resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + ln.Addr().String())

	require.NoError(t, err)
	require.Equal(t, clientTLSConf, client.TLSConfig())
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "tls", resp.String())
}

func Test_Client_TLS_Error(t *testing.T) {
	t.Parallel()

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	clientTLSConf.MaxVersion = tls.VersionTLS12
	serverTLSConf.MinVersion = tls.VersionTLS13
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("tls")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		}))
	}()
	time.Sleep(1 * time.Second)

	client := New()
	resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + ln.Addr().String())

	require.Error(t, err)
	require.Equal(t, clientTLSConf, client.TLSConfig())
	require.Nil(t, resp)
}

func Test_Client_TLS_Empty_TLSConfig(t *testing.T) {
	t.Parallel()

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("tls")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		}))
	}()
	time.Sleep(1 * time.Second)

	client := New()
	resp, err := client.Get("https://" + ln.Addr().String())

	require.Error(t, err)
	require.NotEqual(t, clientTLSConf, client.TLSConfig())
	require.Nil(t, resp)
}

func Test_Client_SetCertificates(t *testing.T) {
	t.Parallel()

	serverTLSConf, _, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	client := New().SetCertificates(serverTLSConf.Certificates...)
	require.Len(t, client.TLSConfig().Certificates, 1)
}

func Test_Client_SetRootCertificate(t *testing.T) {
	t.Parallel()

	client := New().SetRootCertificate("../.github/testdata/ssl.pem")
	require.NotNil(t, client.TLSConfig().RootCAs)
}

func Test_Client_SetRootCertificateFromString(t *testing.T) {
	t.Parallel()

	file, err := os.Open("../.github/testdata/ssl.pem")
	defer func() { require.NoError(t, file.Close()) }()
	require.NoError(t, err)

	pem, err := io.ReadAll(file)
	require.NoError(t, err)

	client := New().SetRootCertificateFromString(string(pem))
	require.NotNil(t, client.TLSConfig().RootCAs)
}

func Test_Client_R(t *testing.T) {
	t.Parallel()

	client := New()
	req := client.R()

	require.Equal(t, "Request", reflect.TypeOf(req).Elem().Name())
	require.Equal(t, client, req.Client())
}

func Test_Replace(t *testing.T) {
	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(string(c.Request().Header.Peek("k1")))
	})

	go start()

	C().SetDial(dial)
	resp, err := Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "", resp.String())

	r := New().SetDial(dial).SetHeader("k1", "v1")
	clean := Replace(r)
	resp, err = Get("http://example.com")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "v1", resp.String())

	clean()

	C().SetDial(dial)
	resp, err = Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "", resp.String())

	C().SetDial(nil)
}

func Test_Set_Config_To_Request(t *testing.T) {
	t.Parallel()

	t.Run("set ctx", func(t *testing.T) {
		t.Parallel()
		key := struct{}{}

		ctx := context.Background()
		ctx = context.WithValue(ctx, key, "v1") //nolint: staticcheck // not needed for tests

		req := AcquireRequest()

		setConfigToRequest(req, Config{Ctx: ctx})

		require.Equal(t, "v1", req.Context().Value(key))
	})

	t.Run("set useragent", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{UserAgent: "agent"})

		require.Equal(t, "agent", req.UserAgent())
	})

	t.Run("set referer", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Referer: "referer"})

		require.Equal(t, "referer", req.Referer())
	})

	t.Run("set header", func(t *testing.T) {
		req := AcquireRequest()

		setConfigToRequest(req, Config{Header: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.Header("k1")[0])
	})

	t.Run("set params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Param: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.Param("k1")[0])
	})

	t.Run("set cookies", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Cookie: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.Cookie("k1"))
	})

	t.Run("set pathparam", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{PathParam: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.PathParam("k1"))
	})

	t.Run("set timeout", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Timeout: 1 * time.Second})

		require.Equal(t, 1*time.Second, req.Timeout())
	})

	t.Run("set maxredirects", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{MaxRedirects: 1})

		require.Equal(t, 1, req.MaxRedirects())
	})

	t.Run("set body", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Body: "test"})

		require.Equal(t, "test", req.body)
	})

	t.Run("set file", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{File: []*File{
			{
				name: "test",
				path: "path",
			},
		}})

		require.Equal(t, "path", req.File("test").path)
	})
}

func Test_Client_SetProxyURL(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Get("isProxy"))
	})

	go start()

	fasthttpClient := &fasthttp.Client{
		Dial:                     dial,
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	}

	// Create a simple proxy sever
	proxyServer := fiber.New()

	proxyServer.Use("*", func(c fiber.Ctx) error {
		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()

		req.SetRequestURI(c.BaseURL())
		req.Header.SetMethod(fasthttp.MethodGet)

		c.Request().Header.VisitAll(func(key, value []byte) {
			req.Header.AddBytesKV(key, value)
		})

		req.Header.Set("isProxy", "true")

		if err := fasthttpClient.Do(req, resp); err != nil {
			return err
		}

		c.Status(resp.StatusCode())
		c.Context().SetBody(resp.Body())

		return nil
	})

	addrChan := make(chan string)
	go func() {
		assert.NoError(t, proxyServer.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				addrChan <- addr.String()
			},
		}))
	}()

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
	})

	time.Sleep(1 * time.Second)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		client := New()
		err := client.SetProxyURL(<-addrChan)

		require.NoError(t, err)

		resp, err := client.Get("http://localhost:3000")
		require.NoError(t, err)

		require.Equal(t, 200, resp.StatusCode())
		require.Equal(t, "true", string(resp.Body()))
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		client := New()

		err := client.SetProxyURL(":this is not a proxy")
		require.NoError(t, err)

		_, err = client.Get("http://localhost:3000")
		require.Error(t, err)
	})
}

func Test_Client_SetRetryConfig(t *testing.T) {
	t.Parallel()
	retryConfig := &retry.Config{
		InitialInterval: 1 * time.Second,
		MaxRetryCount:   3,
	}

	core, client, req := newCore(), New(), AcquireRequest()
	req.SetURL("http://exampleretry.com")
	client.SetRetryConfig(retryConfig)
	_, err := core.execute(context.Background(), client, req)

	require.Error(t, err)
	require.Equal(t, retryConfig.InitialInterval, client.RetryConfig().InitialInterval)
	require.Equal(t, retryConfig.MaxRetryCount, client.RetryConfig().MaxRetryCount)
}

func Benchmark_Client_Request(b *testing.B) {
	app, dial, start := createHelperServer(b)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello world")
	})

	go start()

	client := New().SetDial(dial)

	b.ResetTimer()
	b.ReportAllocs()

	var err error
	var resp *Response
	for i := 0; i < b.N; i++ {
		resp, err = client.Get("http://example.com")
		resp.Close()
	}
	require.NoError(b, err)
}

func Benchmark_Client_Request_Parallel(b *testing.B) {
	app, dial, start := createHelperServer(b)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello world")
	})

	go start()

	client := New().SetDial(dial)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		var err error
		var resp *Response
		for pb.Next() {
			resp, err = client.Get("http://example.com")
			resp.Close()
		}
		require.NoError(b, err)
	})
}
