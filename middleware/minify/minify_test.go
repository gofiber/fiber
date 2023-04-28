package minify

import (
	"io"
	"log"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"gopkg.in/yaml.v2"
)

var filedata struct {
	html             []byte
	htmlWithCss      []byte
	htmlWithJs       []byte
	htmlWithCssAndJs []byte
	css              []byte
	js               []byte
}

var expectedData struct {
	HtmlMinify                   string `yaml:"HtmlMinify"`
	HTMLWithCssDefault           string `yaml:"html_with_css_default"`
	HTMLWithJsDefault            string `yaml:"html_with_js_default"`
	HTMLWithCssAndJs             string `yaml:"html_with_css_and_js"`
	HTMLConfigOptionsStylesFalse string `yaml:"html_config_options_styles_false"`
	HTMLConfigOptionsJSFalse     string `yaml:"html_config_options_js_false"`
	HTMLConfigOptionsCssJSFalse  string `yaml:"html_config_options_css_js_false"`
	HTMLConfigOptionsCssJSTrue   string `yaml:"html_config_options_css_js_true"`
	CSSMinify                    string `yaml:"css_minify"`
	JsMinify                     string `yaml:"js_minify"`
}

func init() {
	// set expected data
	data, err := os.ReadFile("../../.github/testdata2/minify_expected_data.yaml")
	if err != nil {
		log.Fatalf("failed to read YAML file: %v", err)
	}

	if err := yaml.Unmarshal(data, &expectedData); err != nil {
		log.Fatalf("failed to unmarshal YAML data: %v", err)
	}

	// set file data
	html, err := os.ReadFile("../../.github/testdata2/template.html")
	if err != nil {
		panic(err)
	}
	filedata.html = html
	filedata.htmlWithCss, err = os.ReadFile("../../.github/testdata2/template-with-css.html")
	if err != nil {
		panic(err)
	}
	filedata.htmlWithJs, err = os.ReadFile("../../.github/testdata2/template-with-js.html")
	if err != nil {
		panic(err)
	}
	filedata.htmlWithCssAndJs, err = os.ReadFile("../../.github/testdata2/template-with-css-and-js.html")
	if err != nil {
		panic(err)
	}
	filedata.css, err = os.ReadFile("../../.github/testdata2/template.css")
	if err != nil {
		panic(err)
	}
	filedata.js, err = os.ReadFile("../../.github/testdata2/template.js")
	if err != nil {
		panic(err)
	}
}

// go test -run Test_Minify_HTML_Config_Method
func Test_Minify_HTML_Config_Method(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		configMethod string
		reqMethod    string
		code         int
		minify       bool
	}{
		{
			configMethod: fiber.MethodGet,
			reqMethod:    fiber.MethodGet,
			code:         200,
			minify:       true,
		},
		{
			configMethod: fiber.MethodGet,
			reqMethod:    fiber.MethodPost,
			code:         200,
			minify:       false,
		},
		{
			configMethod: fiber.MethodPost,
			reqMethod:    fiber.MethodPost,
			code:         200,
			minify:       true,
		},
		{
			configMethod: fiber.MethodPost,
			reqMethod:    fiber.MethodGet,
			code:         200,
			minify:       false,
		},
		{
			configMethod: "ALL",
			reqMethod:    fiber.MethodGet,
			code:         200,
			minify:       true,
		},
		{
			configMethod: "ALL",
			reqMethod:    fiber.MethodPost,
			code:         200,
			minify:       true,
		},
	}
	for _, tc := range testCases {
		app := fiber.New()
		app.Use(New(Config{
			MinifyHTML: true,
			Method:     Method(tc.configMethod),
		}))
		app.Get("/", func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.Send(filedata.html)
		})
		app.Post("/", func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.Send(filedata.html)
		})

		req := httptest.NewRequest(tc.reqMethod, "/", nil)
		resp, err := app.Test(req)

		utils.AssertEqual(t, nil, err)
		utils.AssertEqual(t, tc.code, resp.StatusCode, "Status code")

		// check if resp body equals expectedData
		body, err := io.ReadAll(resp.Body)
		utils.AssertEqual(t, nil, err)
		if tc.minify {
			utils.AssertEqual(t, []byte(expectedData.HtmlMinify), body)
		} else {
			utils.AssertEqual(t, filedata.html, body)
		}
	}
}

