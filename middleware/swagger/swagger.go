package swagger

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/handlers"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"os"
	"path"
)

type Config struct {
	BasePath string
	FilePath string
}

// ConfigDefault is the default config
var ConfigDefault = Config{
	BasePath: "/",
	FilePath: "./swagger.json",
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := ConfigDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if len(cfg.BasePath) == 0 {
			cfg.BasePath = ConfigDefault.BasePath
		}
		if len(cfg.FilePath) == 0 {
			cfg.FilePath = ConfigDefault.FilePath
		}
	}

	if _, err := os.Stat(cfg.FilePath); os.IsNotExist(err) {
		panic(errors.New(fmt.Sprintf("%s file is not exist", cfg.FilePath)))
	}

	specDoc, err := loads.Spec(cfg.FilePath)
	if err != nil {
		panic(err)
	}

	b, err := json.MarshalIndent(specDoc.Spec(), "", "  ")
	if err != nil {
		panic(err)
	}

	specUrl := path.Join(cfg.BasePath, "swagger.json")
	swaggerUIOpts := middleware.SwaggerUIOpts{
		BasePath: cfg.BasePath,
		SpecURL:  specUrl,
		Path:     "docs",
	}

	swaggerUiHandler := middleware.SwaggerUI(swaggerUIOpts, nil)
	specFileHandler, err := handlers.CORS()(middleware.Spec(cfg.BasePath, b, swaggerUiHandler)), nil

	// Return new handler
	return func(c *fiber.Ctx) error {
		handler := fasthttpadaptor.NewFastHTTPHandler(specFileHandler)
		handler(c.Context())
		return nil
	}
}