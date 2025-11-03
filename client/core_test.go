package client

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"

	"github.com/gofiber/fiber/v3"
)

func Test_AddMissing_Port(t *testing.T) {
	t.Parallel()

	type args struct {
		addr  string
		isTLS bool
	}
	tests := []struct {
		name string
		want string
		args args
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
			t.Parallel()
			require.Equal(t, tt.want, addMissingPort(tt.args.addr, tt.args.isTLS))
		})
	}
}

func Test_Exec_Func(t *testing.T) {
	t.Parallel()
	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New()

	app.Get("/normal", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	app.Get("/return-error", func(_ fiber.Ctx) error {
		return errors.New("the request is error")
	})

	app.Get("/redirect", func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusFound).To("/normal")
	})

	app.Get("/hang-up", func(c fiber.Ctx) error {
		time.Sleep(time.Second)
		return c.SendString(c.Hostname() + " hang up")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}))
	}()

	time.Sleep(300 * time.Millisecond)

	t.Run("normal request", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		core.ctx = context.Background()
		core.client = client
		core.req = req

		client.SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })
		req.RawRequest.SetRequestURI("http://example.com/normal")

		resp, err := core.execFunc()
		require.NoError(t, err)
		require.Equal(t, 200, resp.RawResponse.StatusCode())
		require.Equal(t, "example.com", string(resp.RawResponse.Body()))
	})

	t.Run("follow redirect with retry config", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		core.ctx = context.Background()
		core.client = client
		core.req = req

		client.SetRetryConfig(&RetryConfig{MaxRetryCount: 1})
		client.SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })
		req.SetMaxRedirects(1)
		req.RawRequest.Header.SetMethod(fiber.MethodGet)
		req.RawRequest.SetRequestURI("http://example.com/redirect")

		resp, err := core.execFunc()
		require.NoError(t, err)
		require.Equal(t, 200, resp.RawResponse.StatusCode())
		require.Equal(t, "example.com", string(resp.RawResponse.Body()))
	})

	t.Run("the request return an error", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		core.ctx = context.Background()
		core.client = client
		core.req = req

		client.SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })
		req.RawRequest.SetRequestURI("http://example.com/return-error")

		resp, err := core.execFunc()

		require.NoError(t, err)
		require.Equal(t, 500, resp.RawResponse.StatusCode())
		require.Equal(t, "the request is error", string(resp.RawResponse.Body()))
	})

	t.Run("the request timeout", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		core.ctx = ctx
		core.client = client
		core.req = req

		client.SetDial(func(_ string) (net.Conn, error) { return ln.Dial() })
		req.RawRequest.SetRequestURI("http://example.com/hang-up")

		_, err := core.execFunc()

		require.Equal(t, ErrTimeoutOrCancel, err)
	})

	t.Run("cancel drains errChan", func(t *testing.T) {
		core, client, req := newCore(), New(), AcquireRequest()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		core.ctx = ctx
		core.client = client
		core.req = req

		req.RawRequest.SetRequestURI("http://example.com/drain-err")

		blockingTransport := newBlockingErrTransport(errors.New("upstream failure"))
		client.transport = blockingTransport
		defer blockingTransport.release()

		type execResult struct {
			resp *Response
			err  error
		}

		resultCh := make(chan execResult, 1)
		go func() {
			resp, err := core.execFunc()
			resultCh <- execResult{resp: resp, err: err}
		}()

		select {
		case <-blockingTransport.called:
		case <-time.After(time.Second):
			t.Fatal("transport Do was not invoked")
		}

		cancel()

		var result execResult
		select {
		case result = <-resultCh:
		case <-time.After(time.Second):
			t.Fatal("execFunc did not return")
		}

		require.Nil(t, result.resp)
		require.ErrorIs(t, result.err, ErrTimeoutOrCancel)

		blockingTransport.release()

		select {
		case <-blockingTransport.finished:
		case <-time.After(time.Second):
			t.Fatal("transport Do did not finish")
		}
	})
}