// go test -run Test_Minify_HTML_Config_Next
func Test_Minify_HTML_Config_Next(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		code   int
		minify bool
		next   func(c *fiber.Ctx) bool
	}{
		{
			name:   "next return true",
			code:   200,
			minify: false,
			next: func(c *fiber.Ctx) bool {
				return true
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(New(
				Config{
					Next: tc.next,
				},
			))
			app.Get("/", func(c *fiber.Ctx) error {
				c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
				return c.Send(filedata.html)
			})
			req := httptest.NewRequest(fiber.MethodGet, "/", nil)
			resp, err := app.Test(req)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, tc.code, resp.StatusCode, "Status code")

			// check if resp body equals expectedData
			body, err := io.ReadAll(resp.Body)
			utils.AssertEqual(t, nil, err)
			if tc.minify {
				utils.AssertEqual(t, []byte(expectedData.HtmlMinify), body)
			} else {
				utils.AssertEqual(t, filedata.html, body)
			}
		})
	}
}

// go test -run Test_Minify_HTML_Default
func Test_Minify_HTML_Default(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.html)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, []byte(expectedData.HtmlMinify), body)
}

// go test -run Test_Minify_HTML_Default_With_CSS
func Test_Minify_HTML_Default_With_CSS(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCss)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.HTMLWithCssDefault, string(body))
}

// go test -run Test_Minify_HTML_Default_With_JS
func Test_Minify_HTML_Default_With_JS(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.HTMLWithJsDefault, string(body))
}

// go test -run Test_Minify_HTML_Default_With_Css_And_JS
func Test_Minify_HTML_Default_With_Css_And_JS(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New())

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCssAndJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)
	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.HTMLWithCssAndJs, string(body))
}

// go test -run Test_Minify_HTML_Config_Options_Styles_false
func Test_Minify_HTML_Config_Options_Styles_false(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(
		Config{
			MinifyHTML: true,
			MinifyHTMLOptions: MinifyHTMLOptions{
				MinifyStyles:  false,
				MinifyScripts: true,
			},
		},
	))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCssAndJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, []byte(expectedData.HTMLConfigOptionsStylesFalse), body)
}

// go test -run Test_Minify_HTML_Config_Options_Scripts_false
func Test_Minify_HTML_Config_Options_Scripts_false(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(
		Config{
			MinifyHTML: true,
			MinifyHTMLOptions: MinifyHTMLOptions{
				MinifyStyles:  true,
				MinifyScripts: false,
			},
		},
	))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCssAndJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.HTMLConfigOptionsJSFalse, string(body))
}

// go test -run Test_Minify_HTML_Config_Options_Css_JS_false
func Test_Minify_HTML_Config_Options_Css_JS_false(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(
		Config{
			MinifyHTML: true,
			MinifyHTMLOptions: MinifyHTMLOptions{
				MinifyStyles:  false,
				MinifyScripts: false,
			},
		},
	))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCssAndJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.HTMLConfigOptionsCssJSFalse, string(body))
}

// go test -run Test_Minify_HTML_Config_Options_Css_JS_true
func Test_Minify_HTML_Config_Options_Css_JS_true(t *testing.T) {
	t.Parallel()
	app := fiber.New()

	app.Use(New(
		Config{
			MinifyHTML: true,
			MinifyHTMLOptions: MinifyHTMLOptions{
				MinifyStyles:  true,
				MinifyScripts: true,
			},
		},
	))

	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(filedata.htmlWithCssAndJs)
	})
	req := httptest.NewRequest(fiber.MethodGet, "/", nil)
	resp, err := app.Test(req)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

	// check if resp body equals expectedData
	body, err := io.ReadAll(resp.Body)

	utils.AssertEqual(t, nil, err)
	utils.AssertEqual(t, expectedData.HTMLConfigOptionsCssJSTrue, string(body))
}

