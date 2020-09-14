package filesystem

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/internal/utils"
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

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Use("/spatest", New(Config{
		Root:         http.Dir("../../.github/testdata/fs"),
		NotFoundFile: "index.html",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.url, nil)
			resp, err := app.Test(req)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, tt.statusCode, resp.StatusCode)

			if tt.contentType != "" {
				ct := resp.Header.Get("Content-Type")
				utils.AssertEqual(t, tt.contentType, ct)
			}
		})
	}
}
