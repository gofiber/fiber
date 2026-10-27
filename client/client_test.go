package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	stdlog "log" //nolint:depguard // log.DefaultLogger is keyed by the standard library logger type the default logger exposes.
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/addon/retry"
	"github.com/gofiber/fiber/v3/internal/crosshost"
	"github.com/gofiber/fiber/v3/internal/tlstest"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func startTestServerWithPort(t *testing.T, beforeStarting func(app *fiber.App)) (app *fiber.App, addr string) { //nolint:nonamedreturns // gocritic unnamedResult requires explicit result names for clarity when returning app and address
	t.Helper()

	app = fiber.New()

	if beforeStarting != nil {
		beforeStarting(app)
	}

	addrChan := make(chan string)
	errChan := make(chan error, 1)
	go func(server *fiber.App) {
		err := server.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				addrChan <- addr.String()
			},
		})
		if err != nil {
			errChan <- err
		}
	}(app)

	select {
	case addr = <-addrChan:
		return app, addr
	case err := <-errChan:
		t.Fatalf("Failed to start test server: %v", err)
	}

	return nil, ""
}

func Test_New_With_Client(t *testing.T) {
	t.Parallel()

	t.Run("with valid client", func(t *testing.T) {
		t.Parallel()

		c := &fasthttp.Client{
			MaxConnsPerHost: 5,
		}
		client := NewWithClient(c)

		require.NotNil(t, client)
	})

	t.Run("with nil client", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "fasthttp.Client must not be nil", func() {
			NewWithClient(nil)
		})
	})
}

func Test_New_With_HostClient(t *testing.T) {
	t.Parallel()

	t.Run("with valid host client", func(t *testing.T) {
		t.Parallel()

		hc := &fasthttp.HostClient{Addr: "example.com:80"}
		client := NewWithHostClient(hc)

		require.NotNil(t, client)
		require.Equal(t, hc, client.HostClient())
		require.Nil(t, client.FasthttpClient())
	})

	t.Run("with nil host client", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "fasthttp.HostClient must not be nil", func() {
			NewWithHostClient(nil)
		})
	})
}

func Test_New_With_LBClient(t *testing.T) {
	t.Parallel()

	t.Run("with valid lb client", func(t *testing.T) {
		t.Parallel()

		lb := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{
				&fasthttp.HostClient{Addr: "example.com:80"},
			},
		}

		client := NewWithLBClient(lb)

		require.NotNil(t, client)
		require.Equal(t, lb, client.LBClient())
		require.Nil(t, client.FasthttpClient())
		require.Nil(t, client.HostClient())
	})

	t.Run("with nil lb client", func(t *testing.T) {
		t.Parallel()

		require.PanicsWithValue(t, "fasthttp.LBClient must not be nil", func() {
			NewWithLBClient(nil)
		})
	})
}

func TestClientUnderlyingTransports(t *testing.T) {
	t.Parallel()

	std := New()
	require.NotNil(t, std.FasthttpClient())
	require.Nil(t, std.HostClient())
	require.Nil(t, std.LBClient())

	hostTransport := &fasthttp.HostClient{Addr: "example.com:80"}
	host := NewWithHostClient(hostTransport)
	require.Nil(t, host.FasthttpClient())
	require.Same(t, hostTransport, host.HostClient())
	require.Nil(t, host.LBClient())

	lbClient := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{hostTransport}}
	lb := NewWithLBClient(lbClient)
	require.Nil(t, lb.FasthttpClient())
	require.Nil(t, lb.HostClient())
	require.Same(t, lbClient, lb.LBClient())
}

func TestClientCBORUnmarshalOverride(t *testing.T) {
	t.Parallel()

	client := New()
	initial := client.CBORUnmarshal()
	require.NotNil(t, initial)

	called := false
	custom := func(b []byte, v any) error {
		_ = b
		_ = v
		called = true
		return nil
	}

	client.SetCBORUnmarshal(custom)
	fn := client.CBORUnmarshal()
	require.NotNil(t, fn)

	var payload []byte
	require.NoError(t, fn(payload, nil))
	require.True(t, called)
}

func TestClientSetRootCertificateErrors(t *testing.T) {
	t.Parallel()

	client := New()
	require.Panics(t, func() {
		client.SetRootCertificate("does-not-exist.pem")
	})

	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "invalid.pem")
	require.NoError(t, os.WriteFile(badPath, []byte("not a pem"), 0o600))

	require.Panics(t, func() {
		client.SetRootCertificate(badPath)
	})
}

func TestClientSetRootCertificateFromStringError(t *testing.T) {
	t.Parallel()

	client := New()
	require.Panics(t, func() {
		client.SetRootCertificateFromString("invalid pem data")
	})
}

func TestClientLoggerAccessors(t *testing.T) {
	t.Parallel()

	client := New()
	_ = client.Logger()
	contextual := log.WithContext(context.Background())
	client.SetLogger(contextual)
	require.Equal(t, contextual, client.Logger())
}

// Test_Client_DefaultLogger covers the logger a client starts with. It used to
// ask for the default logger with Fiber's Logger interface as the type
// argument instead of the standard library's *log.Logger, which no logger
// implements, so every client began without one: Debug printed nothing and the
// error paths of SetRootCertificate dereferenced it.
func Test_Client_DefaultLogger(t *testing.T) {
	t.Parallel()

	clients := map[string]*Client{
		"New":               New(),
		"NewWithClient":     NewWithClient(&fasthttp.Client{}),
		"NewWithHostClient": NewWithHostClient(&fasthttp.HostClient{}),
		"NewWithLBClient":   NewWithLBClient(&fasthttp.LBClient{}),
	}
	for name, c := range clients {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.NotNil(t, c.Logger())
			require.Equal(t, log.DefaultLogger[*stdlog.Logger](), c.Logger())
		})
	}
}

// recoverValue runs fn and returns what it panicked with, nil when it did not.
func recoverValue(fn func()) any {
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		fn()
	}()
	return recovered
}

// Test_Client_SetRootCertificate_InvalidInput pins the documented panic for a
// certificate that cannot be loaded. The panic has to say why: a client with no
// logger turned it into a nil pointer dereference that named nothing.
func Test_Client_SetRootCertificate_InvalidInput(t *testing.T) {
	t.Parallel()

	noCertificate := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(noCertificate, []byte("not a pem"), 0o600))
	missing := filepath.Join(t.TempDir(), "missing.pem")

	tests := []struct {
		call func(c *Client)
		name string
		want string
	}{
		{name: "string without a certificate", want: ErrFailedToAppendCert.Error(), call: func(c *Client) {
			c.SetRootCertificateFromString("not a pem")
		}},
		{name: "empty string", want: ErrFailedToAppendCert.Error(), call: func(c *Client) {
			c.SetRootCertificateFromString("")
		}},
		{name: "file without a certificate", want: ErrFailedToAppendCert.Error(), call: func(c *Client) {
			c.SetRootCertificate(noCertificate)
		}},
		{name: "missing file", want: "client: open " + missing, call: func(c *Client) {
			c.SetRootCertificate(missing)
		}},
	}

	for _, tc := range tests {
		for loggerName, setup := range map[string]func(*Client){
			"default logger": func(*Client) {},
			"no logger":      func(c *Client) { c.SetLogger(nil) },
		} {
			t.Run(tc.name+"/"+loggerName, func(t *testing.T) {
				t.Parallel()

				c := New()
				setup(c)

				recovered := recoverValue(func() { tc.call(c) })
				require.NotNil(t, recovered, "an unusable certificate must not be ignored")

				var runtimeErr runtime.Error
				if err, ok := recovered.(error); ok {
					require.NotErrorAs(t, err, &runtimeErr, "got a runtime panic")
				}
				require.Contains(t, fmt.Sprint(recovered), "client: ")
				require.Contains(t, fmt.Sprint(recovered), tc.want)
			})
		}
	}
}

func TestClient_RedirectSensitiveHeaders(t *testing.T) {
	t.Parallel()

	t.Run("the list", func(t *testing.T) {
		t.Parallel()

		client := New()
		requireNameList(t, func(names ...string) { client.SetRedirectSensitiveHeaders(names...) }, client.RedirectSensitiveHeaders)
		require.Same(t, client, client.SetRedirectSensitiveHeaders())
	})

	t.Run("the options for a request carry the client's and the request's names together", func(t *testing.T) {
		t.Parallel()

		client := New().SetRedirectSensitiveHeaders("X-One")
		own := New().R().SetRedirectSensitiveHeaders("X-Two")

		opts := client.redirectOptionsFor(3, own.redirectSensitiveHeaders)
		require.Equal(t, 3, opts.maxRedirects)
		require.ElementsMatch(t, []string{"X-One", "X-Two"}, opts.sensitiveHeaders.List())

		again := client.redirectOptionsFor(3, own.redirectSensitiveHeaders)
		require.ElementsMatch(t, []string{"X-One", "X-Two"}, again.sensitiveHeaders.List())
		require.Equal(t, []string{"X-One"}, client.RedirectSensitiveHeaders())
		require.Equal(t, []string{"X-Two"}, own.RedirectSensitiveHeaders())

		require.Nil(t, New().redirectOptionsFor(3, crosshost.HeaderNames{}).sensitiveHeaders.List())
	})
}

