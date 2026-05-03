package log

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
)

// TagContextValue reads a value from the bound context-like value using the tag parameter as the key.
// Use it inside a format string as `${value:KEY}`. The trailing colon is required: it marks the tag
// as parametric. Registering a tag named "value:" via RegisterContextTag is rejected — the
// renderer is reserved for context-value lookups.
const TagContextValue = "value:"

// Tag names registered by Fiber's built-in middlewares. Treat them as the
// canonical identifiers for the values produced by requestid, basicauth,
// keyauth, csrf, and session — keeping format strings derived from these
// constants means renaming a tag here cascades automatically.
const (
	TagRequestID       = "requestid"
	TagRequestIDDashed = "request-id"
	TagUsername        = "username"
	TagAPIKey          = "api-key"
	TagCSRFToken       = "csrf-token"
	TagSessionID       = "session-id"
)

const (
	// DefaultFormat disables contextual fields for the default logger.
	DefaultFormat = ""
	// RequestIDFormat renders the request ID registered by the requestid middleware.
	RequestIDFormat = "[${" + TagRequestID + "}] "
	// KeyValueFormat renders commonly registered middleware context values as
	// key/value fields. Sensitive values (api-key, csrf-token, session-id) are
	// redacted by the registering middleware before reaching the log line.
	KeyValueFormat = "request-id=${" + TagRequestIDDashed + "} " +
		"username=${" + TagUsername + "} " +
		"api-key=${" + TagAPIKey + "} " +
		"csrf-token=${" + TagCSRFToken + "} " +
		"session-id=${" + TagSessionID + "} "
)

// Buffer abstracts the buffer operations used when rendering contextual log fields.
type Buffer = logtemplate.Buffer

// ContextData is reserved for data shared by contextual log tags. It currently
// has no fields; the type exists so the ContextTagFunc signature can evolve
// without breaking custom-tag implementations.
type ContextData struct{}

// ContextTagFunc renders one contextual log tag.
type ContextTagFunc = logtemplate.Func[any, ContextData]

// ContextConfig defines how WithContext enriches logs emitted by Fiber's default logger.
type ContextConfig struct {
	// CustomTags defines additional contextual tags available to Format.
	// The built-in TagContextValue ("value:") tag cannot be overridden.
	CustomTags map[string]ContextTagFunc
	// Format defines the contextual prefix rendered before the log message.
	// Use CustomTags to expose package-specific values such as request IDs.
	Format string
}

// contextTemplate holds the precompiled format. Loads on the log hot path
// happen lock-free; rebuilds (write side) hold contextMu.
var contextTemplate atomic.Pointer[logtemplate.Template[any, ContextData]]

var (
	// contextMu guards rebuilds of contextFormat / contextTags. Readers of
	// the compiled template (writeContext) use contextTemplate.Load directly.
	contextMu     sync.RWMutex
	contextFormat = DefaultFormat
	contextTags   = defaultContextTagMap()
)

var (
	// ErrContextTagInvalid is returned by RegisterContextTag and SetContextTemplate
	// when the supplied tag name or renderer is empty.
	ErrContextTagInvalid = errors.New("log: context tag name and function are required")
	// ErrContextTagReserved is returned by RegisterContextTag and SetContextTemplate
	// when the caller attempts to override the reserved TagContextValue ("value:") tag.
	ErrContextTagReserved = errors.New("log: context tag is reserved")
)

// SetContextTemplate configures contextual fields rendered by WithContext for Fiber's default logger.
// Pass an empty ContextConfig (or ContextConfig{Format: DefaultFormat}) to disable contextual fields.
// It returns an error if config.Format cannot be parsed or if config.CustomTags attempts to
// override the reserved TagContextValue tag.
func SetContextTemplate(config ContextConfig) error {
	if _, ok := config.CustomTags[TagContextValue]; ok {
		return ErrContextTagReserved
	}

	contextMu.Lock()
	defer contextMu.Unlock()

	// Cloning the live tag map preserves prior RegisterContextTag entries —
	// callers that interleave RegisterContextTag with SetContextTemplate
	// expect the registration to remain visible. CustomTags layer on top.
	tags := maps.Clone(contextTags)
	maps.Copy(tags, config.CustomTags)
	tags[TagContextValue] = defaultContextValueTag

	var tmpl *logtemplate.Template[any, ContextData]
	if config.Format != "" {
		var err error
		tmpl, err = logtemplate.Build[any, ContextData](config.Format, tags)
		if err != nil {
			return err
		}
	}

	contextFormat = config.Format
	contextTags = tags
	contextTemplate.Store(tmpl)
	return nil
}

// MustSetContextTemplate configures contextual fields and panics if the format cannot be parsed.
func MustSetContextTemplate(config ContextConfig) {
	if err := SetContextTemplate(config); err != nil {
		panic(err)
	}
}

