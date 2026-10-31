package compress

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/internal/apptest"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var filedata []byte

var testConfig = fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true}

func init() {
	dat, err := os.ReadFile("../../README.md")
	if err != nil {
		panic(err)
	}
	filedata = dat
}

// compressTestApp returns an app that serves handler at GET / behind the
// compress middleware, configured with cfg when one is given.
func compressTestApp(handler fiber.Handler, cfg ...Config) *fiber.App {
	app := fiber.New()
	app.Use(New(cfg...))
	app.Get("/", handler)
	return app
}

// compressTestGet sends GET / to app, with Accept-Encoding set to
// acceptEncoding unless that is empty, and returns the response.
func compressTestGet(t *testing.T, app *fiber.App, acceptEncoding string, opts ...apptest.Option) *http.Response {
	t.Helper()

	req := apptest.Req(fiber.MethodGet, "/", append(opts, apptest.WithHeaderIf(fiber.HeaderAcceptEncoding, acceptEncoding))...)
	return apptest.Do(t, app, req, testConfig)
}

// acceptGzip makes a request accept gzip.
var acceptGzip = apptest.WithHeader(fiber.HeaderAcceptEncoding, "gzip")

// compressTestSendReadme is the handler of the tests that need a response large
// enough to be compressed: it sends the README as text.
func compressTestSendReadme(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
	return c.Send(filedata)
}

// compressTestSendUntypedReadme sends the README and leaves the Content-Type to
// its default.
func compressTestSendUntypedReadme(c fiber.Ctx) error {
	return c.Send(filedata)
}

// compressTestExpectEncoding asserts that resp is a 200 response whose
// Content-Encoding is encoding ("" for none) and whose body is smaller than the
// README when it is encoded, as large as the README when it is not.
func compressTestExpectEncoding(t *testing.T, resp *http.Response, encoding string) {
	t.Helper()

	require.Equal(t, fiber.StatusOK, resp.StatusCode, "Status code")
	require.Equal(t, encoding, resp.Header.Get(fiber.HeaderContentEncoding))

	body := apptest.BodyBytes(t, resp)
	if encoding == "" {
		// Validate the file size is not shrunk
		require.Len(t, body, len(filedata))
	} else {
		// Validate that the file size has shrunk
		require.Less(t, len(body), len(filedata), "Compressed size should be smaller than original")
	}
}

func Test_Compress_Algorithms(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		handler  fiber.Handler
		name     string
		encoding string
	}{
		// Only gzip sets the Content-Type explicitly, the other codings rely on the default.
		{name: "gzip", encoding: "gzip", handler: compressTestSendReadme},
		{name: "deflate", encoding: "deflate", handler: compressTestSendUntypedReadme},
		{name: "brotli", encoding: "br", handler: compressTestSendUntypedReadme},
		{name: "zstd", encoding: "zstd", handler: compressTestSendUntypedReadme},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := compressTestApp(tc.handler)

			compressTestExpectEncoding(t, compressTestGet(t, app, tc.encoding), tc.encoding)
		})
	}
}

func Test_Compress_Different_Level(t *testing.T) {
	t.Parallel()
	levels := []Level{LevelDefault, LevelBestSpeed, LevelBestCompression}
	algorithms := []string{"gzip", "deflate", "br", "zstd"}

	for _, algo := range algorithms {
		for _, level := range levels {
			t.Run(fmt.Sprintf("%s_level %d", algo, level), func(t *testing.T) {
				t.Parallel()
				app := compressTestApp(compressTestSendReadme, Config{Level: level})

				compressTestExpectEncoding(t, compressTestGet(t, app, algo), algo)
			})
		}
	}
}

func Test_Compress_Disabled(t *testing.T) {
	t.Parallel()
	app := compressTestApp(compressTestSendUntypedReadme, Config{Level: LevelDisabled})

	compressTestExpectEncoding(t, compressTestGet(t, app, "br"), "")
}

