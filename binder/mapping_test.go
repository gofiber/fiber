package binder

import (
	"fmt"
	"maps"
	"mime/multipart"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
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
		{err: nil, input: "foo[bar]", expected: "foo.bar"},
		{err: nil, input: "foo[bar][baz]", expected: "foo.bar.baz"},
		{err: ErrUnmatchedBrackets, input: "foo[bar", expected: ""},
		{err: ErrUnmatchedBrackets, input: "foo[bar][baz", expected: ""},
		{err: ErrUnmatchedBrackets, input: "foo]bar[", expected: ""},
		{err: nil, input: "foo[bar[baz]]", expected: "foo.bar.baz"},
		{err: nil, input: "", expected: ""},
		{err: nil, input: "[]", expected: ""},
		{err: nil, input: "foo[]", expected: "foo"},
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
	input := map[string][]string{"key1": {"value1", "value2"}, "key2": {"value3"}, "key3": {"value4"}}
	withEmpty := map[string][]string{"empty": {}, "key1": {"value1"}}

	for _, tc := range []struct {
		dst  any // a map, or a pointer to a nil one
		data map[string][]string
		want any
		err  error
		name string
	}{
		{
			name: "string map keeps the last value",
			dst:  map[string]string{}, data: input,
			want: map[string]string{"key1": "value2", "key2": "value3", "key3": "value4"},
		},
		{name: "slice map keeps every value", dst: map[string][]string{}, data: input, want: input},
		{name: "any map is left alone", dst: map[string]any{}, data: input, want: map[string]any{}},
		{
			name: "nil string map is made",
			dst:  new(map[string]string), data: input,
			want: map[string]string{"key1": "value2", "key2": "value3", "key3": "value4"},
		},
		{name: "nil slice map is made", dst: new(map[string][]string), data: input, want: input},
		{name: "nil map by value", dst: map[string]string(nil), data: input, err: ErrMapNilDestination},
		{name: "int map is left alone", dst: map[string]int{}, data: input, want: map[string]int{}},
		{name: "int slice map is not convertible", dst: map[string][]int{}, data: withEmpty, err: ErrMapNotConvertible},
		{
			name: "string map holds a key without values as empty",
			dst:  map[string]string{}, data: withEmpty, want: map[string]string{"empty": "", "key1": "value1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dst := reflect.ValueOf(tc.dst)
			if dst.Kind() == reflect.Pointer {
				dst = dst.Elem()
			}
			if tc.err != nil {
				require.ErrorIs(t, parseToMap(dst, tc.data), tc.err)
				return
			}
			require.NoError(t, parseToMap(dst, tc.data))
			require.Equal(t, tc.want, dst.Interface())
		})
	}
}

func Test_FilterFlags(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{input: "text/javascript; charset=utf-8", expected: "text/javascript"},
		{input: "text/javascript", expected: "text/javascript"},

		{input: "text/javascript; charset=utf-8; foo=bar", expected: "text/javascript"},
		{input: "text/javascript charset=utf-8", expected: "text/javascript"},
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
		"text/javascript; charset=utf-8", "application/json",
		"text/plain; charset=utf-8; foo=bar", "text/javascript charset=utf-8",
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

	data := map[string][]string{"name": {"John"}, "age": {"invalidAge"}}
	require.EqualError(t, parseToStruct("query", &User{}, data), "schema: error converting value for \"age\"")
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
		SetAliasTag:       "custom",
		ParserType:        []ParserType{{CustomType: customInt(0), Converter: conv}},
		IgnoreUnknownKeys: false,
		ZeroEmpty:         false,
	}
	decAny := decoderBuilder(bindingForm, parserConfig)
	dec, ok := decAny.(*pooledDecoder)
	require.True(t, ok)
	var out struct {
		X customInt `custom:"x"`
	}
	require.NoError(t, dec.Decode(&out, map[string][]string{"x": {"7"}}))
	require.Equal(t, customInt(7), out.X)
}

func Test_decoderPoolMapInit(t *testing.T) {
	t.Parallel()
	for _, tag := range tags {
		decAny := getDecoderPool(tag).Get()
		dec, ok := decAny.(*pooledDecoder)
		require.True(t, ok)
		require.NotNil(t, dec)
		getDecoderPool(tag).Put(decAny)
	}
}

