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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gofiber/fiber/v3"
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

		client := New().SetDial(server.dial())

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

		client := New().SetDial(server.dial())

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

		client := New().SetDial(server.dial())

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

		client := New().SetDial(server.dial())

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

		client := New().SetDial(server.dial())

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

		client := New()
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

	client := New().SetDial(server.dial())

	resp, err := AcquireRequest().
		SetClient(client).
		Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, "bar", resp.Header("foo"))
	resp.Close()
}

func Test_Response_Headers(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Response().Header.Add("foo", "bar")
			c.Response().Header.Add("foo", "bar2")
			c.Response().Header.Add("foo2", "bar")

			return c.SendString("hello world")
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial())

	resp, err := AcquireRequest().
		SetClient(client).
		Get("http://example.com")

	require.NoError(t, err)

	headers := make(map[string][]string)
	for k, v := range resp.Headers() {
		headers[k] = append(headers[k], v...)
	}

	require.Equal(t, "hello world", resp.String())

	require.Contains(t, headers["Foo"], "bar")
	require.Contains(t, headers["Foo"], "bar2")
	require.Contains(t, headers["Foo2"], "bar")

	require.Len(t, headers, 5) // Foo + Foo2 + Date + Content-Length + Content-Type

	resp.Close()
}

func Benchmark_Headers(b *testing.B) {
	server := startTestServer(
		b,
		func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				c.Response().Header.Add("foo", "bar")
				c.Response().Header.Add("foo", "bar2")
				c.Response().Header.Add("foo", "bar3")

				c.Response().Header.Add("foo2", "bar")
				c.Response().Header.Add("foo2", "bar2")
				c.Response().Header.Add("foo2", "bar3")

				return c.SendString("helo world")
			})
		},
	)

	client := New().SetDial(server.dial())

	resp, err := AcquireRequest().
		SetClient(client).
		Get("http://example.com")
	require.NoError(b, err)

	b.Cleanup(func() {
		resp.Close()
		server.stop()
	})

	b.ReportAllocs()

	for b.Loop() {
		for k, v := range resp.Headers() {
			_ = k
			_ = v
		}
	}
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

	client := New().SetDial(server.dial())

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

			app.Get("/cbor", func(c fiber.Ctx) error {
				type cborData struct {
					Name string `cbor:"name"`
					Age  int    `cbor:"age"`
				}

				return c.CBOR(cborData{
					Name: "foo",
					Age:  12,
				})
			})
		})

		return server
	}

	t.Run("raw body", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := New().SetDial(server.dial())

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

		client := New().SetDial(server.dial())

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

		client := New().SetDial(server.dial())

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

		client := New().SetDial(server.dial())

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

	t.Run("cbor body", func(t *testing.T) {
		t.Parallel()
		type cborData struct {
			Name string `cbor:"name"`
			Age  int    `cbor:"age"`
		}

		data := cborData{
			Name: "foo",
			Age:  12,
		}

		server := setupApp()
		defer server.stop()

		client := New().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/cbor")

		require.NoError(t, err)

		tmp := &cborData{}
		err = resp.CBOR(tmp)
		require.NoError(t, err)
		require.Equal(t, data, *tmp)
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

		client := New().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/json")

		require.NoError(t, err)

		err = resp.Save("./test/tmp.json")
		require.NoError(t, err)
		defer func() {
			_, statErr := os.Stat("./test/tmp.json")
			require.NoError(t, statErr)

			statErr = os.RemoveAll("./test")
			require.NoError(t, statErr)
		}()

		file, err := os.Open("./test/tmp.json")
		require.NoError(t, err)
		defer func(file *os.File) {
			closeErr := file.Close()
			require.NoError(t, closeErr)
		}(file)

		data, err := io.ReadAll(file)
		require.NoError(t, err)
		require.JSONEq(t, "{\"status\":\"success\"}", string(data))
	})

	t.Run("io.Writer", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := New().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/json")

		require.NoError(t, err)

		buf := &bytes.Buffer{}

		err = resp.Save(buf)
		require.NoError(t, err)
		require.JSONEq(t, "{\"status\":\"success\"}", buf.String())
	})

	t.Run("error type", func(t *testing.T) {
		t.Parallel()

		server := setupApp()
		defer server.stop()

		client := New().SetDial(server.dial())

		resp, err := AcquireRequest().
			SetClient(client).
			Get("http://example.com/json")

		require.NoError(t, err)

		err = resp.Save(nil)
		require.Error(t, err)
	})
}

