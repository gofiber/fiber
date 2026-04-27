package log

import (
	"fmt"
	"maps"
	"sync/atomic"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
)

// TagContextValue reads a value from the bound context-like value using the tag parameter as the key.
const TagContextValue = "value:"

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

// SetContextTemplate configures contextual fields rendered by WithContext for Fiber's default logger.
// It returns an error if config.Format cannot be parsed.
func SetContextTemplate(config ContextConfig) error {
	if config.Format == "" {
		contextTemplate.Store(nil)
		return nil
	}

	tmpl, err := logtemplate.Build[any, ContextData](config.Format, createContextTagMap(config.CustomTags))
	if err != nil {
		return err
	}

	contextTemplate.Store(tmpl)
	return nil
}

// MustSetContextTemplate configures contextual fields and panics if the format cannot be parsed.
func MustSetContextTemplate(config ContextConfig) {
	if err := SetContextTemplate(config); err != nil {
		panic(err)
	}
}

func createContextTagMap(customTags map[string]ContextTagFunc) map[string]ContextTagFunc {
	tags := map[string]ContextTagFunc{
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

	maps.Copy(tags, customTags)

	return tags
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
