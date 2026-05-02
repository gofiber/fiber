package log

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
)

var errContextTestWrite = errors.New("context test write failure")

type failingContextBuffer struct{}

func (failingContextBuffer) Len() int { return 0 }
func (failingContextBuffer) ReadFrom(_ io.Reader) (int64, error) {
	return 0, errContextTestWrite
}

func (failingContextBuffer) WriteTo(_ io.Writer) (int64, error) {
	return 0, errContextTestWrite
}
func (failingContextBuffer) Bytes() []byte                   { return nil }
func (failingContextBuffer) Write(_ []byte) (int, error)     { return 0, errContextTestWrite }
func (failingContextBuffer) WriteByte(byte) error            { return errContextTestWrite }
func (failingContextBuffer) WriteString(string) (int, error) { return 0, errContextTestWrite }
func (failingContextBuffer) Set([]byte)                      {}
func (failingContextBuffer) SetString(string)                {}
func (failingContextBuffer) String() string                  { return "" }

func Test_ContextTemplate_ValueTag(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), "request_id", "req-42") //nolint:revive,staticcheck // ${value:key} intentionally reads string context keys.
	tmpl, err := logtemplate.Build[any, ContextData](
		"[${value:request_id}]",
		createContextTagMap(nil),
	)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	err = tmpl.Execute(buf, ctx, &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "[req-42]", buf.String())
}

type testUserValueContext map[any]any

func (c testUserValueContext) UserValue(key any) any {
	return c[key]
}

func Test_ContextTemplate_ValueTagFromUserValueContext(t *testing.T) {
	t.Parallel()

	tmpl, err := logtemplate.Build[any, ContextData](
		"[${value:request_id}]",
		createContextTagMap(nil),
	)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	err = tmpl.Execute(buf, testUserValueContext{"request_id": "req-42"}, &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "[req-42]", buf.String())
}

func Test_ContextTemplate_ValueTagWritesSupportedValues(t *testing.T) {
	t.Parallel()

	tmpl, err := logtemplate.Build[any, ContextData](
		"${value:bytes}|${value:string}|${value:number}|${value:missing}",
		createContextTagMap(nil),
	)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	ctx := testUserValueContext{
		"bytes":  []byte("raw"),
		"string": "text",
		"number": 42,
	}
	err = tmpl.Execute(buf, ctx, &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "raw|text|42|", buf.String())
}

func Test_ContextTemplate_ValueTagWrapsWriteError(t *testing.T) {
	t.Parallel()

	_, err := defaultContextValueTag(failingContextBuffer{}, testUserValueContext{
		"number": 42,
	}, &ContextData{}, "number")
	require.ErrorIs(t, err, errContextTestWrite)
	require.ErrorContains(t, err, "write context value")
}

func Test_ContextTemplate_DefaultTagsRenderEmpty(t *testing.T) {
	t.Parallel()

	tmpl, err := logtemplate.Build[any, ContextData](
		"${api-key}|${csrf-token}|${request-id}|${requestid}|${session-id}|${username}",
		createContextTagMap(nil),
	)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	err = tmpl.Execute(buf, context.Background(), &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "|||||", buf.String())
}

func Test_ContextTemplate_CustomTag(t *testing.T) {
	t.Parallel()

	tmpl, err := logtemplate.Build[any, ContextData](
		"[${requestid}]",
		createContextTagMap(map[string]ContextTagFunc{
			"requestid": func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
				return output.WriteString("req-42")
			},
		}),
	)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	err = tmpl.Execute(buf, context.Background(), &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "[req-42]", buf.String())
}

func Test_ContextTemplate_CustomTagsCannotOverrideValueTag(t *testing.T) {
	t.Parallel()

	tmpl, err := logtemplate.Build[any, ContextData](
		"[${value:request_id}]",
		createContextTagMap(map[string]ContextTagFunc{
			TagContextValue: func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
				return output.WriteString("override")
			},
		}),
	)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	ctx := context.WithValue(context.Background(), "request_id", "req-42") //nolint:revive,staticcheck // ${value:key} intentionally reads string context keys.
	err = tmpl.Execute(buf, ctx, &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "[req-42]", buf.String())
}

// Test_SetContextTemplateCannotOverrideValueTag runs serially because it
// mutates the package-global context template registry.
func Test_SetContextTemplateCannotOverrideValueTag(t *testing.T) {
	t.Cleanup(func() { MustSetContextTemplate(ContextConfig{}) })

	require.NoError(t, SetContextTemplate(ContextConfig{
		Format: "[${value:request_id}]",
		CustomTags: map[string]ContextTagFunc{
			TagContextValue: func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
				return output.WriteString("override")
			},
		},
	}))

	tmpl := contextTemplate.Load()
	require.NotNil(t, tmpl)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	ctx := context.WithValue(context.Background(), "request_id", "req-42") //nolint:revive,staticcheck // ${value:key} intentionally reads string context keys.
	err := tmpl.Execute(buf, ctx, &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "[req-42]", buf.String())
}

