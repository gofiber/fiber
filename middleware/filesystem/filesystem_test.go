package filesystem

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// go test -run Test_FileSystem
func Test_FileSystem(t *testing.T) {
	app := fiber.New()

	app.Use("/test", New(Config{
		Root: http.Dir("../../.github/testdata/fs"),
	}))

	app.Use("/dir", New(Config{
		Root:   http.Dir("../../.github/testdata/fs"),
		Browse: true,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Use("/spatest", New(Config{
		Root:         http.Dir("../../.github/testdata/fs"),
		Index:        "index.html",
		NotFoundFile: "index.html",
	}))

	app.Use("/prefix", New(Config{
		Root:       http.Dir("../../.github/testdata/fs"),
		PathPrefix: "img",
	}))

	tests := []struct {
		name         string
		url          string
		statusCode   int
		contentType  string
		modifiedTime string
	}{
		{
			name:        "Should be returns status 200 with suitable content-type",
			url:         "/test/index.html",
			statusCode:  200,
			contentType: "text/html",
		},
		{
			name:        "Should be returns status 200 with suitable content-type",
			url:         "/test",
			statusCode:  200,
			contentType: "text/html",
		},
		{
			name:        "Should be returns status 200 with suitable content-type",
			url:         "/test/css/style.css",
			statusCode:  200,
			contentType: "text/css",
		},
		{
			name:       "Should be returns status 404",
			url:        "/test/nofile.js",
			statusCode: 404,
		},
		{
			name:       "Should be returns status 404",
			url:        "/test/nofile",
			statusCode: 404,
		},
		{
			name:        "Should be returns status 200",
			url:         "/",
			statusCode:  200,
			contentType: "text/plain; charset=utf-8",
		},
		{
			name:       "Should be returns status 403",
			url:        "/test/img",
			statusCode: 403,
		},
		{
			name:        "Should list the directory contents",
			url:         "/dir/img",
			statusCode:  200,
			contentType: "text/html",
		},
		{
			name:        "Should list the directory contents",
			url:         "/dir/img/",
			statusCode:  200,
			contentType: "text/html",
		},
		{
			name:        "Should be returns status 200",
			url:         "/dir/img/fiber.png",
			statusCode:  200,
			contentType: "image/png",
		},
		{
			name:        "Should be return status 200",
			url:         "/spatest/doesnotexist",
			statusCode:  200,
			contentType: "text/html",
		},
		{
			name:        "PathPrefix should be applied",
			url:         "/prefix/fiber.png",
			statusCode:  200,
			contentType: "image/png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", tt.url, nil))
			require.NoError(t, err)
			require.Equal(t, tt.statusCode, resp.StatusCode)

			if tt.contentType != "" {
				ct := resp.Header.Get("Content-Type")
				require.Equal(t, tt.contentType, ct)
			}
		})
	}
}

// go test -run Test_FileSystem_Next
func Test_FileSystem_Next(t *testing.T) {
	app := fiber.New()
	app.Use(New(Config{
		Root: http.Dir("../../.github/testdata/fs"),
		Next: func(_ fiber.Ctx) bool {
			return true
		},
	}))

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	require.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

func Test_FileSystem_NonGetAndHead(t *testing.T) {
	app := fiber.New()

	app.Use("/test", New(Config{
		Root: http.Dir("../../.github/testdata/fs"),
	}))

	resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/test", nil))
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}

func Test_FileSystem_Head(t *testing.T) {
	app := fiber.New()

	app.Use("/test", New(Config{
		Root: http.Dir("../../.github/testdata/fs"),
	}))

	req, _ := http.NewRequest(fiber.MethodHead, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func Test_FileSystem_NoRoot(t *testing.T) {
	defer func() {
		require.Equal(t, "filesystem: Root cannot be nil", recover())
	}()

	app := fiber.New()
	app.Use(New())
	_, _ = app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
}

func Test_FileSystem_UsingParam(t *testing.T) {
	app := fiber.New()

	app.Use("/:path", func(c fiber.Ctx) error {
		return SendFile(c, http.Dir("../../.github/testdata/fs"), c.Params("path")+".html")
	})

	req, _ := http.NewRequest(fiber.MethodHead, "/index", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func Test_FileSystem_UsingParam_NonFile(t *testing.T) {
	app := fiber.New()

	app.Use("/:path", func(c fiber.Ctx) error {
		return SendFile(c, http.Dir("../../.github/testdata/fs"), c.Params("path")+".html")
	})

	req, _ := http.NewRequest(fiber.MethodHead, "/template", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}
