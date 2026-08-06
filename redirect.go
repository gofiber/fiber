// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 📝 GitHub Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package fiber

import (
	"bytes"
	"encoding/hex"
	"net/url"
	"strings"
	"sync"

	"github.com/gofiber/utils/v2"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"

	"github.com/gofiber/fiber/v3/internal/fieldname"
	"github.com/gofiber/fiber/v3/internal/schemehost"
)

// Pool for redirection
var (
	redirectPool = sync.Pool{
		New: func() any {
			return &Redirect{
				status:   StatusSeeOther,
				messages: make(redirectionMsgs, 0),
			}
		},
	}
	oldInputPool = sync.Pool{
		New: func() any {
			return make(map[string]string)
		},
	}
)

const maxPoolableMapSize = 64

// FlashCookieName Cookie name to send flash messages when to use redirection.
const (
	FlashCookieName = "fiber_flash"
)

var flashCookieNeedle = []byte(FlashCookieName + "=")

// hasFlashCookie is on the request hot path and runs on every request/response cycle.
// Keep this cheap for users who don't use flash messages:
// 1) a fast raw-header prefilter to avoid unnecessary cookie parsing,
// 2) an exact cookie lookup to avoid prefix false positives (e.g. fiber_flashX).
func hasFlashCookie(header *fasthttp.RequestHeader) bool {
	rawHeaders := header.RawHeaders()
	if len(rawHeaders) == 0 {
		return false
	}

	if !bytes.Contains(rawHeaders, flashCookieNeedle) {
		return false
	}

	return header.Cookie(FlashCookieName) != nil
}

// redirectionMsgs is a struct that used to store flash messages and old input data in cookie using MSGP.
// msgp -file="redirect.go" -o="redirect_msgp.go" -unexported
// Cookie payloads are limited to ~4KB, so keep flash message counts bounded but usable.
//
//msgp:limit arrays:256 maps:32 marshal:true
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
	status   int             // Status code of redirection. Default: 303 StatusSeeOther
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
		panic(errRedirectTypeAssertion)
	}

	return redirect
}

// ReleaseRedirect returns c acquired via Redirect to redirect pool.
//
// It is forbidden accessing req and/or its members after returning
// it to redirect pool.
func ReleaseRedirect(r *Redirect) {
	r.release()
	redirectPool.Put(r)
}

func (r *Redirect) release() {
	r.status = StatusSeeOther
	// Zero before truncating: WithInput copies the whole request body into these
	// entries, and a bare [:0] leaves those strings reachable from the backing
	// array for as long as the pooled *Redirect lives.
	clear(r.messages[:cap(r.messages)])
	r.messages = r.messages[:0]
	r.c = nil
}

func acquireOldInput() map[string]string {
	oldInput, ok := oldInputPool.Get().(map[string]string)
	if !ok {
		return make(map[string]string)
	}

	return oldInput
}

func releaseOldInput(oldInput map[string]string) {
	if len(oldInput) > maxPoolableMapSize {
		return
	}

	clear(oldInput)
	oldInputPool.Put(oldInput)
}

// Status sets the status code of redirection.
// If status is not specified, status defaults to 303 See Other.
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
	ctype := bindMediaType(&r.c.RequestCtx().Request.Header)

	oldInput := acquireOldInput()
	defer releaseOldInput(oldInput)

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
	if len(r.c.flashMessages) == 0 {
		return nil
	}

	flashMessages := make([]FlashMessage, 0, len(r.c.flashMessages))
	writeIdx := 0

	for _, msg := range r.c.flashMessages {
		if msg.isOldInput {
			r.c.flashMessages[writeIdx] = msg
			writeIdx++
			continue
		}

		flashMessages = append(flashMessages, FlashMessage{
			Key:   msg.key,
			Value: msg.value,
			Level: msg.level,
		})
	}

	for i := writeIdx; i < len(r.c.flashMessages); i++ {
		r.c.flashMessages[i] = redirectionMsg{}
	}

	r.c.flashMessages = r.c.flashMessages[:writeIdx]

	return flashMessages
}