func Test_Compress_Vary(t *testing.T) {
	t.Parallel()
	// Accept-Encoding joins the Vary header of every response the middleware
	// handles, whether it compresses it or skips it, unless the handler's own
	// Vary already covers it ("*", or a list naming Accept-Encoding).
	for _, tc := range []struct {
		name string
		// contentEncoding and vary are the headers the handler sets, when not
		// empty. The response keeps the former and carries wantVary as its Vary.
		contentEncoding string
		vary            string
		wantVary        string
		// rangeRequest adds a Range header to the request, which makes the
		// middleware skip it. noAcceptEncoding leaves Accept-Encoding out of it.
		rangeRequest     bool
		noAcceptEncoding bool
	}{
		{name: "adds_vary_header", wantVary: "Accept-Encoding"},
		{name: "vary_star", vary: "*", wantVary: "*"},
		{name: "vary_list_star", vary: "User-Agent, *", wantVary: "User-Agent, *"},
		{name: "vary_similar_substring", vary: "Accept-Encoding2", wantVary: "Accept-Encoding2, Accept-Encoding"},
		{name: "content_encoding_set_vary_star", contentEncoding: "gzip", vary: "*", wantVary: "*"},
		{name: "content_encoding_set_vary_list_star", contentEncoding: "gzip", vary: "User-Agent, *", wantVary: "User-Agent, *"},
		{name: "content_encoding_set_vary_similar_substring", contentEncoding: "gzip", vary: "Accept-Encoding2", wantVary: "Accept-Encoding2, Accept-Encoding"},
		{name: "range", rangeRequest: true, wantVary: "Accept-Encoding"},
		{name: "range_no_accept_encoding", rangeRequest: true, noAcceptEncoding: true, wantVary: "Accept-Encoding"},
		{name: "range_vary_star", rangeRequest: true, vary: "*", wantVary: "*"},
		{name: "range_vary_similar_substring", rangeRequest: true, vary: "Accept-Encoding2", wantVary: "Accept-Encoding2, Accept-Encoding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := compressTestApp(func(c fiber.Ctx) error {
				if tc.contentEncoding != "" {
					c.Set(fiber.HeaderContentEncoding, tc.contentEncoding)
				}
				if tc.vary != "" {
					c.Set(fiber.HeaderVary, tc.vary)
				}
				return c.SendString("hello")
			})

			acceptEncoding := "gzip"
			if tc.noAcceptEncoding {
				acceptEncoding = ""
			}
			var opts []apptest.Option
			if tc.rangeRequest {
				opts = append(opts, apptest.WithHeader(fiber.HeaderRange, "bytes=0-1"))
			}

			resp := compressTestGet(t, app, acceptEncoding, opts...)
			require.Equal(t, tc.contentEncoding, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Equal(t, tc.wantVary, resp.Header.Get(fiber.HeaderVary))
		})
	}
}

func Test_Compress_RespectsBodyLimitOnCompressedRequestBody(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{BodyLimit: 40})
	app.Use(New())
	app.Post("/", func(c fiber.Ctx) error { return c.Send(c.Body()) })

	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write([]byte(strings.Repeat("a", 256)))
	require.NoError(t, err)
	require.NoError(t, gz.Flush())
	require.NoError(t, gz.Close())

	req := httptest.NewRequest(fiber.MethodPost, "/", bytes.NewReader(b.Bytes()))
	req.Header.Set(fiber.HeaderContentEncoding, fiber.StrGzip)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMETextPlain)
	req.Header.Set(fiber.HeaderAcceptEncoding, fiber.StrIdentity)

	resp := apptest.Do(t, app, req, testConfig)
	require.Equal(t, fiber.StatusRequestEntityTooLarge, resp.StatusCode)

	require.Equal(t, fasthttp.ErrBodyTooLarge.Error(), apptest.Body(t, resp))
}

// Test_Compress_ETag covers what happens to the ETag a handler set: a strong
// one is recalculated for the compressed body, a weak one and a strong one on a
// response that was not compressed stay as they are.
func Test_Compress_ETag(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		handler      fiber.Handler
		name         string
		wantEncoding string
		wantETag     string // "" for the ETag generated from the compressed body
		wantBody     string // "" for a body that is not compared
	}{
		{
			name: "strong ETag is recalculated", wantEncoding: "gzip",
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderETag, "\"abc\"")
				return compressTestSendReadme(c)
			},
		},
		{
			name: "weak ETag is unchanged", wantEncoding: "gzip", wantETag: "W/\"abc\"",
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderETag, "W/\"abc\"")
				return compressTestSendReadme(c)
			},
		},
		{
			name: "strong ETag is unchanged when not compressed", wantETag: "\"abc\"",
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderETag, "\"abc\"")
				return c.SendString("tiny")
			},
		},
		{
			name: "Content-Encoding set by the handler skips the middleware", wantEncoding: "gzip", wantETag: "\"abc\"",
			wantBody: "hello",
			handler: func(c fiber.Ctx) error {
				c.Set(fiber.HeaderContentEncoding, "gzip")
				c.Set(fiber.HeaderETag, "\"abc\"")
				return c.SendString("hello")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := compressTestGet(t, compressTestApp(tc.handler), "gzip")
			body := apptest.BodyBytes(t, resp)
			require.Equal(t, tc.wantEncoding, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Equal(t, "Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
			if tc.wantBody != "" {
				require.Equal(t, tc.wantBody, string(body))
			}
			wantETag := tc.wantETag
			if wantETag == "" {
				wantETag = string(etag.Generate(body))
			}
			require.Equal(t, wantETag, resp.Header.Get(fiber.HeaderETag))
		})
	}
}

