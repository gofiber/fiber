package swagger

import (
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func performRequest(method, target string, app *fiber.App) *http.Response {
	r := httptest.NewRequest(method, target, nil)

	resp, _ := app.Test(r, -1)

	return resp
}

func TestNew(t *testing.T) {
	t.Run("Endpoint check with custom config", func(t *testing.T) {
		app := fiber.New()

		cfg := Config{
			BasePath: "/",
			FilePath: "swagger.json",
		}
		app.Use(New(cfg))

		w1 := performRequest("GET", "/docs", app)
		assert.Equal(t, 200, w1.StatusCode)

		w2 := performRequest("GET", "/swagger.json", app)
		assert.Equal(t, 200, w2.StatusCode)

		w3 := performRequest("GET", "/notfound", app)
		assert.Equal(t, 404, w3.StatusCode)
	})

	t.Run("Endpoint check with empty custom config", func(t *testing.T) {
		app := fiber.New()

		cfg := Config{}

		app.Use(New(cfg))

		w1 := performRequest("GET", "/docs", app)
		assert.Equal(t, 200, w1.StatusCode)

		w2 := performRequest("GET", "/swagger.json", app)
		assert.Equal(t, 200, w2.StatusCode)

		w3 := performRequest("GET", "/notfound", app)
		assert.Equal(t, 404, w3.StatusCode)
	})

	t.Run("Endpoint check with default config", func(t *testing.T) {
		app := fiber.New()

		app.Use(New())

		w1 := performRequest("GET", "/docs", app)
		assert.Equal(t, 200, w1.StatusCode)

		w2 := performRequest("GET", "/swagger.json", app)
		assert.Equal(t, 200, w2.StatusCode)

		w3 := performRequest("GET", "/notfound", app)
		assert.Equal(t, 404, w3.StatusCode)
	})

	t.Run("Swagger.json file is not exist", func(t *testing.T) {
		app := fiber.New()

		cfg := Config{
			FilePath: "./docs/swagger.json",
		}

		assert.Panics(t, func() {
			app.Use(New(cfg))
		}, "/swagger.json file is not exist")
	})

	t.Run("Swagger.json missing file", func(t *testing.T) {
		app := fiber.New()

		cfg := Config{
			FilePath: "./docs/swagger_missing.json",
		}

		assert.Panics(t, func() {
			app.Use(New(cfg))
		}, "invalid character ':' after object key:value pair")
	})
}
