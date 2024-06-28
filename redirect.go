// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3/binder"
	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
)

// Pool for redirection
var redirectPool = sync.Pool{
	New: func() any {
		return &Redirect{
			status:   StatusFound,
			oldInput: make(map[string]string, 0),
		}
	},
}

// Cookie name to send flash messages when to use redirection.
const (
	FlashCookieName     = "fiber_flash"
	OldInputDataPrefix  = "old_input_data_"
	CookieDataSeparator = ","
	CookieDataAssigner  = ":"
)

// Redirect is a struct that holds the redirect data.
type Redirect struct {
	c      *DefaultCtx // Embed ctx
	status int         // Status code of redirection. Default: StatusFound

	messages []string          // Flash messages
	oldInput map[string]string // Old input data
}

// RedirectConfig A config to use with Redirect().Route()
// You can specify queries or route parameters.
// NOTE: We don't use net/url to parse parameters because of it has poor performance. You have to pass map.
type RedirectConfig struct {
	Params  Map               // Route parameters
	Queries map[string]string // Query map
}

// AcquireRedirect return default Redirect reference from the redirect pool
func AcquireRedirect() *Redirect {
	redirect, ok := redirectPool.Get().(*Redirect)
	if !ok {
		panic(errors.New("failed to type-assert to *Redirect"))
	}

	return redirect
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

// With You can send flash messages by using With().
// They will be sent as a cookie.
// You can get them by using: Redirect().Messages(), Redirect().Message()
// Note: You must use escape char before using ',' and ':' chars to avoid wrong parsing.
func (r *Redirect) With(key, value string) *Redirect {
	r.messages = append(r.messages, key+CookieDataAssigner+value)

	return r
}

// WithInput You can send input data by using WithInput().
// They will be sent as a cookie.
// This method can send form, multipart form, query data to redirected route.
// You can get them by using: Redirect().OldInputs(), Redirect().OldInput()
func (r *Redirect) WithInput() *Redirect {
	// Get content-type
	ctype := utils.ToLower(utils.UnsafeString(r.c.Context().Request.Header.ContentType()))
	ctype = binder.FilterFlags(utils.ParseVendorSpecificContentType(ctype))

	switch ctype {
	case MIMEApplicationForm:
		_ = r.c.Bind().Form(r.oldInput) //nolint:errcheck // not needed
	case MIMEMultipartForm:
		_ = r.c.Bind().MultipartForm(r.oldInput) //nolint:errcheck // not needed
	default:
		_ = r.c.Bind().Query(r.oldInput) //nolint:errcheck // not needed
	}

	return r
}

// Messages Get flash messages.
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

// Message Get flash message by key.
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

// OldInputs Get old input data.
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

// OldInput Get old input data by key.
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

// To redirect to the URL derived from the specified path, with specified status.
func (r *Redirect) To(location string) error {
	r.c.setCanonical(HeaderLocation, location)
	r.c.Status(r.status)

	r.processFlashMessages()

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
			queryText.WriteString(k + "=" + v)

			if i != len(cfg.Queries) {
				queryText.WriteString("&")
			}
			i++
		}

		return r.To(location + "?" + r.c.app.getString(queryText.Bytes()))
	}

	return r.To(location)
}

// Back redirect to the URL to referer.
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

// parseAndClearFlashMessages is a method to get flash messages before removing them
func (r *Redirect) parseAndClearFlashMessages() {
	// parse flash messages
	cookieValue := r.c.Cookies(FlashCookieName)

	var commaPos int
	for {
		commaPos = findNextNonEscapedCharsetPosition(cookieValue, []byte(CookieDataSeparator))
		if commaPos == -1 {
			r.c.redirectionMessages = append(r.c.redirectionMessages, strings.Trim(cookieValue, " "))
			break
		}
		r.c.redirectionMessages = append(r.c.redirectionMessages, strings.Trim(cookieValue[:commaPos], " "))
		cookieValue = cookieValue[commaPos+1:]
	}

	r.c.ClearCookie(FlashCookieName)
}

// processFlashMessages is a helper function to process flash messages and old input data
// and set them as cookies
func (r *Redirect) processFlashMessages() {
	// Flash messages
	if len(r.messages) > 0 || len(r.oldInput) > 0 {
		messageText := bytebufferpool.Get()
		defer bytebufferpool.Put(messageText)

		// flash messages
		for i, message := range r.messages {
			messageText.WriteString(message)
			// when there are more messages or oldInput -> add a comma
			if len(r.messages)-1 != i || (len(r.messages)-1 == i && len(r.oldInput) > 0) {
				messageText.WriteString(CookieDataSeparator)
			}
		}
		r.messages = r.messages[:0]

		// old input data
		i := 1
		for k, v := range r.oldInput {
			messageText.WriteString(OldInputDataPrefix + k + CookieDataAssigner + v)
			if len(r.oldInput) != i {
				messageText.WriteString(CookieDataSeparator)
			}
			i++
		}

		r.c.Cookie(&Cookie{
			Name:        FlashCookieName,
			Value:       r.c.app.getString(messageText.Bytes()),
			SessionOnly: true,
		})
	}
}

// parseMessage is a helper function to parse flash messages and old input data
func parseMessage(raw string) (string, string) { //nolint: revive // not necessary
	if i := findNextNonEscapedCharsetPosition(raw, []byte(CookieDataAssigner)); i != -1 {
		return RemoveEscapeChar(raw[:i]), RemoveEscapeChar(raw[i+1:])
	}

	return RemoveEscapeChar(raw), ""
}
