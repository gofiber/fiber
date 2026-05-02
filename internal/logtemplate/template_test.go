package logtemplate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/bytebufferpool"
)

type testData struct {
	value string
}

func Test_Template_Execute(t *testing.T) {
	t.Parallel()

	tmpl, err := Build[string, testData]("a ${tag} ${param:name} z", map[string]Func[string, testData]{
		"tag": func(output Buffer, ctx string, data *testData, _ string) (int, error) {
			return output.WriteString(ctx + data.value)
		},
		"param:": func(output Buffer, _ string, _ *testData, extraParam string) (int, error) {
			return output.WriteString(extraParam)
		},
	})
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	err = tmpl.Execute(buf, "ctx-", &testData{value: "data"})
	require.NoError(t, err)
	require.Equal(t, "a ctx-data name z", buf.String())
}

func Test_Template_NilTemplateExecuteIsNoop(t *testing.T) {
	t.Parallel()

	var tmpl *Template[string, testData]

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, "ctx", &testData{}))
	require.Equal(t, 0, buf.Len())

	parts, funcs := tmpl.Chains()
	require.Nil(t, parts)
	require.Nil(t, funcs)
}

func Test_Template_UnterminatedTagPreserved(t *testing.T) {
	t.Parallel()

	// An unterminated ${ runs to end-of-format. Build emits the literal "${"
	// from the unterminated branch and preserves the trailing text as the
	// final fixed part, so Execute round-trips the original input verbatim.
	tmpl, err := Build[string, testData]("prefix ${unterminated", nil)
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.NoError(t, tmpl.Execute(buf, "ctx", &testData{}))
	require.Equal(t, "prefix ${unterminated", buf.String())
}

func Test_Template_UnknownParametricTag(t *testing.T) {
	t.Parallel()

	_, err := Build[string, testData]("${missing:name}", nil)
	require.ErrorIs(t, err, ErrUnknownTag)

	var typed *UnknownTagError
	require.True(t, errors.As(err, &typed))
	require.Equal(t, "missing:name", typed.Tag)
	require.Equal(t, "name", typed.Param)
	require.EqualError(t, err, `logtemplate: unknown tag: "missing:name"`)
}

func Test_Template_UnknownBareTag(t *testing.T) {
	t.Parallel()

	_, err := Build[string, testData]("${missing}", nil)
	require.ErrorIs(t, err, ErrUnknownTag)

	var typed *UnknownTagError
	require.True(t, errors.As(err, &typed))
	require.Equal(t, "missing", typed.Tag)
	require.Empty(t, typed.Param)
}

func Test_Template_UnknownBareTagHintsParametric(t *testing.T) {
	t.Parallel()

	_, err := Build[string, testData]("${reqHeader}", map[string]Func[string, testData]{
		"reqHeader:": func(_ Buffer, _ string, _ *testData, _ string) (int, error) {
			return 0, nil
		},
	})
	require.ErrorIs(t, err, ErrUnknownTag)

	var typed *UnknownTagError
	require.True(t, errors.As(err, &typed))
	require.Equal(t, "reqHeader", typed.Tag)
	require.Equal(t, "did you mean ${reqHeader:PARAM}?", typed.Hint)
	require.Contains(t, err.Error(), "did you mean ${reqHeader:PARAM}?")
}

func Test_Template_FuncReturnsError(t *testing.T) {
	t.Parallel()

	tagErr := errors.New("tag failure")
	tmpl, err := Build[string, testData]("${fail}", map[string]Func[string, testData]{
		"fail": func(_ Buffer, _ string, _ *testData, _ string) (int, error) {
			return 0, tagErr
		},
	})
	require.NoError(t, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	require.ErrorIs(t, tmpl.Execute(buf, "ctx", &testData{}), tagErr)
}

func Benchmark_Template_Build(b *testing.B) {
	tagFns := map[string]Func[string, testData]{
		"tag": func(output Buffer, _ string, _ *testData, _ string) (int, error) {
			return output.WriteString("v")
		},
		"param:": func(output Buffer, _ string, _ *testData, extraParam string) (int, error) {
			return output.WriteString(extraParam)
		},
	}
	const format = "prefix ${tag} ${param:name} suffix ${tag}"

	b.ReportAllocs()
	for b.Loop() {
		_, err := Build[string, testData](format, tagFns)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Template_Execute(b *testing.B) {
	tmpl, err := Build[string, testData]("prefix ${tag} ${param:name} suffix ${tag}", map[string]Func[string, testData]{
		"tag": func(output Buffer, ctx string, _ *testData, _ string) (int, error) {
			return output.WriteString(ctx)
		},
		"param:": func(output Buffer, _ string, _ *testData, extraParam string) (int, error) {
			return output.WriteString(extraParam)
		},
	})
	require.NoError(b, err)

	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		if err := tmpl.Execute(buf, "ctx", &testData{}); err != nil {
			b.Fatal(err)
		}
	}
}
