package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

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

	t.Run("streaming with io.Writer without closing", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/stream", func(c fiber.Ctx) error {
				return c.SendString("streaming data")
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/stream")
		require.NoError(t, err)
		defer resp.Close()

		// Custom writer that tracks if it's closed
		closableBuffer := &testClosableBuffer{}

		err = resp.Save(closableBuffer)
		require.NoError(t, err)

		// Check content
		require.Equal(t, "streaming data", closableBuffer.String())

		// Check that the writer was not closed by Save()
		require.False(t, closableBuffer.closed, "Save() should not close the writer")
	})

	t.Run("streaming with io.Writer error during copy", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/stream", func(c fiber.Ctx) error {
				return c.SendString("streaming data that will fail to write")
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/stream")
		require.NoError(t, err)
		defer resp.Close()

		// Use a writer that will fail after a few bytes
		errorWriter := &testErrorWriter{maxBytes: 5}

		err = resp.Save(errorWriter)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to write response body to writer")
		require.Contains(t, err.Error(), "write error after 5 bytes")
	})
}

// testClosableBuffer is a helper for testing writers that should not be closed.
type testClosableBuffer struct {
	bytes.Buffer
	closed bool
}

// Close implements the io.Closer interface.
func (tcb *testClosableBuffer) Close() error {
	tcb.closed = true
	return nil
}

// testErrorWriter is a helper for testing write errors during io.CopyBuffer.
type testErrorWriter struct {
	maxBytes int
	written  int
}

func (tew *testErrorWriter) Write(p []byte) (int, error) {
	if tew.written >= tew.maxBytes {
		return 0, fmt.Errorf("write error after %d bytes", tew.maxBytes)
	}

	remainingBytes := tew.maxBytes - tew.written
	if len(p) <= remainingBytes {
		tew.written += len(p)
		return len(p), nil
	}

	// Write only up to maxBytes, then return error
	tew.written += remainingBytes
	return remainingBytes, fmt.Errorf("write error after %d bytes", tew.maxBytes)
}