func TestClientResetClearsState(t *testing.T) {
	t.Parallel()

	client := New()

	jar := AcquireCookieJar()
	jar.hostCookies = map[string][]storedCookie{"example.com": {}}
	client.SetCookieJar(jar)

	client.SetBaseURL("http://example.com")
	client.SetTimeout(2 * time.Second)
	client.SetUserAgent("reset-agent")
	client.SetReferer("reset-ref")
	client.SetRetryConfig(&RetryConfig{MaxRetryCount: 3})
	client.Debug()
	client.SetDisablePathNormalizing(true)
	client.SetRedirectSensitiveHeaders("X-Api-Key")
	client.SetHeaders(map[string]string{"X-Test": "value"})
	client.SetParams(map[string]string{"p": "1"})
	client.SetCookies(map[string]string{"cookie": "value"})
	client.SetPathParams(map[string]string{"id": "123"})

	client.Reset()

	require.NotNil(t, client.FasthttpClient())
	require.Nil(t, client.HostClient())
	require.Nil(t, client.LBClient())
	require.Empty(t, client.BaseURL())
	require.Zero(t, client.timeout)
	require.Empty(t, client.userAgent)
	require.Empty(t, client.referer)
	require.Nil(t, client.retryConfig)
	require.False(t, client.isDebug)
	require.False(t, client.isPathNormalizingDisabled)
	require.Nil(t, client.RedirectSensitiveHeaders())
	require.Nil(t, client.cookieJar)
	require.Nil(t, jar.hostCookies)
	require.Empty(t, *client.path)
	require.Empty(t, *client.cookies)
	require.Equal(t, 0, client.header.Len())
	require.Equal(t, 0, client.params.Len())
}

func Test_Client_Add_Hook(t *testing.T) {
	t.Parallel()

	t.Run("add request hooks", func(t *testing.T) {
		t.Parallel()

		buf := bytebufferpool.Get()
		defer bytebufferpool.Put(buf)

		client := New().AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook1")
			return nil
		})

		require.Len(t, client.RequestHook(), 1)

		client.AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook2")
			return nil
		}, func(_ *Client, _ *Request) error {
			buf.WriteString("hook3")
			return nil
		})

		require.Len(t, client.RequestHook(), 3)
	})

	t.Run("add final request hooks", func(t *testing.T) {
		t.Parallel()

		client := New().AddFinalRequestHook(func(_ *Client, _ *Request) error {
			return nil
		})

		require.Len(t, client.FinalRequestHook(), 1)
	})

	t.Run("add response hooks", func(t *testing.T) {
		t.Parallel()
		client := New().AddResponseHook(func(_ *Client, _ *Response, _ *Request) error {
			return nil
		})

		require.Len(t, client.ResponseHook(), 1)

		hook1 := func(_ *Client, _ *Response, _ *Request) error {
			return nil
		}
		hook2 := func(_ *Client, _ *Response, _ *Request) error {
			return nil
		}
		client.AddResponseHook(hook1, hook2)

		require.Len(t, client.ResponseHook(), 3)
	})
}

func Test_Client_HostClient_Behavior(t *testing.T) {
	t.Parallel()

	t.Run("do and redirects", func(t *testing.T) {
		t.Parallel()

		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/ok", func(c fiber.Ctx) error {
				return c.SendString("ok")
			})
			app.Get("/redirect", func(c fiber.Ctx) error {
				return c.Redirect().Status(fiber.StatusFound).To("/ok")
			})
		})
		t.Cleanup(func() {
			require.NoError(t, app.Shutdown())
		})

		client := NewWithHostClient(&fasthttp.HostClient{Addr: addr})

		resp, err := client.Get("http://" + addr + "/ok")
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "ok", resp.String())

		resp, err = client.Get("http://"+addr+"/redirect", Config{MaxRedirects: 1})
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "ok", resp.String())
	})

	t.Run("retries respect dial overrides", func(t *testing.T) {
		t.Parallel()

		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("retry")
			})
		})
		t.Cleanup(func() {
			require.NoError(t, app.Shutdown())
		})

		client := NewWithHostClient(&fasthttp.HostClient{Addr: addr})
		client.SetRetryConfig(&RetryConfig{
			InitialInterval: time.Millisecond,
			MaxRetryCount:   2,
		})

		var attempts atomic.Int32
		client.SetDial(func(address string) (net.Conn, error) {
			if attempts.Add(1) == 1 {
				return nil, errors.New("dial failure")
			}
			return fasthttp.Dial(address)
		})

		resp, err := client.Get("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "retry", resp.String())
		require.EqualValues(t, 2, attempts.Load())
	})

	t.Run("tls configuration propagates", func(t *testing.T) {
		t.Parallel()

		serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
		require.NoError(t, err)

		ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
		require.NoError(t, err)
		ln = tls.NewListener(ln, serverTLSConf)

		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("tls-host")
		})

		go func() {
			assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
				DisableStartupMessage: true,
			}))
		}()
		time.Sleep(1 * time.Second)

		client := NewWithHostClient(&fasthttp.HostClient{Addr: ln.Addr().String(), IsTLS: true})

		resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + ln.Addr().String())
		require.NoError(t, err)
		cfg := client.TLSConfig()
		require.Same(t, clientTLSConf, cfg)
		require.NotNil(t, cfg.RootCAs)
		require.Equal(t, clientTLSConf, client.HostClient().TLSConfig)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "tls-host", resp.String())

		require.NoError(t, app.Shutdown())
	})

	t.Run("proxy overrides and reset", func(t *testing.T) {
		t.Parallel()

		client := NewWithHostClient(&fasthttp.HostClient{Addr: "example.com:80"})

		require.NoError(t, client.SetProxyURL("http://127.0.0.1:8080"))
		require.NotNil(t, client.HostClient().Dial)

		var called atomic.Int32
		customDial := func(addr string) (net.Conn, error) {
			_ = addr
			called.Add(1)
			return nil, errors.New("dial")
		}
		client.SetDial(customDial)

		_, err := client.HostClient().Dial("example.com:80")
		require.Error(t, err)
		require.EqualValues(t, 1, called.Load())

		client.Reset()
		require.NotNil(t, client.FasthttpClient())
		require.Nil(t, client.HostClient())
		require.Nil(t, client.LBClient())
	})

	t.Run("timeouts and close idle", func(t *testing.T) {
		t.Parallel()

		client := NewWithHostClient(&fasthttp.HostClient{Addr: "example.com:80"})

		var dialCalls atomic.Int32
		dialErr := errors.New("dial failed")
		client.HostClient().Dial = func(addr string) (net.Conn, error) {
			_ = addr
			dialCalls.Add(1)
			return nil, dialErr
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		req.SetRequestURI("http://example.com/")

		err := client.DoTimeout(req, resp, 5*time.Millisecond)
		require.ErrorIs(t, err, dialErr)

		req.SetRequestURI("http://example.com/")
		err = client.DoDeadline(req, resp, time.Now().Add(5*time.Millisecond))
		require.ErrorIs(t, err, dialErr)

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)

		require.EqualValues(t, 2, dialCalls.Load())
		require.NotPanics(t, client.CloseIdleConnections)
	})
}

