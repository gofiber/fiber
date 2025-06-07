package binder

import (
	"errors"
	"mime/multipart"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_EqualFieldType(t *testing.T) {
	t.Parallel()

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

	var user2 struct {
		User struct {
			Name    string
			Address string `query:"address"`
			Age     int    `query:"AGE"`
		} `query:"user"`
	}

	require.True(t, equalFieldType(&user2, reflect.String, "user.name", "query"))
	require.True(t, equalFieldType(&user2, reflect.String, "user.Name", "query"))
	require.True(t, equalFieldType(&user2, reflect.String, "user.address", "query"))
	require.True(t, equalFieldType(&user2, reflect.String, "user.Address", "query"))
	require.True(t, equalFieldType(&user2, reflect.Int, "user.AGE", "query"))
	require.True(t, equalFieldType(&user2, reflect.Int, "user.age", "query"))
}

func Test_ParseParamSquareBrackets(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

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
	require.ErrorIs(t, err, ErrMapNotConvertible)
}

func Test_FilterFlags(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			result := FilterFlags(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBindData(t *testing.T) {
	t.Parallel()

	t.Run("string value with valid key", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData("query", out, data, "name", "John", false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data["name"]) != 1 || data["name"][0] != "John" {
			t.Fatalf("expected data[\"name\"] = [John], got %v", data["name"])
		}
	})

	t.Run("unsupported value type", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData("query", out, data, "age", 30, false, false) // int is unsupported
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("bracket notation parsing error", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData("query", out, data, "invalid[", "value", false, true) // malformed bracket notation
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("handling multipart file headers", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]*multipart.FileHeader)
		files := []*multipart.FileHeader{
			{Filename: "file1.txt"},
			{Filename: "file2.txt"},
		}
		err := formatBindData("query", out, data, "files", files, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data["files"]) != 2 {
			t.Fatalf("expected 2 files, got %d", len(data["files"]))
		}
	})

	t.Run("type casting error", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := map[string][]int{} // Incorrect type to force a casting error
		err := formatBindData("query", out, data, "key", "value", false, false)
		require.Equal(t, "unsupported value type: string", err.Error())
	})
}

func TestAssignBindData(t *testing.T) {
	t.Parallel()

	t.Run("splitting enabled with comma", func(t *testing.T) {
		t.Parallel()

		out := struct {
			Colors []string `query:"colors"`
		}{}
		data := make(map[string][]string)
		assignBindData("query", &out, data, "colors", "red,blue,green", true)
		require.Len(t, data["colors"], 3)
	})

	t.Run("splitting disabled", func(t *testing.T) {
		t.Parallel()

		var out []string
		data := make(map[string][]string)
		assignBindData("query", out, data, "color", "red,blue", false)
		require.Len(t, data["color"], 1)
	})
}

func Test_parseToStruct_MismatchedData(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `query:"name"`
		Age  int    `query:"age"`
	}

	data := map[string][]string{
		"name": {"John"},
		"age":  {"invalidAge"},
	}

	err := parseToStruct("query", &User{}, data)
	require.Error(t, err)
	require.EqualError(t, err, "bind: schema: error converting value for \"age\"")
}

func Test_formatBindData_ErrorCases(t *testing.T) {
	t.Parallel()

	t.Run("unsupported value type int", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData("query", out, data, "age", 30, false, false) // int is unsupported
		require.Error(t, err)
		require.EqualError(t, err, "unsupported value type: int")
	})

	t.Run("unsupported value type map", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData("query", out, data, "map", map[string]string{"key": "value"}, false, false) // map is unsupported
		require.Error(t, err)
		require.EqualError(t, err, "unsupported value type: map[string]string")
	})

	t.Run("bracket notation parsing error", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData("query", out, data, "invalid[", "value", false, true) // malformed bracket notation
		require.Error(t, err)
		require.EqualError(t, err, "unmatched brackets")
	})

	t.Run("type casting error for []string", func(t *testing.T) {
		t.Parallel()

		out := struct{}{}
		data := make(map[string][]string)
		err := formatBindData("query", out, data, "names", 123, false, false) // invalid type for []string
		require.Error(t, err)
		require.EqualError(t, err, "unsupported value type: int")
	})
}

func Benchmark_equalFieldType(b *testing.B) {
	type Nested struct {
		Name string `query:"name"`
	}
	type User struct {
		Name   string `query:"name"`
		Nested Nested `query:"user"`
		Age    int    `query:"age"`
	}
	var user User

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		equalFieldType(&user, reflect.String, "name", "query")
		equalFieldType(&user, reflect.Int, "age", "query")
		equalFieldType(&user, reflect.String, "user.name", "query")
	}
}
