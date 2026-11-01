package client

import (
	"bufio"
	"context"
	"crypto/tls"
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
			ListenerAddrFunc:      func(addr net.Addr) { addrChan <- addr.String() },
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

// clientTestARequireOK fails the test unless the request that produced resp and err
// succeeded with a 200 and body as its (trimmed) body.
func clientTestARequireOK(t *testing.T, resp *Response, err error, body string) {
	t.Helper()

	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode())
	require.Equal(t, body, resp.String())
}

// clientAStartTLS serves handler for GET / over TLS on a loopback port. It returns the
// app, the address it listens on and the client config that trusts its certificate.
// Each tune function that is not nil may adjust the server and client configs before
// the listener starts. Shutting the app down is left to the caller.
func clientAStartTLS(t *testing.T, handler fiber.Handler, tune ...func(server, client *tls.Config)) (*fiber.App, string, *tls.Config) {
	t.Helper()

	serverTLSConf, clientTLSConf, err := tlstest.GetTLSConfigs()
	require.NoError(t, err)
	for _, adjust := range tune {
		if adjust != nil {
			adjust(serverTLSConf, clientTLSConf)
		}
	}

	ln, err := net.Listen(fiber.NetworkTCP4, "127.0.0.1:0")
	require.NoError(t, err)
	ln = tls.NewListener(ln, serverTLSConf)

	app := fiber.New()
	app.Get("/", handler)

	go func() { assert.NoError(t, app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})) }()

	return app, ln.Addr().String(), clientTLSConf
}

// clientTestAStartTLSServer is [clientAStartTLS] for a handler that answers with body.
func clientTestAStartTLSServer(t *testing.T, body string, tune ...func(server, client *tls.Config)) (*fiber.App, string, *tls.Config) {
	t.Helper()

	return clientAStartTLS(t, func(c fiber.Ctx) error { return c.SendString(body) }, tune...)
}

// clientAServeTCP serves the routes from a fresh app on a loopback port and returns the
// address it listens on. The app shuts down when the test ends.
func clientAServeTCP(t *testing.T, routes func(app *fiber.App)) string {
	t.Helper()

	app, addr := startTestServerWithPort(t, routes)
	t.Cleanup(func() { require.NoError(t, app.Shutdown()) })

	return addr
}

// clientTestAClient starts a test server whose routes are registered by routes and
// returns a client that dials it. The server stops when the test ends.
func clientTestAClient(t *testing.T, routes func(app *fiber.App)) *Client {
	t.Helper()

	server := startTestServer(t, routes)
	t.Cleanup(server.stop)

	return New().SetDial(server.dial())
}

// clientAOKClient returns a client for a server whose GET / answers "ok".
func clientAOKClient(t *testing.T) *Client {
	t.Helper()

	return clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	})
}