func Test_Client_LBClient_Behavior(t *testing.T) {
	t.Parallel()

	newLBClient := func(addr string) *fasthttp.LBClient {
		return &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{
				&fasthttp.HostClient{Addr: addr},
			},
		}
	}

	t.Run("do and redirects", func(t *testing.T) {
		t.Parallel()

		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/ok", func(c fiber.Ctx) error {
				return c.SendString("ok")
			})
			app.Get("/redirect", func(c fiber.Ctx) error {
				return c.Redirect().Status(fiber.StatusFound).To("/ok")
			})
		})
		t.Cleanup(func() {
			require.NoError(t, app.Shutdown())
		})

		client := NewWithLBClient(newLBClient(addr))

		resp, err := client.Get("http://" + addr + "/ok")
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "ok", resp.String())

		resp, err = client.Get("http://"+addr+"/redirect", Config{MaxRedirects: 1})
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "ok", resp.String())
	})

	t.Run("retries respect dial overrides", func(t *testing.T) {
		t.Parallel()

		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString("retry")
			})
		})
		t.Cleanup(func() {
			require.NoError(t, app.Shutdown())
		})

		client := NewWithLBClient(newLBClient(addr))
		client.SetRetryConfig(&RetryConfig{
			InitialInterval: time.Millisecond,
			MaxRetryCount:   2,
		})

		var attempts atomic.Int32
		client.SetDial(func(address string) (net.Conn, error) {
			if attempts.Add(1) == 1 {
				return nil, errors.New("dial failure")
			}
			return fasthttp.Dial(address)
		})

		resp, err := client.Get("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "retry", resp.String())
		require.EqualValues(t, 2, attempts.Load())
	})

	t.Run("tls configuration propagates", func(t *testing.T) {
		t.Parallel()

		serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
		require.NoError(t, err)

		ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
		require.NoError(t, err)
		ln = tls.NewListener(ln, serverTLSConf)

		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("tls-lb")
		})

		go func() {
			assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
				DisableStartupMessage: true,
			}))
		}()
		time.Sleep(1 * time.Second)

		lb := newLBClient(ln.Addr().String())
		host, ok := lb.Clients[0].(*fasthttp.HostClient)
		require.True(t, ok)
		host.IsTLS = true
		client := NewWithLBClient(lb)

		resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + ln.Addr().String())
		require.NoError(t, err)
		cfg := client.TLSConfig()
		require.Same(t, clientTLSConf, cfg)
		require.NotNil(t, cfg.RootCAs)

		hc, ok := client.LBClient().Clients[0].(*fasthttp.HostClient)
		require.True(t, ok)
		require.Equal(t, clientTLSConf, hc.TLSConfig)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "tls-lb", resp.String())

		require.NoError(t, app.Shutdown())
	})

	t.Run("proxy overrides and reset", func(t *testing.T) {
		t.Parallel()

		client := NewWithLBClient(&fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{
				&fasthttp.HostClient{Addr: "example.com:80"},
			},
		})

		require.NoError(t, client.SetProxyURL("http://127.0.0.1:8080"))

		hc, ok := client.LBClient().Clients[0].(*fasthttp.HostClient)
		require.True(t, ok)
		require.NotNil(t, hc.Dial)

		var called atomic.Int32
		customDial := func(addr string) (net.Conn, error) {
			_ = addr
			called.Add(1)
			return nil, errors.New("dial")
		}
		client.SetDial(customDial)

		_, err := hc.Dial("example.com:80")
		require.Error(t, err)
		require.EqualValues(t, 1, called.Load())

		client.Reset()
		require.NotNil(t, client.FasthttpClient())
		require.Nil(t, client.LBClient())
		require.Nil(t, client.HostClient())
	})

	t.Run("timeouts and close idle", func(t *testing.T) {
		t.Parallel()

		client := NewWithLBClient(&fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{
				&fasthttp.HostClient{Addr: "example.com:80"},
				&fasthttp.HostClient{Addr: "example.org:80"},
			},
		})

		var dialCalls atomic.Int32
		dialErr := errors.New("dial failed")
		for _, bc := range client.LBClient().Clients {
			if hc, ok := bc.(*fasthttp.HostClient); ok {
				hc.Dial = func(addr string) (net.Conn, error) {
					_ = addr
					dialCalls.Add(1)
					return nil, dialErr
				}
			}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		req.SetRequestURI("http://example.com/")

		err := client.DoTimeout(req, resp, 5*time.Millisecond)
		require.ErrorIs(t, err, dialErr)

		req.SetRequestURI("http://example.com/")
		err = client.DoDeadline(req, resp, time.Now().Add(5*time.Millisecond))
		require.ErrorIs(t, err, dialErr)

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)

		require.GreaterOrEqual(t, dialCalls.Load(), int32(2))
		require.NotPanics(t, client.CloseIdleConnections)
	})
}

func Test_Client_Add_Hook_CheckOrder(t *testing.T) {
	t.Parallel()

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	client := New().
		AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook1")
			return nil
		}).
		AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook2")
			return nil
		}).
		AddRequestHook(func(_ *Client, _ *Request) error {
			buf.WriteString("hook3")
			return nil
		})

	for _, hook := range client.RequestHook() {
		require.NoError(t, hook(client, &Request{}))
	}

	require.Equal(t, "hook1hook2hook3", buf.String())
}

func Test_Client_Marshal(t *testing.T) {
	t.Parallel()

	t.Run("set json marshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetJSONMarshal(func(_ any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.JSONMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set json marshal error", func(t *testing.T) {
		t.Parallel()

		emptyErr := errors.New("empty json")
		client := New().
			SetJSONMarshal(func(_ any) ([]byte, error) {
				return nil, emptyErr
			})

		val, err := client.JSONMarshal()(nil)
		require.Nil(t, val)
		require.ErrorIs(t, err, emptyErr)
	})

	t.Run("set json unmarshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetJSONUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty json")
			})

		err := client.JSONUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty json"), err)
	})

	t.Run("set json unmarshal error", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetJSONUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty json")
			})

		err := client.JSONUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty json"), err)
	})

	t.Run("set xml marshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLMarshal(func(_ any) ([]byte, error) {
				return []byte("hello"), nil
			})
		val, err := client.XMLMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("set xml marshal error", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLMarshal(func(_ any) ([]byte, error) {
				return nil, errors.New("empty xml")
			})

		val, err := client.XMLMarshal()(nil)
		require.Nil(t, val)
		require.Equal(t, errors.New("empty xml"), err)
	})

	t.Run("set cbor marshal", func(t *testing.T) {
		t.Parallel()
		bs, err := hex.DecodeString("f6")
		if err != nil {
			t.Error(err)
		}
		client := New().
			SetCBORMarshal(func(_ any) ([]byte, error) {
				return bs, nil
			})
		val, err := client.CBORMarshal()(nil)

		require.NoError(t, err)
		require.Equal(t, bs, val)
	})

	t.Run("set cbor marshal error", func(t *testing.T) {
		t.Parallel()
		client := New().SetCBORMarshal(func(_ any) ([]byte, error) {
			return nil, errors.New("invalid struct")
		})

		val, err := client.CBORMarshal()(nil)
		require.Nil(t, val)
		require.Equal(t, errors.New("invalid struct"), err)
	})

	t.Run("set xml unmarshal", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty xml")
			})

		err := client.XMLUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty xml"), err)
	})

	t.Run("set xml unmarshal error", func(t *testing.T) {
		t.Parallel()
		client := New().
			SetXMLUnmarshal(func(_ []byte, _ any) error {
				return errors.New("empty xml")
			})

		err := client.XMLUnmarshal()(nil, nil)
		require.Equal(t, errors.New("empty xml"), err)
	})
}

func Test_Client_SetBaseURL(t *testing.T) {
	t.Parallel()

	client := New().SetBaseURL("http://example.com")

	require.Equal(t, "http://example.com", client.BaseURL())
}

func Test_Client_Invalid_URL(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname())
	})

	go start()

	_, err := New().SetDial(dial).
		R().
		Get("http//example")

	require.ErrorIs(t, err, ErrURLFormat)
}

func Test_Client_Unsupported_Protocol(t *testing.T) {
	t.Parallel()

	_, err := New().
		R().
		Get("ftp://example.com")

	require.ErrorIs(t, err, ErrURLFormat)
}

func Test_Client_ConcurrencyRequests(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.All("/", func(c fiber.Ctx) error {
		return c.SendString(c.Hostname() + " " + c.Method())
	})
	go start()

	client := New().SetDial(dial)

	wg := sync.WaitGroup{}
	for range 5 {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
			m := method
			wg.Go(func() {
				resp, err := client.Custom("http://example.com", m)
				require.NoError(t, err)
				assert.Equal(t, "example.com "+m, utils.UnsafeString(resp.RawResponse.Body()))
			})
		}
	}

	wg.Wait()
}

func Test_Get(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.SendString(c.Hostname())
			})
		})

		return app, addr
	}

	t.Run("global get function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := Get("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "0.0.0.0", utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client get", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := New().Get("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "0.0.0.0", utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Head(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Head("/", func(c fiber.Ctx) error {
				return c.SendString(c.Hostname())
			})
		})

		return app, addr
	}

	t.Run("global head function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := Head("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "7", resp.Header(fiber.HeaderContentLength))
		require.Empty(t, utils.UnsafeString(resp.RawResponse.Body()))
	})

	t.Run("client head", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		resp, err := New().Head("http://" + addr)
		require.NoError(t, err)
		require.Equal(t, "7", resp.Header(fiber.HeaderContentLength))
		require.Empty(t, utils.UnsafeString(resp.RawResponse.Body()))
	})
}

func Test_Post(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Post("/", func(c fiber.Ctx) error {
				return c.Status(fiber.StatusCreated).
					SendString(c.FormValue("foo"))
			})
		})

		return app, addr
	}

	t.Run("global post function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := Post("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client post", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := New().Post("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusCreated, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Put(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Put("/", func(c fiber.Ctx) error {
				return c.SendString(c.FormValue("foo"))
			})
		})

		return app, addr
	}

	t.Run("global put function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := Put("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client put", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := New().Put("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Delete(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Delete("/", func(c fiber.Ctx) error {
				return c.Status(fiber.StatusNoContent).
					SendString("deleted")
			})
		})

		return app, addr
	}

	t.Run("global delete function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		time.Sleep(1 * time.Second)

		for range 5 {
			resp, err := Delete("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Empty(t, resp.String())
		}
	})

	t.Run("client delete", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := New().Delete("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Empty(t, resp.String())
		}
	})
}

func Test_Options(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Options("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderAllow, "GET, POST, PUT, DELETE, PATCH")
				return c.Status(fiber.StatusNoContent).SendString("")
			})
		})

		return app, addr
	}

	t.Run("global options function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := Options("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, "GET, POST, PUT, DELETE, PATCH", resp.Header(fiber.HeaderAllow))
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Empty(t, resp.String())
		}
	})

	t.Run("client options", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := New().Options("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, "GET, POST, PUT, DELETE, PATCH", resp.Header(fiber.HeaderAllow))
			require.Equal(t, fiber.StatusNoContent, resp.StatusCode())
			require.Empty(t, resp.String())
		}
	})
}

