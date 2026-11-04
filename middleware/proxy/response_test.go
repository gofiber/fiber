package proxy

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/internal/apptest"
)

// Test_Proxy_UpstreamBodyLimit checks that a streamed body delivers what its limit allows and fails with
// fasthttp.ErrBodyTooLarge in place of the byte past it, however the reads cut the body up, and keeps failing.
func Test_Proxy_UpstreamBodyLimit(t *testing.T) {
	t.Parallel()
	const limit = 10
	for _, tc := range []struct {
		name  string
		limit int64
		size  int
		over  bool
	}{
		{name: "under the limit", limit: limit, size: limit - 1},
		{name: "at the limit", limit: limit, size: limit},
		{name: "one past the limit", limit: limit, size: limit + 1, over: true},
		{name: "far past the limit", limit: limit, size: 10 * limit, over: true},
		{name: "empty", limit: limit},
		{name: "no limit", size: 10 * limit},
		{name: "negative limit", limit: -1, size: 10 * limit},
		{name: "the largest limit", limit: math.MaxInt64, size: 10 * limit},
	} {
		for reads, wrap := range map[string]func(io.Reader) io.Reader{
			"whole":         func(r io.Reader) io.Reader { return r },
			"byte by byte":  iotest.OneByteReader,
			"data with EOF": iotest.DataErrReader,
		} {
			t.Run(tc.name+", "+reads, func(t *testing.T) {
				t.Parallel()
				body := &upstreamBody{Reader: wrap(strings.NewReader(strings.Repeat("x", tc.size))), limit: tc.limit}

				got, err := io.ReadAll(body)
				if !tc.over {
					require.NoError(t, err)
					require.Len(t, got, tc.size)
					return
				}
				require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
				require.Len(t, got, int(tc.limit), "what the limit allows, and no byte more")
				n, again := body.Read(make([]byte, 8))
				require.Zero(t, n)
				require.ErrorIs(t, again, fasthttp.ErrBodyTooLarge, "the failure is kept")
			})
		}
	}
}

// Test_Proxy_StreamedBodyLimit pins the size a body that Balancer streams is held to.
func Test_Proxy_StreamedBodyLimit(t *testing.T) {
	t.Parallel()
	custom := &fasthttp.LBClient{}
	for name, tc := range map[string]struct {
		config Config
		want   int
	}{
		"servers, size left out":           {config: Config{Servers: []string{"http://upstream.example"}}, want: defaultMaxResponseBodySize},
		"servers, size set":                {config: Config{Servers: []string{"http://upstream.example"}, MaxResponseBodySize: 4096}, want: 4096},
		"servers, no bound":                {config: Config{Servers: []string{"http://upstream.example"}, MaxResponseBodySize: -1}},
		"client of the caller's, left out": {config: Config{Client: custom}},
		"client of the caller's, set":      {config: Config{Client: custom, MaxResponseBodySize: 4096}, want: 4096},
		"client of the caller's, no bound": {config: Config{Client: custom, MaxResponseBodySize: -1}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := configDefault(tc.config)
			require.Equal(t, tc.want, streamedBodyLimit(&cfg, tc.config.MaxResponseBodySize))
		})
	}
}

// proxyTestSizedUpstream serves a body of the size the request names in X-Size, declaring its length unless it
// sends X-Chunked, and returns its address.
func proxyTestSizedUpstream(t *testing.T) string {
	t.Helper()

	upstream := fiber.New()
	upstream.Get("/", func(c fiber.Ctx) error {
		size, err := strconv.Atoi(c.Get("X-Size"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "X-Size: "+err.Error())
		}
		if c.Get("X-Chunked") == "" {
			return c.SendStream(strings.NewReader(strings.Repeat("x", size)), size)
		}
		return c.SendStreamWriter(func(w *bufio.Writer) {
			for left := size; left > 0; {
				n := min(left, 1024)
				if _, err := w.WriteString(strings.Repeat("x", n)); err != nil || w.Flush() != nil {
					return
				}
				left -= n
			}
		})
	})
	return listenUpstream(t, upstream)
}

// proxyTestStreamedHandlers returns every way a response reaches the proxy as a stream, each holding it to bound
// bytes, none when that is zero: Balancer with a response set to stream, Balancer with a client of the caller's
// that streams, Do with a client that streams or a response set to stream, and Forward with a client that streams.
func proxyTestStreamedHandlers(addr string, bound int) map[string]fiber.Handler {
	policy := proxyTestAllowPrivatePolicy()
	configured := bound // Config's way to say none
	if bound == 0 {
		configured = -1
	}
	send := func(cli *fasthttp.Client, req *fasthttp.Request, res *fasthttp.Response, _ *url.URL) error {
		return cli.Do(req, res)
	}
	do := func(streamed bool, cli *fasthttp.Client) fiber.Handler {
		return func(c fiber.Ctx) error {
			c.Response().StreamBody = streamed
			return doActionWithPolicy(c, "http://"+addr, normalizePolicy(policy), nil, send, cli)
		}
	}
	return map[string]fiber.Handler{
		"Balancer, response streamed": Balancer(Config{
			Servers: []string{addr}, SecurityPolicy: &policy, MaxResponseBodySize: configured,
			ModifyRequest: func(c fiber.Ctx) error { c.Response().StreamBody = true; return nil },
		}),
		"Balancer, streaming client": Balancer(Config{
			Client:         &fasthttp.LBClient{Clients: []fasthttp.BalancingClient{&fasthttp.HostClient{Addr: addr, StreamResponseBody: true}}},
			SecurityPolicy: &policy, MaxResponseBodySize: configured,
		}),
		"Do, streaming client":  do(false, &fasthttp.Client{StreamResponseBody: true, MaxResponseBodySize: bound}),
		"Do, response streamed": do(true, &fasthttp.Client{MaxResponseBodySize: bound}),
		"Forward, streaming client": func(c fiber.Ctx) error {
			return forwardWithRealIP(c, "http://"+addr, normalizePolicy(policy), &fasthttp.Client{StreamResponseBody: true, MaxResponseBodySize: bound})
		},
	}
}

