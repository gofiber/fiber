package fiber

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ConstraintMatchConstraint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		constraint *Constraint
		name       string
		valid      []string // parameters matchConstraint must accept
		invalid    []string // parameters matchConstraint must reject
	}{
		{
			name:       "nil handler resolves handler from Name",
			constraint: &Constraint{Name: ConstraintMinLen, Data: []string{"3"}},
			valid:      []string{"hello"},
			invalid:    []string{"hi"},
		},
		{
			name:       "nil handler returns true for unknown constraint name",
			constraint: &Constraint{Name: "unknownConstraint", Data: []string{"5"}},
			valid:      []string{"anything"},
		},
		{
			name:       "nil handler resolves datetime handler",
			constraint: &Constraint{Name: ConstraintDatetime, Data: []string{"2006-01-02"}},
			valid:      []string{"2024-01-15"},
			invalid:    []string{"not-a-date"},
		},
		{
			name:       "nil handler with analyzer error",
			constraint: &Constraint{Name: ConstraintMinLen, Data: []string{"notanumber"}},
			invalid:    []string{"hello"},
		},
		{
			name: "typed data", constraint: newConstraint(minLenConstraintType{}, ConstraintMinLen, []string{"3"}),
			valid: []string{"hello"}, invalid: []string{"hi"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, param := range tc.valid {
				require.True(t, tc.constraint.matchConstraint(param), "%q should match", param)
			}
			for _, param := range tc.invalid {
				require.False(t, tc.constraint.matchConstraint(param), "%q should not match", param)
			}
		})
	}
}

func Test_ConstraintAnalyze_Errors(t *testing.T) {
	t.Parallel()
	noArgs := []string{}

	testCases := []struct {
		handler ConstraintAnalyzer
		name    string
		args    []string
	}{
		{handler: datetimeConstraintType{}, name: "datetime", args: noArgs},
		{handler: minLenConstraintType{}, name: "minLen", args: noArgs},
		{handler: maxLenConstraintType{}, name: "maxLen", args: noArgs},
		{handler: lenConstraintType{}, name: "len", args: noArgs},
		{handler: minConstraintType{}, name: "min", args: noArgs},
		{handler: maxConstraintType{}, name: "max", args: noArgs},
		{handler: regexConstraintType{}, name: "regex", args: noArgs},
		{handler: betweenLenConstraintType{}, name: "betweenLen missing second arg", args: []string{"1"}},
		{handler: rangeConstraintType{}, name: "range missing second arg", args: []string{"1"}},
		{handler: regexConstraintType{}, name: "regex invalid pattern", args: []string{"("}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := tc.handler.Analyze(tc.args)
			require.Error(t, err)
		})
	}
}

func Test_ConstraintExecute(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^\d+$`)

	testCases := []struct {
		handler ConstraintHandler
		name    string
		data    []any
		valid   []string // parameters the constraint must accept
		invalid []string // parameters the constraint must reject
	}{
		{name: "int", handler: intConstraintType{}, valid: []string{"42"}, invalid: []string{"abc"}},
		{
			name: "bool", handler: boolConstraintType{}, valid: []string{"true", "false", "1", "0"},
			invalid: []string{"maybe"},
		},
		{
			name:    "float",
			handler: floatConstraintType{},
			// Values within the float64 range but outside float32 must be accepted.
			valid:   []string{"3.14", "1e308", "3.5e38", "-1.7976931348623157e308"},
			invalid: []string{"abc"},
		},
		{
			name:    "alpha",
			handler: alphaConstraintType{},
			valid: []string{
				"hello", "",
				// Word-at-a-time fast-path coverage: inputs of exactly one word
				// and more than one word.
				"abcdefgh", "AbCdEfGhIjKlMnOpQ",
				// Unicode letters must still be accepted via the rune fallback,
				// regardless of where the first non-ASCII byte sits.
				"héllo", "abcdefghé",
			},
			invalid: []string{
				"hello123",
				// Word-at-a-time fast-path coverage: rejections at word and tail
				// positions.
				"abcdefg1", "abcdefghijklmnop9", "abcdefgh-jkl",
				// The rune fallback rejects too.
				"héllo1", "abcdefghé1",
				string([]byte{0xC3, 0x28}), // invalid UTF-8
			},
		},
		{
			name: "guid", handler: guidConstraintType{}, valid: []string{"12345678-1234-1234-1234-123456789abc"},
			invalid: []string{"not-a-guid"},
		},
		{
			name: "regex compiled", handler: regexConstraintType{}, data: []any{re}, valid: []string{"123"},
			invalid: []string{"abc"},
		},
		// A constraint rejects every parameter when its data is missing or unusable instead of panicking.
		{name: "datetime nil data", handler: datetimeConstraintType{}, invalid: []string{"2024-01-15"}},
		{
			name: "datetime empty layout", handler: datetimeConstraintType{}, data: []any{""},
			invalid: []string{"2024-01-15"},
		},
		{name: "minLen nil data", handler: minLenConstraintType{}, invalid: []string{"hello"}},
		{name: "maxLen nil data", handler: maxLenConstraintType{}, invalid: []string{"hello"}},
		{name: "len nil data", handler: lenConstraintType{}, invalid: []string{"hello"}},
		{name: "betweenLen nil data", handler: betweenLenConstraintType{}, invalid: []string{"hello"}},
		{name: "min nil data", handler: minConstraintType{}, invalid: []string{"10"}},
		{name: "max nil data", handler: maxConstraintType{}, invalid: []string{"10"}},
		{name: "range nil data", handler: rangeConstraintType{}, invalid: []string{"5"}},
		{name: "regex nil data", handler: regexConstraintType{}, invalid: []string{"hello"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, param := range tc.valid {
				require.True(t, tc.handler.Execute(param, tc.data), "%q should be accepted", param)
			}
			for _, param := range tc.invalid {
				require.False(t, tc.handler.Execute(param, tc.data), "%q should be rejected", param)
			}
		})
	}
}

func Test_FindConstraintHandler(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		lookup   string
		wantName string // checked when set
		custom   []CustomConstraint
		found    bool
	}{
		{
			name: "CustomPriority", lookup: "customTest", found: true,
			custom: []CustomConstraint{&testCustomConstraintForCoverage{allowed: "x"}},
		},
		{name: "Builtin", lookup: "int", wantName: "int", found: true},
		{name: "Unknown", lookup: "nonexistent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler := findConstraintHandler(tc.lookup, nil, tc.custom)
			if !tc.found {
				require.Nil(t, handler)
				return
			}
			require.NotNil(t, handler)
			if tc.wantName != "" {
				require.Equal(t, tc.wantName, handler.Name())
			}
		})
	}
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
