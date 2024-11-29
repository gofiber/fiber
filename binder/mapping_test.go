package binder

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_EqualFieldType(t *testing.T) {
	var out int
	require.False(t, equalFieldType(&out, reflect.Int, "key", "query"))

	var dummy struct{ f string }
	require.False(t, equalFieldType(&dummy, reflect.String, "key", "query"))

	var dummy2 struct{ f string }
	require.False(t, equalFieldType(&dummy2, reflect.String, "f", "query"))

	var user struct {
		Name    string
		Address string `query:"address"`
		Age     int    `query:"AGE"`
	}
	require.True(t, equalFieldType(&user, reflect.String, "name", "query"))
	require.True(t, equalFieldType(&user, reflect.String, "Name", "query"))
	require.True(t, equalFieldType(&user, reflect.String, "address", "query"))
	require.True(t, equalFieldType(&user, reflect.String, "Address", "query"))
	require.True(t, equalFieldType(&user, reflect.Int, "AGE", "query"))
	require.True(t, equalFieldType(&user, reflect.Int, "age", "query"))
}

func Test_ParseParamSquareBrackets(t *testing.T) {
	tests := []struct {
		err      error
		input    string
		expected string
	}{
		{
			err:      nil,
			input:    "foo[bar]",
			expected: "foo.bar",
		},
		{
			err:      nil,
			input:    "foo[bar][baz]",
			expected: "foo.bar.baz",
		},
		{
			err:      errors.New("unmatched brackets"),
			input:    "foo[bar",
			expected: "",
		},
		{
			err:      errors.New("unmatched brackets"),
			input:    "foo[bar][baz",
			expected: "",
		},
		{
			err:      errors.New("unmatched brackets"),
			input:    "foo]bar[",
			expected: "",
		},
		{
			err:      nil,
			input:    "foo[bar[baz]]",
			expected: "foo.bar.baz",
		},
		{
			err:      nil,
			input:    "",
			expected: "",
		},
		{
			err:      nil,
			input:    "[]",
			expected: "",
		},
		{
			err:      nil,
			input:    "foo[]",
			expected: "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseParamSquareBrackets(tt.input)
			if tt.err != nil {
				require.Error(t, err)
				require.EqualError(t, err, tt.err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}
