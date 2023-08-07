package client

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func Test_Response_Status(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("foo")
	})
	app.Get("/fail", func(c fiber.Ctx) error {
		return c.SendStatus(407)
	})
	go start()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example")

		require.NoError(t, err)
		require.Equal(t, "OK", resp.Status())
		resp.Close()
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example/fail")

		require.NoError(t, err)
		require.Equal(t, "Proxy Authentication Required", resp.Status())
		resp.Close()
	})
}

func Test_Response_Status_Code(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("foo")
	})
	app.Get("/fail", func(c fiber.Ctx) error {
		return c.SendStatus(407)
	})
	go start()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example")

		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode())
		resp.Close()
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example/fail")

		require.NoError(t, err)
		require.Equal(t, 407, resp.StatusCode())
		resp.Close()
	})
}

func Test_Response_Protocol(t *testing.T) {
	t.Parallel()

	t.Run("http", func(t *testing.T) {
		app, ln, start := createHelperServer(t)
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("foo")
		})
		go start()

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example")

		require.NoError(t, err)
		require.Equal(t, "HTTP/1.1", resp.Protocol())
		resp.Close()
	})

	// TODO: add https test after support https
	t.Run("https", func(t *testing.T) {
		t.Parallel()
	})
}

func Test_Response_Header(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		c.Response().Header.Add("foo", "bar")
		return c.SendString("helo world")
	})
	go start()

	resp, err := AcquireRequest().
		SetDial(ln).
		Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, "bar", resp.Header("foo"))
	resp.Close()
}

func Test_Response_Cookie(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:  "foo",
			Value: "bar",
		})
		return c.SendString("helo world")
	})
	go start()

	resp, err := AcquireRequest().
		SetDial(ln).
		Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, "bar", string(resp.Cookies()[0].Value()))
	resp.Close()
}

func Test_Response_Body(t *testing.T) {
	t.Parallel()

	app, ln, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello world")
	})
	app.Get("/json", func(c fiber.Ctx) error {
		return c.SendString("{\"status\":\"success\"}")
	})
	app.Get("/xml", func(c fiber.Ctx) error {
		return c.SendString("<status><name>success</name></status>")
	})

	go start()

	t.Run("raw body", func(t *testing.T) {
		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com")

		require.NoError(t, err)
		require.Equal(t, []byte("hello world"), resp.Body())
		resp.Close()
	})

	t.Run("string body", func(t *testing.T) {
		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com")

		require.NoError(t, err)
		require.Equal(t, "hello world", resp.String())
		resp.Close()
	})

	t.Run("json body", func(t *testing.T) {
		type body struct {
			Status string `json:"status"`
		}

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com/json")

		require.NoError(t, err)

		tmp := &body{}
		err = resp.JSON(tmp)
		require.NoError(t, err)
		require.Equal(t, "success", tmp.Status)
		resp.Close()
	})

	t.Run("xml body", func(t *testing.T) {
		type body struct {
			Name   xml.Name `xml:"status"`
			Status string   `xml:"name"`
		}

		resp, err := AcquireRequest().
			SetDial(ln).
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

	app, ln, start := createHelperServer(t)
	app.Get("/json", func(c fiber.Ctx) error {
		return c.SendString("{\"status\":\"success\"}")
	})

	go start()

	t.Run("file path", func(t *testing.T) {
		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com/json")

		require.NoError(t, err)

		err = resp.Save("./test/tmp.json")
		require.NoError(t, err)
		defer func() {
			if _, err := os.Stat("./test/tmp.json"); err != nil {
				return
			}

			os.RemoveAll("./test")
		}()

		file, err := os.Open("./test/tmp.json")
		defer file.Close()

		require.NoError(t, err)

		data, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "{\"status\":\"success\"}", string(data))
	})

	t.Run("io.Writer", func(t *testing.T) {
		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com/json")

		require.NoError(t, err)

		buf := &bytes.Buffer{}

		err = resp.Save(buf)
		require.NoError(t, err)
		require.Equal(t, "{\"status\":\"success\"}", buf.String())
	})

	t.Run("error type", func(t *testing.T) {
		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com/json")

		require.NoError(t, err)

		err = resp.Save(nil)
		require.Error(t, err)
	})
}