func TestSetParserDecoderConcurrentAccess(t *testing.T) {
	t.Parallel()
	t.Cleanup(func() { SetParserDecoder(ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true}) })

	type queryUser struct {
		Name string `query:"name"`
	}

	data := map[string][]string{"name": {"fiber"}}
	parserConfig := ParserConfig{IgnoreUnknownKeys: true, ZeroEmpty: true}

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
	for _, tag := range []string{"header", "respHeader", "cookie", "form", "uri", "query"} {
		require.NotNil(t, getFieldCache(tag), tag)
	}
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
		{key: "lookup.anything.tags", kind: reflect.Slice, want: false},
		{key: "title.sub", kind: reflect.String, want: false},
		{key: "deep.k.0", kind: reflect.Slice, want: true},
		{key: "", kind: reflect.String, want: false},
		{key: "unknown.tags", kind: reflect.Slice, want: true},
		{key: "unknown.tags", kind: reflect.Bool, want: false},
	}

	var out walkTarget
	for _, tc := range tests {
		require.Equal(t, tc.want, equalFieldType(&out, tc.kind, tc.key, "query"), "key=%q kind=%s", tc.key, tc.kind)
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

	req := newQueryRequest(t, "a=1&b=3&a=2")

	dst := make(map[string][]string)
	require.NoError(t, (&QueryBinding{}).Bind(req, &dst))
	require.Equal(t, map[string][]string{"a": {"1", "2"}, "b": {"3"}}, dst)

	// Bind again through the same pools, from another request: the values
	// are views of their request's buffer, so reusing req would rewrite them.
	other := newQueryRequest(t, "a=overwritten&b=overwritten&a=overwritten")
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
	const input = "a=1&b=2&a=3&c=&b=4,5&a=6"

	for _, src := range pairSources {
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			all := make(map[string][]string)
			require.NoError(t, src.bind(t, input, false, &all))
			want := map[string]string{"kept": "yes"}
			for key, values := range all {
				want[key] = values[len(values)-1]
			}
			require.Contains(t, slices.Collect(maps.Values(want)), "4,5")

			for _, split := range []bool{false, true} {
				got := map[string]string{"kept": "yes"}
				require.NoError(t, src.bind(t, input, split, &got), "split=%v", split)
				require.Equal(t, want, got, "split=%v", split)
			}
		})
	}
}

