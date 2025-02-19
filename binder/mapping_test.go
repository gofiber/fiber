package binder

import (
	"errors"
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_EqualFieldType(t *testing.T) {
	var out int
	require.False(t, equalFieldType(&out, reflect.Int, "key"))

	var dummy struct{ f string }
	require.False(t, equalFieldType(&dummy, reflect.String, "key"))

	var dummy2 struct{ f string }
	require.False(t, equalFieldType(&dummy2, reflect.String, "f"))

	var user struct {
		Name    string
		Address string `query:"address"`
		Age     int    `query:"AGE"`
	}
	require.True(t, equalFieldType(&user, reflect.String, "name"))
	require.True(t, equalFieldType(&user, reflect.String, "Name"))
	require.True(t, equalFieldType(&user, reflect.String, "address"))
	require.True(t, equalFieldType(&user, reflect.String, "Address"))
	require.True(t, equalFieldType(&user, reflect.Int, "AGE"))
	require.True(t, equalFieldType(&user, reflect.Int, "age"))

	var user2 struct {
		User struct {
			Name    string
			Address string `query:"address"`
			Age     int    `query:"AGE"`
		} `query:"user"`
	}

	require.True(t, equalFieldType(&user2, reflect.String, "user.name"))
	require.True(t, equalFieldType(&user2, reflect.String, "user.Name"))
	require.True(t, equalFieldType(&user2, reflect.String, "user.address"))
	require.True(t, equalFieldType(&user2, reflect.String, "user.Address"))
	require.True(t, equalFieldType(&user2, reflect.Int, "user.AGE"))
	require.True(t, equalFieldType(&user2, reflect.Int, "user.age"))
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

func Test_parseToMap(t *testing.T) {
	inputMap := map[string][]string{
		"key1": {"value1", "value2"},
		"key2": {"value3"},
		"key3": {"value4"},
	}

	// Test map[string]string
	m := make(map[string]string)
	err := parseToMap(m, inputMap)
	require.NoError(t, err)

	require.Equal(t, "value2", m["key1"])
	require.Equal(t, "value3", m["key2"])
	require.Equal(t, "value4", m["key3"])

	// Test map[string][]string
	m2 := make(map[string][]string)
	err = parseToMap(m2, inputMap)
	require.NoError(t, err)

	require.Len(t, m2["key1"], 2)
	require.Contains(t, m2["key1"], "value1")
	require.Contains(t, m2["key1"], "value2")
	require.Len(t, m2["key2"], 1)
	require.Len(t, m2["key3"], 1)

	// Test map[string]any
	m3 := make(map[string]any)
	err = parseToMap(m3, inputMap)
	require.ErrorIs(t, err, ErrMapNotConvertable)
}

func Test_FilterFlags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "text/javascript; charset=utf-8",
			expected: "text/javascript",
		},
		{
			input:    "text/javascript",
			expected: "text/javascript",
		},

		{
			input:    "text/javascript; charset=utf-8; foo=bar",
			expected: "text/javascript",
		},
		{
			input:    "text/javascript charset=utf-8",
			expected: "text/javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := FilterFlags(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBindData(t *testing.T) {
	t.Run("string value with valid key", func(t *testing.T) {
		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData(out, data, "name", "John", false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data["name"]) != 1 || data["name"][0] != "John" {
			t.Fatalf("expected data[\"name\"] = [John], got %v", data["name"])
		}
	})

	t.Run("unsupported value type", func(t *testing.T) {
		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData(out, data, "age", 30, false, false) // int is unsupported
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("bracket notation parsing error", func(t *testing.T) {
		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData(out, data, "invalid[", "value", false, true) // malformed bracket notation
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("handling multipart file headers", func(t *testing.T) {
		out := struct{}{}
		data := make(map[string][]*multipart.FileHeader)
		files := []*multipart.FileHeader{
			{Filename: "file1.txt"},
			{Filename: "file2.txt"},
		}
		err := formatBindData(out, data, "files", files, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data["files"]) != 2 {
			t.Fatalf("expected 2 files, got %d", len(data["files"]))
		}
	})

	t.Run("type casting error", func(t *testing.T) {
		out := struct{}{}
		data := map[string][]int{} // Incorrect type to force a casting error
		err := formatBindData(out, data, "key", "value", false, false)
		require.Equal(t, err.Error(), "unsupported value type: string")
	})
}

func TestAssignBindData(t *testing.T) {
	t.Run("splitting enabled with comma", func(t *testing.T) {
		out := struct {
			Colors []string `query:"colors"`
		}{}
		data := make(map[string][]string)
		assignBindData(&out, data, "colors", "red,blue,green", true)
		require.Len(t, data["colors"], 3)
	})

	t.Run("splitting disabled", func(t *testing.T) {
		out := []string{}
		data := make(map[string][]string)
		assignBindData(out, data, "color", "red,blue", false)
		require.Len(t, data["color"], 1)
	})
}
