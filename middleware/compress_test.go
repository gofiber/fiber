package middleware

import (
	"fmt"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gofiber/fiber"
	"github.com/gofiber/utils"
	"github.com/valyala/fasthttp"
)

// go test -run Test_Middleware_Compress
func Test_Middleware_Compress(t *testing.T) {
	app := fiber.New()

	app.Use(Compress())

	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile(compressFilePath(CompressLevelDefault), true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	os.Remove(compressFilePath(CompressLevelDefault, true))
}

// go test -run Test_Middleware_Compress_Config
func Test_Middleware_Compress_Config(t *testing.T) {
	app := fiber.New()

	app.Use(Compress(CompressConfig{
		Level: CompressLevelDefault,
	}))

	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile(compressFilePath(CompressLevelDefault), true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	os.Remove(compressFilePath(CompressLevelDefault, true))
}

// go test -run Test_Middleware_Compress_With_Config
func Test_Middleware_Compress_With_Config(t *testing.T) {
	app := fiber.New()

	app.Use(Compress(CompressConfig{}))

	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile(compressFilePath(CompressLevelDefault), true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(fiber.HeaderAcceptEncoding, "gzip")

	resp, err := app.Test(req)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))
	utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
	os.Remove(compressFilePath(CompressLevelDefault, true))
}

// go test -run Test_Middleware_Compress_Level
func Test_Middleware_Compress_Level(t *testing.T) {
	t.Parallel()

	levels := []int{
		CompressLevelDisabled,
		CompressLevelDefault,
		CompressLevelBestSpeed,
		CompressLevelBestCompression,
	}

	app := fiber.New()
	for _, level := range levels {
		app.Get("/:level", Compress(level), func(c *fiber.Ctx) {
			c.SendFile(compressFilePath(c.Params("level")), true)
		})
	}

	for _, level := range levels {
		name := strconv.FormatInt(int64(level), 10)
		t.Run(name, func(t *testing.T) {
			target := fmt.Sprintf("/%d", level)
			req := httptest.NewRequest("GET", target, nil)
			req.Header.Set(fiber.HeaderAcceptEncoding, "br")

			resp, err := app.Test(req, 3000)
			utils.AssertEqual(t, nil, err, "app.Test(req)")
			utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
			utils.AssertEqual(t, "br", resp.Header.Get(fiber.HeaderContentEncoding))
			utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

			os.Remove(compressFilePath(level, true))
		})
	}
}

// go test -run Test_Middleware_Compress_Skip
func Test_Middleware_Compress_Skip(t *testing.T) {
	app := fiber.New()

	app.Use(Compress(func(c *fiber.Ctx) bool { return true }))

	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile(compressFilePath(CompressLevelDefault), true)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(fiber.HeaderAcceptEncoding, "br")

	resp, err := app.Test(req, 3000)
	utils.AssertEqual(t, nil, err, "app.Test(req)")
	utils.AssertEqual(t, fiber.StatusOK, resp.StatusCode, "Status code")
	utils.AssertEqual(t, "", resp.Header.Get(fiber.HeaderContentEncoding))
	utils.AssertEqual(t, fiber.MIMETextPlainCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
}

// go test -run Test_Middleware_Compress_Panic
func Test_Middleware_Compress_Panic(t *testing.T) {
	defer func() {
		utils.AssertEqual(t,
			"Compress: the following option types are allowed: int, func(*fiber.Ctx) bool, CompressConfig",
			fmt.Sprintf("%s", recover()))
	}()

	Compress("invalid")
}

// go test -v ./... -run=^$ -bench=Benchmark_Middleware_Compress -benchmem -count=4
func Benchmark_Middleware_Compress(b *testing.B) {
	app := fiber.New()
	app.Use(Compress())
	app.Get("/", func(c *fiber.Ctx) {
		c.SendFile(compressFilePath(CompressLevelDefault), true)
	})
	handler := app.Handler()

	c := &fasthttp.RequestCtx{}
	c.Request.SetRequestURI("/")

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		handler(c)
	}
}

func compressFilePath(level interface{}, gz ...bool) string {
	filePath := fmt.Sprintf("./testdata/compress_level_%v.txt", level)
	if len(gz) > 0 && gz[0] {
		filePath += ".fiber.gz"
	}
	return filePath
}
