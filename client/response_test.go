package client

import (
	"encoding/xml"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
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

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "OK", resp.Status())
		resp.Close()
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example/fail")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Proxy Authentication Required", resp.Status())
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

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.StatusCode())
		resp.Close()
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example/fail")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 407, resp.StatusCode())
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

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "HTTP/1.1", resp.Protocol())
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

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "bar", resp.Header("foo"))
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

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, "bar", string(resp.Cookies()[0].Value()))
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

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, []byte("hello world"), resp.Body())
		resp.Close()
	})

	t.Run("string body", func(t *testing.T) {
		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com")

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "hello world", resp.String())
		resp.Close()
	})

	t.Run("json body", func(t *testing.T) {
		type body struct {
			Status string `json:"status"`
		}

		resp, err := AcquireRequest().
			SetDial(ln).
			Get("http://example.com/json")

		utils.AssertEqual(t, nil, err)

		tmp := &body{}
		err = resp.JSON(tmp)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "success", tmp.Status)
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

		utils.AssertEqual(t, nil, err)

		tmp := &body{}
		err = resp.XML(tmp)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "success", tmp.Status)
		resp.Close()
	})
}