func Test_Patch(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Patch("/", func(c fiber.Ctx) error {
				return c.SendString(c.FormValue("foo"))
			})
		})

		return app, addr
	}

	t.Run("global patch function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		time.Sleep(1 * time.Second)

		for range 5 {
			resp, err := Patch("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})

	t.Run("client patch", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := New().Patch("http://"+addr, Config{
				FormData: map[string]string{
					"foo": "bar",
				},
			})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "bar", resp.String())
		}
	})
}

func Test_Query(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Query("/", func(c fiber.Ctx) error {
				return c.Send(c.Body())
			})
		})

		return app, addr
	}

	t.Run("global query function", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		time.Sleep(1 * time.Second)

		for range 5 {
			resp, err := Query("http://"+addr, Config{Body: "query body"})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "query body", resp.String())
		}
	})

	t.Run("client query", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := New().Query("http://"+addr, Config{Body: "query body"})

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "query body", resp.String())
		}
	})
}

func Test_Client_UserAgent(t *testing.T) {
	t.Parallel()

	setupApp := func() (*fiber.App, string) {
		app, addr := startTestServerWithPort(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error {
				return c.Send(c.Request().Header.UserAgent())
			})
		})

		return app, addr
	}

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			resp, err := Get("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, defaultUserAgent, resp.String())
		}
	})

	t.Run("custom", func(t *testing.T) {
		t.Parallel()

		app, addr := setupApp()
		defer func() {
			require.NoError(t, app.Shutdown())
		}()

		for range 5 {
			c := New().
				SetUserAgent("ua")

			resp, err := c.Get("http://" + addr)

			require.NoError(t, err)
			require.Equal(t, fiber.StatusOK, resp.StatusCode())
			require.Equal(t, "ua", resp.String())
		}
	})
}

func Test_Client_Header(t *testing.T) {
	t.Parallel()

	t.Run("add header", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddHeader("foo", "bar").AddHeader("foo", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set header", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddHeader("foo", "bar").SetHeader("foo", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add headers", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.SetHeader("foo", "bar").
			AddHeaders(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Header("foo")
		require.Len(t, res, 3)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
		require.Equal(t, "buaa", res[2])

		res = req.Header("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.SetHeader("foo", "bar").
			SetHeaders(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Header("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])

		res = req.Header("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set header case-insensitive", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.SetHeader("foo", "bar").
			AddHeader("FOO", "fiber")

		res := req.Header("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})
}

func Test_Client_Header_With_Server(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		for key, value := range c.Request().Header.All() {
			if k := string(key); k == "K1" || k == "K2" {
				_, _ = c.Write(key)   //nolint:errcheck // It is fine to ignore the error here
				_, _ = c.Write(value) //nolint:errcheck // It is fine to ignore the error here
			}
		}
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
	t.Parallel()

	t.Run("set cookie", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetCookie("foo", "bar")
		require.Equal(t, "bar", req.Cookie("foo"))

		req.SetCookie("foo", "bar1")
		require.Equal(t, "bar1", req.Cookie("foo"))
	})

	t.Run("set cookies", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetCookies(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))

		req.SetCookies(map[string]string{
			"foo": "bar1",
		})
		require.Equal(t, "bar1", req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))
	})

	t.Run("set cookies with struct", func(t *testing.T) {
		t.Parallel()
		type args struct {
			CookieString string `cookie:"string"`
			CookieInt    int    `cookie:"int"`
		}

		req := New().SetCookiesWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.Cookie("int"))
		require.Equal(t, "foo", req.Cookie("string"))
	})

	t.Run("del cookies", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetCookies(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))

		req.DelCookies("foo")
		require.Empty(t, req.Cookie("foo"))
		require.Equal(t, "foo", req.Cookie("bar"))
	})
}

func Test_Client_Cookie_With_Server(t *testing.T) {
	t.Parallel()

	handler := func(c fiber.Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3") + c.Cookies("k4"),
		)
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

func Test_Client_CookieJar(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		return c.SendString(
			c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"),
		)
	}

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)

	jar.SetKeyValue("example.com", "k1", "v1")
	jar.SetKeyValue("example.com", "k2", "v2")
	jar.SetKeyValue("example", "k3", "v3")

	wrapAgent := func(c *Client) {
		c.SetCookieJar(jar)
	}
	testClient(t, handler, wrapAgent, "v1v2")
}

func Test_Client_CookieJar_Response(t *testing.T) {
	t.Parallel()

	t.Run("without expiration", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:  "k4",
				Value: "v4",
			})
			return c.SendString(
				c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"),
			)
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")
		jar.SetKeyValue("example.com", "k2", "v2")
		jar.SetKeyValue("example", "k3", "v3")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1v2")

		require.Len(t, jar.getCookiesByHost("example.com"), 3)
	})

	t.Run("with expiration", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:    "k4",
				Value:   "v4",
				Expires: time.Now().Add(1 * time.Nanosecond),
			})
			return c.SendString(
				c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"),
			)
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")
		jar.SetKeyValue("example.com", "k2", "v2")
		jar.SetKeyValue("example", "k3", "v3")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1v2")

		require.Len(t, jar.getCookiesByHost("example.com"), 2)
	})

	t.Run("override cookie value", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{
				Name:  "k1",
				Value: "v2",
			})
			return c.SendString(
				c.Cookies("k1") + c.Cookies("k2"),
			)
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")
		jar.SetKeyValue("example.com", "k2", "v2")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1v2")

		for _, cookie := range jar.getCookiesByHost("example.com") {
			if string(cookie.Key()) == "k1" {
				require.Equal(t, "v2", string(cookie.Value()))
			}
		}
	})

	t.Run("different domain", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			return c.SendString(c.Cookies("k1"))
		}

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)

		jar.SetKeyValue("example.com", "k1", "v1")

		wrapAgent := func(c *Client) {
			c.SetCookieJar(jar)
		}
		testClient(t, handler, wrapAgent, "v1")

		require.Len(t, jar.getCookiesByHost("example.com"), 1)
		require.Empty(t, jar.getCookiesByHost("example"))
	})
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

func Test_Client_QueryParam(t *testing.T) {
	t.Parallel()

	t.Run("add param", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddParam("foo", "bar").AddParam("foo", "fiber")

		res := req.Param("foo")
		require.Len(t, res, 2)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
	})

	t.Run("set param", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.AddParam("foo", "bar").SetParam("foo", "fiber")

		res := req.Param("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])
	})

	t.Run("add params", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.SetParam("foo", "bar").
			AddParams(map[string][]string{
				"foo": {"fiber", "buaa"},
				"bar": {"foo"},
			})

		res := req.Param("foo")
		require.Len(t, res, 3)
		require.Equal(t, "bar", res[0])
		require.Equal(t, "fiber", res[1])
		require.Equal(t, "buaa", res[2])

		res = req.Param("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set headers", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			})

		res := req.Param("foo")
		require.Len(t, res, 1)
		require.Equal(t, "fiber", res[0])

		res = req.Param("bar")
		require.Len(t, res, 1)
		require.Equal(t, "foo", res[0])
	})

	t.Run("set params with struct", func(t *testing.T) {
		t.Parallel()

		type args struct {
			TString   string
			TSlice    []string
			TIntSlice []int `param:"int_slice"`
			TInt      int
			TFloat    float64
			TBool     bool
		}

		p := New()
		p.SetParamsWithStruct(&args{
			TInt:      5,
			TString:   "string",
			TFloat:    3.1,
			TBool:     true,
			TSlice:    []string{"foo", "bar"},
			TIntSlice: []int{1, 2},
		})

		require.Empty(t, p.Param("unexport"))

		require.Len(t, p.Param("TInt"), 1)
		require.Equal(t, "5", p.Param("TInt")[0])

		require.Len(t, p.Param("TString"), 1)
		require.Equal(t, "string", p.Param("TString")[0])

		require.Len(t, p.Param("TFloat"), 1)
		require.Equal(t, "3.1", p.Param("TFloat")[0])

		require.Len(t, p.Param("TBool"), 1)

		tslice := p.Param("TSlice")
		require.Len(t, tslice, 2)
		require.Equal(t, "foo", tslice[0])
		require.Equal(t, "bar", tslice[1])

		tint := p.Param("TSlice")
		require.Len(t, tint, 2)
		require.Equal(t, "foo", tint[0])
		require.Equal(t, "bar", tint[1])
	})

	t.Run("del params", func(t *testing.T) {
		t.Parallel()
		req := New()
		req.SetParam("foo", "bar").
			SetParams(map[string]string{
				"foo": "fiber",
				"bar": "foo",
			}).DelParams("foo", "bar")

		res := req.Param("foo")
		require.Empty(t, res)

		res = req.Param("bar")
		require.Empty(t, res)
	})
}

func Test_Client_QueryParam_With_Server(t *testing.T) {
	handler := func(c fiber.Ctx) error {
		_, _ = c.WriteString(c.Query("k1")) //nolint:errcheck // It is fine to ignore the error here
		_, _ = c.WriteString(c.Query("k2")) //nolint:errcheck // It is fine to ignore the error here

		return nil
	}

	wrapAgent := func(c *Client) {
		c.SetParam("k1", "v1").
			AddParam("k2", "v2")
	}

	testClient(t, handler, wrapAgent, "v1v2")
}

