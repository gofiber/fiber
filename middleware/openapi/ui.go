package openapi

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
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

        window.ui = SwaggerUIBundle({
          ...config,
          ...options,
        });
      });
    `

// UIScriptCSPSource is the Content-Security-Policy source expression for the page's one inline script.
const UIScriptCSPSource = "'sha256-aqKS8L4uzWkkwwtrhn5Atl/nhTF9eruvTkjOhUZArQs='"

type swaggerUITemplateData struct {
	InitScript htemplate.JS

	SwaggerOptionsJSON         string
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
    <div id="swagger-ui" data-url="{{ .OpenAPIURL }}" data-swagger-options='{{ .SwaggerOptionsJSON }}'></div>

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

func buildSwaggerUIPage(openAPIURL string, cfg *Config, encode utils.JSONMarshal) ([]byte, error) {
	if encode == nil {
		encode = json.Marshal
	}
	swaggerOptionsJSON, err := encode(cfg.SwaggerOptions)
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
