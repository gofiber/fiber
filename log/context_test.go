package log

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v3/internal/logtemplate"
	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
)

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