func Test_Client_PathParam(t *testing.T) {
	t.Parallel()

	t.Run("set path param", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetPathParam("foo", "bar")
		require.Equal(t, "bar", req.PathParam("foo"))

		req.SetPathParam("foo", "bar1")
		require.Equal(t, "bar1", req.PathParam("foo"))
	})

	t.Run("set path params", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetPathParams(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))

		req.SetPathParams(map[string]string{
			"foo": "bar1",
		})
		require.Equal(t, "bar1", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))
	})

	t.Run("set path params with struct", func(t *testing.T) {
		t.Parallel()
		type args struct {
			CookieString string `path:"string"`
			CookieInt    int    `path:"int"`
		}

		req := New().SetPathParamsWithStruct(&args{
			CookieInt:    5,
			CookieString: "foo",
		})

		require.Equal(t, "5", req.PathParam("int"))
		require.Equal(t, "foo", req.PathParam("string"))
	})

	t.Run("del path params", func(t *testing.T) {
		t.Parallel()
		req := New().
			SetPathParams(map[string]string{
				"foo": "bar",
				"bar": "foo",
			})
		require.Equal(t, "bar", req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))

		req.DelPathParams("foo")
		require.Empty(t, req.PathParam("foo"))
		require.Equal(t, "foo", req.PathParam("bar"))
	})
}

func Test_Client_PathParam_With_Server(t *testing.T) {
	app, dial, start := createHelperServer(t)

	app.Get("/:test", func(c fiber.Ctx) error {
		return c.SendString(c.Params("test"))
	})

	go start()

	resp, err := New().SetDial(dial).
		SetPathParam("path", "test").
		Get("http://example.com/:path")

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "test", resp.String())
}

func Test_Client_TLS(t *testing.T) {
	t.Parallel()

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("tls")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		}))
	}()
	time.Sleep(1 * time.Second)

	client := New()
	resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + ln.Addr().String())

	require.NoError(t, err)
	cfg := client.TLSConfig()
	require.Same(t, clientTLSConf, cfg)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "tls", resp.String())
}

func Test_Client_TLS_Error(t *testing.T) {
	t.Parallel()

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	clientTLSConf.MaxVersion = tls.VersionTLS12
	serverTLSConf.MinVersion = tls.VersionTLS13
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("tls")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		}))
	}()
	time.Sleep(1 * time.Second)

	client := New()
	resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + ln.Addr().String())

	require.Error(t, err)
	cfg := client.TLSConfig()
	require.Same(t, clientTLSConf, cfg)
	require.Nil(t, resp)
}

func Test_Client_TLS_Empty_TLSConfig(t *testing.T) {
	t.Parallel()

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)

	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("tls")
	})

	go func() {
		assert.NoError(t, app.Listener(ln, fiber.ListenConfig{
			DisableStartupMessage: true,
		}))
	}()
	time.Sleep(1 * time.Second)

	client := New()
	resp, err := client.Get("https://" + ln.Addr().String())

	require.Error(t, err)
	require.NotEqual(t, clientTLSConf, client.TLSConfig())
	require.Nil(t, resp)
}

func Test_Client_SetCertificates(t *testing.T) {
	t.Parallel()

	serverTLSConf, _, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)

	client := New().SetCertificates(serverTLSConf.Certificates...)
	require.Len(t, client.TLSConfig().Certificates, 1)
}

func Test_Client_SetRootCertificate(t *testing.T) {
	t.Parallel()

	client := New().SetRootCertificate("../.github/testdata/ssl.pem")
	require.NotNil(t, client.TLSConfig().RootCAs)
}

func Test_Client_SetRootCertificateFromString(t *testing.T) {
	t.Parallel()

	file, err := os.Open("../.github/testdata/ssl.pem")
	defer func() { require.NoError(t, file.Close()) }()
	require.NoError(t, err)

	pem, err := io.ReadAll(file)
	require.NoError(t, err)

	client := New().SetRootCertificateFromString(string(pem))
	require.NotNil(t, client.TLSConfig().RootCAs)
}

func Test_Client_R(t *testing.T) {
	t.Parallel()

	client := New()
	req := client.R()

	require.Equal(t, "Request", reflect.TypeFor[Request]().Name())
	require.Equal(t, client, req.Client())
}

func Test_Replace(t *testing.T) {
	app, dial, start := createHelperServer(t)

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(string(c.Request().Header.Peek("k1")))
	})

	go start()

	C().SetDial(dial)
	resp, err := Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Empty(t, resp.String())

	r := New().SetDial(dial).SetHeader("k1", "v1")
	clean := Replace(r)
	resp, err = Get("http://example.com")
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "v1", resp.String())

	clean()

	C().SetDial(dial)
	resp, err = Get("http://example.com")

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Empty(t, resp.String())

	C().SetDial(nil)
}

func Test_Replace_ConcurrentAccess(t *testing.T) {
	original := C()
	t.Cleanup(func() {
		defaultClient.Store(original)
	})

	clients := []*Client{New(), New(), New()}

	var nilLoads atomic.Int32
	var wg sync.WaitGroup

	for i := range 4 {
		offset := i
		wg.Go(func() {
			for j := range 1_000 {
				restore := Replace(clients[(offset+j)%len(clients)])
				if C() == nil {
					nilLoads.Add(1)
				}
				restore()
			}
		})
	}

	for range 4 {
		wg.Go(func() {
			for range 10_000 {
				if C() == nil {
					nilLoads.Add(1)
				}
			}
		})
	}

	wg.Wait()

	require.Zero(t, nilLoads.Load())
	require.NotNil(t, C())
}

func Test_Set_Config_To_Request(t *testing.T) {
	t.Parallel()

	t.Run("set ctx", func(t *testing.T) {
		t.Parallel()

		type ctxKey struct{}
		var key ctxKey = struct{}{}

		ctx := context.Background()
		ctx = context.WithValue(ctx, key, "v1")
		req := AcquireRequest()

		setConfigToRequest(req, Config{Ctx: ctx})

		require.Equal(t, "v1", req.Context().Value(key))
	})

	t.Run("set useragent", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{UserAgent: "agent"})

		require.Equal(t, "agent", req.UserAgent())
	})

	t.Run("set referer", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Referer: "referer"})

		require.Equal(t, "referer", req.Referer())
	})

	t.Run("set header", func(t *testing.T) {
		req := AcquireRequest()

		setConfigToRequest(req, Config{Header: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.Header("k1")[0])
	})

	t.Run("set params", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Param: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.Param("k1")[0])
	})

	t.Run("set cookies", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Cookie: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.Cookie("k1"))
	})

	t.Run("set pathparam", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{PathParam: map[string]string{
			"k1": "v1",
		}})

		require.Equal(t, "v1", req.PathParam("k1"))
	})

	t.Run("set timeout", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Timeout: 1 * time.Second})

		require.Equal(t, 1*time.Second, req.Timeout())
	})

	t.Run("set maxredirects", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{MaxRedirects: 1})

		require.Equal(t, 1, req.MaxRedirects())
	})

	t.Run("set redirect sensitive headers", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{RedirectSensitiveHeaders: []string{" X-Api-Key", ""}})

		require.Equal(t, []string{"X-Api-Key"}, req.RedirectSensitiveHeaders())
	})

	t.Run("a config without redirect sensitive headers leaves the request's", func(t *testing.T) {
		t.Parallel()

		for _, names := range [][]string{nil, {}} {
			req := AcquireRequest().SetRedirectSensitiveHeaders("X-Api-Key")

			setConfigToRequest(req, Config{MaxRedirects: 1, RedirectSensitiveHeaders: names})

			require.Equal(t, []string{"X-Api-Key"}, req.RedirectSensitiveHeaders())
		}
	})

	t.Run("set body string", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Body: "test"})

		body, ok := req.body.([]byte)
		require.True(t, ok)
		require.Equal(t, "test", string(body))
	})

	t.Run("set body byte", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Body: []byte("test")})

		require.Equal(t, []byte("test"), req.body)
	})

	t.Run("set body json", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		type payload struct {
			Foo string `json:"foo"`
		}

		setConfigToRequest(req, Config{Body: payload{Foo: "bar"}})

		payloadBody, ok := req.body.(payload)
		require.True(t, ok)
		require.Equal(t, payload{Foo: "bar"}, payloadBody)
	})

	t.Run("set body map", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{Body: map[string]string{
			"foo": "bar",
		}})

		require.Equal(t, map[string]string{
			"foo": "bar",
		}, req.body)
	})

	t.Run("set file", func(t *testing.T) {
		t.Parallel()
		req := AcquireRequest()

		setConfigToRequest(req, Config{File: []*File{
			{
				name: "test",
				path: "path",
			},
		}})

		require.Equal(t, "path", req.File("test").path)
	})
}

