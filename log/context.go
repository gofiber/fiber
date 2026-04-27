package log

import (
	"context"
	"fmt"
	"maps"
	"sync/atomic"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
)

// TagContextValue reads a value from context.Context using the tag parameter as the key.
const TagContextValue = "value:"

// Buffer abstracts the buffer operations used when rendering contextual log fields.
type Buffer = logtemplate.Buffer

// ContextData is reserved for data shared by contextual log tags.
type ContextData struct{}

// ContextTagFunc renders one contextual log tag.
type ContextTagFunc = logtemplate.Func[context.Context, ContextData]

// ContextConfig defines how WithContext enriches logs emitted by Fiber's default logger.
type ContextConfig struct {
	// CustomTags defines additional contextual tags available to Format.
	CustomTags map[string]ContextTagFunc
	// Format defines the contextual prefix rendered before the log message.
	// Use CustomTags to expose package-specific values such as request IDs.
	Format string
}

var contextTemplate atomic.Pointer[logtemplate.Template[context.Context, ContextData]]

// SetContextTemplate configures contextual fields rendered by WithContext for Fiber's default logger.
func SetContextTemplate(config ContextConfig) {
	if config.Format == "" {
		contextTemplate.Store(nil)
		return
	}

	tmpl, err := logtemplate.Build[context.Context, ContextData](config.Format, createContextTagMap(config.CustomTags))
	if err != nil {
		panic(err)
	}

	contextTemplate.Store(tmpl)
}

func createContextTagMap(customTags map[string]ContextTagFunc) map[string]ContextTagFunc {
	tags := map[string]ContextTagFunc{
		TagContextValue: func(output Buffer, ctx context.Context, _ *ContextData, extraParam string) (int, error) {
			if ctx == nil {
				return 0, nil
			}

			switch v := ctx.Value(extraParam).(type) {
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