// Test_Proxy_StreamedBodyBound checks that a body the proxy streams is held to the bound its client was given,
// which fasthttp ignores for a body it hands over as a stream: one whose declared length is past it is refused
// before any of it is sent, as a body read into memory would be, one that goes past it without declaring its
// length is aborted, and a body within the bound, or any body when there is none, arrives whole.
func Test_Proxy_StreamedBodyBound(t *testing.T) {
	t.Parallel()
	const bound = 4096
	addr := proxyTestSizedUpstream(t)

	const (
		delivered = iota
		refused
		aborted
	)
	for _, tc := range []struct {
		name    string
		bound   int
		size    int
		want    int
		chunked bool
	}{
		{name: "declared, at the bound", bound: bound, size: bound, want: delivered},
		{name: "declared, past the bound", bound: bound, size: bound + 1, want: refused},
		{name: "chunked, at the bound", bound: bound, size: bound, chunked: true, want: delivered},
		{name: "chunked, past the bound", bound: bound, size: bound + 1, chunked: true, want: aborted},
		{name: "declared, no bound", size: 4 * bound, want: delivered},
		{name: "chunked, no bound", size: 4 * bound, chunked: true, want: delivered},
	} {
		for name, handler := range proxyTestStreamedHandlers(addr, tc.bound) {
			t.Run(tc.name+", "+name, func(t *testing.T) {
				t.Parallel()
				var handled error
				app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
					handled = err
					return c.SendStatus(fiber.StatusBadGateway)
				}})
				app.Get("/", handler)
				chunked := ""
				if tc.chunked {
					chunked = "1"
				}
				req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Size", strconv.Itoa(tc.size)), apptest.WithHeaderIf("X-Chunked", chunked))

				switch tc.want {
				case delivered:
					apptest.ExpectBody(t, app, req, fiber.StatusOK, strings.Repeat("x", tc.size))
					require.NoError(t, handled)
				case refused:
					require.NotContains(t, apptest.Body(t, apptest.Expect(t, app, req, fiber.StatusBadGateway)), "x", "none of the body was sent")
					require.ErrorIs(t, handled, fasthttp.ErrBodyTooLarge)
				case aborted:
					_, err := app.Test(req)
					require.ErrorIs(t, err, fasthttp.ErrBodyTooLarge)
					require.NoError(t, handled, "the response had started")
				}
			})
		}
	}
}

// Test_Proxy_StreamedBodyBound_ModifyResponse checks that a hook that reads the body of a streamed response,
// as ModifyResponse may, never takes in more than the bound: the stream fails there instead of filling memory.
func Test_Proxy_StreamedBodyBound_ModifyResponse(t *testing.T) {
	t.Parallel()
	const bound = 4096
	addr := proxyTestSizedUpstream(t)
	policy := proxyTestAllowPrivatePolicy()

	taken := make(chan int, 1)
	app := fiber.New()
	app.Get("/", Balancer(Config{
		Servers: []string{addr}, SecurityPolicy: &policy, MaxResponseBodySize: bound,
		ModifyRequest:  func(c fiber.Ctx) error { c.Response().StreamBody = true; return nil },
		ModifyResponse: func(c fiber.Ctx) error { taken <- len(c.Response().Body()); return nil },
	}))

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader("X-Size", strconv.Itoa(8*bound)), apptest.WithHeader("X-Chunked", "1"))
	// Not a 200 carrying fasthttp's error text in place of the body the hook could not read.
	apptest.Expect(t, app, req, fiber.StatusInternalServerError)
	require.LessOrEqual(t, apptest.Recv(t, taken, time.Second), bound)
}

// Test_Proxy_Do_StreamedResponseDefaultBound checks that the default client holds a response set to stream to its
// 100 MiB bound as well: one that declares more is refused as soon as its header is read, without waiting for a
// body that never comes.
func Test_Proxy_Do_StreamedResponseDefaultBound(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(fiber.HeaderContentLength, strconv.Itoa(2*defaultMaxResponseBodySize))
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "start")
		if http.NewResponseController(w).Flush() == nil {
			<-release
		}
	}))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() { close(release) })

	var handled error
	app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
		handled = err
		return c.SendStatus(fiber.StatusBadGateway)
	}})
	app.Get("/", func(c fiber.Ctx) error {
		c.Response().StreamBody = true
		return Do(c, upstream.URL)
	})

	apptest.Expect(t, app, "/", fiber.StatusBadGateway)
	require.ErrorIs(t, handled, fasthttp.ErrBodyTooLarge)
}
