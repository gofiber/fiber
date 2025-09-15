package compress

import (
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var filedata []byte

var testConfig = fiber.TestConfig{
	Timeout:       10 * time.Second,
	FailOnTimeout: true,
}

func init() {
	dat, err := os.ReadFile("../../.github/README.md")
	if err != nil {
		panic(err)
	}
	filedata = dat
}

// go test -run Test_Compress_Gzip
func Test_Compress_Gzip(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate that the file size has shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Less(t, len(body), len(filedata))
}

// go test -run Test_Compress_Different_Level
func Test_Compress_Different_Level(t *testing.T) {
	t.Parallel()
	levels := []Level{LevelDefault, LevelBestSpeed, LevelBestCompression}
	algorithms := []string{"gzip", "deflate", "br", "zstd"}

	for _, algo := range algorithms {
		for _, level := range levels {
			t.Run(fmt.Sprintf("%s_level %d", algo, level), func(t *testing.T) {
				t.Parallel()
				app := fiber.New()

				app.Use(New(Config{Level: level}))

				app.Get("/", func(c fiber.Ctx) error {
					c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
					return c.Send(filedata)
				})

				req := httptest.NewRequest(fiber.MethodGet, "/", nil)
				req.Header.Set("Accept-Encoding", algo)

				resp, err := app.Test(req, testConfig)
				require.NoError(t, err, "app.Test(req)")
				require.Equal(t, 200, resp.StatusCode, "Status code")
				require.Equal(t, algo, resp.Header.Get(fiber.HeaderContentEncoding))

				// Validate that the file size has shrunk
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Less(t, len(body), len(filedata))
			})
		}
	}
}

func Test_Compress_Deflate(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "deflate")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "deflate", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate that the file size has shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Less(t, len(body), len(filedata))
}

func Test_Compress_Brotli(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "br")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "br", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate that the file size has shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Less(t, len(body), len(filedata))
}

func Test_Compress_Zstd(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "zstd")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "zstd", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate that the file size has shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Less(t, len(body), len(filedata))
}

func Test_Compress_Disabled(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(Config{Level: LevelDisabled}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "br")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate the file size is not shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, len(body), len(filedata))
}

func Test_Compress_Adds_Vary_Header(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Vary_Star(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderVary, "*")
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "*", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Vary_Similar_Substring(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderVary, "Accept-Encoding2")
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "Accept-Encoding2, Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Skip_When_Content_Encoding_Set(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentEncoding, "gzip")
		c.Set(fiber.HeaderETag, "\"abc\"")
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "\"abc\"", resp.Header.Get(fiber.HeaderETag))
	require.Equal(t, "Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Skip_When_Content_Encoding_Set_Vary_Star(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentEncoding, "gzip")
		c.Set(fiber.HeaderVary, "*")
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "*", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Skip_When_Content_Encoding_Set_Vary_Similar_Substring(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentEncoding, "gzip")
		c.Set(fiber.HeaderVary, "Accept-Encoding2")
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "Accept-Encoding2, Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Strong_ETag_Recalculated(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		c.Set(fiber.HeaderETag, "\"abc\"")
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	expected := string(etag.Generate(body))
	require.Equal(t, expected, resp.Header.Get(fiber.HeaderETag))
}

func Test_Compress_Weak_ETag_Unchanged(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		c.Set(fiber.HeaderETag, "W/\"abc\"")
		return c.Send(filedata)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "W/\"abc\"", resp.Header.Get(fiber.HeaderETag))
}

