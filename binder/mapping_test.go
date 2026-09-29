package binder

import (
	"bytes"
	"fmt"
	"maps"
	"mime/multipart"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/schema"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
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

// Test_BindData_Bind pins how a binder files a string value: under its key,
// and split at its commas only when splitting is on and the key names a slice
// of the destination's.
func Test_BindData_Bind(t *testing.T) {
	t.Parallel()

	t.Run("value", func(t *testing.T) {
		t.Parallel()

		data := &bindData{values: make(map[string][]string), mode: bindMap}
		require.NoError(t, data.bind("query", &map[string][]string{}, "name", "John", false, false))
		require.Equal(t, map[string][]string{"name": {"John"}}, data.values)
	})

	t.Run("splitting enabled with comma", func(t *testing.T) {
		t.Parallel()

		out := struct {
			Color  string   `query:"color"`
			Colors []string `query:"colors"`
		}{}
		data := &bindData{mode: bindPairs}
		require.NoError(t, data.bind("query", &out, "colors", "red,blue,green", true, false))
		require.NoError(t, data.bind("query", &out, "color", "red,blue", true, false))
		require.Equal(t, []string{"colors", "colors", "colors", "color"}, data.keys)
		require.Equal(t, []string{"red", "blue", "green", "red,blue"}, data.pairValues)
	})

	t.Run("splitting disabled", func(t *testing.T) {
		t.Parallel()

		out := struct {
			Colors []string `query:"colors"`
		}{}
		data := &bindData{mode: bindPairs}
		require.NoError(t, data.bind("query", &out, "colors", "red,blue", false, false))
		require.Equal(t, []string{"red,blue"}, data.pairValues)
	})
}

// Test_BindData_BracketNotation pins the key rewrite a source with bracket
// notation gets: its values and files are filed under the dotted key, and a
// malformed key is an error.
func Test_BindData_BracketNotation(t *testing.T) {
	t.Parallel()

	out := &map[string][]string{}
	data := &bindData{values: make(map[string][]string), mode: bindMap}
	require.NoError(t, data.bind("query", out, "user[name]", "john", false, true))
	require.NoError(t, data.bindAll("query", out, "user[tags]", []string{"a", "b"}, false, true))
	// A source without the notation files the key as it came.
	require.NoError(t, data.bind("query", out, "user[age]", "7", false, false))
	require.Equal(t, map[string][]string{"user.name": {"john"}, "user.tags": {"a", "b"}, "user[age]": {"7"}}, data.values)

	files := make(map[string][]*multipart.FileHeader)
	headers := []*multipart.FileHeader{{Filename: "file1.txt"}, {Filename: "file2.txt"}}
	require.NoError(t, bindFiles(files, "files", headers))
	require.NoError(t, bindFiles(files, "user[avatars]", headers[:1]))
	require.Equal(t, map[string][]*multipart.FileHeader{"files": headers, "user.avatars": headers[:1]}, files)

	require.EqualError(t, data.bind("query", out, "invalid[", "value", false, true), "unmatched brackets")
	require.EqualError(t, data.bindAll("query", out, "invalid[", []string{"value"}, false, true), "unmatched brackets")
	require.EqualError(t, bindFiles(files, "invalid[", headers), "unmatched brackets")
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
	cache.types.Store(typ, 1)
	cache.slot(typ).Store(nil)
	defer cache.types.Delete(typ)
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

// Types of their own, so poisoning the cache entry below cannot reach a type
// another test binds.
type mismatchInner struct {
	Tags []string `query:"tags"`
}

type mismatchOuter struct {
	Inner mismatchInner `query:"inner"`
}

func Test_StructKeyKind_NestedCacheTypeMismatch(t *testing.T) {
	t.Parallel()

	cache := getFieldCache("query")
	typ := reflect.TypeFor[mismatchInner]()
	cache.types.Store(typ, 1)
	cache.slot(typ).Store(nil)
	t.Cleanup(func() { cache.types.Delete(typ) })

	var out mismatchOuter
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

// Test_BindData_AlternatingKeys pins that keys whose values alternate cost
// time and memory linear in the number of pairs. An arena that copied a key's
// values to its end whenever another key had followed it once grew for a
// 5,000-pair body of two alternating keys to over four million slots.
func Test_BindData_AlternatingKeys(t *testing.T) {
	t.Parallel()

	const pairs = 5000
	data := &bindData{values: make(map[string][]string), mode: bindMap}
	want := make(map[string][]string)
	for i := range pairs {
		key := "ab"[i%2 : i%2+1]
		value := strconv.Itoa(i)
		data.add(key, value)
		want[key] = append(want[key], value)
	}
	require.Equal(t, want, data.values)
	// Each key's values grow as append grows any slice.
	for key, values := range data.values {
		require.LessOrEqual(t, cap(values), 2*len(values), "values of %q", key)
	}
}

// Test_Bind_MapOfSlices_OwnsValues pins that a map-of-slices destination,
// which keeps the value slices it is given, gets slices of its own: a later
// bind reusing the pool must leave its values as they were bound.
func Test_Bind_MapOfSlices_OwnsValues(t *testing.T) {
	t.Parallel()

	require.Equal(t, bindMap, bindModeFor(&map[string][]string{}))
	require.Equal(t, bindMap, bindModeFor(map[string][]string{}))
	type named map[string][]string
	require.Equal(t, bindMap, bindModeFor(&named{}))
	require.Equal(t, bindMap, bindModeFor(&map[string]any{}))
	require.Equal(t, bindLast, bindModeFor(&map[string]string{}))
	require.Equal(t, bindLast, bindModeFor(map[string]string{}))
	type namedStrings map[string]string
	require.Equal(t, bindMap, bindModeFor(namedStrings{}))
	require.Equal(t, bindPairs, bindModeFor(&struct{ A []string }{}))
	require.Equal(t, bindPairs, bindModeFor(&map[int]string{}), "parse decodes a map without string keys as a struct")
	require.Equal(t, bindPairs, bindModeFor(nil))

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	req.URI().SetQueryString("a=1&b=3&a=2")

	dst := make(map[string][]string)
	require.NoError(t, (&QueryBinding{}).Bind(req, &dst))
	require.Equal(t, map[string][]string{"a": {"1", "2"}, "b": {"3"}}, dst)

	// Bind again through the same pools, from another request: the values
	// are views of their request's buffer, so reusing req would rewrite them.
	other := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(other) })
	other.URI().SetQueryString("a=overwritten&b=overwritten&a=overwritten")
	var into struct {
		B string   `query:"b"`
		A []string `query:"a"`
	}
	require.NoError(t, (&QueryBinding{}).Bind(other, &into))
	intoMap := make(map[string]string)
	require.NoError(t, (&QueryBinding{}).Bind(other, &intoMap))

	require.Equal(t, map[string][]string{"a": {"1", "2"}, "b": {"3"}}, dst)
}

// Test_Bind_StringMap_KeepsLastValue pins that a map[string]string, for which
// the binders keep only the last value filed under each key, gets from each
// of them the last value a map of slices gets under that key, unsplit, and
// that the entries it already held stay unless overwritten.
func Test_Bind_StringMap_KeepsLastValue(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"a", "1"}, {"b", "2"}, {"a", "3"}, {"c", ""}, {"b", "4,5"}, {"a", "6"}}
	encoded := make([]string, 0, len(pairs))
	for _, p := range pairs {
		encoded = append(encoded, p[0]+"="+p[1])
	}
	query := strings.Join(encoded, "&")

	var multipartBody bytes.Buffer
	mw := multipart.NewWriter(&multipartBody)
	for _, p := range pairs {
		require.NoError(t, mw.WriteField(p[0], p[1]))
	}
	require.NoError(t, mw.Close())

	binders := []struct {
		bind func(split bool, out any) error
		name string
	}{
		{name: "query", bind: func(split bool, out any) error {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			req.URI().SetQueryString(query)
			return (&QueryBinding{EnableSplitting: split}).Bind(req, out)
		}},
		{name: "form", bind: func(split bool, out any) error {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			req.Header.SetContentType("application/x-www-form-urlencoded")
			req.SetBodyString(query)
			return (&FormBinding{EnableSplitting: split}).Bind(req, out)
		}},
		{name: "multipart", bind: func(split bool, out any) error {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			req.Header.SetContentType(mw.FormDataContentType())
			req.SetBody(multipartBody.Bytes())
			return (&FormBinding{EnableSplitting: split}).Bind(req, out)
		}},
		{name: "header", bind: func(split bool, out any) error {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			for _, p := range pairs {
				req.Header.Add(p[0], p[1])
			}
			return (&HeaderBinding{EnableSplitting: split}).Bind(req, out)
		}},
		{name: "resp_header", bind: func(split bool, out any) error {
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)
			for _, p := range pairs {
				resp.Header.Add(p[0], p[1])
			}
			return (&RespHeaderBinding{EnableSplitting: split}).Bind(resp, out)
		}},
		{name: "cookie", bind: func(split bool, out any) error {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			req.Header.Set(fasthttp.HeaderCookie, strings.ReplaceAll(query, "&", "; "))
			return (&CookieBinding{EnableSplitting: split}).Bind(req, out)
		}},
	}
	for _, b := range binders {
		all := make(map[string][]string)
		require.NoError(t, b.bind(false, &all), b.name)
		want := map[string]string{"kept": "yes"}
		for key, values := range all {
			want[key] = values[len(values)-1]
		}
		require.Contains(t, slices.Collect(maps.Values(want)), "4,5", b.name)

		for _, split := range []bool{false, true} {
			got := map[string]string{"kept": "yes"}
			require.NoError(t, b.bind(split, &got), "%s split=%v", b.name, split)
			require.Equal(t, want, got, "%s split=%v", b.name, split)
		}
	}
}