func Test_New_With_Client(t *testing.T) {
	t.Parallel()
	t.Run("with valid client", func(t *testing.T) {
		t.Parallel()
		require.NotNil(t, NewWithClient(&fasthttp.Client{MaxConnsPerHost: 5}))
	})

	for _, tc := range []struct {
		build func()
		name  string
		kind  string // the type the panic names
	}{
		{name: "with nil client", build: func() { NewWithClient(nil) }, kind: "fasthttp.Client"},
		{name: "with nil host client", build: func() { NewWithHostClient(nil) }, kind: "fasthttp.HostClient"},
		{name: "with nil lb client", build: func() { NewWithLBClient(nil) }, kind: "fasthttp.LBClient"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.PanicsWithValue(t, tc.kind+" must not be nil", tc.build)
		})
	}
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
	fromString, fromFile := (*Client).SetRootCertificateFromString, (*Client).SetRootCertificate
	noCert := ErrFailedToAppendCert.Error()

	tests := []struct {
		set  func(c *Client, arg string) *Client
		name string
		arg  string
		want string
	}{
		{name: "string without a certificate", set: fromString, arg: "not a pem", want: noCert},
		{name: "string of invalid pem data", set: fromString, arg: "invalid pem data", want: noCert},
		{name: "empty string", set: fromString, arg: "", want: noCert},
		{name: "file without a certificate", set: fromFile, arg: noCertificate, want: noCert},
		{name: "missing file", set: fromFile, arg: missing, want: "client: open " + missing},
		{
			name: "missing relative file", set: fromFile, arg: "does-not-exist.pem",
			want: "client: open does-not-exist.pem",
		},
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

				recovered := recoverValue(func() { tc.set(c, tc.arg) })
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
		requireNameList(t, New(), (*Client).SetRedirectSensitiveHeaders, (*Client).RedirectSensitiveHeaders)
	})

	t.Run("the options for a request carry the client's and the request's names together", func(t *testing.T) {
		t.Parallel()
		client := New().SetRedirectSensitiveHeaders("X-One")
		own := clientTestBRequest(t).SetRedirectSensitiveHeaders("X-Two")

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
	noop := func(_ *Client, _ *Request) error { return nil }
	noopResponse := func(_ *Client, _ *Response, _ *Request) error { return nil }

	t.Run("add request hooks", func(t *testing.T) {
		t.Parallel()
		client := New().AddRequestHook(noop)
		require.Len(t, client.RequestHook(), 1)

		client.AddRequestHook(noop, func(_ *Client, _ *Request) error { return nil })
		require.Len(t, client.RequestHook(), 3)
	})

	t.Run("add final request hooks", func(t *testing.T) {
		t.Parallel()
		client := New().AddFinalRequestHook(noop)

		require.Len(t, client.FinalRequestHook(), 1)
	})

	t.Run("add response hooks", func(t *testing.T) {
		t.Parallel()
		client := New().AddResponseHook(noopResponse)
		require.Len(t, client.ResponseHook(), 1)

		client.AddResponseHook(noopResponse, func(_ *Client, _ *Response, _ *Request) error { return nil })
		require.Len(t, client.ResponseHook(), 3)
	})

	t.Run("request hooks run in the order they were added", func(t *testing.T) {
		t.Parallel()
		buf := bytebufferpool.Get()
		defer bytebufferpool.Put(buf)
		write := func(s string) RequestHook {
			return func(_ *Client, _ *Request) error {
				buf.WriteString(s)
				return nil
			}
		}

		client := New().AddRequestHook(write("hook1")).AddRequestHook(write("hook2")).AddRequestHook(write("hook3"))
		for _, hook := range client.RequestHook() {
			require.NoError(t, hook(client, &Request{}))
		}

		require.Equal(t, "hook1hook2hook3", buf.String())
	})
}

// clientTestATransport describes one kind of fasthttp transport a Client can be
// built on, so the scenarios every kind has to pass are written once.
type clientTestATransport struct {
	// newClient returns a client of this kind that sends to addr, over TLS if isTLS.
	newClient func(addr string, isTLS bool) *Client
	// balancer returns the transport underneath c that its requests end up on.
	balancer func(c *Client) fasthttp.BalancingClient
	// offline returns a client of this kind that has not dialed yet, and the host
	// clients it dials through.
	offline func() (*Client, []*fasthttp.HostClient)
	// name is "host" or "lb" and tells the replies of the TLS scenario apart.
	name string
	// retries is whether one send may dial more than once, as a balancer over
	// several hosts does.
	retries bool
}

// hostClient returns the fasthttp.HostClient that c's requests end up on.
func (tr clientTestATransport) hostClient(t *testing.T, c *Client) *fasthttp.HostClient {
	t.Helper()

	hc, ok := tr.balancer(c).(*fasthttp.HostClient)
	require.True(t, ok)
	return hc
}

// run registers the scenarios that hold for every kind of transport. Each one starts
// a server and builds a client of its own.
func (tr clientTestATransport) run(t *testing.T) {
	t.Helper()

	t.Run("do and redirects", func(t *testing.T) {
		t.Parallel()
		addr := clientAServeTCP(t, func(app *fiber.App) {
			app.Get("/ok", func(c fiber.Ctx) error { return c.SendString("ok") })
			app.Get("/redirect", func(c fiber.Ctx) error { return c.Redirect().Status(fiber.StatusFound).To("/ok") })
		})

		client := tr.newClient(addr, false)

		resp, err := client.Get("http://" + addr + "/ok")
		clientTestARequireOK(t, resp, err, "ok")

		resp, err = client.Get("http://"+addr+"/redirect", Config{MaxRedirects: 1})
		clientTestARequireOK(t, resp, err, "ok")
	})

	t.Run("retries respect dial overrides", func(t *testing.T) {
		t.Parallel()
		addr := clientAServeTCP(t, func(app *fiber.App) {
			app.Get("/", func(c fiber.Ctx) error { return c.SendString("retry") })
		})

		client := tr.newClient(addr, false)
		client.SetRetryConfig(&RetryConfig{InitialInterval: time.Millisecond, MaxRetryCount: 2})

		var attempts atomic.Int32
		client.SetDial(func(address string) (net.Conn, error) {
			if attempts.Add(1) == 1 {
				return nil, errors.New("dial failure")
			}
			return fasthttp.Dial(address)
		})

		resp, err := client.Get("http://" + addr)
		clientTestARequireOK(t, resp, err, "retry")
		require.EqualValues(t, 2, attempts.Load())
	})

	t.Run("tls configuration propagates", func(t *testing.T) {
		t.Parallel()
		app, addr, clientTLSConf := clientTestAStartTLSServer(t, "tls-"+tr.name)

		client := tr.newClient(addr, true)

		resp, err := client.SetTLSConfig(clientTLSConf).Get("https://" + addr)
		require.NoError(t, err)
		cfg := client.TLSConfig()
		require.Same(t, clientTLSConf, cfg)
		require.NotNil(t, cfg.RootCAs)
		require.Equal(t, clientTLSConf, tr.hostClient(t, client).TLSConfig)
		require.Equal(t, fiber.StatusOK, resp.StatusCode())
		require.Equal(t, "tls-"+tr.name, resp.String())

		require.NoError(t, app.Shutdown())
	})

	t.Run("proxy overrides and reset", func(t *testing.T) {
		t.Parallel()
		client := tr.newClient("example.com:80", false)

		require.NoError(t, client.SetProxyURL("http://127.0.0.1:8080"))
		hc := tr.hostClient(t, client)
		require.NotNil(t, hc.Dial)

		var called atomic.Int32
		customDial := func(string) (net.Conn, error) {
			called.Add(1)
			return nil, errors.New("dial")
		}
		client.SetDial(customDial)

		_, err := hc.Dial("example.com:80")
		require.Error(t, err)
		require.EqualValues(t, 1, called.Load())

		client.Reset()
		require.NotNil(t, client.FasthttpClient())
		require.Nil(t, client.HostClient())
		require.Nil(t, client.LBClient())
	})

	t.Run("timeouts and close idle", func(t *testing.T) {
		t.Parallel()
		client, hosts := tr.offline()

		var dialCalls atomic.Int32
		dialErr := errors.New("dial failed")
		for _, hc := range hosts {
			hc.Dial = func(string) (net.Conn, error) {
				dialCalls.Add(1)
				return nil, dialErr
			}
		}

		req := fasthttp.AcquireRequest()
		resp := fasthttp.AcquireResponse()
		req.SetRequestURI("http://example.com/")
		require.ErrorIs(t, client.DoTimeout(req, resp, 5*time.Millisecond), dialErr)

		req.SetRequestURI("http://example.com/")
		require.ErrorIs(t, client.DoDeadline(req, resp, time.Now().Add(5*time.Millisecond)), dialErr)

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)

		if tr.retries {
			require.GreaterOrEqual(t, dialCalls.Load(), int32(2))
		} else {
			require.EqualValues(t, 2, dialCalls.Load())
		}
		require.NotPanics(t, client.CloseIdleConnections)
	})
}

