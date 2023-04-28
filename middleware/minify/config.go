package minify

import (
	"github.com/gofiber/fiber/v2"
)

// Config defines the config for middleware.
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	// Optional. Default: nil
	Next func(c *fiber.Ctx) bool

	// MinifyHTML is a boolean value that indicates whether HTML responses should be minified.
	// Optional. Default: true
	MinifyHTML bool

	// MinifyHTMLOptions is used to configure the HTML minifier.
	// Optional.
	MinifyHTMLOptions MinifyHTMLOptions

	// MinifyCSS is a boolean value that indicates whether CSS responses should be minified.
	// Optional. Default: false
	MinifyCSS bool

	// MinifyCSSOptions is used to configure the CSS minifier.
	// Optional.
	MinifyCSSOptions MinifyCSSOptions

	// MinifyJS is a boolean value that indicates whether JavaScript responses should be minified.
	// Optional. Default: false
	MinifyJS bool

	// MinifyJSOptions is used to configure the JavaScript minifier.
	// Optional.
	MinifyJSOptions MinifyJSOptions

	// Method is a string representation of minify route method.
	// Possible values: GET, HEAD, POST, ALL.
	// Optional. Default: GET (only minify GET requests).
	Method Method
}

type MinifyHTMLOptions struct {
	// boolean value that indicates whether scripts inside the HTML should be minified or not.
	// Optional. Default: false
	MinifyScripts bool

	// boolean value that indicates whether styles inside the HTML should be minified or not.
	// Optional. Default: false
	MinifyStyles bool

	// ExcludeURLs is a slice of strings that contains URLs that should be excluded from minification.
	// Possible patterns: "/exact-url", "urlgroup/*"
	// Optional. Default: nil
	ExcludeURLs []string
}

type MinifyCSSOptions struct {
	// ExcludeURLs is a slice of strings that contains URLs to the styles that should be excluded from minification.
	// Possible patterns: "/path/to/style.css", "/path/to/*", "*.min.css"
	// Optional. Default: "*.min.css", "*.bundle.css"
	ExcludeStyles []string
}

type MinifyJSOptions struct {
	// ExcludeURLs is a slice of strings that contains URLs to the scripts that should be excluded from minification.
	// Possible patterns: "/path/to/script.js", "/path/to/*", "*.min.js"
	// Optional. Default: "*.min.js", "*.bundle.js"
	ExcludeScripts []string
}

// Method is a string representation of minify route method
type Method string

// Represents minify method that will be used in the middleware
const (
	GET  Method = "GET"
	HEAD Method = "HEAD"
	POST Method = "POST"
	ALL  Method = "ALL"
)

// ConfigDefault is the default config
var ConfigDefault = Config{
	Next:       nil,
	MinifyHTML: true,
	MinifyHTMLOptions: MinifyHTMLOptions{
		MinifyScripts: false,
		MinifyStyles:  false,
		ExcludeURLs:   nil,
	},
	MinifyCSS: false,
	MinifyCSSOptions: MinifyCSSOptions{
		ExcludeStyles: []string{"*.min.css", "*.bundle.css"},
	},
	MinifyJS: false,
	MinifyJSOptions: MinifyJSOptions{
		ExcludeScripts: []string{"*.min.js", "*.bundle.js"},
	},
	Method: GET,
}

// Helper function to set default values
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Method == "" {
		cfg.Method = ConfigDefault.Method
	}

	if equalMinifyHTMLOptions(cfg.MinifyHTMLOptions, &MinifyHTMLOptions{}) {
		cfg.MinifyHTMLOptions = ConfigDefault.MinifyHTMLOptions
	}

	if cfg.MinifyCSSOptions.ExcludeStyles == nil {
		cfg.MinifyCSSOptions = ConfigDefault.MinifyCSSOptions
	}

	if cfg.MinifyJSOptions.ExcludeScripts == nil {
		cfg.MinifyJSOptions = ConfigDefault.MinifyJSOptions
	}
	return cfg
}

// Helper function to compare MinifyHTMLOptions
func equalMinifyHTMLOptions(a MinifyHTMLOptions, b *MinifyHTMLOptions) bool {
	return a.MinifyScripts == b.MinifyScripts &&
		a.MinifyStyles == b.MinifyStyles &&
		equalStringSlices(a.ExcludeURLs, b.ExcludeURLs)
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