// Test_Bind_StringMap_Destinations pins how a bind into a map[string]string
// treats its destination, as parseToMap and parse do: a map given by value is
// filled, a nil one given by value is an error, a pointer to a nil map gets
// one, and a nil pointer fails as a nil pointer to a map of slices does.
func Test_Bind_StringMap_Destinations(t *testing.T) {
	t.Parallel()
	req := newQueryRequest(t, "a=1&a=2&b=3")
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

func Test_CheckKeyDepth(t *testing.T) {
	t.Parallel()
	atLimit := strings.Repeat("a.", maxKeySegments-1) + "a"
	require.NoError(t, checkKeyDepth(atLimit))
	require.ErrorIs(t, checkKeyDepth(atLimit+".a"), ErrKeyTooDeep)
	require.ErrorIs(t, checkKeyDepth(strings.Repeat(".", maxKeySegments)), ErrKeyTooDeep)

	key, err := bindKey(atLimit+"[a]", true)
	require.NoError(t, err)
	require.ErrorIs(t, checkKeyDepth(key), ErrKeyTooDeep)
}

// The decoding cost of these types grows with the depth of a key.
type depthTree struct {
	Name     string
	Children []depthTree
}

type depthChain struct {
	Parent *depthChain `form:"parent"`
	Text   string      `form:"text,default:none"`
}

// Two self pointers: every index-free chain reaches a fresh phones slice.
type fanOutPerson struct {
	Mother *fanOutPerson `form:"mother"`
	Father *fanOutPerson `form:"father"`
	Phones []fanOutPhone `form:"phones"`
}

type fanOutPhone struct {
	Number string `form:"number"`
}

type stockVariant struct {
	Stock map[string]int `form:"stock"`
	SKU   string         `form:"sku"`
}

type stockProduct struct {
	Variants []stockVariant `form:"variants"`
}

type depthRequired struct {
	A string `form:"a,required"`
	B string `form:"b,required"`
	C string `form:"c,required"`
	D string `form:"d,required"`
	E string `form:"e,required"`
	F string `form:"f,required"`
	G string `form:"g,required"`
	H string `form:"h,required"`
	I string `form:"i,required"`
}

// bindFormBody binds a urlencoded body into out with a default form binder.
func bindFormBody(t *testing.T, body string, out any) error {
	t.Helper()

	return (&FormBinding{}).Bind(newFormRequest(t, body), out)
}

func Test_FormBinder_KeyDepthLimit(t *testing.T) {
	t.Parallel()

	// "children.0." is two segments, so these keys have 63 and 65.
	const levels = (maxKeySegments - 1) / 2

	t.Run("self-referencing slice", func(t *testing.T) {
		t.Parallel()
		root := new(depthTree)
		require.NoError(t, bindFormBody(t, strings.Repeat("children.0.", levels)+"name=x", root))
		leaf := root
		for range levels {
			require.Len(t, leaf.Children, 1)
			leaf = &leaf.Children[0]
		}
		require.Equal(t, "x", leaf.Name)
		require.ErrorIs(t, bindFormBody(t, strings.Repeat("children.0.", levels+1)+"name=x", new(depthTree)), ErrKeyTooDeep)

		deep := strings.Repeat("children.0.", levels+1) + "name"
		m := make(map[string]string)
		require.NoError(t, bindFormBody(t, deep+"=x", &m))
		require.Equal(t, map[string]string{deep: "x"}, m)
	})

	t.Run("self-referencing pointer with a default", func(t *testing.T) {
		t.Parallel()
		root := new(depthChain)
		require.NoError(t, bindFormBody(t, strings.Repeat("parent.", maxKeySegments-1)+"text=x", root))
		leaf := root
		for range maxKeySegments - 1 {
			require.NotNil(t, leaf.Parent)
			leaf = leaf.Parent
		}
		require.Equal(t, "x", leaf.Text)
		require.ErrorIs(t, bindFormBody(t, strings.Repeat("parent.", maxKeySegments)+"text=x", new(depthChain)), ErrKeyTooDeep)
	})

	t.Run("required keys left out", func(t *testing.T) {
		t.Parallel()
		err := bindFormBody(t, strings.Repeat("x.", maxKeySegments-1)+"x=1", new(depthRequired))
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrKeyTooDeep)
		require.ErrorIs(t, bindFormBody(t, strings.Repeat("x.", maxKeySegments)+"x=1", new(depthRequired)), ErrKeyTooDeep)
	})
}

func Test_Binders_KeyDepthLimit(t *testing.T) {
	t.Parallel()
	const levels = (maxKeySegments - 1) / 2
	dotted := func(n int) string { return strings.Repeat("children.0.", n) + "name" }
	// The query string takes the key in brackets, which are rewritten to dots before its depth is checked.
	bracketed := func(n int) string { return "children" + strings.Repeat("[0][children]", n-1) + "[0][name]" }
	// A file part carries its content as a file instead of as a value.
	file := source{name: "multipart file", bind: func(t *testing.T, urlEncoded string, _ bool, out any) error {
		t.Helper()

		key, _, _ := strings.Cut(urlEncoded, "=")
		req := newMultipartRequestWith(t, func(mw *multipart.Writer) { writeFormFile(t, mw, key, "x.txt", "x") })
		return (&FormBinding{}).Bind(req, out)
	}}

	for _, tc := range []struct {
		key func(n int) string
		src source
	}{
		{src: pairSources[2], key: dotted}, // multipart
		{src: file, key: dotted},
		{src: pairSources[0], key: bracketed}, // query
		{src: pairSources[3], key: dotted},    // header
		{src: pairSources[5], key: dotted},    // cookie
	} {
		t.Run(tc.src.name, func(t *testing.T) {
			t.Parallel()
			bind := func(n int, out any) error { return tc.src.bind(t, tc.key(n)+"=x", false, out) }

			require.NoError(t, bind(levels, new(depthTree)))
			require.ErrorIs(t, bind(levels+1, new(depthTree)), ErrKeyTooDeep)

			// A map destination is filled without the decoder, so its keys are not limited.
			m := make(map[string][]string)
			require.NoError(t, bind(levels+1, &m))
			if tc.src.name != file.name {
				require.Len(t, m, 1)
			}
		})
	}
}

