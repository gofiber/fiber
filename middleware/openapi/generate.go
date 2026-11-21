package openapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"
)

// buildDocument generates the document of app under cfg and encodes it with the app's JSON encoder, the way
// the middleware serves it: Transform runs on a decoded copy before the final encoding.
func buildDocument(app *fiber.App, cfg *Config) ([]byte, error) {
	appCfg := app.Config()
	spec := generateSpec(app.GetRoutes(false), cfg, specEnv{validator: appCfg.StructValidator, equal: segmentEqualFor(&appCfg), strict: appCfg.StrictRouting})
	data, err := appCfg.JSONEncoder(spec)
	if err != nil {
		return nil, fmt.Errorf("openapi: marshal spec: %w", err)
	}
	if cfg.Transform == nil {
		return data, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document map[string]any
	if err = decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("openapi: decode spec for Transform: %w", err)
	}
	if err = cfg.Transform(document); err != nil {
		return nil, fmt.Errorf("openapi: Transform: %w", err)
	}
	if data, err = appCfg.JSONEncoder(document); err != nil {
		return nil, fmt.Errorf("openapi: marshal transformed spec: %w", err)
	}
	return data, nil
}

// Generate returns the OpenAPI document of app, encoded with the app's JSON encoder, without serving it: for
// writing the document to a file, diffing it in CI or feeding a client generator. It prepares the app the way
// serving does, so mounted apps are included, and builds the document byte for byte as New serves it for
// the same config. Only the config's document fields matter; the UI settings are ignored.
func Generate(app *fiber.App, config ...Config) ([]byte, error) {
	if app == nil {
		return nil, errors.New("openapi: Generate needs an app")
	}
	cfg := configDefault(config...)
	app.Handler() // expands the mounted apps and derives the automatic HEAD routes, as serving would
	return buildDocument(app, &cfg)
}
