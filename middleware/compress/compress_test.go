package compress

import (
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
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
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		return c.Send(filedata)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate that the file size has shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(body) < len(filedata))
}

// go test -run Test_Compress_Different_Level
func Test_Compress_Different_Level(t *testing.T) {
	levels := []Level{LevelBestSpeed, LevelBestCompression}
	for _, level := range levels {
		t.Run(fmt.Sprintf("level %d", level), func(t *testing.T) {
			app := fiber.New()

			app.Use(New(Config{Level: level}))

			app.Get("/", func(c fiber.Ctx) error {
				c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
				return c.Send(filedata)
			})

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")

			resp, err := app.Test(req)
			require.NoError(t, err, "app.Test(req)")
			require.Equal(t, 200, resp.StatusCode, "Status code")
			require.Equal(t, "gzip", resp.Header.Get(fiber.HeaderContentEncoding))

			// Validate that the file size has shrunk
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.True(t, len(body) < len(filedata))
		})
	}
}

func Test_Compress_Deflate(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(filedata)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "deflate")

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "deflate", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate that the file size has shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(body) < len(filedata))
}

func Test_Compress_Brotli(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(filedata)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "br")

	resp, err := app.Test(req, 10000)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "br", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate that the file size has shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(body) < len(filedata))
}

func Test_Compress_Disabled(t *testing.T) {
	app := fiber.New()

	app.Use(New(Config{Level: LevelDisabled}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.Send(filedata)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "br")

	resp, err := app.Test(req)
	require.NoError(t, err, "app.Test(req)")
	require.Equal(t, 200, resp.StatusCode, "Status code")
	require.Equal(t, "", resp.Header.Get(fiber.HeaderContentEncoding))

	// Validate the file size is not shrunk
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, len(body) == len(filedata))
}

func Test_Compress_Next_Error(t *testing.T) {
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c fiber.Ctx) error {
		return errors.New("next error")
	})

	req := httptest.NewRequest("GET", "/", nil)
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
	app := fiber.New()
	app.Use(New(Config{
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}