// newFormDecoder returns a form decoder as the binders pool it, to admit keys to directly.
func newFormDecoder(t *testing.T, cfg ParserConfig) *pooledDecoder {
	t.Helper()

	d, ok := decoderBuilder(bindingForm, cfg).(*pooledDecoder)
	require.True(t, ok)
	return d
}

func Test_PooledDecoder_KeyBudget(t *testing.T) {
	t.Parallel()
	type item struct {
		SKU string `form:"sku"`
	}
	type order struct {
		Items []item `form:"items"`
	}
	type cart struct {
		Items []item `form:"items"`
	}

	t.Run("repeated keys", func(t *testing.T) {
		t.Parallel()
		d := newFormDecoder(t, ParserConfig{})
		for range 10_000 {
			require.NoError(t, d.admit(new(order), "items.0.sku"))
			require.NoError(t, d.admit(new(order), "name"))
		}
		require.Equal(t, cachedKeyBytes("items.0.sku"), d.keyBytes)
	})

	t.Run("per destination type", func(t *testing.T) {
		t.Parallel()
		d := newFormDecoder(t, ParserConfig{})
		for _, out := range []any{new(order), new(cart), new(order)} {
			require.NoError(t, d.admit(out, "items.0.sku"))
			putDecoder(&sync.Pool{}, d)
		}
		require.Equal(t, 2*cachedKeyBytes("items.0.sku"), d.keyBytes)
	})

	t.Run("table form stays pooled", func(t *testing.T) {
		t.Parallel()
		d := newFormDecoder(t, ParserConfig{})
		out := new(order)
		for row := range 400 {
			for field := range 10 {
				require.NoError(t, d.admit(out, "items."+strconv.Itoa(row)+".field"+strconv.Itoa(field)))
			}
		}
		require.LessOrEqual(t, d.keyBytes, maxDecoderKeyBytes)
	})

	// Every key is a new spelling of one of 1000 items, as a client choosing them would send.
	perBind := maxDecoderKeyBytes/cachedKeyBytes("items.0.sku") + 1
	keys := make([]string, 0, perBind)
	for n := range perBind {
		keys = append(keys, "items."+strings.Repeat("0", n/1000)+strconv.Itoa(n%1000)+".sku")
	}
	values := slices.Repeat([]string{"a"}, len(keys))
	data := make(map[string][]string, len(keys))
	files := make(map[string][]*multipart.FileHeader, len(keys))
	for _, key := range keys {
		data[key] = []string{"a"}
		files[key] = nil
	}

	for name, decode := range map[string]func(out any) error{
		"pairs": func(out any) error { return parseValuesToStruct(bindingForm, out, keys, values) },
		"map":   func(out any) error { return parseToStruct(bindingForm, out, data) },
		"files": func(out any) error { return parseToStruct(bindingForm, out, nil, files) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pool := getDecoderPool(bindingForm)
			// Under the race detector a sync.Pool drops some of what it is given.
			for range 8 {
				out := new(order)
				require.NoError(t, decode(out))
				require.Len(t, out.Items, 1000)
				dec, ok := pool.Get().(*pooledDecoder)
				require.True(t, ok)
				require.LessOrEqual(t, dec.keyBytes, maxDecoderKeyBytes)
				pool.Put(dec)
			}
		})
	}
}

// respell sets the case of the letters of key from the bits of n.
func respell(key string, n int) string {
	b := []byte(key)
	for i, bit := 0, 0; i < len(b); i++ {
		if b[i] >= 'a' && b[i] <= 'z' {
			if n&(1<<bit) != 0 {
				b[i] -= 'a' - 'A'
			}
			bit++
		}
	}
	return string(b)
}

