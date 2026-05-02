package log

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
)

var errContextTestWrite = errors.New("context test write failure")

type failingContextBuffer struct{}

func (failingContextBuffer) Write(_ []byte) (int, error)     { return 0, errContextTestWrite }
func (failingContextBuffer) WriteByte(byte) error            { return errContextTestWrite }
func (failingContextBuffer) WriteString(string) (int, error) { return 0, errContextTestWrite }

// buildTestTemplate compiles a context template using the package-default tag
// map merged with the supplied custom tags. It mirrors the production merge
// performed by SetContextTemplate so test expectations stay aligned.
func buildTestTemplate(t *testing.T, format string, custom map[string]ContextTagFunc) *logtemplate.Template[any, ContextData] {
	t.Helper()
	tags := defaultContextTagMap()
	for k, v := range custom {
		tags[k] = v
	}
	tags[TagContextValue] = defaultContextValueTag
	tmpl, err := logtemplate.Build[any, ContextData](format, tags)
	require.NoError(t, err)
	return tmpl
}

func Test_ContextTemplate_ValueTag(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), "request_id", "req-42") //nolint:revive,staticcheck // ${value:key} intentionally reads string context keys.
	tmpl := buildTestTemplate(t, "[${value:request_id}]", nil)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, ctx, &ContextData{}))
	require.Equal(t, "[req-42]", buf.String())
}

type testUserValueContext map[any]any

func (c testUserValueContext) UserValue(key any) any {
	return c[key]
}

func Test_ContextTemplate_ValueTagFromUserValueContext(t *testing.T) {
	t.Parallel()

	tmpl := buildTestTemplate(t, "[${value:request_id}]", nil)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, testUserValueContext{"request_id": "req-42"}, &ContextData{}))
	require.Equal(t, "[req-42]", buf.String())
}

func Test_ContextTemplate_ValueTagSanitizesControlChars(t *testing.T) {
	t.Parallel()

	tmpl := buildTestTemplate(t, "${value:str}|${value:bytes}|${value:struct}", nil)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	type composite struct{ A, B string }
	ctx := testUserValueContext{
		"str":    "good\r\nbad",
		"bytes":  []byte{'a', 0x00, 'b', 0x1B, 'c', '\t', 'd'},
		"struct": composite{A: "x\ny", B: "z"},
	}
	require.NoError(t, tmpl.Execute(buf, ctx, &ContextData{}))

	out := buf.String()
	require.NotContains(t, out, "\r")
	require.NotContains(t, out, "\n")
	require.NotContains(t, out, "\x00")
	require.NotContains(t, out, "\x1b")
	require.Contains(t, out, "\t", "tabs must pass through to preserve operator-friendly delimiters")
}

func Test_ContextTemplate_ValueTagWritesSupportedValues(t *testing.T) {
	t.Parallel()

	tmpl := buildTestTemplate(t, "${value:bytes}|${value:string}|${value:number}|${value:missing}", nil)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	ctx := testUserValueContext{
		"bytes":  []byte("raw"),
		"string": "text",
		"number": 42,
	}
	require.NoError(t, tmpl.Execute(buf, ctx, &ContextData{}))
	require.Equal(t, "raw|text|42|", buf.String())
}

func Test_ContextTemplate_ValueTagWrapsWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  any
		key  string
	}{
		{name: "default fmt path (int)", ctx: testUserValueContext{"k": 42}, key: "k"},
		{name: "string path", ctx: testUserValueContext{"k": "value"}, key: "k"},
		{name: "byte slice path", ctx: testUserValueContext{"k": []byte("value")}, key: "k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := defaultContextValueTag(failingContextBuffer{}, tt.ctx, &ContextData{}, tt.key)
			require.ErrorIs(t, err, errContextTestWrite)
		})
	}
}

func Test_ContextTemplate_DefaultTagsRenderEmpty(t *testing.T) {
	t.Parallel()

	// Build the format from the same names that defaultContextTagMap stubs so a
	// new default tag automatically extends coverage instead of silently
	// drifting from the assertion.
	defaults := defaultContextTagMap()
	delete(defaults, TagContextValue)

	parts := make([]string, 0, len(defaults))
	for name := range defaults {
		parts = append(parts, "${"+name+"}|")
	}
	format := strings.Join(parts, "")

	tmpl, err := logtemplate.Build[any, ContextData](format, defaults)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, context.Background(), &ContextData{}))
	// Each stub renders empty, so the only bytes left are the "|" separators.
	require.Len(t, buf.String(), len(defaults))
}

func Test_ContextTemplate_CustomTag(t *testing.T) {
	t.Parallel()

	tmpl := buildTestTemplate(t, "[${requestid}]", map[string]ContextTagFunc{
		"requestid": func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
			return output.WriteString("req-42")
		},
	})

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, context.Background(), &ContextData{}))
	require.Equal(t, "[req-42]", buf.String())
}

// Test_SetContextTemplateRejectsReservedValueTag locks in M9 — overriding the
// reserved TagContextValue via CustomTags must surface as an error, matching
// RegisterContextTag's behavior.
func Test_SetContextTemplateRejectsReservedValueTag(t *testing.T) {
	t.Cleanup(func() { MustSetContextTemplate(ContextConfig{}) })

	err := SetContextTemplate(ContextConfig{
		Format: "[${value:request_id}]",
		CustomTags: map[string]ContextTagFunc{
			TagContextValue: func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
				return output.WriteString("override")
			},
		},
	})
	require.ErrorIs(t, err, ErrContextTagReserved)
}

