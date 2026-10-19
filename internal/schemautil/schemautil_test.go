package schemautil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_DefaultType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		schema map[string]any
		want   string
		ok     bool
	}{
		{"empty", map[string]any{}, "string", true},
		{"format only", map[string]any{"format": "uuid"}, "string", true},
		{"has type", map[string]any{"type": "integer"}, "", false},
		{"ref", map[string]any{"$ref": "#/components/schemas/X"}, "", false},
		{"oneOf", map[string]any{"oneOf": []any{}}, "", false},
		{"allOf", map[string]any{"allOf": []any{}}, "", false},
		{"const", map[string]any{"const": 1}, "", false},
		{"string enum", map[string]any{"enum": []string{"a", "b"}}, "string", true},
		{"int enum", map[string]any{"enum": []int{1, 2}}, "integer", true},
		{"any enum of numbers", map[string]any{"enum": []any{1.5, 2.0}}, "number", true},
		{"bool enum", map[string]any{"enum": []any{true}}, "boolean", true},
		{"mixed enum", map[string]any{"enum": []any{1, "a"}}, "", false},
		{"integers and numbers", map[string]any{"enum": []any{1, 2.5}}, "number", true},
		{"json numbers", map[string]any{"enum": []json.Number{"1", "2"}}, "number", true},
		{"nullable enum", map[string]any{"enum": []any{nil, "a"}}, "string", true},
		{"only null", map[string]any{"enum": []any{nil}}, "", false},
		{"empty enum", map[string]any{"enum": []any{}}, "", false},
		{"object enum", map[string]any{"enum": []any{map[string]any{}}}, "", false},
		{"enum not a list", map[string]any{"enum": "x"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := DefaultType(tc.schema, "string")
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