// go test -run Test_Minify_CSS
func Test_Minify_CSS(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		config      MinifyCSSOptions
		css         []byte
		expectedCSS string
		reqURL      string
		minify      bool
	}{
		{
			name:        "default config",
			css:         filedata.css,
			expectedCSS: expectedData.CSSMinify,
			reqURL:      "/style.css",
			minify:      true,
		},
		{
			name:        "options test default exclude extension *.min.css",
			css:         filedata.css,
			expectedCSS: expectedData.CSSMinify,
			reqURL:      "/style.min.css",
			minify:      false,
		},
		{
			name:        "options test default exclude extension *.bundle.css",
			css:         filedata.css,
			expectedCSS: expectedData.CSSMinify,
			reqURL:      "/style.bundle.css",
			minify:      false,
		},
		{
			name: "options exclude url /path/style.css",
			css:  filedata.css,
			config: MinifyCSSOptions{
				ExcludeStyles: []string{"style.css"},
			},
			expectedCSS: expectedData.CSSMinify,
			reqURL:      "/path/style.css",
			minify:      false,
		},
		{
			name: "options exclude wilcad urls *path/",
			css:  filedata.css,
			config: MinifyCSSOptions{
				ExcludeStyles: []string{"style.css"},
			},
			expectedCSS: expectedData.CSSMinify,
			reqURL:      "/path/style.css",
			minify:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(New(
				Config{
					MinifyCSS:        true,
					MinifyCSSOptions: tc.config,
				},
			))

			app.Get(tc.reqURL, func(c *fiber.Ctx) error {
				c.Set(fiber.HeaderContentType, "text/css")
				return c.Send(tc.css)
			})

			req := httptest.NewRequest(fiber.MethodGet, tc.reqURL, nil)
			resp, err := app.Test(req)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

			// check if resp body equals expectedData
			body, err := io.ReadAll(resp.Body)
			utils.AssertEqual(t, nil, err)
			if tc.minify {
				utils.AssertEqual(t, tc.expectedCSS, string(body))
			} else {
				utils.AssertEqual(t, tc.css, body)
			}
		})
	}
}

// go test -run Test_Minify_JS
func Test_Minify_JS(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		config     MinifyJSOptions
		js         []byte
		expectedJs string
		reqURL     string
		minify     bool
	}{
		{
			name:       "default config",
			js:         filedata.js,
			expectedJs: expectedData.JsMinify,
			reqURL:     "/script.js",
			minify:     true,
		},
		{
			name:       "options test default exclude extension *.min.js",
			js:         filedata.js,
			expectedJs: expectedData.JsMinify,
			reqURL:     "/script.min.js",
			minify:     false,
		},
		{
			name:       "options test default exclude extension *.bundle.js",
			js:         filedata.js,
			expectedJs: expectedData.JsMinify,
			reqURL:     "/script.bundle.js",
			minify:     false,
		},
		{
			name: "options exclude url /path/script.js",
			js:   filedata.js,
			config: MinifyJSOptions{
				ExcludeScripts: []string{"script.js"},
			},
			reqURL: "/path/script.js",
			minify: false,
		},
		{
			name: "options exclude wilcad urls path/*",
			js:   filedata.js,
			config: MinifyJSOptions{
				ExcludeScripts: []string{"path/*"},
			},
			reqURL: "/path/script.js",
			minify: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(New(
				Config{
					MinifyJS:        true,
					MinifyJSOptions: tc.config,
				},
			))

			app.Get(tc.reqURL, func(c *fiber.Ctx) error {
				c.Set(fiber.HeaderContentType, "text/javascript")
				return c.Send(tc.js)
			})

			req := httptest.NewRequest(fiber.MethodGet, tc.reqURL, nil)
			resp, err := app.Test(req)
			utils.AssertEqual(t, nil, err)
			utils.AssertEqual(t, 200, resp.StatusCode, "Status code")

			// check if resp body equals expectedData
			body, err := io.ReadAll(resp.Body)
			utils.AssertEqual(t, nil, err)
			if tc.minify {
				utils.AssertEqual(t, tc.expectedJs, string(body))
			} else {
				utils.AssertEqual(t, tc.js, body)
			}
		})
	}
}