// Message Get flash message by key.
func (r *Redirect) Message(key string) FlashMessage {
	if len(r.c.flashMessages) == 0 {
		return FlashMessage{}
	}

	var flashMessage FlashMessage
	found := false
	writeIdx := 0

	for _, msg := range r.c.flashMessages {
		if msg.isOldInput || found || msg.key != key {
			r.c.flashMessages[writeIdx] = msg
			writeIdx++
			continue
		}

		flashMessage = FlashMessage{
			Key:   msg.key,
			Value: msg.value,
			Level: msg.level,
		}
		found = true
	}

	for i := writeIdx; i < len(r.c.flashMessages); i++ {
		r.c.flashMessages[i] = redirectionMsg{}
	}

	r.c.flashMessages = r.c.flashMessages[:writeIdx]

	return flashMessage
}

// OldInputs Get old input data.
func (r *Redirect) OldInputs() []OldInputData {
	// Count old inputs first to avoid allocation if none exist
	count := 0
	for _, msg := range r.c.flashMessages {
		if msg.isOldInput {
			count++
		}
	}

	if count == 0 {
		return nil
	}

	inputs := make([]OldInputData, 0, count)
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

	// Get location from route name. The composed path is already held to this
	// origin — see asRoutePath — so only the query is left to place.
	route := r.c.App().GetRoute(name)
	location, err := r.c.getLocationFromRoute(&route, cfg.Params)
	if err != nil {
		return err
	}

	// Check queries
	if len(cfg.Queries) > 0 {
		queryText := bytebufferpool.Get()
		defer bytebufferpool.Put(queryText)

		first := true
		for k, v := range cfg.Queries {
			if !first {
				queryText.WriteByte('&')
			}
			first = false
			queryText.WriteString(k)
			queryText.WriteByte('=')
			// Escape the value so characters like &, =, # or + don't break the
			// query string. Keys are left as-is to preserve the nested-key
			// convention (e.g. `data[0][name]`) used elsewhere.
			queryText.B = utils.AppendQueryEscape(queryText.B, v)
		}

		// Concatenated here rather than in a helper: the result goes straight to
		// To, so escape analysis can build it on the stack. Returning it from a
		// function cost an allocation per redirect.
		query := r.c.app.toString(queryText.Bytes())
		separator, fragment := queryMergePoint(location)
		if fragment < 0 {
			return r.To(location + separator + query)
		}
		return r.To(location[:fragment] + separator + query + location[fragment:])
	}

	return r.To(location)
}

// queryMergePoint says where a query belongs in location: the separator that
// introduces it, and where the fragment starts, or -1. A parameter spliced into
// a route path may hold both, and appending "?" folded them into one value.
func queryMergePoint(location string) (separator string, fragment int) { //nolint:nonamedreturns // the two results are easy to swap without names
	fragment = strings.IndexByte(location, '#')

	query := location
	if fragment >= 0 {
		query = location[:fragment]
	}
	if strings.IndexByte(query, '?') >= 0 {
		return "&", fragment
	}
	return "?", fragment
}