func Test_Client_HostClient_Behavior(t *testing.T) {
	t.Parallel()
	clientTestATransport{
		name: "host",
		newClient: func(addr string, isTLS bool) *Client {
			return NewWithHostClient(&fasthttp.HostClient{Addr: addr, IsTLS: isTLS})
		},
		balancer: func(c *Client) fasthttp.BalancingClient { return c.HostClient() },
		offline: func() (*Client, []*fasthttp.HostClient) {
			hc := &fasthttp.HostClient{Addr: "example.com:80"}
			return NewWithHostClient(hc), []*fasthttp.HostClient{hc}
		},
	}.run(t)
}

func Test_Client_LBClient_Behavior(t *testing.T) {
	t.Parallel()
	clientTestATransport{
		name: "lb",
		newClient: func(addr string, isTLS bool) *Client {
			return NewWithLBClient(&fasthttp.LBClient{
				Clients: []fasthttp.BalancingClient{&fasthttp.HostClient{Addr: addr, IsTLS: isTLS}},
			})
		},
		balancer: func(c *Client) fasthttp.BalancingClient { return c.LBClient().Clients[0] },
		offline: func() (*Client, []*fasthttp.HostClient) {
			hosts := []*fasthttp.HostClient{{Addr: "example.com:80"}, {Addr: "example.org:80"}}
			return NewWithLBClient(&fasthttp.LBClient{Clients: []fasthttp.BalancingClient{hosts[0], hosts[1]}}), hosts
		},
		retries: true,
	}.run(t)
}

// clientTestAMarshal runs the marshal scenarios of one format: the function handed
// to set is the one get returns, and what it yields, a payload or an error, reaches
// the caller untouched. The accessor pairs differ only in their named function
// type, which F absorbs, so each pair is passed as method expressions.
func clientTestAMarshal[F ~func(any) ([]byte, error)](t *testing.T, format string, set func(*Client, F) *Client, get func(*Client) F, payload []byte, failure error) {
	t.Helper()

	for _, tc := range []struct {
		err     error
		name    string
		payload []byte
	}{
		{name: "set " + format + " marshal", payload: payload},
		{name: "set " + format + " marshal error", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := set(New(), func(_ any) ([]byte, error) { return tc.payload, tc.err })

			val, err := get(client)(nil)
			require.Equal(t, tc.payload, val)
			require.Equal(t, tc.err, err)
		})
	}
}

// clientTestAUnmarshal is [clientTestAMarshal] for the unmarshal accessors of one
// format: a new client has a function, and once a function that reports failure
// is set, the function get returns must report it.
func clientTestAUnmarshal[F ~func([]byte, any) error](t *testing.T, format string, set func(*Client, F) *Client, get func(*Client) F, failure error) {
	t.Helper()

	t.Run("set "+format+" unmarshal", func(t *testing.T) {
		t.Parallel()
		require.NotNil(t, get(New()))
		client := set(New(), func(_ []byte, _ any) error { return failure })

		require.Equal(t, failure, get(client)(nil, nil))
	})
}

func Test_Client_Marshal(t *testing.T) {
	t.Parallel()
	clientTestAMarshal(t, "json", (*Client).SetJSONMarshal, (*Client).JSONMarshal, []byte("hello"), errors.New("empty json"))
	clientTestAMarshal(t, "xml", (*Client).SetXMLMarshal, (*Client).XMLMarshal, []byte("hello"), errors.New("empty xml"))
	clientTestAMarshal(t, "cbor", (*Client).SetCBORMarshal, (*Client).CBORMarshal, []byte{0xf6}, errors.New("invalid struct"))

	clientTestAUnmarshal(t, "json", (*Client).SetJSONUnmarshal, (*Client).JSONUnmarshal, errors.New("empty json"))
	clientTestAUnmarshal(t, "xml", (*Client).SetXMLUnmarshal, (*Client).XMLUnmarshal, errors.New("empty xml"))
	clientTestAUnmarshal(t, "cbor", (*Client).SetCBORUnmarshal, (*Client).CBORUnmarshal, errors.New("invalid cbor"))
}

func Test_Client_SetBaseURL(t *testing.T) {
	t.Parallel()
	client := New().SetBaseURL("http://example.com")

	require.Equal(t, "http://example.com", client.BaseURL())
}

