package minify

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	// Set default config
	cfg := configDefault(config...)

	return func(c *fiber.Ctx) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Check if request method is in the list of methods to minify
		method := string(cfg.Method)
		if method != "ALL" {
			if c.Method() != method {
				return c.Next()
			}
		}

		// Continue stack
		if err := c.Next(); err != nil {
			return err
		}

		// Get the response content type
		contentType := string(c.Response().Header.ContentType())

		// check if content type has text/html from text/html; charset=utf-8
		if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "text/css") || strings.Contains(contentType, "text/javascript") {
			// the response body has a supported content type, minify it
			if strings.Contains(contentType, "text/html") {
				if cfg.MinifyHTML {
					// check if the path is in the list of paths to exclude from minification
					exclude := excludeMinify(cfg.MinifyHTMLOptions.ExcludeURLs, c.Path())
					if exclude {
						return nil
					}
					// set the options for minification
					opt := &Options{
						MinifyScripts: cfg.MinifyHTMLOptions.MinifyScripts,
						MinifyStyles:  cfg.MinifyHTMLOptions.MinifyStyles,
					}
					// Minify HTML response
					minifiedHTML, err := htmlMinify(c.Response().Body(), opt)
					if err != nil {
						c.Response().SetBody(minifiedHTML)
					}
					c.Response().SetBody(minifiedHTML)
				}
			}
			// the response body has a text/css content type, minify it
			if strings.Contains(contentType, "text/css") {
				if cfg.MinifyCSS {
					// check if the path is in the list of paths to exclude from minification
					exclude := excludeMinify(cfg.MinifyCSSOptions.ExcludeStyles, c.Path())
					if exclude {
						return nil
					}
					// minify the css
					minifiedCss := cssMinify(c.Response().Body())
					c.Response().SetBody(minifiedCss)
				}
			}
			// the response body has a text/javascript content type, minify it
			if strings.Contains(contentType, "text/javascript") {
				if cfg.MinifyJS {
					// check if the path is in the list of paths to exclude from minification
					exclude := excludeMinify(cfg.MinifyJSOptions.ExcludeScripts, c.Path())
					if exclude {
						return nil
					}
					// minify the javascript
					minifiedJS, err := jsMinify(c.Response().Body())
					if err != nil {
						return nil
					}
					c.Response().SetBody(minifiedJS)
				}
			}
		}
		return nil
	}
}

// excludeMinify checks if the path is in the list of paths to exclude from minification
func excludeMinify(excludes []string, path string) bool {
	if len(excludes) > 0 {
		for _, exc := range excludes {
			if strings.Contains(path, exc) {
				return true
			}
			// if it has a wildcard, check if the path contains the wildcard
			if strings.Contains(exc, "*") {
				if strings.Contains(path, strings.Replace(exc, "*", "", -1)) {
					return true
				}
			}
		}
	}
	return false
}