func Test_Compress_Head_Metadata(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	handler := func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, "\"abc\"")
		return c.SendString("hello")
	}
	app.Get("/", handler)
	app.Head("/", handler)

	getReq := httptest.NewRequest(fiber.MethodGet, "/", nil)
	getReq.Header.Set("Accept-Encoding", "gzip")

	getResp, err := app.Test(getReq, testConfig)
	require.NoError(t, err, "app.Test(getReq)")
	getBody, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.NotEmpty(t, getBody)
	expectedCL := strconv.Itoa(len(getBody))
	require.Equal(t, expectedCL, getResp.Header.Get(fiber.HeaderContentLength))

	headReq := httptest.NewRequest(fiber.MethodHead, "/", nil)
	headReq.Header.Set("Accept-Encoding", "gzip")

	headResp, err := app.Test(headReq, testConfig)
	require.NoError(t, err, "app.Test(headReq)")
	headBody, err := io.ReadAll(headResp.Body)
	require.NoError(t, err)
	require.Empty(t, headBody)

	require.Equal(t, getResp.Header.Get(fiber.HeaderContentEncoding), headResp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, getResp.Header.Get(fiber.HeaderVary), headResp.Header.Get(fiber.HeaderVary))
	require.Equal(t, getResp.Header.Get(fiber.HeaderETag), headResp.Header.Get(fiber.HeaderETag))
	require.Equal(t, expectedCL, headResp.Header.Get(fiber.HeaderContentLength))
}

func Test_Compress_Skip_Status_NoContent(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, "\"abc\"")
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, fiber.StatusNoContent, resp.StatusCode)
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "\"abc\"", resp.Header.Get(fiber.HeaderETag))
}

func Test_Compress_Skip_Status_NotModified(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderETag, "\"abc\"")
		c.Status(fiber.StatusNotModified)
		return nil
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, fiber.StatusNotModified, resp.StatusCode)
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "\"abc\"", resp.Header.Get(fiber.HeaderETag))
}

func Test_Compress_Skip_Range(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-1")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Skip_Range_NoAcceptEncoding(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes=0-1")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Skip_Range_Vary_Star(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderVary, "*")
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-1")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "*", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Skip_Range_Vary_Similar_Substring(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderVary, "Accept-Encoding2")
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=0-1")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	require.Equal(t, "Accept-Encoding2, Accept-Encoding", resp.Header.Get(fiber.HeaderVary))
}

func Test_Compress_Skip_Status_PartialContent(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Status(fiber.StatusPartialContent)
		return c.SendString("hello")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req, testConfig)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, fiber.StatusPartialContent, resp.StatusCode)
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
}

func Test_Compress_Skip_NoTransform(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		setRequest bool
	}{
		{name: "request", setRequest: true},
		{name: "response", setRequest: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()

			app.Use(New())

			app.Get("/", func(c fiber.Ctx) error {
				if !tt.setRequest {
					c.Set(fiber.HeaderCacheControl, "no-transform")
				}
				return c.SendString("hello")
			})

			req := httptest.NewRequest(fiber.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			if tt.setRequest {
				req.Header.Set(fiber.HeaderCacheControl, "no-transform")
			}

			resp, err := app.Test(req, testConfig)
			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
		})
	}
}

func Test_Compress_Next_Error(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(_ fiber.Ctx) error {
		return errors.New("next error")
	})

	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 500, resp.StatusCode, "Status code")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "next error", string(body))
}

// go test -run Test_Compress_Next
func Test_Compress_Next(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// go test -bench=Benchmark_Compress
func Benchmark_Compress(b *testing.B) {
	tests := []struct {
		name           string
		acceptEncoding string
	}{
		{name: "Gzip", acceptEncoding: "gzip"},
		{name: "Deflate", acceptEncoding: "deflate"},
		{name: "Brotli", acceptEncoding: "br"},
		{name: "Zstd", acceptEncoding: "zstd"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			app := fiber.New()
			app.Use(New())
			app.Get("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
				return c.Send(filedata)
			})

			h := app.Handler()
			fctx := &fasthttp.RequestCtx{}
			fctx.Request.Header.SetMethod(fiber.MethodGet)
			fctx.Request.SetRequestURI("/")

			if tt.acceptEncoding != "" {
				fctx.Request.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			b.ReportAllocs()

			for b.Loop() {
				h(fctx)
			}
		})
	}
}