func Test_Client_SetProxyURL(t *testing.T) {
	t.Parallel()

	app, dial, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString(c.Get("isProxy"))
	})

	go start()

	fasthttpClient := &fasthttp.Client{
		Dial:                     dial,
		NoDefaultUserAgentHeader: true,
		DisablePathNormalizing:   true,
	}

	// Create a simple proxy server
	proxyServer := fiber.New()

	proxyServer.Use("*", func(c fiber.Ctx) error {
		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()

		req.SetRequestURI(c.BaseURL())
		req.Header.SetMethod(fasthttp.MethodGet)

		for key, value := range c.Request().Header.All() {
			req.Header.AddBytesKV(key, value)
		}

		req.Header.Set("isProxy", "true")

		if err := fasthttpClient.Do(req, resp); err != nil {
			return err
		}

		c.Status(resp.StatusCode())
		c.RequestCtx().SetBody(resp.Body())

		return nil
	})

	addrChan := make(chan string)
	go func() {
		assert.NoError(t, proxyServer.Listen(":0", fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				addrChan <- addr.String()
			},
		}))
	}()

	t.Cleanup(func() {
		require.NoError(t, app.Shutdown())
	})

	time.Sleep(1 * time.Second)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		client := New()
		err := client.SetProxyURL(<-addrChan)

		require.NoError(t, err)

		resp, err := client.Get("http://localhost:3000")
		require.NoError(t, err)

		require.Equal(t, 200, resp.StatusCode())
		require.Equal(t, "true", string(resp.Body()))
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		client := New()

		err := client.SetProxyURL("ftp://127.0.0.1:8080")
		require.Error(t, err)
	})
}

func Test_Client_SetRetryConfig(t *testing.T) {
	t.Parallel()
	retryConfig := &retry.Config{
		InitialInterval: 1 * time.Second,
		MaxRetryCount:   3,
	}

	core, client, req := newCore(), New(), AcquireRequest()
	req.SetURL("http://exampleretry.com")
	client.SetRetryConfig(retryConfig)
	_, err := core.execute(context.Background(), client, req)

	require.Error(t, err)
	require.Equal(t, retryConfig.InitialInterval, client.RetryConfig().InitialInterval)
	require.Equal(t, retryConfig.MaxRetryCount, client.RetryConfig().MaxRetryCount)
}

func Benchmark_Client_Request(b *testing.B) {
	app, dial, start := createHelperServer(b)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello world")
	})

	go start()

	client := New().SetDial(dial)

	b.ReportAllocs()

	var err error
	var resp *Response
	for b.Loop() {
		resp, err = client.Get("http://example.com")
		resp.Close()
	}
	require.NoError(b, err)
}

// Benchmark_Client_Request_RetryRedirects is the request shape that takes a backup.
func Benchmark_Client_Request_RetryRedirects(b *testing.B) {
	app, dial, start := createHelperServer(b)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello world")
	})

	go start()

	client := New().SetDial(dial).SetRetryConfig(&RetryConfig{MaxRetryCount: 2})

	b.ReportAllocs()

	var err error
	var resp *Response
	for b.Loop() {
		resp, err = client.Get("http://example.com", Config{MaxRedirects: 3})
		resp.Close()
	}
	require.NoError(b, err)
}

func Benchmark_Client_Request_Parallel(b *testing.B) {
	app, dial, start := createHelperServer(b)
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello world")
	})

	go start()

	client := New().SetDial(dial)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		var err error
		var resp *Response
		for pb.Next() {
			resp, err = client.Get("http://example.com")
			resp.Close()
		}
		require.NoError(b, err)
	})
}

func Benchmark_Client_Request_Send_ContextCancel(b *testing.B) {
	app, ln, start := createHelperServer(b)

	startedCh := make(chan struct{})
	canceledCh := make(chan struct{})
	errCh := make(chan error)
	respCh := make(chan *Response)

	app.Post("/", func(c fiber.Ctx) error {
		startedCh <- struct{}{}
		// Hold the response until the loop has received Send's outcome:
		// with the reply still pending, the client's response channel
		// cannot be ready, so Send can only return through its
		// ctx.Done() branch — no race against the reply.
		<-canceledCh
		return c.Status(fiber.StatusOK).SendString("post")
	})

	go start()

	client := New().SetDial(ln)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		ctx, cancel := context.WithCancel(context.Background())

		req := AcquireRequest().
			SetClient(client).
			SetURL("http://example.com").
			SetMethod(fiber.MethodPost).
			SetContext(ctx)

		go func(r *Request) {
			defer ReleaseRequest(r)

			resp, err := r.Send()

			respCh <- resp
			errCh <- err
		}(req)

		<-startedCh // request is made, we can cancel the context now
		cancel()

		// Send must return before the handler is released below, so these
		// receives can only observe the cancellation outcome.
		resp := <-respCh
		err := <-errCh
		canceledCh <- struct{}{} // release the handler; core's drainer reclaims the late response

		require.Nil(b, resp)
		require.ErrorIs(b, err, ErrTimeoutOrCancel)
	}
}

func Test_Client_StreamResponseBody(t *testing.T) {
	t.Parallel()

	t.Run("default value", func(t *testing.T) {
		t.Parallel()
		client := New()
		require.False(t, client.StreamResponseBody())
	})

	t.Run("enable streaming", func(t *testing.T) {
		t.Parallel()
		client := New()
		result := client.SetStreamResponseBody(true)
		require.True(t, client.StreamResponseBody())
		require.Equal(t, client, result)
	})

	t.Run("disable streaming", func(t *testing.T) {
		t.Parallel()
		client := New()
		client.SetStreamResponseBody(true)
		require.True(t, client.StreamResponseBody())
		client.SetStreamResponseBody(false)
		require.False(t, client.StreamResponseBody())
	})

	t.Run("with host client", func(t *testing.T) {
		t.Parallel()
		hostClient := &fasthttp.HostClient{}
		client := NewWithHostClient(hostClient)
		client.SetStreamResponseBody(true)
		require.True(t, client.StreamResponseBody())
		require.True(t, hostClient.StreamResponseBody)
	})

	t.Run("with lb client", func(t *testing.T) {
		t.Parallel()
		hostClient := &fasthttp.HostClient{Addr: "example.com:80"}
		lbClient := &fasthttp.LBClient{
			Clients: []fasthttp.BalancingClient{
				hostClient,
			},
		}
		client := NewWithLBClient(lbClient)
		client.SetStreamResponseBody(true)
		require.True(t, client.StreamResponseBody())
		require.True(t, hostClient.StreamResponseBody)
	})
}

func Test_Client_MaxResponseBodySize(t *testing.T) {
	t.Parallel()

	// answerWith serves response on every connection after reading the request.
	answerWith := func(t *testing.T, response string) fasthttp.DialFunc {
		t.Helper()

		ln := fasthttputil.NewInmemoryListener()
		t.Cleanup(func() { require.NoError(t, ln.Close()) })
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				_, err = conn.Read(make([]byte, 4096))
				assert.NoError(t, err)
				_, err = conn.Write([]byte(response))
				assert.NoError(t, err)
				assert.NoError(t, conn.Close())
			}
		}()
		return func(string) (net.Conn, error) { return ln.Dial() }
	}

	t.Run("a new client refuses a declared size over the limit", func(t *testing.T) {
		t.Parallel()

		over := defaultMaxResponseBodySize + 1
		for _, response := range []string{
			"HTTP/1.1 200 OK\r\nContent-Length: " + strconv.Itoa(over) + "\r\n\r\nok",
			"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n" + strconv.FormatInt(int64(over), 16) + "\r\nok",
		} {
			client := New().SetDial(answerWith(t, response))
			require.Equal(t, defaultMaxResponseBodySize, client.MaxResponseBodySize())

			_, err := client.Get("http://example.com/")
			require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
		}
	})

	t.Run("the limit can be set and Reset restores it", func(t *testing.T) {
		t.Parallel()

		dial := answerWith(t, "HTTP/1.1 200 OK\r\nContent-Length: 5\r\n\r\nhello")

		_, err := New().SetDial(dial).SetMaxResponseBodySize(4).Get("http://example.com/")
		require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)

		client := New().SetDial(dial).SetMaxResponseBodySize(5)
		resp, err := client.Get("http://example.com/")
		require.NoError(t, err)
		require.Equal(t, "hello", resp.String())
		resp.Close()

		client.SetMaxResponseBodySize(0).Reset()
		require.Equal(t, defaultMaxResponseBodySize, client.MaxResponseBodySize())
	})

	t.Run("a wrapped client keeps its own setting", func(t *testing.T) {
		t.Parallel()

		host := &fasthttp.HostClient{Addr: "example.com:80"}
		first := &fasthttp.HostClient{Addr: "example.com:80"}
		second := &fasthttp.HostClient{Addr: "example.org:80"}
		for _, client := range []*Client{
			NewWithClient(&fasthttp.Client{}),
			NewWithHostClient(host),
			NewWithLBClient(&fasthttp.LBClient{Clients: []fasthttp.BalancingClient{first, second}}),
		} {
			require.Zero(t, client.MaxResponseBodySize())
			require.Equal(t, 7, client.SetMaxResponseBodySize(7).MaxResponseBodySize())
		}
		require.Equal(t, 7, host.MaxResponseBodySize)
		require.Equal(t, 7, first.MaxResponseBodySize)
		require.Equal(t, 7, second.MaxResponseBodySize)
	})
}

