package compress

import (
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var filedata []byte

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

	resp, err := app.Test(req, 10*time.Second)
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

				resp, err := app.Test(req, 10*time.Second)
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

	resp, err := app.Test(req, 10*time.Second)
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

	resp, err := app.Test(req, 10*time.Second)
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

	resp, err := app.Test(req, 10*time.Second)
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

	resp, err := app.Test(req, 10*time.Second)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate the file size is not shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, len(body), len(filedata))
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
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
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
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
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