// go test -bench=Benchmark_Compress_Levels
func Benchmark_Compress_Levels(b *testing.B) {
	tests := []struct {
		name           string
		acceptEncoding string
	}{
		{name: "Gzip", acceptEncoding: "gzip"},
		{name: "Deflate", acceptEncoding: "deflate"},
		{name: "Brotli", acceptEncoding: "br"},
		{name: "Zstd", acceptEncoding: "zstd"},
	}

	levels := []struct {
		name  string
		level Level
	}{
		{name: "LevelDisabled", level: LevelDisabled},
		{name: "LevelDefault", level: LevelDefault},
		{name: "LevelBestSpeed", level: LevelBestSpeed},
		{name: "LevelBestCompression", level: LevelBestCompression},
	}

	for _, tt := range tests {
		for _, lvl := range levels {
			b.Run(tt.name+"_"+lvl.name, func(b *testing.B) {
				app := fiber.New()
				app.Use(New(Config{Level: lvl.level}))
				app.Get("/", func(c fiber.Ctx) error {
					c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
					return c.Send(filedata)
				})

				h := app.Handler()
				fctx := &fasthttp.RequestCtx{}
				fctx.Request.Header.SetMethod(fiber.MethodGet)
				fctx.Request.SetRequestURI("/")

				if tt.acceptEncoding != "" {
					fctx.Request.Header.Set("Accept-Encoding", tt.acceptEncoding)
				}

				b.ReportAllocs()

				for b.Loop() {
					h(fctx)
				}
			})
		}
	}
}

// go test -bench=Benchmark_Compress_Parallel
func Benchmark_Compress_Parallel(b *testing.B) {
	tests := []struct {
		name           string
		acceptEncoding string
	}{
		{name: "Gzip", acceptEncoding: "gzip"},
		{name: "Deflate", acceptEncoding: "deflate"},
		{name: "Brotli", acceptEncoding: "br"},
		{name: "Zstd", acceptEncoding: "zstd"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			app := fiber.New()
			app.Use(New())
			app.Get("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
				return c.Send(filedata)
			})

			h := app.Handler()

			b.ReportAllocs()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				fctx := &fasthttp.RequestCtx{}
				fctx.Request.Header.SetMethod(fiber.MethodGet)
				fctx.Request.SetRequestURI("/")

				if tt.acceptEncoding != "" {
					fctx.Request.Header.Set("Accept-Encoding", tt.acceptEncoding)
				}

				for pb.Next() {
					h(fctx)
				}
			})
		})
	}
}

// go test -bench=Benchmark_Compress_Levels_Parallel
func Benchmark_Compress_Levels_Parallel(b *testing.B) {
	tests := []struct {
		name           string
		acceptEncoding string
	}{
		{name: "Gzip", acceptEncoding: "gzip"},
		{name: "Deflate", acceptEncoding: "deflate"},
		{name: "Brotli", acceptEncoding: "br"},
		{name: "Zstd", acceptEncoding: "zstd"},
	}

	levels := []struct {
		name  string
		level Level
	}{
		{name: "LevelDisabled", level: LevelDisabled},
		{name: "LevelDefault", level: LevelDefault},
		{name: "LevelBestSpeed", level: LevelBestSpeed},
		{name: "LevelBestCompression", level: LevelBestCompression},
	}

	for _, tt := range tests {
		for _, lvl := range levels {
			b.Run(tt.name+"_"+lvl.name, func(b *testing.B) {
				app := fiber.New()
				app.Use(New(Config{Level: lvl.level}))
				app.Get("/", func(c fiber.Ctx) error {
					c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
					return c.Send(filedata)
				})

				h := app.Handler()

				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					fctx := &fasthttp.RequestCtx{}
					fctx.Request.Header.SetMethod(fiber.MethodGet)
					fctx.Request.SetRequestURI("/")

					if tt.acceptEncoding != "" {
						fctx.Request.Header.Set("Accept-Encoding", tt.acceptEncoding)
					}

					for pb.Next() {
						h(fctx)
					}
				})
			})
		}
	}
}
