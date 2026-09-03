package binder

import (
	"fmt"
	"mime/multipart"
	"reflect"
	"strconv"
	"sync"
	"testing"

	"github.com/gofiber/schema"
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

	var pointerUser struct {
		Tags *[]string `query:"tags"`
	}
	require.True(t, equalFieldType(&pointerUser, reflect.Slice, "tags", "query"))

	type nested struct {
		Values []string `query:"values"`
	}
	var nestedWrapper struct {
		Nested *nested `query:"nested"`
	}
	require.True(t, equalFieldType(&nestedWrapper, reflect.Slice, "nested.values", "query"))

	type nestedPointerSlice struct {
		Values *[]string `query:"values"`
	}
	var nestedPointerWrapper struct {
		Nested *nestedPointerSlice `query:"nested"`
	}
	require.True(t, equalFieldType(&nestedPointerWrapper, reflect.Slice, "nested.values", "query"))
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
			err:      ErrUnmatchedBrackets,
			input:    "foo[bar",
			expected: "",
		},
		{
			err:      ErrUnmatchedBrackets,
			input:    "foo[bar][baz",
			expected: "",
		},
		{
			err:      ErrUnmatchedBrackets,
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
				require.ErrorIs(t, err, tt.err)
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
	err := parseToMap(reflect.ValueOf(m), inputMap)
	require.NoError(t, err)

	require.Equal(t, "value2", m["key1"])
	require.Equal(t, "value3", m["key2"])
	require.Equal(t, "value4", m["key3"])

	// Test map[string][]string
	m2 := make(map[string][]string)
	err = parseToMap(reflect.ValueOf(m2), inputMap)
	require.NoError(t, err)

	require.Len(t, m2["key1"], 2)
	require.Contains(t, m2["key1"], "value1")
	require.Contains(t, m2["key1"], "value2")
	require.Len(t, m2["key2"], 1)
	require.Len(t, m2["key3"], 1)

	// Test map[string]any
	m3 := make(map[string]any)
	err = parseToMap(reflect.ValueOf(m3), inputMap)
	require.NoError(t, err)
	require.Empty(t, m3)

	var zeroStringMap map[string]string
	err = parseToMap(reflect.ValueOf(&zeroStringMap).Elem(), inputMap)
	require.NoError(t, err)
	require.Equal(t, "value2", zeroStringMap["key1"])

	var zeroSliceMap map[string][]string
	err = parseToMap(reflect.ValueOf(&zeroSliceMap).Elem(), inputMap)
	require.NoError(t, err)
	require.Len(t, zeroSliceMap["key1"], 2)

	err = parseToMap(reflect.ValueOf(map[string]string(nil)), inputMap)
	require.ErrorIs(t, err, ErrMapNilDestination)
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

func Benchmark_FilterFlags(b *testing.B) {
	b.ReportAllocs()

	cases := []string{
		"text/javascript; charset=utf-8",
		"application/json",
		"text/plain; charset=utf-8; foo=bar",
		"text/javascript charset=utf-8",
	}

	for i := 0; i < b.N; i++ {
		_ = FilterFlags(cases[i&3])
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
	require.EqualError(t, err, "schema: error converting value for \"age\"")
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

func Test_decoderBuilder(t *testing.T) {
	t.Parallel()
	type customInt int
	conv := func(s string) reflect.Value {
		i, err := strconv.Atoi(s)
		if err != nil {
			panic(err)
		}
		return reflect.ValueOf(customInt(i))
	}
	parserConfig := ParserConfig{
		SetAliasTag: "custom",
		ParserType: []ParserType{{
			CustomType: customInt(0),
			Converter:  conv,
		}},
		IgnoreUnknownKeys: false,
		ZeroEmpty:         false,
	}
	decAny := decoderBuilder(bindingForm, parserConfig)
	dec, ok := decAny.(*schema.Decoder)
	require.True(t, ok)
	var out struct {
		X customInt `custom:"x"`
	}
	err := dec.Decode(&out, map[string][]string{"x": {"7"}})
	require.NoError(t, err)
	require.Equal(t, customInt(7), out.X)
}

func Test_parseToMap_Extended(t *testing.T) {
	t.Parallel()
	data := map[string][]string{
		"empty": {},
		"key1":  {"value1"},
	}

	m := make(map[string]string)
	err := parseToMap(reflect.ValueOf(m), data)
	require.NoError(t, err)
	require.Empty(t, m["empty"])

	m2 := make(map[string][]int)
	err = parseToMap(reflect.ValueOf(m2), data)
	require.ErrorIs(t, err, ErrMapNotConvertible)

	m3 := make(map[string]int)
	err = parseToMap(reflect.ValueOf(m3), data)
	require.NoError(t, err)
}

func Test_decoderPoolMapInit(t *testing.T) {
	t.Parallel()

	for _, tag := range tags {
		decAny := getDecoderPool(tag).Get()
		dec, ok := decAny.(*schema.Decoder)
		require.True(t, ok)
		require.NotNil(t, dec)
		getDecoderPool(tag).Put(decAny)
	}
}

func TestSetParserDecoderConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Cleanup(func() {
		SetParserDecoder(ParserConfig{
			IgnoreUnknownKeys: true,
			ZeroEmpty:         true,
		})
	})

	type queryUser struct {
		Name string `query:"name"`
	}

	data := map[string][]string{
		"name": {"fiber"},
	}
	parserConfig := ParserConfig{
		IgnoreUnknownKeys: true,
		ZeroEmpty:         true,
	}

	start := make(chan struct{})
	const workers = 25
	errCh := make(chan error, workers*2)
	var wg sync.WaitGroup

	runWorker := func(fn func() error) {
		wg.Go(func() {
			<-start

			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("panic: %v", r)
				}
			}()

			if err := fn(); err != nil {
				errCh <- err
			}
		})
	}

	for range workers {
		runWorker(func() error {
			SetParserDecoder(parserConfig)
			return nil
		})

		runWorker(func() error {
			var out queryUser
			if err := parseToStruct("query", &out, data); err != nil {
				return err
			}

			if out.Name != "fiber" {
				return fmt.Errorf("unexpected name %q", out.Name)
			}

			return nil
		})
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}

func Test_getFieldCache(t *testing.T) {
	t.Parallel()
	require.NotNil(t, getFieldCache("header"))
	require.NotNil(t, getFieldCache("respHeader"))
	require.NotNil(t, getFieldCache("cookie"))
	require.NotNil(t, getFieldCache("form"))
	require.NotNil(t, getFieldCache("uri"))
	require.NotNil(t, getFieldCache("query"))
	require.Panics(t, func() { getFieldCache("unknown") })
}

func Test_EqualFieldType_Map(t *testing.T) {
	t.Parallel()

	sliceMap := map[string][]string{}
	require.True(t, equalFieldType(&sliceMap, reflect.Slice, "any", "query"))
	require.True(t, equalFieldType(sliceMap, reflect.Slice, "any", "query"))

	// Only a slice-valued map can hold the pieces of a split value.
	stringMap := map[string]string{}
	require.False(t, equalFieldType(&stringMap, reflect.Slice, "any", "query"))
	require.False(t, equalFieldType(stringMap, reflect.Slice, "any", "query"))

	intMap := map[string]int{}
	require.False(t, equalFieldType(&intMap, reflect.Int, "any", "query"))
}

func Test_EqualFieldType_ScalarNotSplit(t *testing.T) {
	t.Parallel()

	type Filter struct {
		IDs []int `query:"ids"`
	}
	type Request struct {
		Name   string `query:"name"`
		Filter Filter `query:"filter"`
	}
	var req Request

	// A key naming a known non-slice field is never split.
	require.False(t, equalFieldType(&req, reflect.Slice, "name", "query"))
	require.False(t, equalFieldType(&req, reflect.Slice, "filter", "query"))
	require.True(t, equalFieldType(&req, reflect.Slice, "filter.ids", "query"))
	require.False(t, equalFieldType(&req, reflect.Slice, "filter.unknown", "query"))

	// Keys that resolve to nothing keep the coarse nested-kind fallback.
	require.True(t, equalFieldType(&req, reflect.Slice, "unknown", "query"))
}

func Test_EqualFieldType_NestedDepth(t *testing.T) {
	t.Parallel()

	type Inner struct {
		Name string   `query:"name"`
		Tags []string `query:"tags"`
	}
	type Mid struct {
		Inner Inner `query:"inner"`
	}
	type Item struct {
		Name string   `query:"name"`
		Tags []string `query:"tags"`
	}
	type Request struct {
		PtrItems *[]*Item `query:"ptritems"`
		Mid      Mid      `query:"mid"`
		Items    []Item   `query:"items"`
		Arr      [2]Item  `query:"arr"`
	}
	var req Request

	require.True(t, equalFieldType(&req, reflect.Slice, "mid.inner.tags", "query"))
	require.False(t, equalFieldType(&req, reflect.Slice, "mid.inner.name", "query"))
	require.True(t, equalFieldType(&req, reflect.Slice, "Mid.Inner.Tags", "query"))

	// Struct slices are addressed through an element index before their fields.
	require.True(t, equalFieldType(&req, reflect.Slice, "items.0.tags", "query"))
	require.False(t, equalFieldType(&req, reflect.Slice, "items.0.name", "query"))
	require.True(t, equalFieldType(&req, reflect.Slice, "ptritems.3.tags", "query"))
	require.True(t, equalFieldType(&req, reflect.Slice, "arr.1.tags", "query"))

	// A path that stops at the index names the element itself.
	require.True(t, equalFieldType(&req, reflect.Struct, "items.0", "query"))
	require.False(t, equalFieldType(&req, reflect.Slice, "items.0", "query"))
}

func Test_EqualFieldType_EmbeddedStruct(t *testing.T) {
	t.Parallel()

	type Embedded struct {
		Title string   `query:"title"`
		Name  string   `query:"name"`
		Names []string `query:"names"`
	}
	type Other struct {
		Title string   `query:"title"`
		Other []string `query:"other"`
	}
	type Request struct {
		Embedded
		*Other
		Name []string `query:"name"`
	}
	var req Request

	// Promoted fields resolve at the top level and under the embedded alias.
	require.True(t, equalFieldType(&req, reflect.Slice, "names", "query"))
	require.True(t, equalFieldType(&req, reflect.Slice, "embedded.names", "query"))
	require.True(t, equalFieldType(&req, reflect.Slice, "other.other", "query"))
	require.True(t, equalFieldType(&req, reflect.Slice, "other", "query"))

	// The type's own field wins over a promoted one of the same alias.
	require.True(t, equalFieldType(&req, reflect.Slice, "name", "query"))
	require.False(t, equalFieldType(&req, reflect.String, "name", "query"))

	// An alias declared by two embedded structs is ambiguous and dropped.
	require.True(t, equalFieldType(&req, reflect.Slice, "title", "query"))
	require.True(t, equalFieldType(&req, reflect.String, "title", "query"))
}

func Test_EqualFieldType_RecursiveType(t *testing.T) {
	t.Parallel()

	type Node struct {
		Next   *Node    `query:"next"`
		Values []string `query:"values"`
		Value  int      `query:"value"`
	}
	var node Node

	require.True(t, equalFieldType(&node, reflect.Slice, "next.next.next.values", "query"))
	require.False(t, equalFieldType(&node, reflect.Slice, "next.next.value", "query"))
}

type diamondCommon struct {
	Tags []string `query:"tags"`
}

// DiamondLeft and DiamondRight must be exported for their fields to promote.
type DiamondLeft struct{ diamondCommon }

type DiamondRight struct{ diamondCommon }

type DiamondMid struct {
	DiamondLeft
	DiamondRight
}

func Test_EqualFieldType_DiamondEmbedding(t *testing.T) {
	t.Parallel()

	type Outer struct {
		DiamondMid
	}

	// Both sibling branches reach the same tagged field at the same depth, so
	// Go itself cannot resolve the selector and neither may the binder.
	var out Outer
	require.False(t, equalFieldType(&out, reflect.Slice, "tags", "query"))
}

func Test_EqualFieldType_InvalidDestination(t *testing.T) {
	t.Parallel()

	type User struct {
		Names []string `query:"names"`
	}

	// Destinations the decoder rejects must not panic the splitting check first.
	require.NotPanics(t, func() {
		require.False(t, equalFieldType(nil, reflect.Slice, "names", "query"))
		require.False(t, equalFieldType(User{}, reflect.Slice, "names", "query"))
		require.False(t, equalFieldType(map[string][]string(nil), reflect.Slice, "names", "query"))
	})
}

func Test_equalFieldType_CacheTypeMismatch(t *testing.T) {
	type Sample struct {
		Field string `query:"field"`
	}
	cache := getFieldCache("query")
	typ := reflect.TypeFor[Sample]()
	cache.Store(typ, 1)
	defer cache.Delete(typ)
	var s Sample
	require.False(t, equalFieldType(&s, reflect.String, "field", "query"))
}

func Test_buildFieldInfo_Unexported(t *testing.T) {
	t.Parallel()
	type nested struct {
		export   int
		Exported int
	}
	_ = nested{export: 0}
	type outer struct {
		Name   string
		Nested nested
	}
	info := buildFieldInfo(reflect.TypeFor[outer](), "query")
	require.Contains(t, info.fields, "name")
	_, ok := info.nestedKinds[reflect.Int]
	require.True(t, ok)
}

func Test_fieldName(t *testing.T) {
	t.Parallel()

	type Tagged struct {
		Plain    string   `query:"plain"`
		Ignored  string   `query:"-"`
		Options  []string `query:"opts,default:a|b"`
		OnlyOpts []string `query:",default:a|b"`
		Untagged []string
	}
	typ := reflect.TypeFor[Tagged]()

	cases := map[string]string{
		"Plain":    "plain",
		"Options":  "opts",
		"OnlyOpts": "onlyopts", // no alias: the Go field name is matched case-insensitively, like schema
		"Untagged": "untagged",
		"Ignored":  "-",
	}
	for goName, want := range cases {
		f, ok := typ.FieldByName(goName)
		require.True(t, ok)
		require.Equal(t, want, fieldName(&f, "query"))
	}

	require.Empty(t, fieldName(nil, "query"))
}

func Test_formatBindData_BracketNotationSuccess(t *testing.T) {
	t.Parallel()
	out := struct{}{}
	data := make(map[string][]string)
	err := formatBindData("query", out, data, "user[name]", "john", false, true)
	require.NoError(t, err)
	require.Equal(t, "john", data["user.name"][0])
}

func Test_formatBindData_FileHeaderTypeMismatch(t *testing.T) {
	t.Parallel()
	out := struct{}{}
	data := map[string][]int{}
	files := []*multipart.FileHeader{{Filename: "file1.txt"}}
	err := formatBindData("query", out, data, "file", files, false, false)
	require.EqualError(t, err, "unsupported value type: []*multipart.FileHeader")
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
	for b.Loop() {
		equalFieldType(&user, reflect.String, "name", "query")
		equalFieldType(&user, reflect.Int, "age", "query")
		equalFieldType(&user, reflect.String, "user.name", "query")
	}
}

func Test_FieldInfo_ResolveKindsAgreesWithResolve(t *testing.T) {
	t.Parallel()

	type Inner struct {
		Name string `query:"name"`
	}
	type Sample struct {
		DiamondMid
		Inner   Inner    `query:"inner"`
		Title   string   `query:"title"`
		Labels  []string `query:"labels"`
		Numbers []int    `query:"numbers"`
	}

	info := buildFieldInfo(reflect.TypeFor[Sample](), "query")

	for name, kind := range info.kinds {
		typ, resolved := info.resolve(name, false)
		if kind == reflect.Invalid {
			require.False(t, resolved, "name=%q", name)
			continue
		}
		require.True(t, resolved, "name=%q", name)
		require.Equal(t, kind, typ.Kind(), "name=%q", name)
	}

	for _, names := range []map[string]reflect.Type{info.fields, info.embedded} {
		for name := range names {
			require.Contains(t, info.kinds, name)
		}
	}
	for name := range info.promoted {
		require.Contains(t, info.kinds, name)
	}
	require.Equal(t, reflect.Invalid, info.kinds["tags"], "an ambiguous alias stays unresolvable")
	require.Equal(t, reflect.Slice, info.kinds["labels"])
	require.Equal(t, reflect.Struct, info.kinds["inner"])
}

type walkInner struct {
	Name string   `query:"name"`
	Tags []string `query:"tags"`
	Age  int      `query:"age"`
}

type walkTarget struct {
	Lookup map[string]walkInner  `query:"lookup"`
	Deep   map[string][]struct{} `query:"deep"`
	Title  string                `query:"title"`
	List   []walkInner           `query:"list"`
	Inner  walkInner             `query:"inner"`
}

func Test_StructKeyKind_NestedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		kind reflect.Kind
		want bool
	}{
		{key: "title", kind: reflect.String, want: true},
		{key: "inner.name", kind: reflect.String, want: true},
		{key: "inner.tags", kind: reflect.Slice, want: true},
		{key: "inner.age", kind: reflect.Slice, want: false},
		{key: "[inner].tags", kind: reflect.Slice, want: true},
		{key: "inner[tags]", kind: reflect.Slice, want: true},
		{key: "list.0.tags", kind: reflect.Slice, want: true},
		{key: "list.abc.tags", kind: reflect.Slice, want: false},
		{key: "lookup.anything.tags", kind: reflect.Slice, want: true},
		{key: "title.sub", kind: reflect.String, want: false},
		{key: "deep.k.0", kind: reflect.Struct, want: true},
		{key: "", kind: reflect.String, want: false},
		{key: "unknown.tags", kind: reflect.Slice, want: true},
		{key: "unknown.tags", kind: reflect.Bool, want: false},
	}

	var out walkTarget
	for _, tc := range tests {
		require.Equal(t, tc.want, equalFieldType(&out, tc.kind, tc.key, "query"),
			"key=%q kind=%s", tc.key, tc.kind)
	}
}

func Test_IsDigits(t *testing.T) {
	t.Parallel()

	require.False(t, isDigits(""))
	require.False(t, isDigits("12a"))
	require.False(t, isDigits("-1"))
	require.True(t, isDigits("0"))
	require.True(t, isDigits("120"))
}

func Test_StructKeyKind_NestedCacheTypeMismatch(t *testing.T) {
	cache := getFieldCache("query")
	typ := reflect.TypeFor[walkInner]()
	cache.Store(typ, 1)
	defer cache.Delete(typ)

	var out walkTarget
	require.False(t, equalFieldType(&out, reflect.Slice, "inner.tags", "query"))
}

// MutualOuter and MutualInner embed each other, so the promotion walk has to
// stop itself; both must be exported for their fields to promote at all.
type MutualOuter struct {
	MutualInner
}

type MutualInner struct {
	*MutualOuter
	Names []string `query:"names"`
}

func Test_CollectPromoted_MutualEmbedding(t *testing.T) {
	t.Parallel()

	var out MutualOuter
	require.True(t, equalFieldType(&out, reflect.Slice, "names", "query"))
	require.False(t, equalFieldType(&out, reflect.Bool, "names", "query"))
}
