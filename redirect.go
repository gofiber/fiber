// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"strings"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/fiber/v3/utils"
	"github.com/valyala/bytebufferpool"
)

// Redirect is a struct to use it with Ctx.
type Redirect struct {
	c        *DefaultCtx       // Embed ctx
	status   int               // Status code of redirection. Default: StatusFound
	messages map[string]string // Flash messages
	oldInput map[string]string // Old input data
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
		c:        c,
		status:   StatusFound,
		messages: make(map[string]string, 0),
		oldInput: make(map[string]string, 0),
	}
}

// Status sets the status code of redirection.
// If status is not specified, status defaults to 302 Found.
func (r *Redirect) Status(code int) *Redirect {
	r.status = code

	return r
}

// You can send flash messages by using With().
// They will be sent as a cookie.
// You can get them by using: Redirect().Messages(), Redirect().Message()
func (r *Redirect) With(key string, value string) *Redirect {
	r.messages[key] = value

	return r
}

// You can send input data by using WithInput().
// They will be sent as a cookie.
// This method can send form, multipart form, query data to redirected route.
// You can get them by using: Redirect().OldInputs(), Redirect().OldInput()
func (r *Redirect) WithInput() *Redirect {
	// Get content-type
	ctype := utils.ToLower(utils.UnsafeString(r.c.Context().Request.Header.ContentType()))
	ctype = binder.FilterFlags(utils.ParseVendorSpecificContentType(ctype))

	// TODO: Maybe better implementation.
	switch ctype {
	case MIMEApplicationForm:
		_ = r.c.Bind().Form(r.oldInput)
	case MIMEMultipartForm:
		_ = r.c.Bind().MultipartForm(r.oldInput)
	default:
		_ = r.c.Bind().Query(r.oldInput)
	}

	return r
}

// Get flash messages.
func (r *Redirect) Messages() map[string]string {
	return r.c.flashMessages
}

// Get flash message by key.
func (r *Redirect) Message(key string) string {
	return r.c.flashMessages[key]
}

// Get old input data.
func (r *Redirect) OldInputs() map[string]string {
	return r.c.oldInput
}

// Get old input data by key.
func (r *Redirect) OldInput(key string) string {
	return r.c.oldInput[key]
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

	// Flash messages
	if len(r.messages) > 0 || len(r.oldInput) > 0 {
		messageText := bytebufferpool.Get()
		defer bytebufferpool.Put(messageText)

		// flash messages
		i := 1
		for k, v := range r.messages {
			if i != 1 {
				_, _ = messageText.WriteString("k:")
			}

			_, _ = messageText.WriteString(k + ":" + v)
			if len(r.messages) != i {
				_, _ = messageText.WriteString(",")
			}
			i++
		}

		// old input data
		i = 1
		for k, v := range r.oldInput {
			if i == 1 {
				_, _ = messageText.WriteString("k:")
			}

			_, _ = messageText.WriteString("old_input_data_" + k + ":" + v)
			if len(r.oldInput) != i {
				_, _ = messageText.WriteString(",")
			}
			i++
		}

		r.c.Cookie(&Cookie{
			Name:        "fiber_flash",
			Value:       r.c.app.getString(messageText.Bytes()),
			SessionOnly: true,
		})
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

		return r.To(location + "?" + r.c.app.getString(queryText.Bytes()))
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

// setFlash is a method to get flash messages before removing them
func (r *Redirect) setFlash() {
	// parse flash messages
	if r.c.Cookies("fiber_flash") != "" {
		messages := strings.Split(r.c.Cookies("fiber_flash"), ",k:")
		r.c.flashMessages = make(map[string]string)
		r.c.oldInput = make(map[string]string)

		for _, msg := range messages {
			splitMsg := strings.Split(msg, ":")

			// check old input data
			if strings.HasPrefix(msg, "old_input_data_") {
				r.c.oldInput[splitMsg[0][15:]] = splitMsg[1]
			} else {
				r.c.flashMessages[splitMsg[0]] = splitMsg[1]
			}
		}
	}

	r.c.ClearCookie("fiber_flash")
}