func Test_Client_ConcurrencyRequests(t *testing.T) {
	t.Parallel()
	app, dial, start := createHelperServer(t)
	app.All("/", func(c fiber.Ctx) error { return c.SendString(c.Hostname() + " " + c.Method()) })
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

// The handlers that the tests of the HTTP methods of the client and of the request route to.
func clientAHostname(c fiber.Ctx) error { return c.SendString(c.Hostname()) }
func clientAForm(c fiber.Ctx) error     { return c.SendString(c.FormValue("foo")) }
func clientAEcho(c fiber.Ctx) error     { return c.Send(c.Body()) }
func clientACreated(c fiber.Ctx) error {
	return c.Status(fiber.StatusCreated).SendString(c.FormValue("foo"))
}

// Test_Client_HTTPMethods sends a request with every HTTP method helper twice: through
// the package-level function, which uses the default client, and through a client of
// its own. Each of the two starts a server that only knows the method under test.
func Test_Client_HTTPMethods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		global      func(url string, cfg ...Config) (*Response, error)
		client      func(c *Client, url string, cfg ...Config) (*Response, error)
		handler     fiber.Handler
		name        string
		method      string
		body        string
		headerName  string
		headerValue string
		cfg         []Config
		status      int
		calls       int
		globalDelay time.Duration // pause before the package-level variant's first request, as these tests always had
	}{
		{
			name:    "get",
			method:  fiber.MethodGet,
			global:  Get,
			client:  (*Client).Get,
			handler: clientAHostname,
			status:  fiber.StatusOK,
			body:    "0.0.0.0",
			calls:   1,
		},
		{
			name:        "head",
			method:      fiber.MethodHead,
			global:      Head,
			client:      (*Client).Head,
			handler:     clientAHostname,
			status:      fiber.StatusOK,
			headerName:  fiber.HeaderContentLength,
			headerValue: "7",
			calls:       1,
		},
		{
			name:    "post",
			method:  fiber.MethodPost,
			global:  Post,
			client:  (*Client).Post,
			handler: clientACreated,
			cfg:     []Config{{FormData: map[string]string{"foo": "bar"}}},
			status:  fiber.StatusCreated,
			body:    "bar",
			calls:   5,
		},
		{
			name:    "put",
			method:  fiber.MethodPut,
			global:  Put,
			client:  (*Client).Put,
			handler: clientAForm,
			cfg:     []Config{{FormData: map[string]string{"foo": "bar"}}},
			status:  fiber.StatusOK,
			body:    "bar",
			calls:   5,
		},
		{
			name:        "delete",
			method:      fiber.MethodDelete,
			global:      Delete,
			client:      (*Client).Delete,
			handler:     func(c fiber.Ctx) error { return c.Status(fiber.StatusNoContent).SendString("deleted") },
			cfg:         []Config{{FormData: map[string]string{"foo": "bar"}}},
			status:      fiber.StatusNoContent,
			calls:       5,
			globalDelay: time.Second,
		},
		{
			name:   "options",
			method: fiber.MethodOptions,
			global: Options,
			client: (*Client).Options,
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderAllow, "GET, POST, PUT, DELETE, PATCH")
				return c.Status(fiber.StatusNoContent).SendString("")
			},
			status:      fiber.StatusNoContent,
			headerName:  fiber.HeaderAllow,
			headerValue: "GET, POST, PUT, DELETE, PATCH",
			calls:       5,
		},
		{
			name:        "patch",
			method:      fiber.MethodPatch,
			global:      Patch,
			client:      (*Client).Patch,
			handler:     clientAForm,
			cfg:         []Config{{FormData: map[string]string{"foo": "bar"}}},
			status:      fiber.StatusOK,
			body:        "bar",
			calls:       5,
			globalDelay: time.Second,
		},
		{
			name:        "query",
			method:      fiber.MethodQuery,
			global:      Query,
			client:      (*Client).Query,
			handler:     clientAEcho,
			cfg:         []Config{{Body: "query body"}},
			status:      fiber.StatusOK,
			body:        "query body",
			calls:       5,
			globalDelay: time.Second,
		},
	}

	for i := range tests {
		tc := &tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Every variant starts a server of its own, so none of them shares one.
			run := func(variant string, send func(url string, cfg ...Config) (*Response, error), delay time.Duration) {
				t.Run(variant, func(t *testing.T) {
					t.Parallel()
					addr := clientAServeTCP(t, func(app *fiber.App) { app.Add([]string{tc.method}, "/", tc.handler) })

					if delay > 0 {
						time.Sleep(delay)
					}

					for range tc.calls {
						resp, err := send("http://"+addr, tc.cfg...)
						require.NoError(t, err)
						require.Equal(t, tc.status, resp.StatusCode())
						require.Equal(t, tc.body, utils.UnsafeString(resp.RawResponse.Body()))
						if tc.headerName != "" {
							require.Equal(t, tc.headerValue, resp.Header(tc.headerName))
						}
					}
				})
			}

			run("global function", tc.global, tc.globalDelay)
			run("client", func(url string, cfg ...Config) (*Response, error) {
				return tc.client(New(), url, cfg...)
			}, 0)
		})
	}
}

func Test_Client_UserAgent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		send func(url string) (*Response, error)
		name string
		want string
	}{
		{name: "default", want: defaultUserAgent, send: func(url string) (*Response, error) { return Get(url) }},
		{
			name: "custom",
			want: "ua",
			send: func(url string) (*Response, error) { return New().SetUserAgent("ua").Get(url) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			addr := clientAServeTCP(t, func(app *fiber.App) {
				app.Get("/", func(c fiber.Ctx) error { return c.Send(c.Request().Header.UserAgent()) })
			})

			for range 5 {
				resp, err := tc.send("http://" + addr)
				clientTestARequireOK(t, resp, err, tc.want)
			}
		})
	}
}

// clientTestAFresh returns the new client an entry scenario starts from.
func clientTestAFresh(*testing.T) *Client { return New() }

func Test_Client_Header(t *testing.T) {
	t.Parallel()
	headers := clientTestBMultiValue[*Client]{
		noun:    "header",
		fresh:   clientTestAFresh,
		add:     (*Client).AddHeader,
		set:     (*Client).SetHeader,
		addMany: (*Client).AddHeaders,
		setMany: (*Client).SetHeaders,
		get:     (*Client).Header,
	}
	headers.run(t)
	headers.runOverride(t, "set header overrides every value", "set headers overrides every value")

	t.Run("set header case-insensitive", func(t *testing.T) {
		t.Parallel()
		client := New()
		client.SetHeader("foo", "bar").AddHeader("FOO", "fiber")

		require.Equal(t, []string{"bar", "fiber"}, client.Header("foo"))
	})
}