// Test_Bind_StringMap_Destinations pins how a bind into a map[string]string
// treats its destination, as parseToMap and parse do: a map given by value is
// filled, a nil one given by value is an error, a pointer to a nil map gets
// one, and a nil pointer fails as a nil pointer to a map of slices does.
func Test_Bind_StringMap_Destinations(t *testing.T) {
	t.Parallel()

	req := fasthttp.AcquireRequest()
	t.Cleanup(func() { fasthttp.ReleaseRequest(req) })
	req.URI().SetQueryString("a=1&a=2&b=3")
	b := &QueryBinding{}
	want := map[string]string{"a": "2", "b": "3"}

	byValue := map[string]string{}
	require.NoError(t, b.Bind(req, byValue))
	require.Equal(t, want, byValue)

	var nilMap map[string]string
	require.ErrorIs(t, b.Bind(req, nilMap), ErrMapNilDestination)

	var viaPointer map[string]string
	require.NoError(t, b.Bind(req, &viaPointer))
	require.Equal(t, want, viaPointer)

	var nilPointer *map[string]string
	err := b.Bind(req, nilPointer)
	require.Error(t, err)
	var nilSlicesPointer *map[string][]string
	require.EqualError(t, b.Bind(req, nilSlicesPointer), err.Error())
}

// Test_tagIndex_MatchesTags pins tagIndex's switch to the order of tags, which
// is how getDecoderPool finds a tag's pool in a decoderPoolSet.
func Test_tagIndex_MatchesTags(t *testing.T) {
	t.Parallel()

	for i, tag := range tags {
		require.Equal(t, i, tagIndex(tag), tag)
		require.NotNil(t, getDecoderPool(tag), tag)
	}
	require.Equal(t, -1, tagIndex("unknown"))
	require.Panics(t, func() { getDecoderPool("unknown") })
}
