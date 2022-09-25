// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/utils"
	"github.com/valyala/bytebufferpool"
)

var (
	// Pool for redirection
	redirectPool = sync.Pool{
		New: func() any {
			return &Redirect{
				status:   StatusFound,
				oldInput: make(map[string]string, 0),
			}
		},
	}
)

// Cookie name to send flash messages when to use redirection.
const (
	FlashCookieName     = "fiber_flash"
	OldInputDataPrefix  = "old_input_data_"
	CookieDataSeparator = ","
	CookieDataAssigner  = ":"
)

// Redirect is a struct to use it with Ctx.
type Redirect struct {
	c      *DefaultCtx // Embed ctx
	status int         // Status code of redirection. Default: StatusFound

	messages []string          // Flash messages
	oldInput map[string]string // Old input data
}

// A config to use with Redirect().Route()
// You can specify queries or route parameters.
// NOTE: We don't use net/url to parse parameters because of it has poor performance. You have to pass map.
type RedirectConfig struct {
	Params  Map               // Route parameters
	Queries map[string]string // Query map
}

// AcquireRedirect return default Redirect reference from the redirect pool
func AcquireRedirect() *Redirect {
	return redirectPool.Get().(*Redirect)
}

// ReleaseRedirect returns c acquired via Redirect to redirect pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to redirect pool.
func ReleaseRedirect(r *Redirect) {
	r.release()
	redirectPool.Put(r)
}

func (r *Redirect) release() {
	r.status = 302
	r.messages = r.messages[:0]
	// reset map
	for k := range r.oldInput {
		delete(r.oldInput, k)
	}
	r.c = nil
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
// Note: You must use escape char before using ',' and ':' chars to avoid wrong parsing.
func (r *Redirect) With(key string, value string) *Redirect {
	r.messages = append(r.messages, key+CookieDataAssigner+value)

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
	msgs := r.c.redirectionMessages
	flashMessages := make(map[string]string, len(msgs))

	for _, msg := range msgs {
		k, v := parseMessage(msg)

		if !strings.HasPrefix(k, OldInputDataPrefix) {
			flashMessages[k] = v
		}
	}

	return flashMessages
}

// Get flash message by key.
func (r *Redirect) Message(key string) string {
	msgs := r.c.redirectionMessages

	for _, msg := range msgs {
		k, v := parseMessage(msg)

		if !strings.HasPrefix(k, OldInputDataPrefix) && k == key {
			return v
		}
	}
	return ""
}

// Get old input data.
func (r *Redirect) OldInputs() map[string]string {
	msgs := r.c.redirectionMessages
	oldInputs := make(map[string]string, len(msgs))

	for _, msg := range msgs {
		k, v := parseMessage(msg)

		if strings.HasPrefix(k, OldInputDataPrefix) {
			// remove "old_input_data_" part from key
			oldInputs[k[len(OldInputDataPrefix):]] = v
		}
	}
	return oldInputs
}

// Get old input data by key.
func (r *Redirect) OldInput(key string) string {
	msgs := r.c.redirectionMessages

	for _, msg := range msgs {
		k, v := parseMessage(msg)

		if strings.HasPrefix(k, OldInputDataPrefix) && k[len(OldInputDataPrefix):] == key {
			return v
		}
	}
	return ""

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
		for i, message := range r.messages {
			_, _ = messageText.WriteString(message)
			// when there are more messages or oldInput -> add a comma
			if len(r.messages)-1 != i || (len(r.messages)-1 == i && len(r.oldInput) > 0) {
				_, _ = messageText.WriteString(CookieDataSeparator)
			}
		}
		r.messages = r.messages[:0]

		// old input data
		i := 1
		for k, v := range r.oldInput {
			_, _ = messageText.WriteString(OldInputDataPrefix + k + CookieDataAssigner + v)
			if len(r.oldInput) != i {
				_, _ = messageText.WriteString(CookieDataSeparator)
			}
			i++
		}

		r.c.Cookie(&Cookie{
			Name:        FlashCookieName,
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
func (r *Redirect) Back(fallback ...string) error {
	location := r.c.Get(HeaderReferer)
	if location == "" {
		// Check fallback URL
		if len(fallback) == 0 {
			err := ErrRedirectBackNoFallback
			r.c.Status(err.Code)

			return err
		}
		location = fallback[0]
	}

	return r.To(location)
}

// setFlash is a method to get flash messages before removing them
func (r *Redirect) setFlash() {
	// parse flash messages
	cookieValue := r.c.Cookies(FlashCookieName)

	var commaPos int
	for {
		commaPos = findNextNonEscapedCharsetPosition(cookieValue, []byte(CookieDataSeparator))
		if commaPos != -1 {
			r.c.redirectionMessages = append(r.c.redirectionMessages, strings.Trim(cookieValue[:commaPos], " "))
			cookieValue = cookieValue[commaPos+1:]
		} else {
			r.c.redirectionMessages = append(r.c.redirectionMessages, strings.Trim(cookieValue, " "))
			break
		}
	}

	r.c.ClearCookie(FlashCookieName)
}

func parseMessage(raw string) (key, value string) {
	if i := findNextNonEscapedCharsetPosition(raw, []byte(CookieDataAssigner)); i != -1 {
		return RemoveEscapeChar(raw[:i]), RemoveEscapeChar(raw[i+1:])
	}
	return RemoveEscapeChar(raw), ""
}