// Test_Settings_With_Server sends a request configured through its client or
// through the request itself, count times or once when count is zero, and
// checks what the handler made of it.
func Test_Settings_With_Server(t *testing.T) {
	t.Parallel()
	// headers answers with the K1 and K2 lines it was sent, in order. A failed
	// write fails the request, which the 200 check catches.
	headers := func(c fiber.Ctx) error {
		for key, value := range c.Request().Header.All() {
			if k := string(key); k == "K1" || k == "K2" {
				if _, err := c.Write(key); err != nil {
					return err
				}
				if _, err := c.Write(value); err != nil {
					return err
				}
			}
		}
		return nil
	}
	cookies := func(c fiber.Ctx) error {
		return c.SendString(c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3") + c.Cookies("k4"))
	}
	referer := func(c fiber.Ctx) error { return c.Send(c.Request().Header.Referer()) }
	userAgent := func(c fiber.Ctx) error { return c.Send(c.Request().Header.UserAgent()) }

	for _, tc := range []struct {
		handler fiber.Handler
		client  func(c *Client) // configures the client; nil to configure the request with request
		request func(r *Request)
		name    string
		want    string
		count   int
	}{
		{
			name: "client headers", handler: headers, want: "K1v1K1v11K1v22K1v33K2v2K2v22",
			client: func(c *Client) {
				c.SetHeader("k1", "v1").AddHeader("k1", "v11").AddHeaders(map[string][]string{"k1": {"v22", "v33"}}).
					SetHeaders(map[string]string{"k2": "v2"}).AddHeader("k2", "v22")
			},
		},
		{
			name: "request headers", handler: headers, want: "K1v1K1v11K1v22K1v33K2v2K2v22",
			request: func(r *Request) {
				r.SetHeader("k1", "v1").AddHeader("k1", "v11").AddHeaders(map[string][]string{"k1": {"v22", "v33"}}).
					SetHeaders(map[string]string{"k2": "v2"}).AddHeader("k2", "v22")
			},
		},
		{
			name: "client cookies", handler: cookies, want: "v1v2v3",
			client: func(c *Client) {
				c.SetCookie("k1", "v1").
					SetCookies(map[string]string{"k2": "v2", "k3": "v3", "k4": "v4"}).DelCookies("k4")
			},
		},
		{
			name: "request cookies", handler: cookies, want: "v1v2v3",
			request: func(r *Request) {
				r.SetCookie("k1", "v1").
					SetCookies(map[string]string{"k2": "v2", "k3": "v3", "k4": "v4"}).DelCookies("k4")
			},
		},
		{
			name: "client referer", handler: referer, want: "http://referer.com",
			client: func(c *Client) { c.SetReferer("http://referer.com") },
		},
		{
			name: "request referer", handler: referer, want: "http://referer.com",
			request: func(r *Request) { r.SetReferer("http://referer.com") },
		},
		{
			name: "client query params", want: "v1v2",
			handler: func(c fiber.Ctx) error { return c.SendString(c.Query("k1") + c.Query("k2")) },
			client:  func(c *Client) { c.SetParam("k1", "v1").AddParam("k2", "v2") },
		},
		{
			name: "request query string", want: "foo=bar&bar=baz",
			handler: func(c fiber.Ctx) error { return c.Send(c.Request().URI().QueryString()) },
			request: func(r *Request) { r.SetParam("foo", "bar").SetParams(map[string]string{"bar": "baz"}) },
		},
		{
			name: "request default user agent", handler: userAgent, want: defaultUserAgent, count: 5,
			request: func(*Request) {},
		},
		{
			name: "request custom user agent", handler: userAgent, want: "ua", count: 5,
			request: func(r *Request) { r.SetUserAgent("ua") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.client != nil {
				testClient(t, tc.handler, tc.client, tc.want)
				return
			}
			testRequest(t, tc.handler, tc.request, tc.want, max(tc.count, 1))
		})
	}
}

func Test_Client_Cookie(t *testing.T) {
	t.Parallel()
	clientTestBSingleValue[*Client]{
		noun:      "cookie",
		fresh:     clientTestAFresh,
		get:       (*Client).Cookie,
		set:       (*Client).SetCookie,
		setMany:   (*Client).SetCookies,
		setStruct: (*Client).SetCookiesWithStruct,
		del:       (*Client).DelCookies,
	}.run(t)
}

func Test_Client_CookieJar_Response(t *testing.T) {
	t.Parallel()
	// The jar holds k1 and k2 for example.com and k3 for example, which a request
	// to example.com does not carry. The response may set k4 before it answers.
	for _, tc := range []struct {
		set   func(c fiber.Ctx) // sets the response's cookies; nil for none
		name  string
		count int // the cookies the jar holds for example.com afterwards
	}{
		{name: "nothing set", count: 2},
		{
			name: "without expiration", count: 3,
			set: func(c fiber.Ctx) { c.Cookie(&fiber.Cookie{Name: "k4", Value: "v4"}) },
		},
		{
			name: "with expiration", count: 2,
			set: func(c fiber.Ctx) {
				c.Cookie(&fiber.Cookie{Name: "k4", Value: "v4", Expires: time.Now().Add(1 * time.Nanosecond)})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			jar := clientTestCJar(t)
			jar.SetKeyValue("example.com", "k1", "v1")
			jar.SetKeyValue("example.com", "k2", "v2")
			jar.SetKeyValue("example", "k3", "v3")

			handler := func(c fiber.Ctx) error {
				if tc.set != nil {
					tc.set(c)
				}
				return c.SendString(c.Cookies("k1") + c.Cookies("k2") + c.Cookies("k3"))
			}
			testClient(t, handler, func(c *Client) { c.SetCookieJar(jar) }, "v1v2")

			require.Len(t, jar.getCookiesByHost("example.com"), tc.count)
		})
	}

	t.Run("override cookie value", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error {
			c.Cookie(&fiber.Cookie{Name: "k1", Value: "v2"})
			return c.SendString(c.Cookies("k1") + c.Cookies("k2"))
		}

		jar := clientTestCJar(t)
		jar.SetKeyValue("example.com", "k1", "v1")
		jar.SetKeyValue("example.com", "k2", "v2")

		testClient(t, handler, func(c *Client) { c.SetCookieJar(jar) }, "v1v2")

		for _, cookie := range jar.getCookiesByHost("example.com") {
			if string(cookie.Key()) == "k1" {
				require.Equal(t, "v2", string(cookie.Value()))
			}
		}
	})

	t.Run("different domain", func(t *testing.T) {
		t.Parallel()
		handler := func(c fiber.Ctx) error { return c.SendString(c.Cookies("k1")) }

		jar := clientTestCJar(t)
		jar.SetKeyValue("example.com", "k1", "v1")

		testClient(t, handler, func(c *Client) { c.SetCookieJar(jar) }, "v1")

		require.Len(t, jar.getCookiesByHost("example.com"), 1)
		require.Empty(t, jar.getCookiesByHost("example"))
	})
}

func Test_Client_QueryParam(t *testing.T) {
	t.Parallel()
	params := clientTestBMultiValue[*Client]{
		noun:      "param",
		fresh:     clientTestAFresh,
		add:       (*Client).AddParam,
		set:       (*Client).SetParam,
		addMany:   (*Client).AddParams,
		setMany:   (*Client).SetParams,
		setStruct: (*Client).SetParamsWithStruct,
		del:       (*Client).DelParams,
		get:       (*Client).Param,
	}
	params.run(t)
	params.runStruct(t)
	params.runDel(t)
	params.runOverride(t, "set param overrides every value", "set params overrides every value")
}

