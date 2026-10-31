package etag

import (
	"bytes"
	"errors"
	"hash/crc32"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/utils/v2"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// etagTestHello is the handler most tests serve. Its body hashes to the ETag
// "13-1831710635".
func etagTestHello(c fiber.Ctx) error {
	return c.SendString("Hello, World!")
}

func Test_ETag_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

func Test_ETag_SSE(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextEventStream)
		return c.SendString("data: hello\n\n")
	})

	resp := apptest.Expect(t, app, "/", fiber.StatusOK)
	require.Empty(t, resp.Header.Get(fiber.HeaderETag))

	require.Equal(t, "data: hello\n\n", apptest.Body(t, resp))
}

// failOnReadStream is a body stream whose Read fails, proving the middleware
// never materializes it.
type failOnReadStream struct{ readCalls int }

func (s *failOnReadStream) Read(_ []byte) (int, error) {
	s.readCalls++
	return 0, errors.New("stream should not be read")
}

// A real SSE stream must pass through untouched: hashing the body would
// materialize the stream and break real-time delivery. The middleware must add
// no ETag and leave the response a stream. The content type carries a charset
// param to exercise the media-type strip in isEventStream.
func Test_ETag_SSE_Stream(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())

	stream := &failOnReadStream{}
	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextEventStream+"; charset=utf-8")
		c.Status(fiber.StatusOK)
		c.Response().SetBodyStream(stream, -1)
		return nil
	})

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")
	app.Handler()(fctx)

	require.True(t, fctx.Response.IsBodyStream())
	require.Zero(t, stream.readCalls)
	require.Empty(t, string(fctx.Response.Header.Peek(fiber.HeaderETag)))
}

// A stream of unknown length may never end, so hashing it would hold it; a sized one is still hashed.
func Test_ETag_StreamOfUnknownLength(t *testing.T) {
	t.Parallel()
	for name, size := range map[string]int{"unknown length": -1, "declared length": 5} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New())

			stream := &failOnReadStream{}
			app.Get("/", func(c fiber.Ctx) error {
				if size < 0 {
					c.Response().SetBodyStream(stream, size)
				} else {
					c.Response().SetBodyStream(bytes.NewReader([]byte("hello")), size)
				}
				return nil
			})

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")
			app.Handler()(fctx)

			etag := string(fctx.Response.Header.Peek(fiber.HeaderETag))
			if size < 0 {
				require.True(t, fctx.Response.IsBodyStream())
				require.Zero(t, stream.readCalls)
				require.Empty(t, etag)
				return
			}
			require.Equal(t, string(Generate([]byte("hello"))), etag)
		})
	}
}

// Body keeps a read error as the body, and an outer cache would store it under this ETag.
func Test_ETag_SizedStreamIsTaggedOnlyWhole(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		stream  func() io.Reader
		maxRead int
	}{
		"fails": {
			maxRead: 4,
			stream: func() io.Reader {
				return io.MultiReader(strings.NewReader("part"), iotest.ErrReader(errors.New("upstream reset")))
			},
		},
		"longer than declared": {
			maxRead: 64 << 10, // short of the whole MiB
			stream:  func() io.Reader { return strings.NewReader(strings.Repeat("x", 1<<20)) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New())

			var read strings.Builder
			app.Get("/", func(c fiber.Ctx) error { return c.SendStream(io.TeeReader(tc.stream(), &read), 10) })

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")
			app.Handler()(fctx)

			require.Empty(t, fctx.Response.Header.Peek(fiber.HeaderETag))
			require.True(t, fctx.Response.IsBodyStream())
			require.Equal(t, 10, fctx.Response.Header.ContentLength())
			require.LessOrEqual(t, read.Len(), tc.maxRead)
		})
	}
}

// The middleware leaves a handler error, a status other than 200 and a response
// without a body alone.
func Test_ETag_PassThrough(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		handler fiber.Handler
		name    string
		status  int
	}{
		{func(_ fiber.Ctx) error { return fiber.ErrForbidden }, "skip error", fiber.StatusForbidden},
		{func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }, "not status OK", fiber.StatusCreated},
		{func(_ fiber.Ctx) error { return nil }, "no body", fiber.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New())
			app.Get("/", tc.handler)

			apptest.Expect(t, app, "/", tc.status)
		})
	}
}