// Test_SetContextTemplate_RegistersCustomTag runs serially because it mutates
// the package-global context template registry.
func Test_SetContextTemplate_RegistersCustomTag(t *testing.T) {
	t.Cleanup(func() { MustSetContextTemplate(ContextConfig{}) })

	require.NoError(t, SetContextTemplate(ContextConfig{
		Format: "[${tenant}]",
		CustomTags: map[string]ContextTagFunc{
			"tenant": func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
				return output.WriteString("acme")
			},
		},
	}))

	tmpl := contextTemplate.Load()
	require.NotNil(t, tmpl)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, context.Background(), &ContextData{}))
	require.Equal(t, "[acme]", buf.String())
}

// Test_RegisterContextTagThenSetFormat runs serially because it mutates the
// package-global context tag registry.
func Test_RegisterContextTagThenSetFormat(t *testing.T) {
	t.Cleanup(func() { MustSetContextTemplate(ContextConfig{}) })

	require.NoError(t, RegisterContextTag("tenant", func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
		return output.WriteString("acme")
	}))
	// With no format active, the template stays nil even after registration.
	require.Nil(t, contextTemplate.Load())

	require.NoError(t, SetContextTemplate(ContextConfig{Format: "[${tenant}]"}))
	tmpl := contextTemplate.Load()
	require.NotNil(t, tmpl)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, context.Background(), &ContextData{}))
	require.Equal(t, "[acme]", buf.String())
}

// Test_DefaultFormatDisablesContextTemplate runs serially because it mutates
// the package-global context template registry.
func Test_DefaultFormatDisablesContextTemplate(t *testing.T) {
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

	require.NoError(t, SetContextTemplate(ContextConfig{Format: DefaultFormat}))
	require.Nil(t, contextTemplate.Load())
}

func Test_MustSetContextTemplate_PanicsOnBuildError(t *testing.T) {
	t.Parallel()

	require.PanicsWithError(t, `logtemplate: unknown tag: "missing:value"`, func() {
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
	require.ErrorIs(t, err, logtemplate.ErrUnknownTag)

	var typed *logtemplate.UnknownTagError
	require.ErrorAs(t, err, &typed)
	require.Equal(t, "missing:value", typed.Tag)
	require.Equal(t, "value", typed.Param)
}

func Test_RegisterContextTagRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, RegisterContextTag("", func(Buffer, any, *ContextData, string) (int, error) {
		return 0, nil
	}), ErrContextTagInvalid)
	require.ErrorIs(t, RegisterContextTag("missing", nil), ErrContextTagInvalid)
	require.ErrorIs(t, RegisterContextTag(TagContextValue, func(Buffer, any, *ContextData, string) (int, error) {
		return 0, nil
	}), ErrContextTagReserved)
}

func Test_MustRegisterContextTag_PanicsOnInvalidInput(t *testing.T) {
	t.Parallel()

	require.PanicsWithError(t, ErrContextTagInvalid.Error(), func() {
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

// Test_ContextTemplate_ConcurrentRegistration exercises the lock split between
// the atomic-pointer read path and the mutex-guarded rebuild path. It must be
// run with -race to be meaningful.
func Test_ContextTemplate_ConcurrentRegistration(t *testing.T) {
	t.Cleanup(func() { MustSetContextTemplate(ContextConfig{}) })

	require.NoError(t, SetContextTemplate(ContextConfig{Format: "[${tenant}]"}))

	const goroutines = 8
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	register := func(id int) {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			//nolint:errcheck // race-stress test intentionally ignores transient errors
			_ = RegisterContextTag("tenant", func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
				return output.WriteString("acme")
			})
			_ = id
		}
	}
	reformat := func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			//nolint:errcheck // race-stress test intentionally ignores transient errors
			_ = SetContextTemplate(ContextConfig{Format: "[${tenant}]"})
		}
	}
	emit := func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tmpl := contextTemplate.Load()
			if tmpl == nil {
				continue
			}
			buf := bytebufferpool.Get()
			//nolint:errcheck // race-stress test intentionally ignores transient errors
			_ = tmpl.Execute(buf, context.Background(), &ContextData{})
			bytebufferpool.Put(buf)
		}
	}

	for i := 0; i < goroutines; i++ {
		go register(i)
		go reformat()
		go emit()
	}

	wg.Wait()
}

func Benchmark_ContextTemplate_Execute(b *testing.B) {
	tmpl, err := logtemplate.Build[any, ContextData]("[${requestid}] ", map[string]ContextTagFunc{
		"requestid": func(output Buffer, _ any, _ *ContextData, _ string) (int, error) {
			return output.WriteString("req-42")
		},
	})
	require.NoError(b, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		if err := tmpl.Execute(buf, context.Background(), &ContextData{}); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_DefaultContextValueTag(b *testing.B) {
	ctx := testUserValueContext{"request_id": "req-42"}

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		if _, err := defaultContextValueTag(buf, ctx, &ContextData{}, "request_id"); err != nil {
			b.Fatal(err)
		}
	}
}