func Test_Client_PathParam(t *testing.T) {
	t.Parallel()
	clientTestBSingleValue[*Client]{
		noun:      "path param",
		fresh:     clientTestAFresh,
		get:       (*Client).PathParam,
		set:       (*Client).SetPathParam,
		setMany:   (*Client).SetPathParams,
		setStruct: (*Client).SetPathParamsWithStruct,
		del:       (*Client).DelPathParams,
	}.run(t)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Client_PathParam_With_Server(t *testing.T) {
	app, dial, start := createHelperServer(t)

	app.Get("/:test", func(c fiber.Ctx) error { return c.SendString(c.Params("test")) })

	go start()

	resp, err := New().SetDial(dial).SetPathParam("path", "test").Get("http://example.com/:path")

	clientTestARequireOK(t, resp, err, "test")
}

func Test_Client_TLS(t *testing.T) {
	t.Parallel()
	// The server accepts TLS 1.3 only and the client offers TLS 1.2 at most.
	versionMismatch := func(server, client *tls.Config) {
		client.MaxVersion = tls.VersionTLS12
		server.MinVersion = tls.VersionTLS13
	}

	for _, tc := range []struct {
		tune    func(server, client *tls.Config)
		name    string
		trusted bool // whether the client is given the config that trusts the server
		works   bool
	}{
		{name: "trusted config", trusted: true, works: true},
		{name: "protocol version mismatch", tune: versionMismatch, trusted: true},
		{name: "empty TLSConfig"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, addr, clientTLSConf := clientTestAStartTLSServer(t, "tls", tc.tune)

			client := New()
			if tc.trusted {
				client.SetTLSConfig(clientTLSConf)
			}
			resp, err := client.Get("https://" + addr)

			if tc.trusted {
				require.Same(t, clientTLSConf, client.TLSConfig())
			} else {
				require.NotEqual(t, clientTLSConf, client.TLSConfig())
			}
			if tc.works {
				clientTestARequireOK(t, resp, err, "tls")
				return
			}
			require.Error(t, err)
			require.Nil(t, resp)
		})
	}
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

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Replace(t *testing.T) {
	dial := clientTestCServeGet(t, func(c fiber.Ctx) error {
		return c.SendString(string(c.Request().Header.Peek("k1")))
	})

	C().SetDial(dial)
	resp, err := Get("http://example.com")
	clientTestARequireOK(t, resp, err, "")

	r := New().SetDial(dial).SetHeader("k1", "v1")
	clean := Replace(r)
	resp, err = Get("http://example.com")
	clientTestARequireOK(t, resp, err, "v1")

	clean()

	C().SetDial(dial)
	resp, err = Get("http://example.com")
	clientTestARequireOK(t, resp, err, "")

	C().SetDial(nil)
}

// conventions:ignore parallel: legacy, sequential before the rule; make it parallel and delete this line
func Test_Replace_ConcurrentAccess(t *testing.T) {
	original := C()
	t.Cleanup(func() { defaultClient.Store(original) })

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

// clientAAny adapts a getter of a request to the form the rows of the config test use.
func clientAAny[T any](get func(req *Request) T) func(req *Request) any {
	return func(req *Request) any { return get(req) }
}

func Test_Set_Config_To_Request(t *testing.T) {
	t.Parallel()
	type ctxKey struct{}
	var key ctxKey = struct{}{}

	type payload struct {
		Foo string `json:"foo"`
	}

	body := func(req *Request) any { return req.body }

	// Each row applies cfg to a fresh request and expects got to read want back from it.
	tests := []struct {
		got  func(req *Request) any
		want any
		name string
		cfg  Config
	}{
		{
			name: "set ctx",
			cfg:  Config{Ctx: context.WithValue(context.Background(), key, "v1")},
			got:  func(req *Request) any { return req.Context().Value(key) },
			want: "v1",
		},
		{name: "set useragent", cfg: Config{UserAgent: "agent"}, got: clientAAny((*Request).UserAgent), want: "agent"},
		{name: "set referer", cfg: Config{Referer: "referer"}, got: clientAAny((*Request).Referer), want: "referer"},
		{
			name: "set header",
			cfg:  Config{Header: map[string]string{"k1": "v1"}},
			got:  func(req *Request) any { return req.Header("k1")[0] },
			want: "v1",
		},
		{
			name: "set params",
			cfg:  Config{Param: map[string]string{"k1": "v1"}},
			got:  func(req *Request) any { return req.Param("k1")[0] },
			want: "v1",
		},
		{
			name: "set cookies",
			cfg:  Config{Cookie: map[string]string{"k1": "v1"}},
			got:  func(req *Request) any { return req.Cookie("k1") },
			want: "v1",
		},
		{
			name: "set pathparam",
			cfg:  Config{PathParam: map[string]string{"k1": "v1"}},
			got:  func(req *Request) any { return req.PathParam("k1") },
			want: "v1",
		},
		{
			name: "set timeout", cfg: Config{Timeout: 1 * time.Second},
			got: clientAAny((*Request).Timeout), want: 1 * time.Second,
		},
		{name: "set maxredirects", cfg: Config{MaxRedirects: 1}, got: clientAAny((*Request).MaxRedirects), want: 1},
		{
			name: "set redirect sensitive headers", cfg: Config{RedirectSensitiveHeaders: []string{" X-Api-Key", ""}},
			got: clientAAny((*Request).RedirectSensitiveHeaders), want: []string{"X-Api-Key"},
		},
		{
			// a string body is stored as raw bytes
			name: "set body string", cfg: Config{Body: "test"}, got: body, want: []byte("test"),
		},
		{name: "set body byte", cfg: Config{Body: []byte("test")}, got: body, want: []byte("test")},
		{
			// any other body is kept as given, to be JSON-encoded when the request is sent
			name: "set body json", cfg: Config{Body: payload{Foo: "bar"}}, got: body, want: payload{Foo: "bar"},
		},
		{
			name: "set body map", cfg: Config{Body: map[string]string{"foo": "bar"}},
			got: body, want: map[string]string{"foo": "bar"},
		},
		{
			name: "set file",
			cfg:  Config{File: []*File{{name: "test", path: "path"}}},
			got:  func(req *Request) any { return req.File("test").path },
			want: "path",
		},
	}

	for i := range tests {
		tc := &tests[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := AcquireRequest()

			setConfigToRequest(req, tc.cfg)

			require.Equal(t, tc.want, tc.got(req))
		})
	}

	t.Run("a config without redirect sensitive headers leaves the request's", func(t *testing.T) {
		t.Parallel()
		for _, names := range [][]string{nil, {}} {
			req := AcquireRequest().SetRedirectSensitiveHeaders("X-Api-Key")

			setConfigToRequest(req, Config{MaxRedirects: 1, RedirectSensitiveHeaders: names})

			require.Equal(t, []string{"X-Api-Key"}, req.RedirectSensitiveHeaders())
		}
	})
}

func Test_Client_SetProxyURL(t *testing.T) {
	t.Parallel()
	app, dial, start := createHelperServer(t)
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(c.Get("isProxy")) })

	go start()

	fasthttpClient := &fasthttp.Client{Dial: dial, NoDefaultUserAgentHeader: true, DisablePathNormalizing: true}

	t.Cleanup(func() { require.NoError(t, app.Shutdown()) })

	proxyAddr := clientAServeTCP(t, func(proxy *fiber.App) {
		proxy.Use("*", func(c fiber.Ctx) error {
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
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		client := New()

		require.NoError(t, client.SetProxyURL(proxyAddr))

		resp, err := client.Get("http://localhost:3000")
		require.NoError(t, err)

		require.Equal(t, 200, resp.StatusCode())
		require.Equal(t, "true", string(resp.Body()))
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()
		client := New()

		require.Error(t, client.SetProxyURL("ftp://127.0.0.1:8080"))
	})
}

func Test_Client_SetRetryConfig(t *testing.T) {
	t.Parallel()
	retryConfig := &retry.Config{InitialInterval: 1 * time.Second, MaxRetryCount: 3}

	core, client, req := newCore(), New(), AcquireRequest()
	req.SetURL("http://exampleretry.com")
	client.SetRetryConfig(retryConfig)
	_, err := core.execute(context.Background(), client, req)

	require.Error(t, err)
	require.Equal(t, retryConfig.InitialInterval, client.RetryConfig().InitialInterval)
	require.Equal(t, retryConfig.MaxRetryCount, client.RetryConfig().MaxRetryCount)
}

func Benchmark_Client_Request(b *testing.B) {
	client := New().SetDial(clientTestCServeGet(b, func(c fiber.Ctx) error { return c.SendString("hello world") }))

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
	client := New().SetDial(clientTestCServeGet(b, func(c fiber.Ctx) error { return c.SendString("hello world") })).
		SetRetryConfig(&RetryConfig{MaxRetryCount: 2})

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
	client := New().SetDial(clientTestCServeGet(b, func(c fiber.Ctx) error { return c.SendString("hello world") }))

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

		req := AcquireRequest().SetClient(client).SetURL("http://example.com").SetMethod(fiber.MethodPost).
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
		lbClient := &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{hostClient}}
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

	over := defaultMaxResponseBodySize + 1
	for _, tc := range []struct{ name, response string }{
		{name: "length", response: "HTTP/1.1 200 OK\r\nContent-Length: " + strconv.Itoa(over) + "\r\n\r\nok"},
		{
			name: "chunk size",
			response: "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n" +
				strconv.FormatInt(int64(over), 16) + "\r\nok",
		},
	} {
		t.Run("a new client refuses a declared "+tc.name+" over the limit", func(t *testing.T) {
			t.Parallel()
			client := New().SetDial(answerWith(t, tc.response))
			require.Equal(t, defaultMaxResponseBodySize, client.MaxResponseBodySize())

			_, err := client.Get("http://example.com/")
			require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
		})
	}

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
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Response().Header.Add(fiber.HeaderSetCookie, "a=1; Max-Age=-1")
			c.Response().Header.Add(fiber.HeaderSetCookie, "b=2; Expires=not-a-date")
			c.Response().Header.Add(fiber.HeaderSetCookie, "c=3")
			return c.SendString("ok")
		})
	})

	resp := clientTestBClientGet(t, client, "http://example.com/")
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
	client := clientAOKClient(t)
	client.AddResponseHook(func(_ *Client, _ *Response, _ *Request) error { return errors.New("hook failed") })

	req := clientTestBRequest(t)
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
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
			return c.SendStreamWriter(func(w *bufio.Writer) {
				_, _ = w.WriteString("chunk") //nolint:errcheck // not needed
				_ = w.Flush()                 //nolint:errcheck // not needed
			})
		})
	})
	client.SetStreamResponseBody(true).Debug()

	resp := clientTestBClientGet(t, client, "http://example.com/")
	clientTestBRequireStream(t, resp, "chunk", "the debug log must not consume the streamed body")
}

