package fiber

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ConstraintMatchConstraint_NilHandler(t *testing.T) {
	t.Parallel()

	t.Run("resolves handler from Name", func(t *testing.T) {
		t.Parallel()
		c := &Constraint{
			Name: ConstraintMinLen,
			Data: []any{3},
		}
		require.True(t, c.matchConstraint("hello"))
		require.False(t, c.matchConstraint("hi"))
	})

	t.Run("returns true for unknown constraint name", func(t *testing.T) {
		t.Parallel()
		c := &Constraint{
			Name: "unknownConstraint",
			Data: []any{"5"},
		}
		require.True(t, c.matchConstraint("anything"))
	})

	t.Run("resolves datetime handler", func(t *testing.T) {
		t.Parallel()
		c := &Constraint{
			Name: ConstraintDatetime,
			Data: []any{"2006-01-02"},
		}
		require.True(t, c.matchConstraint("2024-01-15"))
		require.False(t, c.matchConstraint("not-a-date"))
	})
}

func Test_ConstraintAnalyze_MissingArgs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		handler ConstraintAnalyzer
		name    string
	}{
		{datetimeConstraintType{}, "datetime"},
		{minLenConstraintType{}, "minLen"},
		{maxLenConstraintType{}, "maxLen"},
		{lenConstraintType{}, "len"},
		{minConstraintType{}, "min"},
		{maxConstraintType{}, "max"},
		{regexConstraintType{}, "regex"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.handler.Analyze([]string{})
			require.Error(t, err)
		})
	}
}

func Test_ConstraintAnalyze_BetweenLen(t *testing.T) {
	t.Parallel()

	t.Run("missing second arg", func(t *testing.T) {
		t.Parallel()
		_, err := betweenLenConstraintType{}.Analyze([]string{"1"})
		require.Error(t, err)
	})
}

func Test_ConstraintAnalyze_Range(t *testing.T) {
	t.Parallel()

	t.Run("missing second arg", func(t *testing.T) {
		t.Parallel()
		_, err := rangeConstraintType{}.Analyze([]string{"1"})
		require.Error(t, err)
	})
}

func Test_ConstraintAnalyze_Regex(t *testing.T) {
	t.Parallel()

	t.Run("invalid pattern", func(t *testing.T) {
		t.Parallel()
		_, err := regexConstraintType{}.Analyze([]string{"("})
		require.Error(t, err)
	})
}

func Test_ConstraintExecute_IntConstraint(t *testing.T) {
	t.Parallel()

	handler := intConstraintType{}
	require.True(t, handler.Execute("42", nil))
	require.False(t, handler.Execute("abc", nil))
}

func Test_ConstraintExecute_BoolConstraint(t *testing.T) {
	t.Parallel()

	handler := boolConstraintType{}
	require.True(t, handler.Execute("true", nil))
	require.True(t, handler.Execute("false", nil))
	require.True(t, handler.Execute("1", nil))
	require.True(t, handler.Execute("0", nil))
	require.False(t, handler.Execute("maybe", nil))
}

func Test_ConstraintExecute_FloatConstraint(t *testing.T) {
	t.Parallel()

	handler := floatConstraintType{}
	require.True(t, handler.Execute("3.14", nil))
	require.False(t, handler.Execute("abc", nil))
}

func Test_ConstraintExecute_AlphaConstraint(t *testing.T) {
	t.Parallel()

	handler := alphaConstraintType{}
	require.True(t, handler.Execute("hello", nil))
	require.False(t, handler.Execute("hello123", nil))
}

func Test_ConstraintExecute_GuidConstraint(t *testing.T) {
	t.Parallel()

	handler := guidConstraintType{}
	require.True(t, handler.Execute("12345678-1234-1234-1234-123456789abc", nil))
	require.False(t, handler.Execute("not-a-guid", nil))
}

func Test_ConstraintExecute_DatetimeConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := datetimeConstraintType{}
	require.False(t, handler.Execute("2024-01-15", nil))
}

func Test_ConstraintExecute_DatetimeConstraint_EmptyLayout(t *testing.T) {
	t.Parallel()

	handler := datetimeConstraintType{}
	require.False(t, handler.Execute("2024-01-15", []any{""}))
}

func Test_ConstraintExecute_MinLenConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := minLenConstraintType{}
	require.False(t, handler.Execute("hello", nil))
}

func Test_ConstraintExecute_MaxLenConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := maxLenConstraintType{}
	require.False(t, handler.Execute("hello", nil))
}

func Test_ConstraintExecute_LenConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := lenConstraintType{}
	require.False(t, handler.Execute("hello", nil))
}

func Test_ConstraintExecute_BetweenLenConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := betweenLenConstraintType{}
	require.False(t, handler.Execute("hello", nil))
}

func Test_ConstraintExecute_MinConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := minConstraintType{}
	require.False(t, handler.Execute("10", nil))
}

func Test_ConstraintExecute_MaxConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := maxConstraintType{}
	require.False(t, handler.Execute("10", nil))
}

func Test_ConstraintExecute_RangeConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := rangeConstraintType{}
	require.False(t, handler.Execute("5", nil))
}

func Test_ConstraintExecute_RegexConstraint_NilData(t *testing.T) {
	t.Parallel()

	handler := regexConstraintType{}
	require.False(t, handler.Execute("hello", nil))
}

func Test_ConstraintExecute_RegexConstraint_Compiled(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`^\d+$`)
	handler := regexConstraintType{}
	require.True(t, handler.Execute("123", []any{re}))
	require.False(t, handler.Execute("abc", []any{re}))
}

func Test_ConstraintMatchConstraint_WithTypedData(t *testing.T) {
	t.Parallel()

	handler := minLenConstraintType{}
	c := newConstraint(handler, []string{"3"})
	require.True(t, c.matchConstraint("hello"))
	require.False(t, c.matchConstraint("hi"))
}

func Test_ConstraintMatchConstraint_NilHandlerWithAnalyzerError(t *testing.T) {
	t.Parallel()

	c := &Constraint{
		Name: ConstraintMinLen,
		Data: []any{"notanumber"},
	}
	require.False(t, c.matchConstraint("hello"))
}

func Test_FindConstraintHandler_CustomPriority(t *testing.T) {
	t.Parallel()

	custom := &testCustomConstraintForCoverage{allowed: "x"}
	handler := findConstraintHandler("customTest", []CustomConstraint{custom})
	require.NotNil(t, handler)
}

func Test_FindConstraintHandler_Builtin(t *testing.T) {
	t.Parallel()

	handler := findConstraintHandler("int", nil)
	require.NotNil(t, handler)
	require.Equal(t, "int", handler.Name())
}

func Test_FindConstraintHandler_Unknown(t *testing.T) {
	t.Parallel()

	handler := findConstraintHandler("nonexistent", nil)
	require.Nil(t, handler)
}

type testCustomConstraintForCoverage struct {
	allowed string
}

func (*testCustomConstraintForCoverage) Name() string { return "customTest" }
func (t *testCustomConstraintForCoverage) Execute(param string, _ ...string) bool {
	return param == t.allowed
}