// Back redirect to the URL to referer.
// It validates that the Referer is same-origin to prevent open redirect attacks.
// If the Referer is missing, invalid, or cross-origin, the fallback URL is used.
func (r *Redirect) Back(fallback ...string) error {
	location := r.refererHeader()
	if location != "" {
		// Reject invalid or cross-origin referrers, and redirect to the
		// normalized form so every client resolves it the same way the
		// same-origin check did.
		normalized, ok := r.sameOriginReferer(location)
		if !ok {
			normalized = ""
		}
		location = normalized
	}

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

// refererHeader returns the Referer, matching the field name the way a recipient
// must (RFC 9110 §5.1). Ctx.Get is byte-exact, so under DisableHeaderNormalizing
// a lower-case "referer:" read as absent and Back always took the fallback.
func (r *Redirect) refererHeader() string {
	if v := r.c.Get(HeaderReferer); v != "" {
		return v
	}
	if !r.c.app.config.DisableHeaderNormalizing {
		return ""
	}

	return r.c.app.toString(fieldname.First(&r.c.fasthttp.Request.Header, HeaderReferer, false))
}

// sameOriginReferer reports whether the Referer resolves back to the origin now
// being served, returning the normalized form safe to echo. A browser's
// normalization decides that, not net/url: " //evil.com", "/\evil.com", "///x".
func (r *Redirect) sameOriginReferer(location string) (string, bool) {
	// The normalized value is what gets redirected to, so a client that does
	// not apply these rules itself cannot resolve a different origin than the
	// one validated here.
	location = normalizeRefererURL(location)
	if location == "" {
		return "", false
	}

	// An absolute path stays on the current origin, but "//host" is a
	// network-path reference that switches to another one.
	if location[0] == '/' && !strings.HasPrefix(location, "//") {
		return location, true
	}

	parsed, err := url.Parse(location)
	if err != nil {
		return "", false
	}
	if parsed.Host == "" {
		// A scheme with no authority ("javascript:", "mailto:") is no same-origin
		// reference; anything else is a relative path — except an empty authority
		// ("//?x", "//#f", "//@"), which names no origin and resolves nowhere.
		return location, parsed.Scheme == "" && !strings.HasPrefix(location, "//")
	}

	return location, schemehost.Match(parsed.Scheme, parsed.Host, r.c.Scheme(), r.c.Host())
}

// normalizeRefererURL applies the parts of the WHATWG parser that decide which
// origin a Referer resolves to: strip surrounding C0 controls, drop tab and
// newline, fold backslashes, collapse a leading slash run to two.
func normalizeRefererURL(location string) string {
	location = strings.TrimFunc(location, func(c rune) bool { return c <= ' ' })

	var b []byte
	inPath := true
	for i := 0; i < len(location); i++ {
		c := location[i]
		switch {
		case c == '\t' || c == '\n' || c == '\r':
			if b == nil {
				b = append(make([]byte, 0, len(location)), location[:i]...)
			}
			continue
		case c == '?' || c == '#':
			inPath = false
		case c == '\\' && inPath:
			if b == nil {
				b = append(make([]byte, 0, len(location)), location[:i]...)
			}
			b = append(b, '/')
			continue
		}
		if b != nil {
			b = append(b, c)
		}
	}
	if b != nil {
		location = string(b)
	}

	// "///evil.com" reaches the same authority state as "//evil.com", so keep
	// at most the two slashes that introduce it.
	if strings.HasPrefix(location, "//") {
		n := 0
		for n < len(location) && location[n] == '/' {
			n++
		}
		if n == len(location) {
			return "/"
		}
		location = location[n-2:]
	}

	return location
}

// parseAndClearFlashMessages is a method to get flash messages before they are getting removed
func (r *Redirect) parseAndClearFlashMessages() {
	// UnmarshalMsg re-slices this capacity rather than allocating, and the
	// generated decoder assigns only the fields the message carries — so an empty
	// map keeps whatever the slot held, and the destination must arrive zeroed.
	clear(r.c.flashMessages[:cap(r.c.flashMessages)])
	r.c.flashMessages = r.c.flashMessages[:0]

	// parse flash messages
	if cookieValue, err := hex.DecodeString(r.c.Cookies(FlashCookieName)); err == nil {
		if _, err := r.c.flashMessages.UnmarshalMsg(cookieValue); err != nil {
			// UnmarshalMsg re-slices to the length the payload declared before
			// it decodes any element, so a failure partway leaves the slice
			// holding zero-valued entries. Drop them.
			r.c.flashMessages = r.c.flashMessages[:0]
		}
	}

	// Expire the cookie whatever the payload turned out to be, including the
	// hex-decode failure above. A value this application cannot decode it never
	// will, so leaving it means re-running hex and msgp on every later request.
	r.c.Cookie(&Cookie{
		Name:     FlashCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
		Secure:   r.c.Secure(),
	})
}

// processFlashMessages sets the flash messages and old input data as cookies.
//
// The payload is hex-encoded, not signed, and WithInput copies the whole form
// into it — so the cookie is HTTPOnly, and Secure whenever the request was TLS.
func (r *Redirect) processFlashMessages() {
	if len(r.messages) == 0 {
		return
	}

	val, err := r.messages.MarshalMsg(nil)
	if err != nil {
		return
	}

	dst := make([]byte, hex.EncodedLen(len(val)))
	hex.Encode(dst, val)

	r.c.Cookie(&Cookie{
		Name:        FlashCookieName,
		Value:       r.c.app.toString(dst),
		SessionOnly: true,
		HTTPOnly:    true,
		Secure:      r.c.Secure(),
	})
}