func Test_Execute(t *testing.T) {
	t.Parallel()
	ln := fasthttputil.NewInmemoryListener()
	app := fiber.New()

	app.Get("/normal", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	app.Get("/return-error", func(_ fiber.Ctx) error {
		return errors.New("the request is error")
	})

	app.Get("/hang-up", func(c fiber.Ctx) error {
		time.Sleep(time.Second)
		return c.SendString(c.Hostname() + " hang up")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}))
	}()

	t.Run("add user request hooks", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		client.AddRequestHook(func(_ *Client, _ *Request) error {
			require.Equal(t, "http://example.com", req.URL())
			return nil
		})
		client.SetDial(func(_ string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com")

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "Not Found", string(resp.RawResponse.Body()))
	})

	t.Run("add user response hooks", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		client.AddResponseHook(func(_ *Client, _ *Response, req *Request) error {
			require.Equal(t, "http://example.com", req.URL())
			return nil
		})
		client.SetDial(func(_ string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com")

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "Not Found", string(resp.RawResponse.Body()))
	})

	t.Run("no timeout", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()

		client.SetDial(func(_ string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com/hang-up")

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "example.com hang up", string(resp.RawResponse.Body()))
	})

	t.Run("client timeout", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		client.SetTimeout(500 * time.Millisecond)
		client.SetDial(func(_ string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com/hang-up")

		_, err := core.execute(context.Background(), client, req)
		require.Equal(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()

		client.SetDial(func(_ string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com/hang-up").
			SetTimeout(300 * time.Millisecond)

		_, err := core.execute(context.Background(), client, req)
		require.Equal(t, ErrTimeoutOrCancel, err)
	})

	t.Run("request timeout has higher level", func(t *testing.T) {
		t.Parallel()
		core, client, req := newCore(), New(), AcquireRequest()
		client.SetTimeout(30 * time.Millisecond)

		client.SetDial(func(_ string) (net.Conn, error) {
			return ln.Dial()
		})
		req.SetURL("http://example.com/hang-up").
			SetTimeout(3000 * time.Millisecond)

		resp, err := core.execute(context.Background(), client, req)
		require.NoError(t, err)
		require.Equal(t, "example.com hang up", string(resp.RawResponse.Body()))
	})
}

type blockingErrTransport struct {
	err error

	called   chan struct{}
	unblock  chan struct{}
	finished chan struct{}

	calledOnce   sync.Once
	releaseOnce  sync.Once
	finishedOnce sync.Once
}

func newBlockingErrTransport(err error) *blockingErrTransport {
	return &blockingErrTransport{
		err:      err,
		called:   make(chan struct{}),
		unblock:  make(chan struct{}),
		finished: make(chan struct{}),
	}
}

func (b *blockingErrTransport) Do(_ *fasthttp.Request, _ *fasthttp.Response) error {
	b.calledOnce.Do(func() { close(b.called) })
	<-b.unblock
	b.finishedOnce.Do(func() { close(b.finished) })
	return b.err
}

func (b *blockingErrTransport) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, _ time.Duration) error {
	return b.Do(req, resp)
}

func (b *blockingErrTransport) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, _ time.Time) error {
	return b.Do(req, resp)
}

func (b *blockingErrTransport) DoRedirects(req *fasthttp.Request, resp *fasthttp.Response, _ int) error {
	return b.Do(req, resp)
}

func (*blockingErrTransport) CloseIdleConnections() {
}

func (*blockingErrTransport) TLSConfig() *tls.Config {
	return nil
}

func (*blockingErrTransport) SetTLSConfig(_ *tls.Config) {
}

func (*blockingErrTransport) SetDial(_ fasthttp.DialFunc) {
}

func (*blockingErrTransport) Client() any {
	return nil
}

func (*blockingErrTransport) StreamResponseBody() bool {
	return false
}

func (*blockingErrTransport) SetStreamResponseBody(_ bool) {
}

func (b *blockingErrTransport) release() {
	b.releaseOnce.Do(func() { close(b.unblock) })
}
