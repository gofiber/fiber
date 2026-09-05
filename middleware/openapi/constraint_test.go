package openapi

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_PathParamSchema(t *testing.T) {
	t.Parallel()

	cases := []struct {
		expected map[string]any
		name     string
		raw      string
	}{
		{map[string]any{"type": "string"}, "no constraint", ""},
		{map[string]any{"type": "string"}, "only separators", ";;"},
		{map[string]any{"type": "string"}, "blank entry", "  "},
		{map[string]any{"type": "integer"}, "int", "int"},
		{map[string]any{"type": "integer"}, "case insensitive", "INT"},
		{map[string]any{"type": "boolean"}, "bool", "bool"},
		{map[string]any{"type": "number"}, "float", "float"},
		{map[string]any{"type": "string"}, "alpha carries no pattern", "alpha"},
		{map[string]any{"type": "string", "format": "uuid"}, "guid", "guid"},
		{map[string]any{"type": "string"}, "unknown constraint", "totallyMadeUp"},
		{map[string]any{"type": "string"}, "custom with args", "myConstraint(1,2)"},

		{map[string]any{"type": "string", "format": "date-time"}, "datetime rfc3339", "datetime(" + time.RFC3339 + ")"},
		{map[string]any{"type": "string", "format": "date-time"}, "datetime rfc3339 nano", "datetime(" + time.RFC3339Nano + ")"},
		{map[string]any{"type": "string", "format": "date"}, "datetime date only", "datetime(" + time.DateOnly + ")"},
		{map[string]any{"type": "string", "format": "time"}, "datetime time only", "datetime(" + time.TimeOnly + ")"},
		{map[string]any{"type": "string"}, "datetime custom layout", "datetime(2006/01/02)"},
		{map[string]any{"type": "string"}, "datetime without layout", "datetime"},

		{map[string]any{"type": "string", "pattern": "^[a-z]+$"}, "regex", "regex(^[a-z]+$)"},
		{map[string]any{"type": "string"}, "regex without pattern", "regex()"},
		{map[string]any{"type": "string"}, "regex bare", "regex"},

		{map[string]any{"type": "string", "minLength": 3}, "minLen", "minLen(3)"},
		{map[string]any{"type": "string", "minLength": 3}, "minLen lowercase alias", "minlen(3)"},
		{map[string]any{"type": "string", "maxLength": 9}, "maxLen", "maxLen(9)"},
		{map[string]any{"type": "string", "maxLength": 9}, "maxLen lowercase alias", "maxlen(9)"},
		{map[string]any{"type": "string", "minLength": 4, "maxLength": 4}, "len", "len(4)"},
		{map[string]any{"type": "string", "minLength": 2, "maxLength": 6}, "betweenLen", "betweenLen(2,6)"},
		{map[string]any{"type": "string", "minLength": 2, "maxLength": 6}, "betweenLen alias", "betweenlen(2,6)"},
		{map[string]any{"type": "string", "minLength": 2}, "betweenLen missing upper", "betweenLen(2)"},
		{map[string]any{"type": "string"}, "minLen without argument", "minLen"},
		{map[string]any{"type": "string"}, "minLen with non-numeric argument", "minLen(abc)"},
		{map[string]any{"type": "string", "minLength": 3}, "argument is trimmed", "minLen( 3 )"},

		{map[string]any{"type": "integer", "minimum": 5}, "min", "min(5)"},
		{map[string]any{"type": "integer", "maximum": 7}, "max", "max(7)"},
		{map[string]any{"type": "integer", "minimum": 1, "maximum": 10}, "range", "range(1,10)"},
		{map[string]any{"type": "integer", "minimum": -3}, "negative bound", "min(-3)"},

		{map[string]any{"type": "integer", "minimum": 5}, "chain type then bound", "int;min(5)"},
		{map[string]any{"type": "integer", "minimum": 5}, "chain bound then type", "min(5);int"},
		{map[string]any{"type": "string", "minLength": 2}, "conflicting type is ignored", "minLen(2);int"},
		{map[string]any{"type": "integer", "minimum": 1}, "first bound wins", "min(1);min(9)"},

		{map[string]any{"type": "string", "pattern": `a\;b`}, "escaped separator", `regex(a\;b)`},
		{map[string]any{"type": "string", "pattern": `a\,b`}, "escaped argument separator", `regex(a\,b)`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, pathParamSchema(tc.raw))
		})
	}
}