func Test_Client_UnparsableCookieAttributesIgnored(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Response().Header.Add(fiber.HeaderSetCookie, "a=1; Max-Age=-1")
			c.Response().Header.Add(fiber.HeaderSetCookie, "b=2; Expires=not-a-date")
			c.Response().Header.Add(fiber.HeaderSetCookie, "c=3")
			return c.SendString("ok")
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial())
	resp, err := client.Get("http://example.com/")
	require.NoError(t, err)
	defer resp.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, "ok", resp.String())

	values := map[string]string{}
	for _, cookie := range resp.Cookies() {
		values[string(cookie.Key())] = string(cookie.Value())
	}
	require.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, values)
}

func Test_Client_ResponseHookError_KeepsCallerRequest(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	})
	defer server.stop()

	client := New().SetDial(server.dial())
	client.AddResponseHook(func(_ *Client, _ *Response, _ *Request) error {
		return errors.New("hook failed")
	})

	req := AcquireRequest()
	defer ReleaseRequest(req)
	req.SetClient(client).SetURL("http://example.com/")

	_, err := req.Send()
	require.ErrorContains(t, err, "hook failed")
	require.Equal(t, "http://example.com/", req.URL(), "the caller's request must be left intact")
	require.Same(t, client, req.Client())
}

func Test_Request_Reset_ClearsClient(t *testing.T) {
	t.Parallel()

	custom := New()
	req := AcquireRequest()
	req.SetClient(custom)
	req.Reset()
	require.Nil(t, req.Client(), "a reset request must not keep the previous owner's client")
	ReleaseRequest(req)
}

func Test_Client_Debug_DoesNotDrainStream(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
			return c.SendStreamWriter(func(w *bufio.Writer) {
				_, _ = w.WriteString("chunk") //nolint:errcheck // not needed
				_ = w.Flush()                 //nolint:errcheck // not needed
			})
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial()).SetStreamResponseBody(true).Debug()

	resp, err := client.Get("http://example.com/")
	require.NoError(t, err)
	defer resp.Close()
	stream := resp.BodyStream()
	require.NotNil(t, stream, "the debug log must not consume the streamed body")
	body, err := io.ReadAll(stream)
	require.NoError(t, err)
	require.Equal(t, "chunk", string(body))
}

func Test_CookieJar_MaxAge(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/set", func(c fiber.Ctx) error {
			c.Response().Header.Add(fiber.HeaderSetCookie, "sid=abc; Max-Age=1")
			return c.SendString("set")
		})
		app.Get("/del", func(c fiber.Ctx) error {
			c.Response().Header.Add(fiber.HeaderSetCookie, "sid=abc; Max-Age=0")
			return c.SendString("del")
		})
		app.Get("/echo", func(c fiber.Ctx) error {
			return c.SendString(c.Cookies("sid"))
		})
	})
	defer server.stop()

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)
	client := New().SetDial(server.dial()).SetCookieJar(jar)

	get := func(path string) string {
		resp, err := client.Get("http://example.com" + path)
		require.NoError(t, err)
		defer resp.Close()
		return resp.String()
	}

	get("/set")
	require.Equal(t, "abc", get("/echo"))
	get("/del")
	require.Empty(t, get("/echo"), "Max-Age=0 must delete the cookie")

	get("/set")
	require.Equal(t, "abc", get("/echo"))
	time.Sleep(1500 * time.Millisecond)
	require.Empty(t, get("/echo"), "the cookie must expire after Max-Age seconds")
}

// requireOwnedRequestReleased sends until the client hands a pooled request out
// twice. Only a release puts one back where Acquire can find it, so a repeat is
// proof that a failing send releases; with the release gone no send returns one
// to the pool, so no pointer can ever repeat however many follow.
//
// Which pointer comes back is not assertable: sync.Pool promises no relation
// between what is put and what a later Get returns, and under -race with these
// tests in parallel a specific request does not come back to a specific caller.
func requireOwnedRequestReleased(t *testing.T, send func() (*Request, error)) {
	t.Helper()

	seen := make(map[*Request]struct{}, 64)
	for range 64 {
		req, err := send()
		require.ErrorContains(t, err, "boom")
		require.NotNil(t, req)
		if _, repeat := seen[req]; repeat {
			return
		}
		seen[req] = struct{}{}
	}
	t.Fatal("the client never reused a pooled request: the failing send strands it")
}

func Test_Client_PreHookError_ReleasesOwnedRequest(t *testing.T) {
	t.Parallel()

	var seen *Request
	client := New()
	client.AddRequestHook(func(_ *Client, req *Request) error {
		seen = req
		require.True(t, req.clientOwned)
		return errors.New("boom")
	})

	// Nothing is dispatched, so no response will ever carry the helper's own
	// request back to the pool.
	requireOwnedRequestReleased(t, func() (*Request, error) {
		seen = nil
		_, err := client.Get("http://example.com")
		return seen, err
	})
}

func Test_Client_DispatchError_ReleasesOwnedRequest(t *testing.T) {
	t.Parallel()

	var seen *Request
	client := New().SetDial(func(_ string) (net.Conn, error) {
		return nil, errors.New("boom")
	})
	client.AddRequestHook(func(_ *Client, req *Request) error {
		seen = req
		return nil
	})

	// The send never produces a response, so the helper's own request is
	// stranded unless this path releases it too.
	requireOwnedRequestReleased(t, func() (*Request, error) {
		seen = nil
		_, err := client.Get("http://example.com")
		return seen, err
	})
}

func Test_Client_AfterHookError_ReleasesOwnedRequest(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString("ok")
		})
	})
	defer server.stop()

	var seen *Request
	client := New().SetDial(server.dial())
	client.AddResponseHook(func(_ *Client, _ *Response, req *Request) error {
		seen = req
		require.True(t, req.clientOwned)
		return errors.New("boom")
	})

	// No response reaches the caller, so the request has to go back here.
	requireOwnedRequestReleased(t, func() (*Request, error) {
		seen = nil
		_, err := client.Get("http://example.com/")
		return seen, err
	})
}

func Test_CookieJar_MaxAgePrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		maxAge string
		kept   bool
	}{
		{name: "not an integer", maxAge: "Max-Age=bogus", kept: true},
		{name: "no value", maxAge: "Max-Age", kept: true},
		{name: "empty value", maxAge: "Max-Age=", kept: true},
		{name: "padded", maxAge: "Max-Age= 3600 ", kept: true},
		{name: "zero", maxAge: "Max-Age=0", kept: false},
		{name: "negative", maxAge: "Max-Age=-1", kept: false},
		{name: "lowercase zero", maxAge: "max-age=0", kept: false},
		{name: "last of duplicates wins", maxAge: "Max-Age=3600; Max-Age=0", kept: false},
		{name: "last of duplicates revives", maxAge: "Max-Age=0; Max-Age=3600", kept: true},
		{name: "unparsable duplicate ignored", maxAge: "Max-Age=0; Max-Age=bogus", kept: false},
		{name: "seconds beyond a duration", maxAge: "Max-Age=10000000000", kept: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			jar := AcquireCookieJar()
			t.Cleanup(jar.Release)

			uri := fasthttp.AcquireURI()
			t.Cleanup(func() { fasthttp.ReleaseURI(uri) })
			require.NoError(t, uri.Parse(nil, []byte("http://example.com/")))

			resp := fasthttp.AcquireResponse()
			t.Cleanup(func() { fasthttp.ReleaseResponse(resp) })
			expires := time.Now().Add(time.Hour).UTC().Format(time.RFC1123)
			resp.Header.Add("Set-Cookie", "sid=x; "+tc.maxAge+"; Expires="+expires)

			jar.parseCookiesFromResp([]byte("example.com"), []byte("/"), resp)

			// RFC 6265 §5.2.2: Max-Age wins over Expires, but only when it is an integer.
			cookies := jar.Get(uri)
			if !tc.kept {
				require.Empty(t, cookies)
				return
			}
			require.Len(t, cookies, 1)
			require.Equal(t, "x", string(cookies[0].Value()))
		})
	}
}