func Test_Response_BodyStream(t *testing.T) {
	t.Parallel()

	t.Run("basic streaming", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/stream", func(c fiber.Ctx) error {
				return c.SendStream(bytes.NewReader([]byte("streaming data")))
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/stream")
		require.NoError(t, err)
		defer resp.Close()
		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)
		data, err := io.ReadAll(bodyStream)
		require.NoError(t, err)
		require.Equal(t, "streaming data", string(data))
	})

	t.Run("large response streaming", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/large", func(c fiber.Ctx) error {
				data := make([]byte, 1024)
				for i := range data {
					data[i] = byte('A' + i%26)
				}
				return c.SendStream(bytes.NewReader(data))
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)
		resp, err := client.Get("http://example.com/large")
		require.NoError(t, err)
		defer resp.Close()
		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)
		buffer := make([]byte, 256)
		var totalRead []byte
		for {
			n, err := bodyStream.Read(buffer)
			if n > 0 {
				totalRead = append(totalRead, buffer[:n]...)
			}
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		require.Len(t, totalRead, 1024)
	})
}

func Test_Response_BodyStream_Fallback(t *testing.T) {
	t.Parallel()
	t.Run("non-streaming response fallback to bytes.Reader", func(t *testing.T) {
		t.Parallel()
		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/regular", func(c fiber.Ctx) error {
				return c.SendString("regular response body")
			})
		})
		defer server.stop()
		client := New().SetDial(server.dial())
		resp, err := client.Get("http://example.com/regular")
		require.NoError(t, err)
		defer resp.Close()
		require.False(t, resp.IsStreaming())
		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)
		data, err := io.ReadAll(bodyStream)
		require.NoError(t, err)
		require.Equal(t, "regular response body", string(data))
	})
}

func Test_Response_IsStreaming(t *testing.T) {
	t.Parallel()

	t.Run("streaming disabled", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/regular", func(c fiber.Ctx) error {
				return c.SendString("regular content")
			})
		})
		defer server.stop()
		client := New().SetDial(server.dial())
		resp, err := client.Get("http://example.com/regular")
		require.NoError(t, err)
		defer resp.Close()
		require.False(t, resp.IsStreaming())
	})

	t.Run("bodystream always works regardless of streaming state", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/test", func(c fiber.Ctx) error {
				return c.SendString("test content")
			})
		})
		defer server.stop()

		// Test with streaming enabled
		client1 := New().SetDial(server.dial()).SetStreamResponseBody(true)
		resp1, err := client1.Get("http://example.com/test")
		require.NoError(t, err)
		defer resp1.Close()
		bodyStream1 := resp1.BodyStream()
		require.NotNil(t, bodyStream1)
		data1, err := io.ReadAll(bodyStream1)
		require.NoError(t, err)
		require.Equal(t, "test content", string(data1))

		// Test with streaming disabled
		client2 := New().SetDial(server.dial()).SetStreamResponseBody(false)
		resp2, err := client2.Get("http://example.com/test")
		require.NoError(t, err)
		defer resp2.Close()
		require.False(t, resp2.IsStreaming())
		bodyStream2 := resp2.BodyStream()
		require.NotNil(t, bodyStream2)
		data2, err := io.ReadAll(bodyStream2)
		require.NoError(t, err)
		require.Equal(t, "test content", string(data2))
	})
}
