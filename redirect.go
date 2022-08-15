// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"github.com/valyala/bytebufferpool"
)

// Redirect is a struct to use it with Ctx.
type Redirect struct {
	// Embed ctx
	c      *DefaultCtx
	status int // Default: StatusFound
}

// A config to use with Redirect().Route()
// You can specify queries or route parameters.
// NOTE: We don't use net/url to parse parameters because of it has poor performance. You have to pass map.
type RedirectConfig struct {
	Params  Map               // Route parameters
	Queries map[string]string // Query map
}

// Return default Redirect reference.
func newRedirect(c *DefaultCtx) *Redirect {
	return &Redirect{
		c:      c,
		status: StatusFound,
	}
}

// Status sets the status code of redirection.
// If status is not specified, status defaults to 302 Found.
func (r *Redirect) Status(code int) *Redirect {
	r.status = code

	return r
}

// Redirect to the URL derived from the specified path, with specified status.
func (r *Redirect) To(location string) error {
	r.c.setCanonical(HeaderLocation, location)
	r.c.Status(r.status)

	return nil
}

// Route redirects to the Route registered in the app with appropriate parameters.
// If you want to send queries or params to route, you should use config parameter.
func (r *Redirect) Route(name string, config ...RedirectConfig) error {
	// Check config
	cfg := RedirectConfig{}
	if len(config) > 0 {
		cfg = config[0]
	}

	// Get location from route name
	location, err := r.c.getLocationFromRoute(r.c.App().GetRoute(name), cfg.Params)
	if err != nil {
		return err
	}

	// Check queries
	if len(cfg.Queries) > 0 {
		queryText := bytebufferpool.Get()
		defer bytebufferpool.Put(queryText)

		i := 1
		for k, v := range cfg.Queries {
			_, _ = queryText.WriteString(k + "=" + v)

			if i != len(cfg.Queries) {
				_, _ = queryText.WriteString("&")
			}
			i++
		}

		return r.To(location + "?" + queryText.String())
	}

	return r.To(location)
}

// Redirect back to the URL to referer.
// TODO: Should fallback be optional?
func (r *Redirect) Back(fallback string) error {
	location := r.c.Get(HeaderReferer)
	if location == "" {
		location = fallback
	}
	return r.To(location)
}