// Test_FormatWithRegisteredContextTag runs serially because Format and
// MustRegisterContextTag mutate the package-global context template registry.
func Test_FormatWithRegisteredContextTag(t *testing.T) {
	t.Cleanup(func() { MustFormat(DefaultFormat) })

	MustRegisterContextTag("traceid", func(output Buffer, ctx any, _ *ContextData, _ string) (int, error) {
		traceID, ok := contextValue(ctx, "trace_id").(string)
		if !ok {
			return 0, nil
		}
		return output.WriteString(traceID)
	})
	require.NoError(t, Format("[${traceid}] "))

	tmpl := contextTemplate.Load()
	require.NotNil(t, tmpl)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	ctx := context.WithValue(context.Background(), "trace_id", "trace-42") //nolint:revive,staticcheck // Context-value string keys are part of the public template contract.
	err := tmpl.Execute(buf, ctx, &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "[trace-42] ", buf.String())
}

// Test_RegisterContextTagWithoutActiveFormat runs serially because it mutates
// the package-global context tag registry.
func Test_RegisterContextTagWithoutActiveFormat(t *testing.T) {
	t.Cleanup(func() { MustSetContextTemplate(ContextConfig{}) })

	require.NoError(t, Format(DefaultFormat))
	require.NoError(t, RegisterContextTag("tenant", func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
		return output.WriteString("acme")
	}))
	require.Nil(t, contextTemplate.Load())

	require.NoError(t, Format("[${tenant}]"))
	tmpl := contextTemplate.Load()
	require.NotNil(t, tmpl)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	err := tmpl.Execute(buf, context.Background(), &ContextData{})
	require.NoError(t, err)
	require.Equal(t, "[acme]", buf.String())
}

// Test_FormatDefaultDisablesContextTemplate runs serially because Format
// mutates the package-global context template registry.
func Test_FormatDefaultDisablesContextTemplate(t *testing.T) {
	t.Cleanup(func() { MustSetContextTemplate(ContextConfig{}) })

	MustSetContextTemplate(ContextConfig{
		Format: "[${requestid}] ",
		CustomTags: map[string]ContextTagFunc{
			"requestid": func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
				return output.WriteString("req-42")
			},
		},
	})
	require.NotNil(t, contextTemplate.Load())

	require.NoError(t, Format(DefaultFormat))
	require.Nil(t, contextTemplate.Load())
}

func Test_MustSetContextTemplate_PanicsOnBuildError(t *testing.T) {
	t.Parallel()

	require.PanicsWithError(t, `logtemplate: template parameter missing: "missing:value"`, func() {
		MustSetContextTemplate(ContextConfig{
			Format: "${missing:value}",
		})
	})
}

func Test_SetContextTemplate_ReturnsBuildError(t *testing.T) {
	t.Parallel()

	err := SetContextTemplate(ContextConfig{
		Format: "${missing:value}",
	})
	require.ErrorIs(t, err, logtemplate.ErrParameterMissing)
}

func Test_Format_ReturnsBuildError(t *testing.T) {
	t.Parallel()

	err := Format("${missing:value}")
	require.ErrorIs(t, err, logtemplate.ErrParameterMissing)
}

func Test_MustFormat_PanicsOnBuildError(t *testing.T) {
	t.Parallel()

	require.PanicsWithError(t, `logtemplate: template parameter missing: "missing:value"`, func() {
		MustFormat("${missing:value}")
	})
}

func Test_RegisterContextTagRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, RegisterContextTag("", func(Buffer, any, *ContextData, string) (int, error) {
		return 0, nil
	}), errContextTagInvalid)
	require.ErrorIs(t, RegisterContextTag("missing", nil), errContextTagInvalid)
	require.ErrorIs(t, RegisterContextTag(TagContextValue, func(Buffer, any, *ContextData, string) (int, error) {
		return 0, nil
	}), errContextTagReserved)
}

func Test_MustRegisterContextTag_PanicsOnInvalidInput(t *testing.T) {
	t.Parallel()

	require.PanicsWithError(t, errContextTagInvalid.Error(), func() {
		MustRegisterContextTag("", func(Buffer, any, *ContextData, string) (int, error) {
			return 0, nil
		})
	})
}

func Test_ContextValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ctx  any
		want any
		name string
	}{
		{
			name: "nil",
		},
		{
			name: "unsupported",
			ctx:  "ctx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, contextValue(tt.ctx, "key"))
		})
	}
}