func Test_Compress_Skip_Head(t *testing.T) {
	t.Parallel()
	handler := func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, "\"abc\"")
		return c.Send(filedata)
	}
	app := compressTestApp(handler)
	app.Head("/", handler)

	getResp := compressTestGet(t, app, "gzip")
	require.NotEmpty(t, apptest.BodyBytes(t, getResp))
	require.Equal(t, "gzip", getResp.Header.Get(fiber.HeaderContentEncoding))

	headResp := apptest.Do(t, app, apptest.Req(fiber.MethodHead, "/", acceptGzip), testConfig)
	require.Empty(t, apptest.BodyBytes(t, headResp))
	require.Empty(t, headResp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "Accept-Encoding", headResp.Header.Get(fiber.HeaderVary))
	require.Equal(t, "\"abc\"", headResp.Header.Get(fiber.HeaderETag))
}

func Test_Compress_Skip_Status(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		send fiber.Handler
		name string
		// etag is set by the handler before it sends the status, and the
		// response must keep it.
		etag   string
		status int
	}{
		{name: "no_content", status: fiber.StatusNoContent, etag: "\"abc\"", send: func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusNoContent)
		}},
		{name: "not_modified", status: fiber.StatusNotModified, etag: "\"abc\"", send: func(c fiber.Ctx) error {
			c.Status(fiber.StatusNotModified)
			return nil
		}},
		{name: "partial_content", status: fiber.StatusPartialContent, send: func(c fiber.Ctx) error {
			c.Status(fiber.StatusPartialContent)
			return c.SendString("hello")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := compressTestApp(func(c fiber.Ctx) error {
				if tc.etag != "" {
					c.Set(fiber.HeaderETag, tc.etag)
				}
				return tc.send(c)
			})

			resp := compressTestGet(t, app, "gzip")
			require.Equal(t, tc.status, resp.StatusCode)
			require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Equal(t, tc.etag, resp.Header.Get(fiber.HeaderETag))
		})
	}
}

func Test_Compress_Skip_NoTransform(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		setRequest bool
	}{{name: "request", setRequest: true}, {name: "response", setRequest: false}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := compressTestApp(func(c fiber.Ctx) error {
				if !tt.setRequest {
					c.Set(fiber.HeaderCacheControl, "no-transform")
				}
				return c.SendString("hello")
			})

			var opts []apptest.Option
			if tt.setRequest {
				opts = append(opts, apptest.WithHeader(fiber.HeaderCacheControl, "no-transform"))
			}
			resp := compressTestGet(t, app, "gzip", opts...)
			require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
		})
	}
}

func Test_Compress_Next_Error(t *testing.T) {
	t.Parallel()
	app := compressTestApp(func(_ fiber.Ctx) error { return errors.New("next error") })

	resp := apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", acceptGzip), 500)
	require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "next error", apptest.Body(t, resp))
}

func Test_Compress_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{Next: func(_ fiber.Ctx) bool { return true }}))

	apptest.Expect(t, app, "/", fiber.StatusNotFound)
}

// benchEncodings are the Accept-Encoding values the benchmarks request.
var benchEncodings = []struct{ name, acceptEncoding string }{
	{name: "Gzip", acceptEncoding: "gzip"},
	{name: "Deflate", acceptEncoding: "deflate"},
	{name: "Brotli", acceptEncoding: "br"},
	{name: "Zstd", acceptEncoding: "zstd"},
}

// benchLevels are the compression levels the level benchmarks configure.
var benchLevels = []struct {
	name  string
	level Level
}{
	{name: "LevelDisabled", level: LevelDisabled},
	{name: "LevelDefault", level: LevelDefault},
	{name: "LevelBestSpeed", level: LevelBestSpeed},
	{name: "LevelBestCompression", level: LevelBestCompression},
}

