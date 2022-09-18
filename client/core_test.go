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

func Test_AddMissing_Port(t *testing.T) {
	type args struct {
		addr  string
		isTLS bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "do anything",
			args: args{
				addr: "example.com:1234",
			},
			want: "example.com:1234",
		},
		{
			name: "add 80 port",
			args: args{
				addr: "example.com",
			},
			want: "example.com:80",
		},
		{
			name: "add 443 port",
			args: args{
				addr:  "example.com",
				isTLS: true,
			},
			want: "example.com:443",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, addMissingPort(tt.args.addr, tt.args.isTLS))
		})
	}
}

func Test_Exec_Func(t *testing.T) {
	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New()

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
		core, client, req := newCore(), AcquireClient(), AcquireRequest()
		core.host.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.RawRequest.SetRequestURI("http://example.com/normal")

		core.ctx = context.Background()
		core.client = client
		core.req = req

		resp, err := core.execFunc()
		require.NoError(t, err)
		require.Equal(t, 200, resp.RawResponse.StatusCode())
		require.Equal(t, "example.com", string(resp.RawResponse.Body()))
	})

	t.Run("the request return an error", func(t *testing.T) {
		core, client, req := newCore(), AcquireClient(), AcquireRequest()
		core.host.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.RawRequest.SetRequestURI("http://example.com/return-error")

		core.ctx = context.Background()
		core.client = client
		core.req = req

		resp, err := core.execFunc()

		require.NoError(t, err)
		require.Equal(t, 500, resp.RawResponse.StatusCode())
		require.Equal(t, "the request is error", string(resp.RawResponse.Body()))
	})

	t.Run("the request timeout", func(t *testing.T) {
		core, client, req := newCore(), AcquireClient(), AcquireRequest()

		core.host.Dial = func(addr string) (net.Conn, error) { return ln.Dial() }
		req.RawRequest.SetRequestURI("http://example.com/hang-up")

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		core.ctx = ctx
		core.client = client
		core.req = req

		_, err := core.execFunc()

		require.Equal(t, ErrTimeoutOrCancel, err)
	})
}

func Test_Execute(t *testing.T) {
	t.Parallel()

	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New()

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
		core, client, req := newCore(), AcquireClient(), AcquireRequest()
		client.AddRequestHook(func(c *Client, r *Request) error {
			require.Equal(t, "http://example.com", req.URL())
			return nil
		})
		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com")

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "Cannot GET /", string(resp.RawResponse.Body()))
	})

	t.Run("add user response hooks", func(t *testing.T) {
		core, client, req := newCore(), AcquireClient(), AcquireRequest()
		client.AddResponseHook(func(c *Client, resp *Response, req *Request) error {
			require.Equal(t, "http://example.com", req.URL())
			return nil
		})
		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com")

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "Cannot GET /", string(resp.RawResponse.Body()))
	})

	t.Run("no timeout", func(t *testing.T) {
		core, client, req := newCore(), AcquireClient(), AcquireRequest()

		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up")

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "example.com hang up", string(resp.RawResponse.Body()))
	})

	t.Run("client timeout", func(t *testing.T) {
		core, client, req := newCore(), AcquireClient(), AcquireRequest()
		client.SetTimeout(500 * time.Millisecond)
		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up")

		_, err := core.execute(context.Background(), client, req)
		require.Equal(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout", func(t *testing.T) {
		core, client, req := newCore(), AcquireClient(), AcquireRequest()

		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up").
			SetTimeout(300 * time.Millisecond)

		_, err := core.execute(context.Background(), client, req)
		require.Equal(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout has higher level", func(t *testing.T) {
		core, client, req := newCore(), AcquireClient(), AcquireRequest()
		client.SetTimeout(30 * time.Millisecond)

		req.SetDial(func(addr string) (net.Conn, error) {
			return ln.Dial()
		}).SetURL("http://example.com/hang-up").
			SetTimeout(3000 * time.Millisecond)

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "example.com hang up", string(resp.RawResponse.Body()))
	})
}
