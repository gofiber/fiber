package client

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
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
		require.Nil(t, app.Listener(ln))
	}()

	t.Run("normal request", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		core := req.core
		core.client.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.RawRequest.SetRequestURI("http://example.com/normal")

		resp, err := core.execFunc(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.RawResponse.StatusCode())
		require.Equal(t, "example.com", string(resp.RawResponse.Body()))
	})

	t.Run("the request return an error", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		core := req.core
		core.client.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.RawRequest.SetRequestURI("http://example.com/return-error")

		resp, err := core.execFunc(context.Background(), client, req)

		require.NoError(t, err)
		require.Equal(t, 500, resp.RawResponse.StatusCode())
		require.Equal(t, "the request is error", string(resp.RawResponse.Body()))
	})

	t.Run("the request timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		core := req.core

		core.client.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.RawRequest.SetRequestURI("http://example.com/hang-up")

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := core.execFunc(ctx, client, req)

		require.Equal(t, ErrTimeoutOrCancel, err)
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
		require.Nil(t, app.Listener(ln))
	}()

	t.Run("add user request hooks", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.AddRequestHook(func(c *Client, r *Request) error {
			require.Equal(t, "http://example.com", req.URL())
			return nil
		})
		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com")

		resp, err := req.core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "Cannot GET /", string(resp.RawResponse.Body()))
	})

	t.Run("add user response hooks", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.AddResponseHook(func(c *Client, resp *Response, req *Request) error {
			require.Equal(t, "http://example.com", req.URL())
			return nil
		})
		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com")

		resp, err := req.core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "Cannot GET /", string(resp.RawResponse.Body()))
	})

	t.Run("no timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()

		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up")

		resp, err := req.core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "example.com hang up", string(resp.RawResponse.Body()))
	})

	t.Run("client timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.SetTimeout(500 * time.Millisecond)
		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up")

		_, err := req.core.execute(context.Background(), client, req)
		require.Equal(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()

		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up").
			SetTimeout(300 * time.Millisecond)

		_, err := req.core.execute(context.Background(), client, req)
		require.Equal(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout has higher level", func(t *testing.T) {
		client, req := AcquireClient(), AcquireRequest()
		client.SetTimeout(30 * time.Millisecond)

		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up").
			SetTimeout(3000 * time.Millisecond)

		resp, err := req.core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "example.com hang up", string(resp.RawResponse.Body()))
	})
}