func Test_Response_BodyStream(t *testing.T) {
	t.Parallel()

	t.Run("basic streaming", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/stream", func(c fiber.Ctx) error {
				return c.SendString("streaming data")
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
				return c.Send(data)
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
		for i := 0; i < 1024; i++ {
			expected := byte('A' + i%26)
			require.Equal(t, expected, totalRead[i])
		}
	})

	t.Run("compare with regular body", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/stream", func(c fiber.Ctx) error {
				return c.SendString("streaming data")
			})
		})
		defer server.stop()

		client1 := New().SetDial(server.dial())
		resp1, err := client1.Get("http://example.com/stream")
		require.NoError(t, err)
		defer resp1.Close()
		normalBody := resp1.Body()
		client2 := New().SetDial(server.dial()).SetStreamResponseBody(true)
		resp2, err := client2.Get("http://example.com/stream")
		require.NoError(t, err)
		defer resp2.Close()
		streamedBody, err := io.ReadAll(resp2.BodyStream())
		require.NoError(t, err)
		require.Equal(t, normalBody, streamedBody)
	})

	t.Run("chunked streaming with delays", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/chunked", func(c fiber.Ctx) error {
				c.Set("Content-Type", "text/plain")
				c.Set("Transfer-Encoding", "chunked")
				chunks := []string{"chunk1", "chunk2", "chunk3"}
				for i, chunk := range chunks {
					if _, err := c.WriteString(chunk); err != nil {
						return err
					}
					c.Response().ImmediateHeaderFlush = true
					if i < len(chunks)-1 {
						time.Sleep(10 * time.Millisecond) // Shorter delay for faster tests
					}
				}
				return nil
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/chunked")
		require.NoError(t, err)
		defer resp.Close()

		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)

		var receivedChunks []string
		buffer := make([]byte, 10)

		for {
			n, err := bodyStream.Read(buffer)
			if n > 0 {
				chunk := string(buffer[:n])
				receivedChunks = append(receivedChunks, chunk)
			}
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		fullContent := strings.Join(receivedChunks, "")
		require.Equal(t, "chunk1chunk2chunk3", fullContent)
		require.GreaterOrEqual(t, len(receivedChunks), 1, "Should receive data chunks")
	})

	t.Run("server sent events with incremental reads", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/sse", func(c fiber.Ctx) error {
				c.Set("Content-Type", "text/event-stream")
				c.Set("Cache-Control", "no-cache")
				c.Set("Connection", "keep-alive")

				messages := []string{
					"data: event 1\n\n",
					"data: event 2\n\n",
					"data: event 3\n\n",
					"data: event 4\n\n",
				}

				for i, msg := range messages {
					if _, err := c.WriteString(msg); err != nil {
						return err
					}
					c.Response().ImmediateHeaderFlush = true
					if i < len(messages)-1 {
						time.Sleep(5 * time.Millisecond)
					}
				}
				return nil
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/sse")
		require.NoError(t, err)
		defer resp.Close()

		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)

		data, err := io.ReadAll(bodyStream)
		require.NoError(t, err)

		content := string(data)
		require.Contains(t, content, "event 1")
		require.Contains(t, content, "event 2")
		require.Contains(t, content, "event 3")
		require.Contains(t, content, "event 4")
		require.Contains(t, content, "data: event")
		require.Contains(t, content, "\n\n")
	})

	t.Run("progressive json streaming", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/json-stream", func(c fiber.Ctx) error {
				c.Set("Content-Type", "application/json")
				jsonParts := []string{
					`[`,
					`{"id":1,"name":"item1"},`,
					`{"id":2,"name":"item2"},`,
					`{"id":3,"name":"item3"}`,
					`]`,
				}
				for i, part := range jsonParts {
					if _, err := c.WriteString(part); err != nil {
						return err
					}
					c.Response().ImmediateHeaderFlush = true

					if i < len(jsonParts)-1 {
						time.Sleep(2 * time.Millisecond)
					}
				}
				return nil
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/json-stream")
		require.NoError(t, err)
		defer resp.Close()

		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)

		data, err := io.ReadAll(bodyStream)
		require.NoError(t, err)

		fullJSON := string(data)
		require.JSONEq(t, `[{"id":1,"name":"item1"},{"id":2,"name":"item2"},{"id":3,"name":"item3"}]`, fullJSON)
		var items []map[string]any
		err = json.Unmarshal([]byte(fullJSON), &items)
		require.NoError(t, err)
		require.Len(t, items, 3)
	})

	t.Run("connection interruption handling", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/interrupted", func(c fiber.Ctx) error {
				c.Set("Content-Type", "text/plain")

				if _, err := c.WriteString("initial data"); err != nil {
					return err
				}
				c.Response().ImmediateHeaderFlush = true

				time.Sleep(10 * time.Millisecond)
				if _, err := c.WriteString(" more data"); err != nil {
					return err
				}

				return nil
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/interrupted")
		require.NoError(t, err)

		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)

		buffer := make([]byte, 64)
		n, err := bodyStream.Read(buffer)
		require.NoError(t, err)
		require.Contains(t, string(buffer[:n]), "initial")

		resp.Close()

		_, err = bodyStream.Read(buffer)
		require.True(t, errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "connection"))
	})

	t.Run("large response streaming validation", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/large", func(c fiber.Ctx) error {
				c.Set("Content-Type", "text/plain")

				const chunkSize = 1024
				const numChunks = 10

				for i := 0; i < numChunks; i++ {
					chunk := make([]byte, chunkSize)
					for j := 0; j < chunkSize; j++ {
						chunk[j] = byte('A' + ((i*chunkSize + j) % 26))
					}

					if _, err := c.Write(chunk); err != nil {
						return err
					}
					c.Response().ImmediateHeaderFlush = true

					if i < numChunks-1 {
						time.Sleep(time.Millisecond)
					}
				}
				return nil
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)

		resp, err := client.Get("http://example.com/large")
		require.NoError(t, err)
		defer resp.Close()

		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)

		buffer := make([]byte, 512)
		var totalRead []byte
		readCount := 0

		for {
			n, err := bodyStream.Read(buffer)
			if n > 0 {
				totalRead = append(totalRead, buffer[:n]...)
				readCount++
			}
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		expectedSize := 1024 * 10
		require.Len(t, totalRead, expectedSize)
		require.Greater(t, readCount, 1, "Should have made multiple reads for streaming")
		for i := 0; i < expectedSize; i++ {
			expected := byte('A' + (i % 26))
			require.Equal(t, expected, totalRead[i], "Data pattern mismatch at position %d", i)
		}
	})

	t.Run("stream object identity when streaming enabled", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/stream", func(c fiber.Ctx) error {
				// Use chunked encoding to force streaming
				c.Set("Transfer-Encoding", "chunked")
				data := make([]byte, 1024*8) // 8KB
				for i := range data {
					data[i] = byte('S')
				}
				return c.Send(data)
			})
		})
		defer server.stop()

		client := New().SetDial(server.dial()).SetStreamResponseBody(true)
		resp, err := client.Get("http://example.com/stream")
		require.NoError(t, err)
		defer resp.Close()
		rawStream := resp.RawResponse.BodyStream()
		if rawStream != nil {
			require.True(t, resp.IsStreaming())
			bodyStream := resp.BodyStream()
			require.NotNil(t, bodyStream)
			require.Same(t, rawStream, bodyStream, "BodyStream() should return the exact same stream object when RawResponse.BodyStream() is not nil")
		} else {
			require.False(t, resp.IsStreaming())
			bodyStream := resp.BodyStream()
			require.NotNil(t, bodyStream)
			_, ok := bodyStream.(*bytes.Reader)
			require.True(t, ok, "When RawResponse.BodyStream() is nil, BodyStream() should return a *bytes.Reader")
		}
		bodyStream := resp.BodyStream()
		data, err := io.ReadAll(bodyStream)
		require.NoError(t, err)
		require.Len(t, data, 1024*8)
		for _, b := range data {
			require.Equal(t, byte('S'), b)
		}
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
		bodyBytes := resp.Body()
		require.Equal(t, "regular response body", string(bodyBytes))
	})

	t.Run("empty response body stream", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/empty", func(c fiber.Ctx) error {
				return c.SendString("")
			})
		})
		defer server.stop()
		client := New().SetDial(server.dial())
		resp, err := client.Get("http://example.com/empty")
		require.NoError(t, err)
		defer resp.Close()
		require.False(t, resp.IsStreaming())
		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)
		data, err := io.ReadAll(bodyStream)
		require.NoError(t, err)
		require.Equal(t, "", string(data))
	})

	t.Run("large non-streaming response", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/large", func(c fiber.Ctx) error {
				data := make([]byte, 50*1024) // 50KB
				for i := range data {
					data[i] = byte('X')
				}
				return c.Send(data)
			})
		})
		defer server.stop()
		client := New().SetDial(server.dial())
		resp, err := client.Get("http://example.com/large")
		require.NoError(t, err)
		defer resp.Close()
		require.False(t, resp.IsStreaming())
		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)
		buffer := make([]byte, 1024)
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

		require.Len(t, totalRead, 50*1024)
		for i, b := range totalRead {
			require.Equal(t, byte('X'), b, "Byte mismatch at position %d", i)
		}
	})
}

