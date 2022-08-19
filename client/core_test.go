package client

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func Test_Exec_Func(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/normal", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	app.Get("/return-error", func(c fiber.Ctx) error {
		return fmt.Errorf("the request is error")
	})

	app.Get("/hang-up", func(c fiber.Ctx) error {
		time.Sleep(time.Second)
		return c.SendString(c.Hostname() + " hang up")
	})

	go func() {
		utils.AssertEqual(t, nil, app.Listener(ln))
	}()

	t.Run("normal request", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		core := client.core
		core.client.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.rawRequest.SetRequestURI("http://example.com/normal")

		resp, err := core.execFunc(context.Background(), client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 200, resp.rawResponse.StatusCode())
		utils.AssertEqual(t, "example.com", string(resp.rawResponse.Body()))
	})

	t.Run("the request return an error", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		core := client.core
		core.client.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.rawRequest.SetRequestURI("http://example.com/return-error")

		resp, err := core.execFunc(context.Background(), client, req)

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, 500, resp.rawResponse.StatusCode())
		utils.AssertEqual(t, "the request is error", string(resp.rawResponse.Body()))
	})

	t.Run("there is no connect", func(t *testing.T) {
		client := AcquireClient()
		core := client.core
		core.client.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		core.client.SetMaxConns(1)

		go func() {
			req := AcquireRequest()
			req.rawRequest.SetRequestURI("http://example.com/normal")
			_, err := core.execFunc(context.Background(), client, req)
			utils.AssertEqual(t, fasthttp.ErrNoFreeConns, err)
		}()

		req := AcquireRequest()
		req.rawRequest.SetRequestURI("http://example.com/hang-up")
		core.execFunc(context.Background(), client, req)
	})

	t.Run("the request timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		core := client.core

		core.client.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.rawRequest.SetRequestURI("http://example.com/hang-up")

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := core.execFunc(ctx, client, req)

		utils.AssertEqual(t, ErrTimeoutOrCancel, err)
	})
}

func Test_Execute(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/normal", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	app.Get("/return-error", func(c fiber.Ctx) error {
		return fmt.Errorf("the request is error")
	})

	app.Get("/hang-up", func(c fiber.Ctx) error {
		time.Sleep(time.Second)
		return c.SendString(c.Hostname() + " hang up")
	})

	go func() {
		utils.AssertEqual(t, nil, app.Listener(ln))
	}()

	t.Run("add user request hooks", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.AddRequestHook(func(c *Client, r *Request) error {
			utils.AssertEqual(t, "http://example.com", req.URL())
			return nil
		}).SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com")

		resp, err := client.core.execute(context.Background(), client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Cannot GET /", string(resp.rawResponse.Body()))
	})

	t.Run("add user response hooks", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.AddResponseHook(func(c *Client, resp *Response, req *Request) error {
			utils.AssertEqual(t, "http://example.com", req.URL())
			return nil
		}).SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com")

		resp, err := client.core.execute(context.Background(), client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "Cannot GET /", string(resp.rawResponse.Body()))
	})

	t.Run("no timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com/hang-up")

		resp, err := client.core.execute(context.Background(), client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "example.com hang up", string(resp.rawResponse.Body()))
	})

	t.Run("client timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetTimeout(500 * time.Millisecond)
		req.SetURL("http://example.com/hang-up")

		_, err := client.core.execute(context.Background(), client, req)
		utils.AssertEqual(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com/hang-up").
			SetTimeout(300 * time.Millisecond)

		_, err := client.core.execute(context.Background(), client, req)
		utils.AssertEqual(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout has higher level", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).
			SetTimeout(30 * time.Millisecond)
		req.SetURL("http://example.com/hang-up").
			SetTimeout(3000 * time.Millisecond)

		resp, err := client.core.execute(context.Background(), client, req)
		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, "example.com hang up", string(resp.rawResponse.Body()))
	})
}