// benchRequest returns a GET / request context accepting acceptEncoding.
func benchRequest(acceptEncoding string) *fasthttp.RequestCtx {
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)
	fctx.Request.SetRequestURI("/")

	if acceptEncoding != "" {
		fctx.Request.Header.Set("Accept-Encoding", acceptEncoding)
	}
	return fctx
}

func Benchmark_Compress(b *testing.B) {
	for _, tt := range benchEncodings {
		b.Run(tt.name, func(b *testing.B) {
			h := compressTestApp(compressTestSendReadme).Handler()
			fctx := benchRequest(tt.acceptEncoding)

			b.ReportAllocs()

			for b.Loop() {
				h(fctx)
			}
		})
	}
}

func Benchmark_Compress_Levels(b *testing.B) {
	for _, tt := range benchEncodings {
		for _, lvl := range benchLevels {
			b.Run(tt.name+"_"+lvl.name, func(b *testing.B) {
				h := compressTestApp(compressTestSendReadme, Config{Level: lvl.level}).Handler()
				fctx := benchRequest(tt.acceptEncoding)

				b.ReportAllocs()

				for b.Loop() {
					h(fctx)
				}
			})
		}
	}
}

func Benchmark_Compress_Parallel(b *testing.B) {
	for _, tt := range benchEncodings {
		b.Run(tt.name, func(b *testing.B) {
			h := compressTestApp(compressTestSendReadme).Handler()

			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				fctx := benchRequest(tt.acceptEncoding)

				for pb.Next() {
					h(fctx)
				}
			})
		})
	}
}

func Benchmark_Compress_Levels_Parallel(b *testing.B) {
	for _, tt := range benchEncodings {
		for _, lvl := range benchLevels {
			b.Run(tt.name+"_"+lvl.name, func(b *testing.B) {
				h := compressTestApp(compressTestSendReadme, Config{Level: lvl.level}).Handler()

				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					fctx := benchRequest(tt.acceptEncoding)

					for pb.Next() {
						h(fctx)
					}
				})
			})
		}
	}
}

type failOnReadStream struct {
	readCalls  int
	closeCalls int
}

func (s *failOnReadStream) Read(_ []byte) (int, error) {
	s.readCalls++
	return 0, errors.New("stream should not be read")
}

func (s *failOnReadStream) Close() error {
	s.closeCalls++
	return nil
}

func Test_Compress_ShouldSkip_DoesNotMaterialize_BodyStream(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.Header.SetMethod(fiber.MethodGet)

	c := apptest.CtxFor(t, app, fctx)

	stream := &failOnReadStream{}
	c.Response().SetStatusCode(fiber.StatusOK)
	c.Response().SetBodyStream(stream, -1)

	require.False(t, shouldSkip(c))
	require.True(t, c.Response().IsBodyStream())
	require.Zero(t, stream.readCalls)
	require.Zero(t, stream.closeCalls)
}

func Test_Compress_Streaming_With_Compression(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New())
	app.Get("/stream", func(c fiber.Ctx) error { return c.SendStream(bytes.NewReader(filedata)) })

	resp := apptest.Do(t, app, apptest.Req(fiber.MethodGet, "/stream", acceptGzip), testConfig)
	compressTestExpectEncoding(t, resp, "gzip")
}

func Test_Compress_StreamedBody_KeepsETagWeak(t *testing.T) {
	t.Parallel()
	payload := strings.Repeat("streamed content ", 500)
	app := compressTestApp(func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, `"abc"`)
		return c.SendStream(strings.NewReader(payload))
	})

	resp := apptest.Expect(t, app, apptest.Req(fiber.MethodGet, "/", acceptGzip), fiber.StatusOK)
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, `W/"abc"`, resp.Header.Get(fiber.HeaderETag))

	zr, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	require.Equal(t, payload, string(body))
}