// RegisterContextTag registers a contextual tag that can be used by SetContextTemplate.
// Re-registering a tag replaces the existing tag function. Registration is package-global;
// prefer ContextConfig.CustomTags for per-application overrides. The reserved TagContextValue
// tag cannot be registered.
func RegisterContextTag(tag string, fn ContextTagFunc) error {
	if tag == "" || fn == nil {
		return ErrContextTagInvalid
	}
	if tag == TagContextValue {
		return ErrContextTagReserved
	}

	contextMu.Lock()
	defer contextMu.Unlock()

	tags := maps.Clone(contextTags)
	tags[tag] = fn

	var tmpl *logtemplate.Template[any, ContextData]
	if contextFormat != "" {
		var err error
		tmpl, err = logtemplate.Build[any, ContextData](contextFormat, tags)
		if err != nil {
			return err
		}
	}

	contextTags = tags
	contextTemplate.Store(tmpl)
	return nil
}

// MustRegisterContextTag registers a contextual tag and panics if registration fails.
func MustRegisterContextTag(tag string, fn ContextTagFunc) {
	if err := RegisterContextTag(tag, fn); err != nil {
		panic(err)
	}
}

// defaultContextTagMap pre-seeds renderers for the tag names used by Fiber's
// built-in middleware (basicauth, csrf, keyauth, requestid, session). The
// stubs render empty strings so a format that references e.g. ${requestid}
// compiles even when the corresponding middleware has not been initialized
// yet — the slot is filled in once the middleware's New() runs.
func defaultContextTagMap() map[string]ContextTagFunc {
	return map[string]ContextTagFunc{
		TagAPIKey:          emptyContextTag,
		TagCSRFToken:       emptyContextTag,
		TagRequestIDDashed: emptyContextTag,
		TagRequestID:       emptyContextTag,
		TagSessionID:       emptyContextTag,
		TagUsername:        emptyContextTag,
		TagContextValue:    defaultContextValueTag,
	}
}

func defaultContextValueTag(output Buffer, ctx any, _ *ContextData, extraParam string) (int, error) {
	switch v := contextValue(ctx, extraParam).(type) {
	case []byte:
		return writeSanitized(output, v)
	case string:
		return writeSanitizedString(output, v)
	case nil:
		return 0, nil
	default:
		// fmt.Fprintf can produce arbitrary text (e.g. %v on a struct). Buffer
		// the formatted output through a small intermediate so the same
		// sanitization applies.
		formatted := fmt.Sprintf("%v", v)
		n, err := writeSanitizedString(output, formatted)
		if err != nil {
			return n, fmt.Errorf("write context value: %w", err)
		}
		return n, nil
	}
}

// writeSanitized writes p to output with ASCII control bytes replaced by
// spaces. Tabs are preserved. The replacement is done in-place on a single
// pass so the hot path stays alloc-free for inputs that are already clean
// (the common case): clean inputs forward directly to output.Write.
func writeSanitized(output Buffer, p []byte) (int, error) {
	if !needsControlSanitize(p) {
		return output.Write(p)
	}
	scrubbed := make([]byte, len(p))
	for i, b := range p {
		if isControlByte(b) {
			scrubbed[i] = ' '
		} else {
			scrubbed[i] = b
		}
	}
	return output.Write(scrubbed)
}

func writeSanitizedString(output Buffer, s string) (int, error) {
	if !needsControlSanitizeString(s) {
		return output.WriteString(s)
	}
	scrubbed := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if isControlByte(b) {
			scrubbed[i] = ' '
		} else {
			scrubbed[i] = b
		}
	}
	return output.Write(scrubbed)
}

func needsControlSanitize(p []byte) bool {
	return slices.ContainsFunc(p, isControlByte)
}

func needsControlSanitizeString(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return r < 0x80 && isControlByte(byte(r))
	}) >= 0
}

// isControlByte reports whether b is an ASCII control byte that must not pass
// through to a log line. Tab is preserved because operators frequently use it
// for delimiting structured fields. CR, LF, NUL, and the other C0/DEL bytes
// are replaced — they are the bytes attackers use to forge log lines or
// corrupt terminal output via ANSI escape sequences.
func isControlByte(b byte) bool {
	if b == '\t' {
		return false
	}
	return b < 0x20 || b == 0x7f
}

func emptyContextTag(_ Buffer, _ any, _ *ContextData, _ string) (int, error) {
	return 0, nil
}

type valueContext interface {
	Value(key any) any
}

type userValueContext interface {
	UserValue(key any) any
}

func contextValue(ctx, key any) any {
	switch typed := ctx.(type) {
	case nil:
		return nil
	case userValueContext:
		return typed.UserValue(key)
	case valueContext:
		return typed.Value(key)
	default:
		return nil
	}
}