func Test_ScanConstraintSpan(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		pattern  string
		raw      string
		open     int
		wantNext int
	}{
		{"simple", "<int>rest", "int", 0, 5},
		{"empty span", "<>", "", 0, 2},
		{"inner angle bracket closes at first '>'", "<regex(^<a>$)>", "regex(^<a", 0, 11},
		{"escaped close", `<regex(a\>b)>`, `regex(a\>b)`, 0, 13},
		{"unterminated runs to end", "<int", "int", 0, 4},
		{"offset start", "x<bool>", "bool", 1, 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, next := scanConstraintSpan(tc.pattern, tc.open)
			require.Equal(t, tc.raw, raw)
			require.Equal(t, tc.wantNext, next)
		})
	}
}

func Test_SplitNonEscaped(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"a", "b", "c"}, splitNonEscaped("a;b;c", ';'))
	require.Equal(t, []string{`a\;b`}, splitNonEscaped(`a\;b`, ';'))
	require.Equal(t, []string{""}, splitNonEscaped("", ';'))
	require.Equal(t, []string{"", "a"}, splitNonEscaped(";a", ';'))
	require.Equal(t, []string{"a", ""}, splitNonEscaped("a;", ';'))
}

func Test_SplitConstraintEntry(t *testing.T) {
	t.Parallel()

	require.Equal(t, parsedConstraint{name: "int"}, splitConstraintEntry("int"))
	require.Equal(t, parsedConstraint{name: "range", args: []string{"1", "10"}}, splitConstraintEntry("range(1,10)"))
	require.Equal(t, parsedConstraint{name: "regex", args: []string{"(a|b)+"}}, splitConstraintEntry("regex((a|b)+)"))
	require.Equal(t, parsedConstraint{name: "odd)name"}, splitConstraintEntry("odd)name"))
	require.Equal(t, parsedConstraint{name: "int"}, splitConstraintEntry("  int  "))
}

func Test_IsPlainJSONTagName(t *testing.T) {
	t.Parallel()

	require.True(t, isPlainJSONTagName("name"))
	require.True(t, isPlainJSONTagName("a b"))
	require.True(t, isPlainJSONTagName("a-b_c.d"))
	require.True(t, isPlainJSONTagName("field2"))
	require.True(t, isPlainJSONTagName("Ünïcøde"))
	require.False(t, isPlainJSONTagName(""))

	for _, name := range []string{`a\b`, `a"b`, "a'b", "a`b", "a\tb"} {
		require.Falsef(t, isPlainJSONTagName(name), "%q should take the probe path", name)
	}
}

func Test_EffectiveJSONTagName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{`a\b`, "a'b", "a`b", "a\tb", `a"b`} {
		want := wireNameFor(t, name)
		require.Equalf(t, want, effectiveJSONTagName(name),
			"probe disagrees with encoding/json for tag %q", name)
	}
}

func wireNameFor(t *testing.T, name string) string {
	t.Helper()

	typ := reflect.StructOf([]reflect.StructField{{
		Name: "Probe",
		Type: reflect.TypeFor[string](),
		Tag:  reflect.StructTag("json:" + strconv.Quote(name)),
	}})

	encoded, err := json.Marshal(reflect.New(typ).Elem().Interface())
	require.NoError(t, err)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	for key := range decoded {
		if key == "Probe" {
			return ""
		}
		return key
	}
	return ""
}