func Test_CookieJar_MaxAge(t *testing.T) {
	t.Parallel()
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/set", func(c fiber.Ctx) error {
			c.Response().Header.Add(fiber.HeaderSetCookie, "sid=abc; Max-Age=1")
			return c.SendString("set")
		})
		app.Get("/del", func(c fiber.Ctx) error {
			c.Response().Header.Add(fiber.HeaderSetCookie, "sid=abc; Max-Age=0")
			return c.SendString("del")
		})
		app.Get("/echo", func(c fiber.Ctx) error { return c.SendString(c.Cookies("sid")) })
	})

	jar := AcquireCookieJar()
	defer ReleaseCookieJar(jar)
	client.SetCookieJar(jar)

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

// requireOwnedRequestReleased sends GETs for url through client, each of which
// must fail with "boom" after a hook of the test recorded its request in seen,
// until the client hands a pooled request out twice. Only a release puts one
// back where Acquire can find it, so a repeat is proof that a failing send
// releases; with the release gone no send returns one to the pool, so no
// pointer can ever repeat however many follow.
//
// Which pointer comes back is not assertable: sync.Pool promises no relation
// between what is put and what a later Get returns, and under -race with these
// tests in parallel a specific request does not come back to a specific caller.
func requireOwnedRequestReleased(t *testing.T, client *Client, url string, seen **Request) {
	t.Helper()

	handedOut := make(map[*Request]struct{}, 64)
	for range 64 {
		*seen = nil
		_, err := client.Get(url)
		req := *seen
		require.ErrorContains(t, err, "boom")
		require.NotNil(t, req)
		if _, repeat := handedOut[req]; repeat {
			return
		}
		handedOut[req] = struct{}{}
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
	requireOwnedRequestReleased(t, client, "http://example.com", &seen)
}

func Test_Client_DispatchError_ReleasesOwnedRequest(t *testing.T) {
	t.Parallel()
	var seen *Request
	client := New().SetDial(func(_ string) (net.Conn, error) { return nil, errors.New("boom") })
	client.AddRequestHook(func(_ *Client, req *Request) error {
		seen = req
		return nil
	})

	// The send never produces a response, so the helper's own request is
	// stranded unless this path releases it too.
	requireOwnedRequestReleased(t, client, "http://example.com", &seen)
}

func Test_Client_AfterHookError_ReleasesOwnedRequest(t *testing.T) {
	t.Parallel()
	client := clientAOKClient(t)

	var seen *Request
	client.AddResponseHook(func(_ *Client, _ *Response, req *Request) error {
		seen = req
		require.True(t, req.clientOwned)
		return errors.New("boom")
	})

	// No response reaches the caller, so the request has to go back here.
	requireOwnedRequestReleased(t, client, "http://example.com/", &seen)
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

			expires := time.Now().Add(time.Hour).UTC().Format(time.RFC1123)
			clientTestCReceive(jar, "example.com", "/", "sid=x; "+tc.maxAge+"; Expires="+expires)

			// RFC 6265 §5.2.2: Max-Age wins over Expires, but only when it is an integer.
			cookies := jarGet(t, jar, "http://example.com/")
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

			clientTestCReceive(jar, "example.com", receivedPath, tc.setCookie)

			tc.check(t, jar)
		})
	}
}