func liveHeap() uint64 {
	// revive:disable-next-line:call-to-gc // only a collection leaves the live heap to read
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// Sequential: it reads the live heap, which parallel tests would disturb.
func Test_CachedKeyBytes_CoversSchemaCache(t *testing.T) {
	type chain struct {
		Parent *chain `form:"parent"`
		Text   string `form:"text"`
	}
	type item struct {
		SKU string `form:"sku"`
	}
	type order struct {
		Items []item `form:"items"`
	}

	for name, tc := range map[string]struct {
		out func() any
		key func(n int) string
	}{
		"shallow": {
			out: func() any { return new(order) },
			key: func(n int) string { return respell("items."+strconv.Itoa(n%1000)+".sku", n/1000) },
		},
		"deep": {
			out: func() any { return new(chain) },
			key: func(n int) string { return respell(strings.Repeat("parent.", maxKeySegments-1)+"text", n) },
		},
	} {
		d := newFormDecoder(t, ParserConfig{IgnoreUnknownKeys: true})
		decode := func(key string) {
			require.NoError(t, d.admit(tc.out(), key))
			require.NoError(t, d.DecodeValues(tc.out(), []string{key}, []string{"x"}))
			putDecoder(&sync.Pool{}, d)
		}
		decode(tc.key(0))
		charged, before := d.keyBytes, liveHeap()
		for n := 1; n <= 500; n++ {
			decode(tc.key(n))
		}
		require.LessOrEqual(t, int64(liveHeap())-int64(before), int64(d.keyBytes-charged), name)
		runtime.KeepAlive(d)
	}
}

func Test_BindShape_Elements(t *testing.T) {
	t.Parallel()
	acyclic := bindShape{elemSize: 1}
	cyclic := bindShape{elemSize: 1, cyclic: true}
	for key, want := range map[string][2]int{
		"name":                      {0, 0},
		"items.5.sku":               {0, 0},
		"items.5.variants.7.sku":    {8, 8},
		"items.0.variants.+3.sku":   {4, 4},
		"items.0.variants.-0.sku":   {1, 1},
		"items.0.variants.-1.sku":   {0, 0},
		"items.0.variants.3x.sku":   {0, 0},
		"items.0.variants.5000.sku": {maxSliceIndex + 1, maxSliceIndex + 1},
		"attrs.7":                   {0, 0},
		"5.x.6":                     {0, 7},
		"a..0.b.2":                  {3, 4},
		"billing.lines.5.sku":       {0, 6},

		"mother.father.phones.1000.number":            {0, maxSliceIndex + 1},
		"children.0.children.1000.children.1000.name": {2 * (maxSliceIndex + 1), 2 * (maxSliceIndex + 1)},
	} {
		require.Equal(t, want[0], acyclic.elements(key), key)
		require.Equal(t, want[1], cyclic.elements(key), key)
	}
	require.Zero(t, bindShape{cyclic: true}.elements("a.b.0.c.5"))

	prefixes := map[string]int{}
	for end, n := range cyclic.indexes("a.b.10.c.5.d") {
		prefixes["a.b.10.c.5.d"[:end]] = n
	}
	require.Equal(t, map[string]int{"a.b.10": 11, "a.b.10.c.5": 6}, prefixes)
}

func Test_ShapeOf(t *testing.T) {
	t.Parallel()
	type item struct{ SKU string }
	type group struct{ Items []item }
	itemSize := reflect.TypeFor[item]().Size()
	for name, tc := range map[string]struct {
		typ  reflect.Type
		want bindShape
	}{
		"slice in a slice element":  {reflect.TypeFor[*struct{ Groups []group }](), bindShape{elemSize: itemSize}},
		"pointer elements":          {reflect.TypeFor[*struct{ Groups []struct{ Items []*item } }](), bindShape{elemSize: reflect.TypeFor[*item]().Size() + itemSize}},
		"slice in an array element": {reflect.TypeFor[*struct{ Groups [][2]group }](), bindShape{elemSize: itemSize}},
		"recursive slice":           {reflect.TypeFor[depthTree](), bindShape{elemSize: reflect.TypeFor[depthTree]().Size()}},
		"recursive pointers":        {reflect.TypeFor[*fanOutPerson](), bindShape{elemSize: reflect.TypeFor[fanOutPhone]().Size(), cyclic: true}},
		"recursive without slices":  {reflect.TypeFor[*depthChain](), bindShape{cyclic: true}},
		"root slice":                {reflect.TypeFor[*struct{ Items []item }](), bindShape{}},
		"slice in a struct field":   {reflect.TypeFor[*struct{ Billing, Shipping *group }](), bindShape{}},
		"map in a slice element":    {reflect.TypeFor[*stockProduct](), bindShape{}},
		"slice in a map value":      {reflect.TypeFor[*struct{ M map[string]group }](), bindShape{}},
		"not a struct":              {reflect.TypeFor[*map[string]string](), bindShape{}},
		"nil":                       {nil, bindShape{}},
	} {
		require.Equal(t, tc.want, shapeOf(tc.typ), name)
	}
}

// keysThatFit returns how many keys fit in maxNestedBytes when each grows perKey slices of maxSliceIndex+1 Ts.
func keysThatFit[T any](perKey int) int {
	return maxNestedBytes / (perKey * (maxSliceIndex + 1) * int(reflect.TypeFor[T]().Size()))
}

func Test_PooledDecoder_ElementBudget(t *testing.T) {
	t.Parallel()

	// Each key grows a depthTree slice of 1001 elements below its first index.
	fit := keysThatFit[depthTree](1)
	keys := make([]string, 0, fit+1)
	for i := range fit + 1 {
		keys = append(keys, "children."+strconv.Itoa(i)+".children.1000.name")
	}
	out := new(depthTree)
	d := newFormDecoder(t, ParserConfig{})
	for _, key := range keys[:fit] {
		require.NoError(t, d.admit(out, key))
	}
	require.NoError(t, d.checkElements(slices.Values(keys[:fit])))
	require.NoError(t, d.admit(out, keys[fit]))
	require.ErrorIs(t, d.checkElements(slices.Values(keys)), ErrTooManyElements)

	// The budget is per bind.
	putDecoder(&sync.Pool{}, d)
	require.NoError(t, d.admit(out, keys[0]))
	require.NoError(t, d.checkElements(slices.Values(keys[:1])))
	putDecoder(&sync.Pool{}, d)

	// The fields of one element share its index.
	var fields []string
	for i := range fit {
		for _, f := range []string{"name", "a", "b", "c"} {
			fields = append(fields, "children."+strconv.Itoa(i)+".children.1000."+f)
		}
	}
	for _, key := range fields {
		require.NoError(t, d.admit(out, key))
	}
	require.Greater(t, d.elements, maxNestedBytes/int(d.bound.shape.elemSize))
	require.NoError(t, d.checkElements(slices.Values(fields)))
}

type catalogVariant struct {
	SKU      string `form:"sku"`
	Name     string `form:"name"`
	Color    string `form:"color"`
	Size     string `form:"size"`
	Material string `form:"material"`
	Pattern  string `form:"pattern"`
	Brand    string `form:"brand"`
	Model    string `form:"model"`
	EAN      string `form:"ean"`
	UPC      string `form:"upc"`
}

var catalogFields = []string{"sku", "name", "color", "size", "material", "pattern", "brand", "model", "ean", "upc"}

type catalogLine struct {
	Variants []catalogVariant `form:"variants"`
}

type catalog struct {
	Lines []catalogLine `form:"lines"`
}

func Test_FormBinder_NestedElementLimit(t *testing.T) {
	t.Parallel()
	root := new(depthTree)
	require.NoError(t, bindFormBody(t, "children.1.children.2.name=x", root))
	require.Len(t, root.Children, 2)
	require.Len(t, root.Children[1].Children, 3)
	require.Equal(t, "x", root.Children[1].Children[2].Name)

	// Each key grows 30 slices of 1001 elements below its first index.
	keys := keysThatFit[depthTree](30) + 1
	pairs := make([]string, 0, keys)
	for k := range keys {
		pairs = append(pairs, "children."+strconv.Itoa(k)+strings.Repeat(".children.1000", 30)+".name=x")
	}
	body := strings.Join(pairs, "&")
	require.ErrorIs(t, bindFormBody(t, body, new(depthTree)), ErrTooManyElements)

	require.ErrorIs(t, (&QueryBinding{}).Bind(newQueryRequest(t, body), new(depthTree)), ErrTooManyElements)

	person := new(fanOutPerson)
	require.NoError(t, bindFormBody(t, "mother.father.phones.1.number=x", person))
	require.Len(t, person.Mother.Father.Phones, 2)
	require.Equal(t, "x", person.Mother.Father.Phones[1].Number)

	chains := make([]string, 0, keysThatFit[fanOutPhone](1)+1)
	for i := 2; len(chains) < cap(chains); i++ {
		var hops []string
		for j := i; j > 1; j >>= 1 {
			hops = append(hops, [...]string{"mother", "father"}[j&1])
		}
		chains = append(chains, strings.Join(hops, ".")+".phones.1000.number=x")
	}
	require.ErrorIs(t, bindFormBody(t, strings.Join(chains, "&"), new(fanOutPerson)), ErrTooManyElements)

	// Numeric map keys past the budget: stockProduct grows no slice per element.
	stock := make([]string, 0, 300)
	for v := range 30 {
		for w := 1000; w < 1010; w++ {
			stock = append(stock, "variants["+strconv.Itoa(v)+"][stock]["+strconv.Itoa(w)+"]=5")
		}
	}
	product := new(stockProduct)
	require.NoError(t, bindFormBody(t, strings.Join(stock, "&"), product))
	require.Len(t, product.Variants, 30)
	require.Equal(t, 5, product.Variants[29].Stock["1009"])
}

func Test_FormBinder_NestedElementBytes(t *testing.T) {
	t.Parallel()
	variant := func(line, v int, field string) string {
		return "lines[" + strconv.Itoa(line) + "][variants][" + strconv.Itoa(v) + "][" + field + "]=x"
	}

	t.Run("large elements", func(t *testing.T) {
		t.Parallel()
		fit := keysThatFit[catalogVariant](1)
		pairs := make([]string, 0, fit+1)
		for l := range fit + 1 {
			pairs = append(pairs, variant(l, maxSliceIndex, "sku"))
		}
		out := new(catalog)
		require.NoError(t, bindFormBody(t, strings.Join(pairs[:fit], "&"), out))
		require.Len(t, out.Lines, fit)
		require.Len(t, out.Lines[fit-1].Variants, maxSliceIndex+1)
		require.ErrorIs(t, bindFormBody(t, strings.Join(pairs, "&"), new(catalog)), ErrTooManyElements)
	})

	t.Run("fields of one element", func(t *testing.T) {
		t.Parallel()
		pairs := make([]string, 0, 30*50*len(catalogFields))
		for l := range 30 {
			for v := range 50 {
				for _, f := range catalogFields {
					pairs = append(pairs, variant(l, v, f))
				}
			}
		}
		out := new(catalog)
		require.NoError(t, bindFormBody(t, strings.Join(pairs, "&"), out))
		require.Len(t, out.Lines, 30)
		require.Len(t, out.Lines[29].Variants, 50)
		require.Equal(t, "x", out.Lines[29].Variants[49].UPC)
	})

	t.Run("first index below a root field", func(t *testing.T) {
		t.Parallel()
		type cartForm struct {
			Cart struct {
				Items []catalogVariant `form:"items"`
			} `form:"cart"`
			Groups []catalogLine `form:"groups"`
		}
		pairs := make([]string, 0, 10*(maxSliceIndex+1))
		for i := range maxSliceIndex + 1 {
			for _, f := range catalogFields {
				pairs = append(pairs, "cart[items]["+strconv.Itoa(i)+"]["+f+"]=x")
			}
		}
		out := new(cartForm)
		require.NoError(t, bindFormBody(t, strings.Join(pairs, "&"), out))
		require.Len(t, out.Cart.Items, maxSliceIndex+1)
	})
}
