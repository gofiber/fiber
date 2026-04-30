package log

import (
	"errors"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
)

// TagContextValue reads a value from the bound context-like value using the tag parameter as the key.
const TagContextValue = "value:"

const (
	// DefaultFormat disables contextual fields for the default logger.
	DefaultFormat = ""
	// RequestIDFormat renders the request ID registered by the requestid middleware.
	RequestIDFormat = "[${requestid}] "
	// KeyValueFormat renders commonly registered middleware context values as key/value fields.
	KeyValueFormat = "request-id=${requestid} username=${username} api-key=${api-key} csrf-token=${csrf-token} session-id=${session-id} "
)

// Buffer abstracts the buffer operations used when rendering contextual log fields.
type Buffer = logtemplate.Buffer

// ContextData is reserved for data shared by contextual log tags.
type ContextData struct{}

// ContextTagFunc renders one contextual log tag.
type ContextTagFunc = logtemplate.Func[any, ContextData]

// ContextConfig defines how WithContext enriches logs emitted by Fiber's default logger.
type ContextConfig struct {
	// CustomTags defines additional contextual tags available to Format.
	CustomTags map[string]ContextTagFunc
	// Format defines the contextual prefix rendered before the log message.
	// Use CustomTags to expose package-specific values such as request IDs.
	Format string
}

var contextTemplate atomic.Pointer[logtemplate.Template[any, ContextData]]

var (
	contextMu     sync.RWMutex
	contextFormat = DefaultFormat
	contextTags   = defaultContextTagMap()
)

var errContextTagInvalid = errors.New("log: context tag name and function are required")

// SetContextTemplate configures contextual fields rendered by WithContext for Fiber's default logger.
// It returns an error if config.Format cannot be parsed.
func SetContextTemplate(config ContextConfig) error {
	contextMu.Lock()
	defer contextMu.Unlock()

	tags := maps.Clone(contextTags)
	maps.Copy(tags, config.CustomTags)

	var tmpl *logtemplate.Template[any, ContextData]
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

// Format configures the contextual fields rendered by WithContext for Fiber's default logger.
// Pass DefaultFormat to disable contextual fields.
func Format(format string) error {
	contextMu.Lock()
	defer contextMu.Unlock()

	var tmpl *logtemplate.Template[any, ContextData]
	if format != "" {
		var err error
		tmpl, err = buildContextTemplate(format, contextTags)
		if err != nil {
			return err
		}
	}

	contextFormat = format
	contextTemplate.Store(tmpl)
	return nil
}

// MustFormat configures contextual fields and panics if the format cannot be parsed.
func MustFormat(format string) {
	if err := Format(format); err != nil {
		panic(err)
	}
}

// RegisterContextTag registers a contextual tag that can be used by Format.
// Re-registering a tag replaces the existing tag function.
func RegisterContextTag(tag string, fn ContextTagFunc) error {
	if tag == "" || fn == nil {
		return errContextTagInvalid
	}

	contextMu.Lock()
	defer contextMu.Unlock()

	tags := maps.Clone(contextTags)
	tags[tag] = fn

	var tmpl *logtemplate.Template[any, ContextData]
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

func createContextTagMap(customTags map[string]ContextTagFunc) map[string]ContextTagFunc {
	tags := defaultContextTagMap()
	maps.Copy(tags, customTags)

	return tags
}

func defaultContextTagMap() map[string]ContextTagFunc {
	return map[string]ContextTagFunc{
		"api-key":    emptyContextTag,
		"csrf-token": emptyContextTag,
		"request-id": emptyContextTag,
		"requestid":  emptyContextTag,
		"session-id": emptyContextTag,
		"username":   emptyContextTag,
		TagContextValue: func(output Buffer, ctx any, _ *ContextData, extraParam string) (int, error) {
			switch v := contextValue(ctx, extraParam).(type) {
			case []byte:
				return output.Write(v)
			case string:
				return output.WriteString(v)
			case nil:
				return 0, nil
			default:
				return fmt.Fprintf(output, "%v", v)
			}
		},
	}
}

func emptyContextTag(_ Buffer, _ any, _ *ContextData, _ string) (int, error) {
	return 0, nil
}

func buildContextTemplate(format string, tags map[string]ContextTagFunc) (*logtemplate.Template[any, ContextData], error) {
	return logtemplate.Build[any, ContextData](format, tags)
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
