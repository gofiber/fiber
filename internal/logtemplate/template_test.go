package logtemplate

import (
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

func Test_Template_MissingParameterTag(t *testing.T) {
	t.Parallel()

	_, err := Build[string, testData]("${missing:name}", nil)
	require.ErrorIs(t, err, ErrParameterMissing)
	require.ErrorContains(t, err, "missing:name")
}

func Test_Template_MissingTag(t *testing.T) {
	t.Parallel()

	_, err := Build[string, testData]("${missing}", nil)
	require.ErrorIs(t, err, ErrParameterMissing)
	require.ErrorContains(t, err, "missing")
}
