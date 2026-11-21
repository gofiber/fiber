package openapi

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"maps"
	"strings"

	"github.com/gofiber/utils/v2"
)

// swaggerInitScript is constant for every page, so a strict CSP can allow it by hash.
const swaggerInitScript = `
      window.addEventListener("load", function () {
        const root = document.getElementById("swagger-ui");
        const options = JSON.parse(root.dataset.swaggerOptions);

        const presets = [SwaggerUIBundle.presets.apis];
        const config = {
          url: root.dataset.url,
          dom_id: "#swagger-ui",
        };
        if (typeof SwaggerUIStandalonePreset !== "undefined") {
          presets.push(SwaggerUIStandalonePreset);
          config.layout = "StandaloneLayout";
        }
        config.presets = presets;
        if (root.dataset.oauth2Redirect) {
          config.oauth2RedirectUrl = new URL(root.dataset.oauth2Redirect, window.location.href).href;
        }

        window.ui = SwaggerUIBundle({
          ...config,
          ...options,
        });
      });
    `

// UIScriptCSPSource is the Content-Security-Policy source expression for the page's one inline script.
const UIScriptCSPSource = "'sha256-5AbdohkyfvJ5VDe2Y7koFQC4oRvlD653dOE1nfPjmYU='"

type swaggerUITemplateData struct {
	InitScript htemplate.JS

	SwaggerOptionsJSON         string
	OAuth2RedirectURL          string
	Title                      string
	OpenAPIURL                 string
	SwaggerCSSURL              string
	SwaggerBundleURL           string
	SwaggerStandalonePresetURL string

	SwaggerCSSIntegrity              string
	SwaggerBundleIntegrity           string
	SwaggerStandalonePresetIntegrity string
}

var swaggerUITemplate = htemplate.Must(htemplate.New("swagger-ui").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{ .Title }} - Swagger UI</title>
    <link
      rel="stylesheet"
      href="{{ .SwaggerCSSURL }}"{{ if .SwaggerCSSIntegrity }}
      integrity="{{ .SwaggerCSSIntegrity }}"
      crossorigin="anonymous"{{ end }}
    />
  </head>
  <body>
    <div id="swagger-ui" data-url="{{ .OpenAPIURL }}" data-swagger-options='{{ .SwaggerOptionsJSON }}'{{ if .OAuth2RedirectURL }} data-oauth2-redirect="{{ .OAuth2RedirectURL }}"{{ end }}></div>

    <script
      src="{{ .SwaggerBundleURL }}"{{ if .SwaggerBundleIntegrity }}
      integrity="{{ .SwaggerBundleIntegrity }}"
      crossorigin="anonymous"{{ end }}
    ></script>
    {{ if .SwaggerStandalonePresetURL }}<script
      src="{{ .SwaggerStandalonePresetURL }}"{{ if .SwaggerStandalonePresetIntegrity }}
      integrity="{{ .SwaggerStandalonePresetIntegrity }}"
      crossorigin="anonymous"{{ end }}
    ></script>{{ end }}
    <script>{{ .InitScript }}</script>
  </body>
</html>
`))

type scriptPageData struct {
	Title      string
	OpenAPIURL string
	BundleURL  string
	Integrity  string
}

var redocTemplate = htemplate.Must(htemplate.New("redoc").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{ .Title }} - Redoc</title>
  </head>
  <body>
    <redoc spec-url="{{ .OpenAPIURL }}"></redoc>
    <script
      src="{{ .BundleURL }}"{{ if .Integrity }}
      integrity="{{ .Integrity }}"
      crossorigin="anonymous"{{ end }}
    ></script>
  </body>
</html>
`))

var scalarTemplate = htemplate.Must(htemplate.New("scalar").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{ .Title }} - API Reference</title>
  </head>
  <body>
    <script id="api-reference" data-url="{{ .OpenAPIURL }}"></script>
    <script
      src="{{ .BundleURL }}"{{ if .Integrity }}
      integrity="{{ .Integrity }}"
      crossorigin="anonymous"{{ end }}
    ></script>
  </body>
</html>
`))

// buildUIPage renders the documentation page cfg selects.
func buildUIPage(openAPIURL, oauth2RedirectURL string, cfg *Config, encode utils.JSONMarshal) ([]byte, error) {
	var (
		tmpl *htemplate.Template
		data scriptPageData
	)
	switch cfg.UI {
	case UIRedoc:
		tmpl, data = redocTemplate, scriptPageData{BundleURL: cfg.RedocBundleURL, Integrity: cfg.RedocBundleIntegrity}
	case UIScalar:
		tmpl, data = scalarTemplate, scriptPageData{BundleURL: cfg.ScalarBundleURL, Integrity: cfg.ScalarBundleIntegrity}
	case UISwagger:
		return buildSwaggerUIPage(openAPIURL, oauth2RedirectURL, cfg, encode)
	default:
		return nil, fmt.Errorf("unknown UI %q", cfg.UI)
	}
	data.Title, data.OpenAPIURL = cfg.Title, openAPIURL

	var builder strings.Builder
	if err := tmpl.Execute(&builder, data); err != nil {
		return nil, fmt.Errorf("execute %s template: %w", cfg.UI, err)
	}
	return []byte(builder.String()), nil
}

func buildSwaggerUIPage(openAPIURL, oauth2RedirectURL string, cfg *Config, encode utils.JSONMarshal) ([]byte, error) {
	if encode == nil {
		encode = json.Marshal
	}
	// Swagger UI otherwise shows a validator badge, which sends the document's URL to validator.swagger.io and
	// has that service fetch it. It stays off unless the options ask for a validator.
	options := make(map[string]any, len(cfg.SwaggerOptions)+1)
	maps.Copy(options, cfg.SwaggerOptions)
	if _, set := options["validatorUrl"]; !set {
		options["validatorUrl"] = nil
	}
	swaggerOptionsJSON, err := encode(options)
	if err != nil {
		return nil, fmt.Errorf("marshal swagger options: %w", err)
	}
	if len(swaggerOptionsJSON) == 0 || string(swaggerOptionsJSON) == "null" {
		swaggerOptionsJSON = []byte("{}")
	}

	data := swaggerUITemplateData{
		InitScript:                 htemplate.JS(swaggerInitScript), //nolint:gosec // G203: a constant script, not input
		Title:                      cfg.Title,
		OpenAPIURL:                 openAPIURL,
		OAuth2RedirectURL:          oauth2RedirectURL,
		SwaggerCSSURL:              cfg.SwaggerCSSURL,
		SwaggerBundleURL:           cfg.SwaggerBundleURL,
		SwaggerStandalonePresetURL: cfg.SwaggerStandalonePresetURL,

		SwaggerCSSIntegrity:              cfg.SwaggerCSSIntegrity,
		SwaggerBundleIntegrity:           cfg.SwaggerBundleIntegrity,
		SwaggerStandalonePresetIntegrity: cfg.SwaggerStandalonePresetIntegrity,
		SwaggerOptionsJSON:               string(swaggerOptionsJSON),
	}

	var builder strings.Builder
	if err := swaggerUITemplate.Execute(&builder, data); err != nil {
		return nil, fmt.Errorf("execute swagger ui template: %w", err)
	}

	return []byte(builder.String()), nil
}