// Test_CookieJar_UnparsableAttributeKeepsTheOthers covers a Set-Cookie header
// with one attribute fasthttp cannot parse. RFC 6265 Section 5.2 has such an
// attribute ignored, but fasthttp stops at the first one, so everything after
// it was lost: "Max-Age=abc; Secure; Path=/" stored a cookie that was neither
// Secure nor on "/", which the jar then sent over plain HTTP, and only to the
// directory that set it.
func Test_CookieJar_UnparsableAttributeKeepsTheOthers(t *testing.T) {
	t.Parallel()

	const (
		receivedPath = "/dir/x" // the request that received the cookie: default path "/dir"
		otherPath    = "/other" // outside the default path
	)

	tests := []struct {
		check     func(t *testing.T, jar *CookieJar)
		name      string
		setCookie string
	}{
		{
			name:      "Secure and Path after a non-integer Max-Age",
			setCookie: "s=1; Max-Age=abc; Secure; Path=/",
			check: func(t *testing.T, jar *CookieJar) {
				t.Helper()
				require.Empty(t, jarGet(t, jar, "http://example.com"+otherPath), "a Secure cookie must not go over plain HTTP")
				got := jarGet(t, jar, "https://example.com"+otherPath)
				require.Len(t, got, 1, "Path=/ must be kept")
				require.True(t, got[0].Secure())
				require.Equal(t, "/", string(got[0].Path()))
			},
		},
		{
			name:      "Path after a fractional Max-Age",
			setCookie: "s=1; Max-Age=1.5; Path=/",
			check: func(t *testing.T, jar *CookieJar) {
				t.Helper()
				require.Len(t, jarGet(t, jar, "http://example.com"+otherPath), 1)
			},
		},
		{
			name:      "attributes after an empty Max-Age",
			setCookie: "s=1; Max-Age=; HttpOnly; Path=/; SameSite=Lax",
			check: func(t *testing.T, jar *CookieJar) {
				t.Helper()
				got := jarGet(t, jar, "http://example.com"+otherPath)
				require.Len(t, got, 1)
				require.True(t, got[0].HTTPOnly())
				require.Equal(t, fasthttp.CookieSameSiteLaxMode, got[0].SameSite())
			},
		},
		{
			name:      "attributes after an unparsable Expires",
			setCookie: "s=1; Expires=bogus; Secure; Path=/",
			check: func(t *testing.T, jar *CookieJar) {
				t.Helper()
				require.Empty(t, jarGet(t, jar, "http://example.com"+otherPath))
				require.Len(t, jarGet(t, jar, "https://example.com"+otherPath), 1)
			},
		},
		{
			name:      "Domain after a non-integer Max-Age",
			setCookie: "s=1; Max-Age=abc; Domain=example.com; Path=/",
			check: func(t *testing.T, jar *CookieJar) {
				t.Helper()
				require.Len(t, jarGet(t, jar, "http://sub.example.com"+otherPath), 1, "a Domain cookie reaches the subdomains")
			},
		},
		{
			name:      "the last integer Max-Age counts and the attributes survive",
			setCookie: "s=1; Max-Age=3600; Max-Age=abc; Secure; Path=/",
			check: func(t *testing.T, jar *CookieJar) {
				t.Helper()
				got := jarGet(t, jar, "https://example.com"+otherPath)
				require.Len(t, got, 1)
				require.WithinDuration(t, time.Now().Add(time.Hour), got[0].Expire(), time.Minute)
			},
		},
		{
			name:      "an unusable name and value pair is dropped with its attributes",
			setCookie: `s=a"b; Path=/`,
			check: func(t *testing.T, jar *CookieJar) {
				t.Helper()
				require.Empty(t, jar.hostCookies, "nothing may be stored, not even an empty cookie")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			jar := AcquireCookieJar()
			t.Cleanup(jar.Release)

			resp := fasthttp.AcquireResponse()
			t.Cleanup(func() { fasthttp.ReleaseResponse(resp) })
			resp.Header.Add("Set-Cookie", tc.setCookie)

			jar.parseCookiesFromResp([]byte("example.com"), []byte(receivedPath), resp)

			tc.check(t, jar)
		})
	}
}

// jarGet returns the cookies jar would send to rawURL.
func jarGet(t *testing.T, jar *CookieJar, rawURL string) []*fasthttp.Cookie {
	t.Helper()

	uri := fasthttp.AcquireURI()
	t.Cleanup(func() { fasthttp.ReleaseURI(uri) })
	require.NoError(t, uri.Parse(nil, []byte(rawURL)))

	return jar.Get(uri)
}

func Test_Request_CancelledContext_NoRace(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			// Slow enough that the response never beats the canceled context.
			time.Sleep(50 * time.Millisecond)
			return c.SendString("ok")
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial())

	// A live request first, so the server is serving before it is shut down.
	warm, err := client.Get("http://example.com/")
	require.NoError(t, err)
	warm.Close()

	for range 20 {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		req := AcquireRequest()
		req.SetClient(client).SetURL("http://example.com/").SetContext(ctx)
		_, err := req.Send()
		require.ErrorIs(t, err, ErrTimeoutOrCancel)
		ReleaseRequest(req)
	}
}

func Test_Client_RequestHeaderOverridesClientHeader(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString(strings.Join(c.GetReqHeaders()["X-Level"], "|"))
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial()).SetHeader("X-Level", "client")

	resp, err := client.Get("http://example.com/")
	require.NoError(t, err)
	require.Equal(t, "client", resp.String())
	resp.Close()

	req := AcquireRequest()
	defer ReleaseRequest(req)
	req.SetClient(client).SetURL("http://example.com/").SetHeader("X-Level", "request")
	resp, err = req.Send()
	require.NoError(t, err)
	defer resp.Close()
	require.Equal(t, "request", resp.String(), "a request-level header overrides the client-level one")
}

func Test_Request_Params_KeepInsertionOrder(t *testing.T) {
	t.Parallel()

	req := AcquireRequest()
	defer ReleaseRequest(req)
	want := make([]string, 0, 20)
	for i := range 20 {
		v := strconv.Itoa(i)
		req.AddParam("a", v)
		want = append(want, v)
	}
	for key, values := range req.Params() {
		require.Equal(t, "a", key)
		require.Equal(t, want, values)
	}
}

func Test_Response_Header_CopiesValue(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("X-Test", "value")
			return c.SendString("ok")
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial())
	resp, err := client.Get("http://example.com/")
	require.NoError(t, err)
	defer resp.Close()

	header := resp.Header("X-Test")
	resp.RawResponse.Header.Set("X-Test", "other")
	require.Equal(t, "value", header, "the returned string must not alias the header storage")
}

func Test_Request_Resend_DoesNotAccumulateHeaders(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			return c.SendString(strconv.Itoa(len(c.GetReqHeaders()["X-A"])))
		})
	})
	defer server.stop()

	client := New().SetDial(server.dial())
	req := AcquireRequest()
	defer ReleaseRequest(req)
	req.SetClient(client).SetURL("http://example.com/").SetHeader("X-A", "1")

	for range 3 {
		resp, err := req.Send()
		require.NoError(t, err)
		require.Equal(t, "1", resp.String(), "each send must carry the header once")
		resp.Close()
	}
}

func Test_Request_Resend_SelectsCookiesAnew(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/login", func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{Name: "sid", Value: "secret", Path: "/"})
			return c.SendStatus(fiber.StatusNoContent)
		})
		app.Get("/logout", func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{Name: "sid", Path: "/", MaxAge: -1})
			return c.SendStatus(fiber.StatusNoContent)
		})
		app.Get("/cookies", func(c fiber.Ctx) error {
			return c.SendString(c.Get(fiber.HeaderCookie))
		})
	})
	t.Cleanup(server.stop)

	send := func(t *testing.T, req *Request, url string) string {
		t.Helper()
		resp, err := req.Get(url)
		require.NoError(t, err)
		defer resp.Close()
		return resp.String()
	}

	t.Run("a jar cookie does not follow the request to another host", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)
		req := AcquireRequest().SetClient(New().SetDial(server.dial()).SetCookieJar(jar))
		defer ReleaseRequest(req)

		send(t, req, "http://a.example/login")
		require.Equal(t, "sid=secret", send(t, req, "http://a.example/cookies"))
		require.Empty(t, send(t, req, "http://b.example/cookies"))
	})

	t.Run("a jar cookie the server expired is not sent again", func(t *testing.T) {
		t.Parallel()

		jar := AcquireCookieJar()
		defer ReleaseCookieJar(jar)
		req := AcquireRequest().SetClient(New().SetDial(server.dial()).SetCookieJar(jar))
		defer ReleaseRequest(req)

		send(t, req, "http://a.example/login")
		send(t, req, "http://a.example/logout")
		require.Empty(t, send(t, req, "http://a.example/cookies"))
	})

	t.Run("a deleted cookie is not sent again", func(t *testing.T) {
		t.Parallel()

		client := New().SetDial(server.dial()).SetCookie("a", "1")
		req := AcquireRequest().SetClient(client).SetCookie("b", "2")
		defer ReleaseRequest(req)

		require.Equal(t, "a=1; b=2", send(t, req, "http://a.example/cookies"))
		require.Equal(t, "a=1", send(t, req.DelCookies("b"), "http://a.example/cookies"))
		client.DelCookies("a")
		require.Empty(t, send(t, req, "http://a.example/cookies"))
	})

	t.Run("a Cookie header set on the request is kept", func(t *testing.T) {
		t.Parallel()

		req := AcquireRequest().SetClient(New().SetDial(server.dial())).SetHeader(fiber.HeaderCookie, "a=1")
		defer ReleaseRequest(req)

		for range 2 {
			require.Equal(t, "a=1", send(t, req, "http://a.example/cookies"))
		}
	})
}

func Test_Client_UppercaseScheme(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	})
	defer server.stop()

	client := New().SetDial(server.dial())
	resp, err := client.Get("HTTP://example.com/")
	require.NoError(t, err)
	defer resp.Close()
	require.Equal(t, "ok", resp.String())
}

func Test_CookieJar_MaxAgeBeyond32Bit(t *testing.T) {
	t.Parallel()

	// int is 32 bits on 386 and arm, where this used to fail to parse and leave
	// a session cookie behind.
	seconds, ok := lastMaxAge([]byte("sid=x; Max-Age=3000000000"))
	require.True(t, ok)
	require.Equal(t, int64(3000000000), seconds)
	require.Equal(t, 3000000000*time.Second, maxAgeDuration(seconds))
}
