// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofibel.io

package fiber

import "strings"

// Layer is a struct that holds all metadata for each registered handler
type Layer struct {
	// Internal fields
	use    bool         // USE matches path prefixes
	star   bool         // Path equals '*' or '/*'
	root   bool         // Path equals '/'
	parsed parsedParams // parsed contains parsed params segments

	// External fields for ctx.Route() method
	Path    string     // Registered route path
	Method  string     // HTTP method
	Params  []string   // Slice containing the params names
	Handler func(*Ctx) // Ctx handler
}

func (l *Layer) match(path string) (match bool, values []string) {
	if l.use {
		if l.root || strings.HasPrefix(path, l.Path) {
			return true, values
		}
		// Check for a simple path match
	} else if len(l.Path) == len(path) && l.Path == path {
		return true, values
		// Middleware routes allow prefix matches
	} else if l.root && path == "/" {
		return true, values
	}
	// '*' wildcard matches any path
	if l.star {
		return true, []string{path}
	}
	// Does this route have parameters
	if len(l.Params) > 0 {
		// Match params
		if values, match = l.parsed.getMatch(path, l.use); match {
			return
		}
	}
	// No match
	return false, values
}