// A GET with If-None-Match against a generated, a weak and a handler-set ETag:
// it is answered with the page unless the tag the client sends matches.
func Test_ETag_ConditionalGet(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		handler  fiber.Handler
		name     string
		tag      string // the ETag of the response, which matches when sent back
		nonMatch string
		cfg      []Config
	}{
		{name: "new etag", handler: etagTestHello, tag: `"13-1831710635"`, nonMatch: `"non-match"`},
		{name: "weak etag", cfg: []Config{{Weak: true}}, handler: etagTestHello, tag: `W/"13-1831710635"`, nonMatch: `W/"non-match"`},
		{
			name: "custom etag",
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderETag, `"custom"`)
				if bytes.Equal(c.Request().Header.Peek(fiber.HeaderIfNoneMatch), []byte(`"custom"`)) {
					return c.SendStatus(fiber.StatusNotModified)
				}
				return c.SendString("Hello, World!")
			},
			tag:      `"custom"`,
			nonMatch: `"non-match"`,
		},
	} {
		t.Run(variant.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name        string
				ifNoneMatch string
				status      int
			}{
				{name: "without HeaderIfNoneMatch", status: fiber.StatusOK},
				{name: "with HeaderIfNoneMatch and not matched", ifNoneMatch: variant.nonMatch, status: fiber.StatusOK},
				{name: "with HeaderIfNoneMatch and matched", ifNoneMatch: variant.tag, status: fiber.StatusNotModified},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					app := fiber.New()
					app.Use(New(variant.cfg...))
					app.Get("/", variant.handler)

					ifNoneMatch := apptest.WithHeaderIf(fiber.HeaderIfNoneMatch, tc.ifNoneMatch)
					req := apptest.Req(fiber.MethodGet, "/", ifNoneMatch)
					resp := apptest.Expect(t, app, req, tc.status)

					if tc.status == fiber.StatusOK {
						require.Equal(t, variant.tag, resp.Header.Get(fiber.HeaderETag))
						return
					}

					require.Empty(t, apptest.BodyBytes(t, resp))
				})
			}
		})
	}
}

func Test_ETag_CustomEtagPut(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Put("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, `"custom"`)
		if !bytes.Equal(c.Request().Header.Peek(fiber.HeaderIfMatch), []byte(`"custom"`)) {
			return c.SendStatus(fiber.StatusPreconditionFailed)
		}
		return c.SendString("Hello, World!")
	})

	req := apptest.Req(fiber.MethodPut, "/", apptest.WithHeader(fiber.HeaderIfMatch, `"non-match"`))
	apptest.Expect(t, app, req, fiber.StatusPreconditionFailed)
}

func Benchmark_Etag(b *testing.B) {
	app := fiber.New()
	app.Use(New())
	app.Get("/", etagTestHello)
	h := app.Handler()

	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	b.ReportAllocs()

	for b.Loop() {
		h(fctx)
	}

	require.Equal(b, 200, fctx.Response.Header.StatusCode())
	require.Equal(b, `"13-1831710635"`, string(fctx.Response.Header.Peek(fiber.HeaderETag)))
}

func Test_ETag_WeakComparison(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		ifNoneMatch    string
		weak           bool
		expectedStatus int
	}{
		{name: "weak client tag matches strong server tag", ifNoneMatch: `W/"13-1831710635"`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "strong client tag matches weak server tag", ifNoneMatch: `"13-1831710635"`, weak: true, expectedStatus: fiber.StatusNotModified},
		{name: "match in list after weak tag", ifNoneMatch: `W/"non-match", "13-1831710635"`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "weak match in list", ifNoneMatch: `"non-match", W/"13-1831710635"`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "wildcard", ifNoneMatch: `*`, weak: false, expectedStatus: fiber.StatusNotModified},
		{name: "no match in list", ifNoneMatch: `W/"non-match", "other"`, weak: false, expectedStatus: fiber.StatusOK},
		{name: "unquoted tag never matches", ifNoneMatch: `13-1831710635`, weak: false, expectedStatus: fiber.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New(Config{Weak: tc.weak}))
			app.Get("/", etagTestHello)

			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderIfNoneMatch, tc.ifNoneMatch))
			resp := apptest.Expect(t, app, req, tc.expectedStatus)

			expectedTag := `"13-1831710635"`
			if tc.weak {
				expectedTag = `W/"13-1831710635"`
			}
			require.Equal(t, expectedTag, resp.Header.Get(fiber.HeaderETag))

			if tc.expectedStatus == fiber.StatusNotModified {
				b := apptest.BodyBytes(t, resp)
				require.Empty(t, b)
			}
		})
	}
}

