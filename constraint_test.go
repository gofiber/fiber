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
			Data: []string{"3"},
		}
		require.True(t, c.matchConstraint("hello"))
		require.False(t, c.matchConstraint("hi"))
	})

	t.Run("returns true for unknown constraint name", func(t *testing.T) {
		t.Parallel()
		c := &Constraint{
			Name: "unknownConstraint",
			Data: []string{"5"},
		}
		require.True(t, c.matchConstraint("anything"))
	})

	t.Run("resolves datetime handler", func(t *testing.T) {
		t.Parallel()
		c := &Constraint{
			Name: ConstraintDatetime,
			Data: []string{"2006-01-02"},
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
	// Values within the float64 range but outside float32 must be accepted.
	require.True(t, handler.Execute("1e308", nil))
	require.True(t, handler.Execute("3.5e38", nil))
	require.True(t, handler.Execute("-1.7976931348623157e308", nil))
	require.False(t, handler.Execute("abc", nil))
}

func Test_ConstraintExecute_AlphaConstraint(t *testing.T) {
	t.Parallel()

	handler := alphaConstraintType{}
	require.True(t, handler.Execute("hello", nil))
	require.True(t, handler.Execute("", nil))
	require.False(t, handler.Execute("hello123", nil))

	// Word-at-a-time fast-path coverage: inputs of exactly one word, more
	// than one word, and rejections at word and tail positions.
	require.True(t, handler.Execute("abcdefgh", nil))
	require.True(t, handler.Execute("AbCdEfGhIjKlMnOpQ", nil))
	require.False(t, handler.Execute("abcdefg1", nil))
	require.False(t, handler.Execute("abcdefghijklmnop9", nil))
	require.False(t, handler.Execute("abcdefgh-jkl", nil))

	// Unicode letters must still be accepted via the rune fallback,
	// regardless of where the first non-ASCII byte sits.
	require.True(t, handler.Execute("héllo", nil))
	require.True(t, handler.Execute("abcdefghé", nil))
	require.False(t, handler.Execute("héllo1", nil))
	require.False(t, handler.Execute("abcdefghé1", nil))
	require.False(t, handler.Execute(string([]byte{0xC3, 0x28}), nil)) // invalid UTF-8
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
	c := newConstraint(handler, ConstraintMinLen, []string{"3"})
	require.True(t, c.matchConstraint("hello"))
	require.False(t, c.matchConstraint("hi"))
}

func Test_ConstraintMatchConstraint_NilHandlerWithAnalyzerError(t *testing.T) {
	t.Parallel()

	c := &Constraint{
		Name: ConstraintMinLen,
		Data: []string{"notanumber"},
	}
	require.False(t, c.matchConstraint("hello"))
}

func Test_FindConstraintHandler_CustomPriority(t *testing.T) {
	t.Parallel()

	custom := &testCustomConstraintForCoverage{allowed: "x"}
	handler := findConstraintHandler("customTest", nil, []CustomConstraint{custom})
	require.NotNil(t, handler)
}

func Test_FindConstraintHandler_Builtin(t *testing.T) {
	t.Parallel()

	handler := findConstraintHandler("int", nil, nil)
	require.NotNil(t, handler)
	require.Equal(t, "int", handler.Name())
}

func Test_FindConstraintHandler_Unknown(t *testing.T) {
	t.Parallel()

	handler := findConstraintHandler("nonexistent", nil, nil)
	require.Nil(t, handler)
}

type testCustomConstraintForCoverage struct {
	allowed string
}

func (*testCustomConstraintForCoverage) Name() string { return "customTest" }
func (t *testCustomConstraintForCoverage) Execute(param string, _ ...string) bool {
	return param == t.allowed
}

type testCustomConstraintWithAnalyzer struct {
	layout string
}

func (*testCustomConstraintWithAnalyzer) Name() string { return "customDatetime" }
func (t *testCustomConstraintWithAnalyzer) Execute(param string, _ ...string) bool {
	return param == t.layout
}

func (t *testCustomConstraintWithAnalyzer) Analyze(args []string) ([]any, error) {
	if len(args) > 0 {
		t.layout = args[0]
	}
	return stringArgsToAny(args), nil
}

func Test_CustomConstraintWrapper_DelegatesAnalyze(t *testing.T) {
	t.Parallel()

	custom := &testCustomConstraintWithAnalyzer{}
	handler := findConstraintHandler("customDatetime", nil, []CustomConstraint{custom})
	require.NotNil(t, handler)

	analyzer, ok := handler.(ConstraintAnalyzer)
	require.True(t, ok)

	typed, err := analyzer.Analyze([]string{"2006-01-02"})
	require.NoError(t, err)
	require.Equal(t, []any{[]string{"2006-01-02"}}, typed)
	require.Equal(t, "2006-01-02", custom.layout)
}

type testCustomConstraintWithTypedAnalyzer struct{}

func (*testCustomConstraintWithTypedAnalyzer) Name() string { return "customRole" }
func (*testCustomConstraintWithTypedAnalyzer) Execute(param string, args ...string) bool {
	return len(args) == 1 && args[0] == "admin" && param == "admin"
}

func (*testCustomConstraintWithTypedAnalyzer) Analyze(args []string) ([]any, error) {
	return stringArgsToAny(args), nil
}

func Test_CustomConstraintWrapper_ExecuteKeepsLegacyArgsWithAnalyzer(t *testing.T) {
	t.Parallel()

	custom := &testCustomConstraintWithTypedAnalyzer{}
	handler := findConstraintHandler("customRole", nil, []CustomConstraint{custom})
	require.NotNil(t, handler)

	c := newConstraint(handler, "customRole", []string{"admin"})
	require.True(t, c.matchConstraint("admin"))
	require.False(t, c.matchConstraint("guest"))
}

func Test_resolveConstraintName_UnicodeFolding(t *testing.T) {
	t.Parallel()

	// Aliases fold with full Unicode simple case mapping, so locale-style
	// uppercase such as the Turkish dotted capital I still canonicalizes;
	// switching to an ASCII-only fold silently drops these constraints.
	require.Equal(t, ConstraintMinLen, resolveConstraintName("minLen"))
	require.Equal(t, ConstraintMinLen, resolveConstraintName("MINLEN"))
	require.Equal(t, ConstraintMinLen, resolveConstraintName("MİNLEN"))
	require.Equal(t, ConstraintMaxLen, resolveConstraintName("MAXLEN"))
	require.Equal(t, "unknown", resolveConstraintName("unknown"))
}
