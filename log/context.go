package log

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3/internal/contextvalue"
	"github.com/gofiber/fiber/v3/internal/logtemplate"
	"github.com/gofiber/utils/v2"
	"github.com/gofiber/utils/v2/swar"
	"github.com/valyala/bytebufferpool"
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
	// redacted by the registering middleware before reaching the log line. A
	// value holding a space, a tab, '=' or '"' is quoted, see SetContextTemplate.
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
var contextTemplate atomic.Pointer[compiledContext]

var (
	// contextMu guards rebuilds of contextFormat / contextTags. Readers of
	// the compiled template (writeContext) use contextTemplate.Load directly.
	contextMu     sync.RWMutex
	contextFormat = DefaultFormat
	contextTags   = defaultContextTagMap()
)

var (
	// ErrContextTagInvalid is returned by RegisterContextTag when the supplied tag
	// name is empty or the renderer is nil. SetContextTemplate does not validate
	// ContextConfig.CustomTags and never returns it.
	ErrContextTagInvalid = errors.New("log: context tag name and function are required")
	// ErrContextTagReserved is returned by RegisterContextTag and SetContextTemplate
	// when the caller attempts to override the reserved TagContextValue ("value:") tag.
	ErrContextTagReserved = errors.New("log: context tag is reserved")
)

// SetContextTemplate configures contextual fields rendered by WithContext for Fiber's default logger.
// Pass an empty ContextConfig (or ContextConfig{Format: DefaultFormat}) to disable contextual fields.
// It returns an error if config.Format cannot be parsed or if config.CustomTags attempts to
// override the reserved TagContextValue tag. A tag right behind a '=' is the value of a key=value
// field: when it holds a space, a tab, '=' or '"' it is written as a Go quoted string.
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

	var tmpl *compiledContext
	if config.Format != "" {
		var err error
		tmpl, err = buildContextTemplate(config.Format, tags)
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

	var tmpl *compiledContext
	if contextFormat != "" {
		var err error
		tmpl, err = buildContextTemplate(contextFormat, tags)
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

// compiledContext is a compiled context format. fields marks the tags right behind a '=': each writes the value of a
// key=value field, which is quoted when it would read as more than one field.
type compiledContext struct {
	tmpl   *logtemplate.Template[any, ContextData]
	fields []bool // nil when no tag is
}

func buildContextTemplate(format string, tags map[string]ContextTagFunc) (*compiledContext, error) {
	tmpl, err := logtemplate.Build[any, ContextData](format, tags)
	if err != nil {
		return nil, err
	}
	c := &compiledContext{tmpl: tmpl}
	fixedParts, funcChain := tmpl.Chains()
	for i := 1; i < len(funcChain); i++ {
		if funcChain[i] != nil && funcChain[i-1] == nil && bytes.HasSuffix(fixedParts[i-1], []byte("=")) {
			if c.fields == nil {
				c.fields = make([]bool, len(funcChain))
			}
			c.fields[i] = true
		}
	}
	return c, nil
}

// Execute renders the format into buf, as logtemplate.Template.Execute does.
func (c *compiledContext) Execute(buf *bytebufferpool.ByteBuffer, ctx any, data *ContextData) error {
	fixedParts, funcChain := c.tmpl.Chains()
	if c.fields == nil {
		return logtemplate.ExecuteChains(buf, ctx, data, fixedParts, funcChain)
	}
	for i, fn := range funcChain {
		if fn == nil {
			buf.B = append(buf.B, fixedParts[i]...)
			continue
		}
		start := len(buf.B)
		if _, err := fn(buf, ctx, data, utils.UnsafeString(fixedParts[i])); err != nil {
			return err
		}
		// A value with a space, a tab, '=' or '"', such as an X-Request-ID, would end its field and start another.
		if c.fields[i] && start < len(buf.B) && endsField(buf.B[start:]) {
			buf.B = append(buf.B[:start], strconv.Quote(string(buf.B[start:]))...)
		}
	}
	return nil
}

// fieldBreaks marks the bytes that end a key=value field or start a quoted one.
var fieldBreaks = [256]bool{' ': true, '\t': true, '=': true, '"': true}

// endsField reports whether value holds a byte of fieldBreaks, eight bytes at a time.
func endsField(value []byte) bool {
	i := 0
	for ; i+8 <= len(value); i += 8 {
		w := binary.LittleEndian.Uint64(value[i:])
		if swar.ZeroLanes(w^(' '*swar.Ones))|swar.ZeroLanes(w^('\t'*swar.Ones))|
			swar.ZeroLanes(w^('='*swar.Ones))|swar.ZeroLanes(w^('"'*swar.Ones)) != 0 {
			return true
		}
	}
	for _, b := range value[i:] {
		if fieldBreaks[b] {
			return true
		}
	}
	return false
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
// spaces. Tabs are preserved. See logtemplate.WriteSanitized.
func writeSanitized(output Buffer, p []byte) (int, error) {
	return logtemplate.WriteSanitized(output, p)
}

// writeSanitizedString is writeSanitized for strings.
func writeSanitizedString(output Buffer, s string) (int, error) {
	return logtemplate.WriteSanitizedString(output, s)
}

func emptyContextTag(_ Buffer, _ any, _ *ContextData, _ string) (int, error) {
	return 0, nil
}

func contextValue(ctx, key any) any {
	value, ok := contextvalue.Value[any](ctx, key)
	if !ok {
		return nil
	}
	return value
}
