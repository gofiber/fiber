package client

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/utils"
	"github.com/stretchr/testify/require"
)

func Test_Client_Invalid_URL(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	_, err := AcquireClient().
		R().
		SetDial(dial).
		Get("http://example.com\r\n\r\nGET /\r\n\r\n")

	require.ErrorIs(t, err, ErrURLForamt)
}

func Test_Client_Unsupported_Protocol(t *testing.T) {
	t.Parallel()

	_, err := AcquireClient().
		R().
		Get("ftp://example.com")

	require.ErrorIs(t, err, ErrURLForamt)
}

func Test_Get(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	t.Run("global get function", func(t *testing.T) {
		resp, err := Get("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "example.com", utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client get", func(t *testing.T) {
		resp, err := AcquireClient().Get("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "example.com", utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Head(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Head("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	t.Run("global head function", func(t *testing.T) {
		resp, err := Head("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "", utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client head", func(t *testing.T) {
		resp, err := AcquireClient().Head("http://example.com", Config{
			Dial: dial,
		})
		require.NoError(t, err)
		require.Equal(t, "", utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Post(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Post("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusCreated).
			SendString(c.FormValue("foo"))
	})

	go start()

	t.Run("global post function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Post("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client post", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Post("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Put(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Put("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()

	t.Run("global put function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Put("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client put", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Put("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Delete(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Delete("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNoContent).
			SendString("deleted")
	})

	go start()

	t.Run("global delete function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Delete("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})

	t.Run("client delete", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Delete("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})
}

func Test_Options(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Options("/", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNoContent).SendString("")
	})

	go start()

	t.Run("global options function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Options("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})

	t.Run("client options", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Options("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Equal(t, "", resp.String())
		}
	})
}
func Test_Patch(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Patch("/", func(c fiber.Ctx) error {
		return c.SendString(c.FormValue("foo"))
	})

	go start()

	t.Run("global patch function", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Patch("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client patch", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := AcquireClient().Patch("http://example.com", Config{
				Dial: dial,
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Client_UserAgent(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.UserAgent())
	})

	go start()

	t.Run("default", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			resp, err := Get("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, defaultUserAgent, resp.String())
		}
	})

	t.Run("custom", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			c := AcquireClient().
				SetUserAgent("ua")

			resp, err := c.Get("http://example.com", Config{
				Dial: dial,
			})

			require.Nil(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "ua", resp.String())
			ReleaseClient(c)
		}
	})
}

func Test_Client_Headers(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		c.Request().Header.VisitAll(func(key, value []byte) {
			if k := string(key); k == "K1" || k == "K2" {
				_, _ = c.Write(key)
				_, _ = c.Write(value)
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

func Test_Client_Referer(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		return c.Send(c.Request().Header.Referer())
	}

	wrapAgent := func(c *Client) {
		c.SetReferer("http://referer.com")
	}

	testClient(t, handler, wrapAgent, "http://referer.com")
}

func Test_Client_Params(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		c.WriteString(c.Query("k1"))
		c.WriteString(c.Query("k2"))

		return nil
	}

	wrapAgent := func(c *Client) {
		c.SetParam("k1", "v1").
			AddParam("k2", "v2")
	}

	testClient(t, handler, wrapAgent, "v1v2")
}

func Test_Client_R(t *testing.T) {
	t.Parallel()

	client := AcquireClient()
	req := client.R()

	require.Equal(t, "Request", reflect.TypeOf(req).Elem().Name())
	require.Equal(t, client, req.Client())
}

func Test_Client_Add_Hook(t *testing.T) {
	t.Parallel()

	t.Run("add request hooks", func(t *testing.T) {
		client := AcquireClient().AddRequestHook(func(c *Client, r *Request) error {
			return nil
		})

		require.Equal(t, 1, len(client.RequestHook()))

		client.AddRequestHook(func(c *Client, r *Request) error {
			return nil
		}, func(c *Client, r *Request) error {
			return nil
		})

		require.Equal(t, 3, len(client.RequestHook()))
	})

	t.Run("add response hooks", func(t *testing.T) {
		client := AcquireClient().AddResponseHook(func(c *Client, resp *Response, r *Request) error {
			return nil
		})

		require.Equal(t, 1, len(client.ResponseHook()))

		client.AddResponseHook(func(c *Client, resp *Response, r *Request) error {
			return nil
		}, func(c *Client, resp *Response, r *Request) error {
			return nil
		})

		require.Equal(t, 3, len(client.ResponseHook()))
	})
}

func Test_Client_Marshal(t *testing.T) {
	t.Run("set json marshal", func(t *testing.T) {
		client := AcquireClient().
			SetJSONMarshal(func(v any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.JSONMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set json unmarshal", func(t *testing.T) {
		client := AcquireClient().
			SetJSONUnmarshal(func(data []byte, v any) error {
				return fmt.Errorf("empty json")
			})

		err := client.JSONUnmarshal()(nil, nil)
		require.Equal(t, fmt.Errorf("empty json"), err)
	})

	t.Run("set xml marshal", func(t *testing.T) {
		client := AcquireClient().
			SetXMLMarshal(func(v any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.XMLMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set xml unmarshal", func(t *testing.T) {
		client := AcquireClient().
			SetXMLUnmarshal(func(data []byte, v any) error {
				return fmt.Errorf("empty xml")
			})

		err := client.XMLUnmarshal()(nil, nil)
		require.Equal(t, fmt.Errorf("empty xml"), err)
	})
}

func Test_Client_SetBaseURL(t *testing.T) {
	t.Parallel()

	client := AcquireClient().SetBaseURL("http://example.com")

	require.Equal(t, "http://example.com", client.BaseURL())
}

func Test_Client_Header(t *testing.T) {
	t.Parallel()

	t.Run("", func(t *testing.T) {

	})
}
