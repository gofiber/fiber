package client

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"io"
	"net"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3/internal/tlstest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Response_Status(t *testing.T) {
	t.Parallel()

	setupApp := func() *testServer {
		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("foo")
			})
			app.Get("/fail", func(c fiber.Ctx) error {
				return c.SendStatus(407)
			})
		})

		return server
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example")

		require.NoError(t, err)
		require.Equal(t, "OK", resp.Status())
		resp.Close()
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example/fail")

		require.NoError(t, err)
		require.Equal(t, "Proxy Authentication Required", resp.Status())
		resp.Close()
	})
}

func Test_Response_Status_Code(t *testing.T) {
	t.Parallel()

	setupApp := func() *testServer {
		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("foo")
			})
			app.Get("/fail", func(c fiber.Ctx) error {
				return c.SendStatus(407)
			})
		})

		return server
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example")

		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		resp.Close()
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example/fail")

		require.NoError(t, err)
		require.Equal(t, 407, resp.StatusCode())
		resp.Close()
	})
}

func Test_Response_Protocol(t *testing.T) {
	t.Parallel()

	t.Run("http", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("foo")
			})
		})
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example")

		require.NoError(t, err)
		require.Equal(t, "HTTP/1.1", resp.Protocol())
		resp.Close()
	})

	t.Run("https", func(t *testing.T) {
		t.Parallel()

		serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
		require.NoError(t, err)

		ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
		require.NoError(t, err)

		ln = tls.NewListener(ln, serverTLSConf)

		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString(c.Scheme())
		})

		go func() {
			assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
				DisableStartupMessage: true,
			}))
		}()

		client := NewClient()
		resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + ln.Addr().String())

		require.NoError(t, err)
		require.Equal(t, clientTLSConf, client.TLSConfig())
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "https", resp.String())
		require.Equal(t, "HTTP/1.1", resp.Protocol())

		resp.Close()
	})
}

func Test_Response_Header(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Response().Header.Add("foo", "bar")
			return c.SendString("helo world")
		})
	})
	defer server.stop()

	client := NewClient().SetDial(server.dial())

	resp, err := AcquireRequest().
		SetClient(client).
		Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, "bar", resp.Header("foo"))
	resp.Close()
}

func Test_Response_Cookie(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:  "foo",
				Value: "bar",
			})
			return c.SendString("helo world")
		})
	})
	defer server.stop()

	client := NewClient().SetDial(server.dial())

	resp, err := AcquireRequest().
		SetClient(client).
		Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, "bar", string(resp.Cookies()[0].Value()))
	resp.Close()
}

func Test_Response_Body(t *testing.T) {
	t.Parallel()

	setupApp := func() *testServer {
		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("hello world")
			})

			app.Get("/json", func(c fiber.Ctx) error {
				return c.SendString("{\"status\":\"success\"}")
			})

			app.Get("/xml", func(c fiber.Ctx) error {
				return c.SendString("<status><name>success</name></status>")
			})
		})

		return server
	}

	t.Run("raw body", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com")

		require.NoError(t, err)
		require.Equal(t, []byte("hello world"), resp.Body())
		resp.Close()
	})

	t.Run("string body", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com")

		require.NoError(t, err)
		require.Equal(t, "hello world", resp.String())
		resp.Close()
	})

	t.Run("json body", func(t *testing.T) {
		t.Parallel()
		type body struct {
			Status string `json:"status"`
		}

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/json")

		require.NoError(t, err)

		tmp := &body{}
		err = resp.JSON(tmp)
		require.NoError(t, err)
		require.Equal(t, "success", tmp.Status)
		resp.Close()
	})

	t.Run("xml body", func(t *testing.T) {
		t.Parallel()
		type body struct {
			Name   xml.Name `xml:"status"`
			Status string   `xml:"name"`
		}

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/xml")

		require.NoError(t, err)

		tmp := &body{}
		err = resp.XML(tmp)
		require.NoError(t, err)
		require.Equal(t, "success", tmp.Status)
		resp.Close()
	})
}

func Test_Response_Save(t *testing.T) {
	t.Parallel()

	setupApp := func() *testServer {
		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/json", func(c fiber.Ctx) error {
				return c.SendString("{\"status\":\"success\"}")
			})
		})

		return server
	}

	t.Run("file path", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/json")

		require.NoError(t, err)

		err = resp.Save("./test/tmp.json")
		require.NoError(t, err)
		defer func() {
			_, err := os.Stat("./test/tmp.json")
			require.NoError(t, err)

			err = os.RemoveAll("./test")
			require.NoError(t, err)
		}()

		file, err := os.Open("./test/tmp.json")
		require.NoError(t, err)
		defer func(file *os.File) {
			err := file.Close()
			require.NoError(t, err)
		}(file)

		data, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "{\"status\":\"success\"}", string(data))
	})

	t.Run("io.Writer", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/json")

		require.NoError(t, err)

		buf := &bytes.Buffer{}

		err = resp.Save(buf)
		require.NoError(t, err)
		require.Equal(t, "{\"status\":\"success\"}", buf.String())
	})

	t.Run("error type", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := NewClient().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/json")

		require.NoError(t, err)

		err = resp.Save(nil)
		require.Error(t, err)
	})
}