func Test_Compress_AcceptEncoding_Negotiation(t *testing.T) {
	t.Parallel()
	compressible := strings.Repeat("compressible ", 500)
	testCases := []struct {
		accept   string
		encoding string
	}{
		{accept: "gzip;q=1.0", encoding: "gzip"},
		{accept: "identity,gzip", encoding: "gzip"},
		{accept: "deflate,gzip,br", encoding: "br"},
		{accept: "*", encoding: "br"},
		{accept: "br;q=0.5, gzip;q=1", encoding: "gzip"},
		{accept: "identity;q=0, gzip;q=0.5", encoding: "gzip"},
		{accept: "gzip;q=0", encoding: ""},
		{accept: "gzip;foo=bar;q=0.4, br;q=0.2", encoding: "gzip"},
		{accept: "gzip;foo=bar", encoding: "gzip"},
		{accept: "gzip;q=bogus", encoding: "gzip"},
		{accept: "*;foo=bar", encoding: "br"},
		{accept: "identity", encoding: ""},
		{accept: "", encoding: ""},
	}
	for _, tc := range testCases {
		t.Run(tc.accept, func(t *testing.T) {
			t.Parallel()
			app := compressTestApp(func(c fiber.Ctx) error { return c.SendString(compressible) })

			req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeaderIf(fiber.HeaderAcceptEncoding, tc.accept))
			resp := apptest.Expect(t, app, req, fiber.StatusOK)
			require.Equal(t, tc.encoding, resp.Header.Get(fiber.HeaderContentEncoding))
			require.Contains(t, resp.Header.Get(fiber.HeaderVary), fiber.HeaderAcceptEncoding)
		})
	}
}

func Test_Compress_RequestHeaderUntouched(t *testing.T) {
	t.Parallel()
	const accept = "br;q=0.5, gzip;q=1"
	app := fiber.New()
	seen := make(chan string, 1)
	app.Use(func(c fiber.Ctx) error {
		err := c.Next()
		seen <- c.Get(fiber.HeaderAcceptEncoding)
		return err
	})
	app.Use(New())
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(strings.Repeat("compressible ", 500)) })

	req := apptest.Req(fiber.MethodGet, "/", apptest.WithHeader(fiber.HeaderAcceptEncoding, accept))
	resp := apptest.Do(t, app, req)
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, accept, <-seen, "the client's header must survive negotiation")
}

func Test_Compress_Repeated_Accept_Encoding_Lines(t *testing.T) {
	t.Parallel()
	app := compressTestApp(compressTestSendReadme)

	// RFC 9110 §5.3: repeated field lines are one comma-separated list, so an
	// encoding on the second line must be negotiated like one on the first.
	req := apptest.Req(fiber.MethodGet, "/")
	req.Header.Add(fiber.HeaderAcceptEncoding, "identity")
	req.Header.Add(fiber.HeaderAcceptEncoding, "gzip")

	compressTestExpectEncoding(t, apptest.Do(t, app, req, testConfig), "gzip")
}

func Test_Compress_Cache_Control_No_Transform(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		// wantEncoding is the Content-Encoding of the response.
		wantEncoding string
		// lines are the Cache-Control field lines of the request, in order.
		lines []string
	}{
		// no-transform on a second Cache-Control line must still be honored.
		{name: "repeated_lines_no_transform", lines: []string{"public", "no-transform"}},
		// An escaped quote inside a directive's quoted argument must not hide the
		// no-transform that follows it.
		{name: "no_transform_after_escaped_quote", lines: []string{`ext="a\"", no-transform`}},
		// "no-transform" only appears inside a quoted directive argument, which is
		// one member, so the response must still be compressed.
		{name: "quoted_argument_is_not_no_transform", lines: []string{"public", `ext="x,no-transform,y"`}, wantEncoding: "gzip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := compressTestApp(compressTestSendReadme)

			req := apptest.Req(fiber.MethodGet, "/", acceptGzip)
			for _, line := range tc.lines {
				req.Header.Add(fiber.HeaderCacheControl, line)
			}

			compressTestExpectEncoding(t, apptest.Do(t, app, req, testConfig), tc.wantEncoding)
		})
	}
}

func Test_Compress_Response_Repeated_Cache_Control_Lines_No_Transform(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		c.Response().Header.Add(fiber.HeaderCacheControl, "public")
		c.Response().Header.Add(fiber.HeaderCacheControl, "no-transform")
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Len(t, body, len(filedata))
}

func Test_Compress_Response_No_Transform_After_Escaped_Quote(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		c.Set(fiber.HeaderCacheControl, `ext="a\"", no-transform`)
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Empty(t, resp.Header.Get(fiber.HeaderContentEncoding))
}

func Test_Compress_Response_Quoted_Cache_Control_Argument_Is_Not_No_Transform(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		c.Set(fiber.HeaderCacheControl, `public, ext="x,no-transform,y"`)
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
}