func Test_Response_IsStreaming(t *testing.T) {
	t.Parallel()

	t.Run("streaming with large response", func(t *testing.T) {
		t.Parallel()

		server := startTestServer(t, func(app *fiber.App) {
			app.Get("/large-stream", func(c fiber.Ctx) error {
				data := make([]byte, 64*1024) // 64KB
				for i := range data {
					data[i] = byte('S')
				}
				return c.Send(data)
			})
		})
		defer server.stop()
		client := New().SetDial(server.dial()).SetStreamResponseBody(true)
		resp, err := client.Get("http://example.com/large-stream")
		require.NoError(t, err)
		defer resp.Close()
		bodyStream := resp.BodyStream()
		require.NotNil(t, bodyStream)
		data, err := io.ReadAll(bodyStream)
		require.NoError(t, err)
		require.Len(t, data, 64*1024)
	})

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
		client1 := New().SetDial(server.dial()).SetStreamResponseBody(true)
		resp1, err := client1.Get("http://example.com/test")
		require.NoError(t, err)
		defer resp1.Close()
		bodyStream1 := resp1.BodyStream()
		require.NotNil(t, bodyStream1)
		data1, err := io.ReadAll(bodyStream1)
		require.NoError(t, err)
		require.Equal(t, "test content", string(data1))
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
		require.Equal(t, string(data1), string(data2))
	})
}