func Test_ETag_IfNoneMatchOnlyForGetHead(t *testing.T) {
	t.Parallel()
	// RFC 9110 §13.1.2 reserves 304 for GET and HEAD.
	testCases := []struct {
		method         string
		expectedStatus int
	}{
		{method: fiber.MethodGet, expectedStatus: fiber.StatusNotModified},
		{method: fiber.MethodHead, expectedStatus: fiber.StatusNotModified},
		{method: fiber.MethodPost, expectedStatus: fiber.StatusOK},
		{method: fiber.MethodPut, expectedStatus: fiber.StatusOK},
		{method: fiber.MethodPatch, expectedStatus: fiber.StatusOK},
		{method: fiber.MethodDelete, expectedStatus: fiber.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.method, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(New())
			app.All("/", etagTestHello)

			req := apptest.Req(tc.method, "/", apptest.WithHeader(fiber.HeaderIfNoneMatch, `"13-1831710635"`))
			resp := apptest.Expect(t, app, req, tc.expectedStatus)
			require.Equal(t, `"13-1831710635"`, resp.Header.Get(fiber.HeaderETag))

			body := apptest.BodyBytes(t, resp)
			if tc.expectedStatus == fiber.StatusOK {
				require.Equal(t, "Hello, World!", string(body))
			} else {
				require.Empty(t, body)
			}
		})
	}
}

func Test_ETag_SplitIfNoneMatch(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/", etagTestHello)

	resp := apptest.Do(t, app, "/")
	generated := resp.Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, generated)

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Add(fiber.HeaderIfNoneMatch, `"other"`)
	req.Header.Add(fiber.HeaderIfNoneMatch, generated)
	apptest.Expect(t, app, req, fiber.StatusNotModified)
}

// Test_ETag_HandlerETagWithoutNormalizing covers a handler's own ETag under
// DisableHeaderNormalizing, set through c.Set and under every spelling of the
// name: the middleware must keep it and add no second ETag line.
func Test_ETag_HandlerETagWithoutNormalizing(t *testing.T) {
	t.Parallel()
	// Without a spelling the handler sets its ETag through c.Set.
	for _, tc := range []struct{ name, spelling, etag string }{
		{"c.Set", "", `"custom"`},
		{"ETag", "ETag", `"upstream"`},
		{"Etag", "Etag", `"upstream"`},
		{"etag", "etag", `"upstream"`},
		{"ETAG", "ETAG", `"upstream"`},
		{"eTaG", "eTaG", `"upstream"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(fiber.Config{DisableHeaderNormalizing: true})
			app.Use(New())
			app.Get("/", func(c fiber.Ctx) error {
				if tc.spelling == "" {
					c.Set("ETag", tc.etag)
				} else {
					c.Response().Header.Set(tc.spelling, tc.etag)
				}
				return c.SendString("Hello, World!")
			})

			fctx := &fasthttp.RequestCtx{}
			fctx.Request.SetRequestURI("/")
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.Header.DisableNormalizing()
			fctx.Response.Header.DisableNormalizing()
			app.Handler()(fctx)

			var lines []string
			for k, v := range fctx.Response.Header.All() {
				if utils.EqualFold(string(k), fiber.HeaderETag) {
					lines = append(lines, string(v))
				}
			}
			require.Equal(t, []string{tc.etag}, lines)
		})
	}
}

func Test_Generate_MatchesCRC(t *testing.T) {
	t.Parallel()
	tab := crc32.MakeTable(0xD5828281)
	r := rand.New(rand.NewPCG(1, 2))
	check := func(n int) {
		body := make([]byte, n)
		for i := range body {
			body[i] = byte(r.UintN(256))
		}
		want := `"` + strconv.Itoa(n) + "-" + strconv.FormatUint(uint64(crc32.Checksum(body, tab)), 10) + `"`
		require.Equal(t, want, string(Generate(body)), "length %d", n)
	}
	for n := range 300 {
		check(n)
	}
	for _, n := range []int{1 << 10, 4<<10 + 3, 64<<10 - 1} {
		check(n)
	}
}

func Benchmark_Generate(b *testing.B) {
	for _, size := range []int{13, 100, 1 << 10, 10 << 10, 100 << 10} {
		body := bytes.Repeat([]byte("0123456789abcdef"), size/16+1)[:size]
		b.Run(utils.FormatInt(int64(size))+"B", func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				_ = Generate(body)
			}
		})
	}
}
