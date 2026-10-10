package openapi

import (
	"encoding/json"
	"fmt"
	htemplate "html/template"
	"strings"

	"github.com/gofiber/utils/v2"
)

type swaggerUITemplateData struct {
	SwaggerOptionsJSON         string
	Title                      string
	OpenAPIURL                 string
	SwaggerCSSURL              string
	SwaggerBundleURL           string
	SwaggerStandalonePresetURL string
}

var swaggerUITemplate = htemplate.Must(htemplate.New("swagger-ui").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{ .Title }} - Swagger UI</title>
    <link
      rel="stylesheet"
      href="{{ .SwaggerCSSURL }}"
    />
  </head>
  <body>
    <div id="swagger-ui" data-swagger-options='{{ .SwaggerOptionsJSON }}'></div>

    <script
      src="{{ .SwaggerBundleURL }}"
      crossorigin="anonymous"
    ></script>
    {{ if .SwaggerStandalonePresetURL }}<script
      src="{{ .SwaggerStandalonePresetURL }}"
      crossorigin="anonymous"
    ></script>{{ end }}
    <script>
      window.addEventListener("load", function () {
        const options = JSON.parse(document.getElementById("swagger-ui").dataset.swaggerOptions);

        const presets = [SwaggerUIBundle.presets.apis];
        const config = {
          url: "{{ .OpenAPIURL }}",
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
    </script>
  </body>
</html>
`))

// buildSwaggerUIPage renders the UI page for a spec URL. Options use the app's
// JSON encoder, and html/template escapes the result whichever encoder ran.
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
		Title:                      cfg.Title,
		OpenAPIURL:                 openAPIURL,
		SwaggerCSSURL:              cfg.SwaggerCSSURL,
		SwaggerBundleURL:           cfg.SwaggerBundleURL,
		SwaggerStandalonePresetURL: cfg.SwaggerStandalonePresetURL,
		SwaggerOptionsJSON:         string(swaggerOptionsJSON),
	}

	var builder strings.Builder
	if err := swaggerUITemplate.Execute(&builder, data); err != nil {
		return nil, fmt.Errorf("execute swagger ui template: %w", err)
	}

	return []byte(builder.String()), nil
}