// jarGet returns the cookies jar would send to rawURL.
func jarGet(t *testing.T, jar *CookieJar, rawURL string) []*fasthttp.Cookie {
	t.Helper()

	return jar.Get(clientTestCURI(t, rawURL))
}

func Test_Request_CancelledContext_NoRace(t *testing.T) {
	t.Parallel()
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			// Slow enough that the response never beats the canceled context.
			time.Sleep(50 * time.Millisecond)
			return c.SendString("ok")
		})
	})

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
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error { return c.SendString(strings.Join(c.GetReqHeaders()["X-Level"], "|")) })
	})
	client.SetHeader("X-Level", "client")

	resp, err := client.Get("http://example.com/")
	require.NoError(t, err)
	require.Equal(t, "client", resp.String())
	resp.Close()

	req := clientTestBRequest(t)
	req.SetClient(client).SetURL("http://example.com/").SetHeader("X-Level", "request")
	resp, err = req.Send()
	require.NoError(t, err)
	defer resp.Close()
	require.Equal(t, "request", resp.String(), "a request-level header overrides the client-level one")
}

func Test_Request_Params_KeepInsertionOrder(t *testing.T) {
	t.Parallel()
	req := clientTestBRequest(t)
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
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error {
			c.Set("X-Test", "value")
			return c.SendString("ok")
		})
	})

	resp := clientTestBClientGet(t, client, "http://example.com/")

	header := resp.Header("X-Test")
	resp.RawResponse.Header.Set("X-Test", "other")
	require.Equal(t, "value", header, "the returned string must not alias the header storage")
}

func Test_Request_Resend_DoesNotAccumulateHeaders(t *testing.T) {
	t.Parallel()
	client := clientTestAClient(t, func(app *fiber.App) {
		app.Get("/", func(c fiber.Ctx) error { return c.SendString(strconv.Itoa(len(c.GetReqHeaders()["X-A"]))) })
	})

	req := clientTestBRequest(t)
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
		app.Get("/cookies", func(c fiber.Ctx) error { return c.SendString(c.Get(fiber.HeaderCookie)) })
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
		req := clientTestBRequest(t).SetClient(New().SetDial(server.dial()).SetCookieJar(clientTestCJar(t)))

		send(t, req, "http://a.example/login")
		require.Equal(t, "sid=secret", send(t, req, "http://a.example/cookies"))
		require.Empty(t, send(t, req, "http://b.example/cookies"))
	})

	t.Run("a jar cookie the server expired is not sent again", func(t *testing.T) {
		t.Parallel()
		req := clientTestBRequest(t).SetClient(New().SetDial(server.dial()).SetCookieJar(clientTestCJar(t)))

		send(t, req, "http://a.example/login")
		send(t, req, "http://a.example/logout")
		require.Empty(t, send(t, req, "http://a.example/cookies"))
	})

	t.Run("a deleted cookie is not sent again", func(t *testing.T) {
		t.Parallel()
		client := New().SetDial(server.dial()).SetCookie("a", "1")
		req := clientTestBRequest(t).SetClient(client).SetCookie("b", "2")

		require.Equal(t, "a=1; b=2", send(t, req, "http://a.example/cookies"))
		require.Equal(t, "a=1", send(t, req.DelCookies("b"), "http://a.example/cookies"))
		client.DelCookies("a")
		require.Empty(t, send(t, req, "http://a.example/cookies"))
	})

	t.Run("a Cookie header set on the request is kept", func(t *testing.T) {
		t.Parallel()
		req := clientTestBRequest(t).SetClient(New().SetDial(server.dial())).SetHeader(fiber.HeaderCookie, "a=1")

		for range 2 {
			require.Equal(t, "a=1", send(t, req, "http://a.example/cookies"))
		}
	})
}

func Test_Client_UppercaseScheme(t *testing.T) {
	t.Parallel()
	client := clientAOKClient(t)

	resp := clientTestBClientGet(t, client, "HTTP://example.com/")
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
