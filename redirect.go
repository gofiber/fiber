// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"errors"
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
			messages: make(redirectionMsgs, 0),
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

// redirectionMsgs is a struct that used to store flash messages and old input data in cookie using MSGP.
// msgp -file="redirect.go" -o="redirect_msgp.go" -unexported
//
//msgp:ignore Redirect RedirectConfig OldInputData FlashMessage
type redirectionMsg struct {
	key        string
	value      string
	level      uint8
	isOldInput bool
}

type redirectionMsgs []redirectionMsg

// OldInputData is a struct that holds the old input data.
type OldInputData struct {
	Key   string
	Value string
}

// FlashMessage is a struct that holds the flash message data.
type FlashMessage struct {
	Key   string
	Value string
	Level uint8
}

// Redirect is a struct that holds the redirect data.
type Redirect struct {
	c        *DefaultCtx     // Embed ctx
	messages redirectionMsgs // Flash messages and old input data
	status   int             // Status code of redirection. Default: StatusFound
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
func (r *Redirect) With(key, value string, level ...uint8) *Redirect {
	// Get level
	var msgLevel uint8
	if len(level) > 0 {
		msgLevel = level[0]
	}

	// Override old message if exists
	for i, msg := range r.messages {
		if msg.key == key && !msg.isOldInput {
			r.messages[i].value = value
			r.messages[i].level = msgLevel

			return r
		}
	}

	r.messages = append(r.messages, redirectionMsg{
		key:   key,
		value: value,
		level: msgLevel,
	})

	return r
}

// WithInput You can send input data by using WithInput().
// They will be sent as a cookie.
// This method can send form, multipart form, query data to redirected route.
// You can get them by using: Redirect().OldInputs(), Redirect().OldInput()
func (r *Redirect) WithInput() *Redirect {
	// Get content-type
	ctype := utils.ToLower(utils.UnsafeString(r.c.RequestCtx().Request.Header.ContentType()))
	ctype = binder.FilterFlags(utils.ParseVendorSpecificContentType(ctype))

	oldInput := make(map[string]string)
	switch ctype {
	case MIMEApplicationForm, MIMEMultipartForm:
		_ = r.c.Bind().Form(oldInput) //nolint:errcheck // not needed
	default:
		_ = r.c.Bind().Query(oldInput) //nolint:errcheck // not needed
	}

	// Add old input data
	for k, v := range oldInput {
		r.messages = append(r.messages, redirectionMsg{
			key:        k,
			value:      v,
			isOldInput: true,
		})
	}

	return r
}

// Messages Get flash messages.
func (r *Redirect) Messages() []FlashMessage {
	flashMessages := make([]FlashMessage, 0)

	for _, msg := range r.c.flashMessages {
		if !msg.isOldInput {
			flashMessages = append(flashMessages, FlashMessage{
				Key:   msg.key,
				Value: msg.value,
				Level: msg.level,
			})
		}
	}

	return flashMessages
}

// Message Get flash message by key.
func (r *Redirect) Message(key string) FlashMessage {
	msgs := r.c.flashMessages

	for _, msg := range msgs {
		if msg.key == key && !msg.isOldInput {
			return FlashMessage{
				Key:   msg.key,
				Value: msg.value,
				Level: msg.level,
			}
		}
	}

	return FlashMessage{}
}

// OldInputs Get old input data.
func (r *Redirect) OldInputs() []OldInputData {
	inputs := make([]OldInputData, 0)

	for _, msg := range r.c.flashMessages {
		if msg.isOldInput {
			inputs = append(inputs, OldInputData{
				Key:   msg.key,
				Value: msg.value,
			})
		}
	}

	return inputs
}

// OldInput Get old input data by key.
func (r *Redirect) OldInput(key string) OldInputData {
	msgs := r.c.flashMessages

	for _, msg := range msgs {
		if msg.key == key && msg.isOldInput {
			return OldInputData{
				Key:   msg.key,
				Value: msg.value,
			}
		}
	}

	return OldInputData{}
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

// parseAndClearFlashMessages is a method to get flash messages before they are getting removed
func (r *Redirect) parseAndClearFlashMessages() {
	// parse flash messages
	cookieValue := r.c.Cookies(FlashCookieName)

	_, err := r.c.flashMessages.UnmarshalMsg(r.c.app.getBytes(cookieValue))
	if err != nil {
		return
	}
}

// processFlashMessages is a helper function to process flash messages and old input data
// and set them as cookies
func (r *Redirect) processFlashMessages() {
	if len(r.messages) == 0 {
		return
	}

	val, err := r.messages.MarshalMsg(nil)
	if err != nil {
		return
	}

	r.c.Cookie(&Cookie{
		Name:        FlashCookieName,
		Value:       r.c.app.getString(val),
		SessionOnly: true,
	})
}
