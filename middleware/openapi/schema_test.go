package openapi

import (
	"encoding/json"
	"encoding/xml"
	"mime/multipart"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// rfM shortens the schema literals the tables in this file compare against.
type rfM = map[string]any

func embedStruct(embedded []any, extra ...reflect.StructField) any {
	fields := make([]reflect.StructField, 0, len(embedded)+len(extra))
	for _, value := range embedded {
		typ := reflect.TypeOf(value)
		fields = append(fields, reflect.StructField{Name: typ.Name(), Type: typ, Anonymous: true})
	}
	fields = append(fields, extra...)
	return reflect.New(reflect.StructOf(fields)).Elem().Interface()
}

func requireProps(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok, "expected properties to be map[string]any")
	return props
}

func requireRequired(t *testing.T, schema map[string]any) []string {
	t.Helper()
	req, ok := schema["required"].([]string)
	require.True(t, ok, "expected required to be []string")
	return req
}

func requireProp(t *testing.T, props map[string]any, name string) map[string]any {
	t.Helper()
	prop, ok := props[name].(map[string]any)
	require.True(t, ok, "expected property %q to be map[string]any", name)
	return prop
}

type rfSchemaAddress struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

type rfSchemaItem struct {
	Name string `json:"name"`
}

// rfSchemaFixture holds one field per independent field rule SchemaOf applies.
type rfSchemaFixture struct {
	At        time.Time       `json:"at"`
	Meta      map[string]int  `json:"meta"`
	ByInt     map[int]string  `json:"by_int"`
	Any       any             `json:"any"`
	Ptr       *string         `json:"ptr"`
	Ch        chan int        `json:"ch"`
	Fn        func() bool     `json:"fn"`
	Address   rfSchemaAddress `json:"address"`
	S         string          `json:"s"`
	Omit      string          `json:"omit,omitempty"`
	Skipped   string          `json:"-"`
	private   string          //nolint:unused // an unexported field is skipped
	FieldName string
	Name      string         `json:"name" openapi:"description:Product name,example:Widget"`
	Email     string         `json:"email" openapi:"format:email"`
	Status    string         `json:"status" openapi:"enum:active|inactive|pending"`
	Comma     string         `json:"comma" openapi:"enum:active|inactive,description:Status, including a comma"`
	Mode      string         `json:"mode" openapi:"enum:on|off"`
	Tags      []string       `json:"tags"`
	Items     []rfSchemaItem `json:"items"`
	Data      []byte         `json:"data"`
	I64       int64          `json:"i64"`
	U64       uint64         `json:"u64"`
	F64       float64        `json:"f64"`
	Price     float64        `json:"price" openapi:"example:9.99"`
	Rate      float64        `json:"rate" openapi:"enum:0.5|1.0"`
	QuotedF   float64        `json:"quoted_f,string"`
	QuotedI   int64          `json:"quoted_i,string"`
	I         int            `json:"i"`
	U         uint           `json:"u"`
	Count     int            `json:"count" openapi:"example:42"`
	Level     int            `json:"level" openapi:"enum:1|2|3"`
	Codes     [3]int         `json:"codes"`
	F32       float32        `json:"f32"`
	I32       int32          `json:"i32"`
	U32       uint32         `json:"u32"`
	FixedID   [4]byte        `json:"fixed_id"`
	I16       int16          `json:"i16"`
	U16       uint16         `json:"u16"`
	I8        int8           `json:"i8"`
	U8        uint8          `json:"u8"`
	B         bool           `json:"b"`
	Active    bool           `json:"active" openapi:"example:true"`
	Flag      bool           `json:"flag" openapi:"enum:true|false"`
	QuotedB   bool           `json:"quoted_b,string"`
}

func Test_SchemaOf_Fields(t *testing.T) {
	t.Parallel()
	schema := SchemaOf(rfSchemaFixture{})
	require.Equal(t, "object", schema["type"])
	str, integer, number := rfM{"type": "string"}, rfM{"type": "integer"}, rfM{"type": "number"}
	require.Equal(t, rfM{
		"at":      rfM{"type": "string", "format": "date-time"},
		"meta":    rfM{"type": "object", "additionalProperties": integer},
		"by_int":  rfM{"type": "object"},
		"any":     rfM{},
		"ptr":     str,
		"address": rfM{"type": "object", "properties": rfM{"street": str, "city": str}, "required": []string{"street", "city"}},
		"s":       str, "omit": str, "FieldName": str,
		"name":     rfM{"type": "string", "description": "Product name", "example": "Widget"},
		"email":    rfM{"type": "string", "format": "email"},
		"status":   rfM{"type": "string", "enum": []any{"active", "inactive", "pending"}},
		"comma":    rfM{"type": "string", "enum": []any{"active", "inactive"}, "description": "Status, including a comma"},
		"mode":     rfM{"type": "string", "enum": []any{"on", "off"}},
		"tags":     rfM{"type": "array", "items": str},
		"items":    rfM{"type": "array", "items": rfM{"type": "object", "properties": rfM{"name": str}, "required": []string{"name"}}},
		"data":     rfM{"type": "string", "format": "byte"},
		"price":    rfM{"type": "number", "example": 9.99},
		"rate":     rfM{"type": "number", "enum": []any{0.5, 1.0}},
		"count":    rfM{"type": "integer", "example": int64(42)},
		"level":    rfM{"type": "integer", "enum": []any{int64(1), int64(2), int64(3)}},
		"codes":    rfM{"type": "array", "items": integer},
		"fixed_id": rfM{"type": "array", "items": integer},
		"active":   rfM{"type": "boolean", "example": true},
		"flag":     rfM{"type": "boolean", "enum": []any{true, false}},
		"quoted_f": str, "quoted_i": str, "quoted_b": str,
		"i": integer, "i8": integer, "i16": integer, "i32": integer, "i64": integer,
		"u": integer, "u8": integer, "u16": integer, "u32": integer, "u64": integer,
		"f32": number, "f64": number, "b": rfM{"type": "boolean"},
	}, requireProps(t, schema))
	required := requireRequired(t, schema)
	require.NotContains(t, required, "ptr", "a pointer is optional")
	require.NotContains(t, required, "omit")
	require.Contains(t, required, "s")
	require.Contains(t, required, "FieldName")
}

func Test_SchemaOf_NonStructValues(t *testing.T) {
	t.Parallel()
	require.Nil(t, SchemaOf(nil))
	require.Equal(t, rfM{"type": "string"}, SchemaOf("hello"))
	require.Equal(t, rfM{"type": "integer"}, SchemaOf(42))
	require.Equal(t, rfM{"type": "boolean"}, SchemaOf(true))
	require.Equal(t, rfM{"type": "number"}, SchemaOf(3.14))
	require.Equal(t, rfM{"type": "object", "properties": rfM{"name": rfM{"type": "string"}}, "required": []string{"name"}}, SchemaOf(&rfSchemaItem{}))
	type AllOptional struct {
		A string `json:"a,omitempty"`
		B int    `json:"b,omitempty"`
	}
	require.NotContains(t, SchemaOf(AllOptional{}), "required")
}

func Test_SchemaOf_RecursiveStructs(t *testing.T) {
	t.Parallel()
	type Node struct {
		Next  *Node `json:"next"`
		Value int   `json:"value"`
	}
	props := requireProps(t, SchemaOf(Node{}))
	require.Equal(t, rfM{"type": "integer"}, requireProp(t, props, "value"))
	require.Equal(t, rfM{"type": "object"}, requireProp(t, props, "next"))

	type B struct{}
	type A struct {
		B *B `json:"b"`
	}
	type Pair struct {
		First  A `json:"first"`
		Second A `json:"second"`
	}
	props = requireProps(t, SchemaOf(Pair{}))
	for _, name := range []string{"first", "second"} {
		require.Equal(t, "object", requireProp(t, props, name)["type"])
		require.Contains(t, requireProps(t, requireProp(t, props, name)), "b")
	}
}

type unexportedBase struct {
	ID int `json:"id"`
}

func Test_SchemaOf_Embedding(t *testing.T) {
	t.Parallel()
	type Base struct {
		ID int `json:"id"`
	}
	type Extended struct {
		Name string `json:"name"`
		Base
	}
	type Inner struct {
		ID int `json:"id"`
	}
	type PtrOuter struct {
		*Inner
		Name string `json:"name"`
	}
	type ShadowInner struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	}
	type ShadowOuter struct { //nolint:govet // fieldalignment: the embedded struct must follow the parent field to exercise shadowing; betteralign:ignore
		ID int `json:"id"`
		ShadowInner
	}
	type B1 struct {
		X int `json:"x"`
	}
	type B2 struct {
		X string `json:"x"`
	}
	type DepthB struct {
		B1
		Y int `json:"y"`
	}
	type Depth struct {
		B1
		DepthB
	}
	type E1 struct {
		Val int `json:"v"`
	}
	type E2 struct {
		V string
	}
	type E3 struct {
		V bool `json:"v"`
	}
	type P2 struct { //nolint:govet // fieldalignment: embed order mirrors the documented scenario; betteralign:ignore
		E1
		E2
	}
	type User struct { //nolint:govet // fieldalignment: embed order mirrors the documented scenario; betteralign:ignore
		unexportedBase
		Name string `json:"name"`
	}
	type DiamondB struct {
		B1
	}
	type DiamondC struct {
		B1
	}
	y := func(typ reflect.Type) reflect.StructField {
		return reflect.StructField{Name: "Y", Type: typ, Tag: `json:"y"`}
	}
	str, integer := rfM{"type": "string"}, rfM{"type": "integer"}
	for _, tc := range []struct {
		value    any
		props    rfM
		name     string
		required []string
	}{
		{name: "embedded", value: Extended{}, props: rfM{"id": integer, "name": str}, required: []string{"name", "id"}},
		{name: "embedded pointer", value: PtrOuter{}, props: rfM{"id": integer, "name": str}, required: []string{"name"}},
		{name: "parent field wins", value: ShadowOuter{}, props: rfM{"id": integer, "note": str}, required: []string{"id", "note"}},
		{name: "conflict dropped", value: embedStruct([]any{B1{}, B2{}}, y(reflect.TypeFor[string]())), props: rfM{"y": str}, required: []string{"y"}},
		{name: "shallower wins", value: Depth{}, props: rfM{"x": integer, "y": integer}, required: []string{"x", "y"}},
		{name: "tagged conflict dropped", value: embedStruct([]any{E1{}, E3{}}), props: rfM{}},
		{name: "tag and name differ", value: P2{}, props: rfM{"v": integer, "V": str}, required: []string{"v", "V"}},
		{name: "unexported embedded", value: User{}, props: rfM{"id": integer, "name": str}, required: []string{"id", "name"}},
		{name: "diamond", value: embedStruct([]any{DiamondB{}, DiamondC{}}, y(reflect.TypeFor[int]())), props: rfM{"y": integer}, required: []string{"y"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema := SchemaOf(tc.value)
			require.Equal(t, tc.props, requireProps(t, schema))
			if tc.required == nil {
				require.NotContains(t, schema, "required")
				return
			}
			require.ElementsMatch(t, tc.required, requireRequired(t, schema))
		})
	}
}

func Test_SchemaOf_EmbeddedRequiredDeterministic(t *testing.T) {
	t.Parallel()
	type Base struct {
		A string `json:"a"`
		B string `json:"b"`
		C string `json:"c"`
		D string `json:"d"`
		E string `json:"e"`
	}
	type T struct {
		Base
	}
	for range 10 {
		require.Equal(t, []string{"a", "b", "c", "d", "e"}, requireRequired(t, SchemaOf(T{})))
	}
}

type customMarshaler struct {
	Hidden int `json:"hidden"`
}

func (customMarshaler) MarshalJSON() ([]byte, error) { return []byte(`"custom"`), nil }

type textID struct {
	Raw int `json:"raw"`
}

func (textID) MarshalText() ([]byte, error) { return []byte("id"), nil }

type ptrTextID struct {
	V int `json:"v"`
}

func (*ptrTextID) MarshalText() ([]byte, error) { return []byte("id"), nil } //nolint:revive // the pointer receiver is what this test exercises

func Test_SchemaOf_CustomMarshalers(t *testing.T) {
	t.Parallel()
	require.Empty(t, SchemaOf(customMarshaler{}))
	require.Equal(t, rfM{schemaKeyType: schemaTypeString}, SchemaOf(textID{}))
	type Payload struct {
		time.Time
		N int `json:"n"`
	}
	require.Empty(t, SchemaOf(Payload{}))
	type Wrapper struct {
		N     json.Number     `json:"n"`
		Stamp customMarshaler `json:"stamp"`
		P     ptrTextID       `json:"p"`
	}
	props := requireProps(t, SchemaOf(Wrapper{}))
	require.Empty(t, requireProp(t, props, "stamp"))
	require.Equal(t, rfM{schemaKeyType: schemaTypeNumber}, requireProp(t, props, "n"))
	require.Empty(t, requireProp(t, props, "p"))
}

type validateStructure struct {
	Scores  map[string]int    `json:"scores" validate:"dive,keys,min=2,endkeys,gte=1"`
	Pointer *string           `json:"pointer" validate:"omitempty,min=3"`
	Extra   map[string]string `json:"extra"`
	Nick    string            `json:"nick,omitempty" validate:"omitempty,min=3,max=9"`
	Status  string            `json:"status" validate:"oneof='in progress' done"`
	Tags    []string          `json:"tags" validate:"required,min=1,dive,min=3,max=10"`
	Levels  []int             `json:"levels" validate:"dive,lte=100"`
	Roles   []string          `json:"roles" validate:"dive,oneof=a b"`
	Needed  []string          `json:"needed,omitempty" validate:"dive,required"`
	Hash    []byte            `json:"hash" validate:"required,len=32"`
	Blob    []byte            `json:"blob" validate:"min=1,max=16"`
	Age     int               `json:"age,string" validate:"gte=18,lte=120"`
	Price   float64           `json:"price" openapi:"example:19.99,enum:0.3|1.1" validate:"min=0.7"`
}

func Test_SchemaOf_ValidateStructure(t *testing.T) {
	t.Parallel()
	schema := SchemaOf(validateStructure{})
	str := rfM{"type": "string"}
	require.Equal(t, rfM{
		// Rules before dive constrain the slice, rules after it the elements.
		"tags":   rfM{"type": "array", "minItems": int64(1), "items": rfM{"type": "string", "minLength": int64(3), "maxLength": int64(10)}},
		"levels": rfM{"type": "array", "items": rfM{"type": "integer", "maximum": int64(100)}},
		"roles":  rfM{"type": "array", "items": rfM{"type": "string", "enum": []any{"a", "b"}}},
		"needed": rfM{"type": "array", "items": str},
		// Key rules are skipped, value rules apply.
		"scores": rfM{"type": "object", "additionalProperties": rfM{"type": "integer", "minimum": int64(1)}},
		"extra":  rfM{"type": "object", "additionalProperties": str},
		// An empty value skips the rules after omitempty, but an upper bound holds for it; a non-nil pointer is validated.
		"nick":    rfM{"type": "string", "maxLength": int64(9)},
		"pointer": rfM{"type": "string", "minLength": int64(3)},
		// A quoted number is compared as a number, not a length; 32 bytes are 44 base64 characters.
		"age":    str,
		"hash":   rfM{"type": "string", "format": "byte", "minLength": int64(44), "maxLength": int64(44)},
		"blob":   rfM{"type": "string", "format": "byte", "minLength": int64(4), "maxLength": int64(24)},
		"status": rfM{"type": "string", "enum": []any{"in progress", "done"}},
		"price":  rfM{"type": "number", "minimum": 0.7, "example": 19.99, "enum": []any{0.3, 1.1}},
	}, requireProps(t, schema))
	// Required after dive is about the elements, so needed stays optional.
	require.Equal(t, []string{"scores", "extra", "status", "tags", "levels", "roles", "hash", "blob", "age", "price"}, requireRequired(t, schema))
}

func Test_splitRuleParams(t *testing.T) {
	t.Parallel()
	for in, want := range map[string][]string{
		"a b":                       {"a", "b"},
		"'red green' 'blue yellow'": {"red green", "blue yellow"},
		"'in progress'  done":       {"in progress", "done"},
		"a'b'":                      {"ab"},
	} {
		require.Equal(t, want, splitRuleParams(in), in)
	}
	require.Empty(t, splitRuleParams("  "))
}

type fileUploadForm struct {
	File  *multipart.FileHeader    `form:"file"`
	Many  *[]*multipart.FileHeader `form:"many"`
	Title string                   `form:"title"`
	Files []*multipart.FileHeader  `form:"files"`
}

func Test_SchemaOf_MultipartFileHeaderIsBinary(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Post("/up", listUsers).Accepts(fileUploadForm{}, fiber.MIMEMultipartForm)
	schemas := modelSchemas(t, modelSpec(t, app))
	require.NotContains(t, schemas, "FileHeader_form", "no component for the file header")
	props := requireMap(t, requireMap(t, schemas["fileUploadForm_form"])["properties"])

	binary := rfM{"type": "string", "format": "binary"}
	require.Equal(t, binary, requireMap(t, props["file"]))
	require.Equal(t, rfM{"type": "array", "items": binary}, requireMap(t, props["files"]))
	require.Equal(t, "array", requireMap(t, props["many"])["type"])
}

// Test_parseJSONTag_EmptyNameSkipsTheProbe is sequential on purpose: testing.AllocsPerRun panics in a parallel test.
func Test_parseJSONTag_EmptyNameSkipsTheProbe(t *testing.T) {
	field := reflect.TypeFor[struct {
		V string `json:",omitempty"` //nolint:tagliatelle // the empty name is what this test is about
	}]().Field(0)

	info := parseJSONTag(&field, tagJSON)
	require.Empty(t, info.name)
	require.True(t, info.omit)
	require.InDelta(t, 0, testing.AllocsPerRun(50, func() { _ = parseJSONTag(&field, tagJSON) }), 0, "the encoding/json probe allocates")
}

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
		{map[string]any{"type": "string"}, "regex needing a slash is not a segment pattern", `regex(\d+/x)`},
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
		{"inner angle bracket stays in the span", "<regex(^<a>$)>", "regex(^<a>$)", 0, 14},
		{"a terminator after the first close ends it", "<int>/rest>", "int", 0, 5},
		{"a class with '>' before the real close", "<regex(^[^>]+$)>?", "regex(^[^>]+$)", 0, 16},
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
	for in, want := range map[string][]string{"a;b;c": {"a", "b", "c"}, `a\;b`: {`a\;b`}, "": {""}, ";a": {"", "a"}, "a;": {"a", ""}} {
		require.Equal(t, want, splitNonEscaped(in, ';'), in)
	}
}

func Test_SplitConstraintEntry(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]parsedConstraint{
		"int":           {name: "int"},
		"range(1,10)":   {name: "range", args: []string{"1", "10"}},
		"regex((a|b)+)": {name: "regex", args: []string{"(a|b)+"}},
		"odd)name":      {name: "odd)name"},
		"  int  ":       {name: "int"},
	} {
		require.Equal(t, want, splitConstraintEntry(in), in)
	}
}

func Test_IsPlainJSONTagName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"name", "a b", "a-b_c.d", "field2", "Ünïcøde"} {
		require.True(t, isPlainJSONTagName(name), name)
	}
	require.False(t, isPlainJSONTagName(""))
	for _, name := range []string{`a\b`, `a"b`, "a'b", "a`b", "a\tb"} {
		require.Falsef(t, isPlainJSONTagName(name), "%q should take the probe path", name)
		require.Equalf(t, wireNameFor(t, name), effectiveJSONTagName(name), "probe disagrees with encoding/json for tag %q", name)
	}
}

// wireNameFor returns the key encoding/json writes for a field tagged with name, or "" for the Go field name.
func wireNameFor(t *testing.T, name string) string {
	t.Helper()
	typ := reflect.StructOf([]reflect.StructField{{Name: "Probe", Type: reflect.TypeFor[string](), Tag: reflect.StructTag("json:" + strconv.Quote(name))}})
	encoded, err := json.Marshal(reflect.New(typ).Elem().Interface())
	require.NoError(t, err)
	var decoded map[string]string
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	for key := range decoded {
		if key != "Probe" {
			return key
		}
	}
	return ""
}

type nestedLocationNamed struct {
	Page int `query:"page"`
}

type nestedLocationOuter struct {
	Items []struct {
		Name string `json:"name"`
		Page int    `query:"page"`
	} `json:"items"`
}

type nestedLocationHolder struct {
	Items []nestedLocationNamed `json:"items"`
}

type hiddenLocation struct {
	Name   string `json:"name"`
	Secret string `json:"-" query:"secret"`
	hidden string `query:"hidden"` //nolint:unused,revive // an unexported field is not in the schema
}

type customLocation struct {
	Page int `query:"page"`
}

func (customLocation) OpenAPISchema() map[string]any { return map[string]any{"type": "string"} }

type customHolder struct {
	Inner customLocation `json:"inner"`
}

func Test_OpenAPI_LocationOnlyFieldsAreFoundInNestedStructs(t *testing.T) {
	t.Parallel()
	reg := newSchemaRegistry(&Config{}, dialect{})
	require.True(t, reg.hasLocationOnlyFields(reflect.TypeFor[nestedLocationOuter]()))
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[nestedLocationHolder]()), "a named struct is its own component")
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[userResponse]()))
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[hiddenLocation]()), "a field the schema leaves out does not count")
	require.False(t, reg.hasLocationOnlyFields(reflect.TypeFor[customHolder]()))
}

func Test_OpenAPI_RequestComponentsAreEmittedOnlyWhenTheyDiffer(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Post("/a", listUsers).Accepts(nestedLocationHolder{}).Returns(fiber.StatusOK, nestedLocationHolder{})
	app.Post("/b", listUsers).Accepts(hiddenLocation{}).Returns(fiber.StatusOK, hiddenLocation{})
	app.Post("/c", listUsers).Accepts(userRequest{}).Returns(fiber.StatusOK, userRequest{})
	schemas := modelSchemas(t, modelSpec(t, app))
	require.Contains(t, schemas, "nestedLocationHolder")
	require.NotContains(t, schemas, "nestedLocationHolder_request")
	require.NotContains(t, schemas, "hiddenLocation_request")
	require.Contains(t, schemas, "userRequest_request", "a type with a query field differs as a body")
}

type directiveModel struct {
	//nolint:revive // struct-tag: a tag on the blank field is how a struct describes itself
	_       struct{} `openapi:"title:A directive model,description:Every directive,x-model:7,example:{\"qty\":4}"`
	Status  string   `json:"status" openapi:"default:open,enum:open|closed,pattern:^[a-z]+$,title:Status,x-order:1,x-group:main"`
	Comment string   `json:"comment,omitempty" openapi:"x-flags:[1,2],x-ok:true"`
	Tags    []string `json:"tags" openapi:"uniqueItems"`
	Qty     int      `json:"qty" openapi:"multipleOf:5,nullable"`
	Price   float64  `json:"price" openapi:"default:9.5"`
}

func Test_OpenAPI_TagDirectivesBeyondTheBasics(t *testing.T) {
	t.Parallel()
	schema := SchemaOf(directiveModel{})
	require.Equal(t, "A directive model", schema["title"])
	require.Equal(t, "Every directive", schema["description"])
	require.InDelta(t, 7, schema["x-model"], 0)
	require.Equal(t, map[string]any{"qty": float64(4)}, schema["example"], "an example on the blank field is JSON and replaces the assembled one")

	require.Equal(t, rfM{
		"status":  rfM{"type": "string", "default": "open", "enum": []any{"open", "closed"}, "pattern": "^[a-z]+$", "title": "Status", "x-group": "main", "x-order": 1.0},
		"comment": rfM{"type": "string", "x-flags": []any{1.0, 2.0}, "x-ok": true},
		"tags":    rfM{"type": "array", "items": rfM{"type": "string"}, "uniqueItems": true},
		"qty":     rfM{"type": "integer", "multipleOf": 5.0},
		"price":   rfM{"type": "number", "default": 9.5},
	}, requireProps(t, schema))
}

func Test_OpenAPI_NullableTagIsSpelledPerVersion(t *testing.T) {
	t.Parallel()
	qty := func(version string) map[string]any {
		spec := rfSpec(t, func(app *fiber.App) { app.Get("/a", listUsers).Returns(fiber.StatusOK, directiveModel{}) }, Config{OpenAPIVersion: version})
		return requireMap(t, requireMap(t, requireMap(t, modelSchemas(t, spec)["directiveModel"])["properties"])["qty"])
	}
	require.Equal(t, []any{"integer", "null"}, qty("3.1.0")["type"])
	require.Equal(t, map[string]any{"type": "integer", "nullable": true, "multipleOf": float64(5)}, qty("3.0.3"))
}

type duration struct{ time.Duration }

type decimal struct{ _ int64 }

func (decimal) OpenAPISchema() map[string]any {
	return map[string]any{"type": "string", "pattern": `^-?\d+(\.\d+)?$`, "example": "12.50"}
}

type node struct {
	Next *node  `json:"next"`
	Kind string `json:"kind"`
}

func (*node) OpenAPISchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"next": &node{}, "kind": map[string]any{"type": "string"}}}
}

type invoice struct {
	At    time.Time `json:"at"`
	Head  node      `json:"head"`
	Total decimal   `json:"total"`
	Wait  duration  `json:"wait"`
}

func Test_OpenAPI_TypesDescribeThemselves(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) { app.Get("/invoices", listUsers).Returns(fiber.StatusOK, invoice{}) }, Config{TypeSchemas: map[reflect.Type]any{
		reflect.TypeFor[duration]():  map[string]any{"type": "integer", "format": "int64", "description": "nanoseconds"},
		reflect.TypeFor[time.Time](): map[string]any{"type": "string", "format": "date"},
	}})
	schemas := modelSchemas(t, spec)
	require.Contains(t, schemas, "decimal", "a provider's struct is a component")
	require.Equal(t, "string", requireMap(t, schemas["decimal"])["type"])
	require.NotContains(t, schemas, "duration", "a configured type is inline")

	props := requireMap(t, requireMap(t, schemas["invoice"])["properties"])
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/decimal"}, props["total"])
	require.Equal(t, "nanoseconds", requireMap(t, props["wait"])["description"])
	require.Equal(t, "date", requireMap(t, props["at"])["format"], "a configured type wins over the built-in one")
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/node"}, props["head"])

	next := requireMap(t, requireMap(t, schemas["node"])["properties"])["next"]
	require.Equal(t, map[string]any{"$ref": "#/components/schemas/node"}, next, "a type that mentions itself terminates")

	require.Panics(t, func() { New(Config{TypeSchemas: map[reflect.Type]any{nil: map[string]any{}}}) })
	require.NotPanics(t, func() { New(Config{TypeSchemas: map[reflect.Type]any{reflect.TypeFor[duration](): cat{}}}) })
}

func Test_OpenAPI_SchemaOfUsesTheProvider(t *testing.T) {
	t.Parallel()
	require.Equal(t, "string", SchemaOf(decimal{})["type"])
	require.Equal(t, "string", SchemaOf(&decimal{})["type"])
	list := SchemaOf([]decimal{})
	require.Equal(t, "array", list["type"])
	require.Equal(t, "string", requireMap(t, list["items"])["type"])
	require.NotPanics(t, func() { SchemaOf(node{}) }, "a self-referencing provider ends")
}

type cat struct {
	Kind  string `json:"kind"`
	Lives int    `json:"lives"`
}

type dog struct {
	Kind string `json:"kind"`
	Bark bool   `json:"bark"`
}

type pet struct{}

func (pet) OpenAPISchema() map[string]any {
	return WithDiscriminator(OneOf(cat{}, &dog{}), "kind", map[string]any{"cat": cat{}, "dog": dog{}, "text": "#/components/schemas/cat"})
}

type household struct {
	Main pet   `json:"main"`
	Pets []pet `json:"pets"`
}

func Test_OpenAPI_OneOfWithADiscriminator(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"3.0.3", "3.1.0", "3.2.0"} {
		schemas := modelSchemas(t, rfSpec(t, func(app *fiber.App) { app.Post("/homes", listUsers).Accepts(household{}) }, Config{OpenAPIVersion: version}))
		require.Contains(t, schemas, "cat", version)
		require.Contains(t, schemas, "dog", version)

		got := requireMap(t, schemas["pet"])
		require.Equal(t, []any{
			map[string]any{"$ref": "#/components/schemas/cat"},
			map[string]any{"$ref": "#/components/schemas/dog"},
		}, got["oneOf"], version)
		discriminator := requireMap(t, got["discriminator"])
		require.Equal(t, "kind", discriminator["propertyName"])
		require.Equal(t, map[string]any{
			"cat":  "#/components/schemas/cat",
			"dog":  "#/components/schemas/dog",
			"text": "#/components/schemas/cat",
		}, discriminator["mapping"], version)
	}

	inline := SchemaOf(pet{})
	oneOf := requireSlice(t, inline["oneOf"])
	require.Len(t, oneOf, 2)
	require.Equal(t, "object", requireMap(t, oneOf[0])["type"], "SchemaOf inlines the models")
	require.Equal(t, map[string]any{"text": "#/components/schemas/cat"}, requireMap(t, inline["discriminator"])["mapping"], "only a reference can be named without a registry")
}

func Test_OpenAPI_CompositionBuilders(t *testing.T) {
	t.Parallel()
	require.Equal(t, []any{cat{}, dog{}}, AnyOf(cat{}, dog{})["anyOf"])
	require.Equal(t, []any{cat{}}, AllOf(cat{})["allOf"])
	base := map[string]any{"description": "kept"}
	got := WithDiscriminator(base, "kind", nil)
	require.NotContains(t, base, "discriminator", "the input is left alone")
	require.Equal(t, map[string]any{"propertyName": "kind"}, got["discriminator"])
	require.Equal(t, "kept", got["description"])
	require.Contains(t, WithDiscriminator(nil, "kind", nil), "discriminator")
}

func Test_OpenAPI_SchemaNameHook(t *testing.T) {
	t.Parallel()
	spec := rfSpec(t, func(app *fiber.App) {
		app.Get("/a", listUsers).Returns(fiber.StatusOK, cat{})
		app.Get("/b", listUsers).Returns(fiber.StatusOK, dog{})
		app.Get("/c", listUsers).Returns(fiber.StatusOK, directiveModel{})
	}, Config{SchemaName: func(typ reflect.Type) string {
		switch typ {
		case reflect.TypeFor[cat](), reflect.TypeFor[dog]():
			return "pets/animal v1" // sanitized, and the second type with the name is numbered
		default:
			return ""
		}
	}})
	require.ElementsMatch(t, []string{"pets_animal_v1", "pets_animal_v1_2", "directiveModel"}, rfKeys(modelSchemas(t, spec)), "an empty name keeps the default")
}

type xmlItem struct {
	Name string `xml:"name"`
}

type xmlPet struct {
	XMLName xml.Name  `xml:"pet"`
	Name    string    `xml:"name"`
	Nick    string    `xml:"nick,omitempty"`
	Skip    string    `xml:"-"`
	Extra   string    `xml:",any"`
	Hidden  string    `xml:",comment"`
	JSONOne string    `json:"json_one" xml:"-"`
	Items   []xmlItem `xml:"items>item"`
	Notes   []string  `xml:"note"`
	ID      int       `xml:"id,attr"`
}

func Test_OpenAPI_XMLTagsDescribeXMLBodies(t *testing.T) {
	t.Parallel()
	schemas := modelSchemas(t, rfSpec(t, func(app *fiber.App) {
		app.Get("/pets", listUsers).Returns(fiber.StatusOK, xmlPet{}, fiber.MIMEApplicationXML)
		app.Get("/json-pets", listUsers).Returns(fiber.StatusOK, xmlPet{})
	}))
	xmlSchema := requireMap(t, schemas["xmlPet_xml"])
	require.Equal(t, map[string]any{"name": "pet"}, xmlSchema["xml"])
	props := requireMap(t, xmlSchema["properties"])
	require.Equal(t, map[string]any{"attribute": true}, requireMap(t, props["id"])["xml"])
	// A skipped field, a catch-all, a comment and a field only JSON names are not elements.
	require.ElementsMatch(t, []string{"id", "name", "nick", "items", "note"}, rfKeys(props))
	items := requireMap(t, props["items"])
	require.Equal(t, map[string]any{"wrapped": true}, items["xml"])
	item := requireMap(t, items["items"])
	require.Equal(t, "#/components/schemas/xmlItem_xml", item["$ref"])
	require.Equal(t, map[string]any{"name": "item"}, item["xml"])
	require.ElementsMatch(t, []any{"id", "name", "items", "note"}, xmlSchema["required"], "omitempty is optional, the rest is always written")

	jsonSchema := requireMap(t, schemas["xmlPet"])
	require.NotContains(t, jsonSchema, "xml", "a JSON body reads json tags")
	require.Contains(t, requireMap(t, jsonSchema["properties"]), "json_one")
}

func Test_OpenAPI_XMLMediaTypes(t *testing.T) {
	t.Parallel()
	for mediaType, want := range map[string]bool{
		"application/xml":                   true,
		"text/xml; charset=utf-8":           true,
		"application/atom+xml":              true,
		"image/svg+xml":                     true,
		"application/json":                  false,
		"application/x-www-form-urlencoded": false,
		"+xml":                              false,
	} {
		require.Equal(t, want, isXMLMediaType(mediaType), mediaType)
	}
}

type validated struct {
	Handle   string   `json:"handle" validate:"alphanum,startswith=u_"`
	Word     string   `json:"word" validate:"alpha,endswith=ing,contains=in"`
	Code     string   `json:"code" validate:"numeric"`
	Hex      string   `json:"hex" validate:"hexcolor"`
	Phone    string   `json:"phone" validate:"e164"`
	Text     string   `json:"text" validate:"ascii"`
	Token    string   `json:"token" validate:"jwt"`
	Kind     string   `json:"kind" validate:"eq=a.b"`
	Banned   string   `json:"banned" validate:"ne=root"`
	Optional string   `json:"optional,omitempty" validate:"omitempty,alpha,eq=x,startswith=a"`
	OptASCII string   `json:"opt_ascii,omitempty" validate:"omitempty,ascii"`
	Labels   []string `json:"labels" validate:"unique,eq=2"`
	People   []string `json:"people" validate:"unique=Name"`
	Raw      []byte   `json:"raw" validate:"eq=3"`
	Level    int      `json:"level" validate:"eq=3"`
	NotZero  int      `json:"not_zero" validate:"ne=0"`
	Port     int      `json:"port" validate:"port"`
	OptPort  int      `json:"opt_port,omitempty" validate:"omitempty,port"`
	Quoted   int      `json:"quoted,string" validate:"eq=4"`
	Flag     bool     `json:"flag" validate:"eq=true"`
}

func Test_OpenAPI_MoreValidateRules(t *testing.T) {
	t.Parallel()
	str, list := rfM{"type": "string"}, rfM{"type": "array", "items": rfM{"type": "string"}}
	pattern := func(p string) rfM { return rfM{"type": "string", "pattern": p} }
	require.Equal(t, rfM{
		"handle":   pattern(`^[a-zA-Z0-9]+$`), // the first pattern rule keeps the pattern
		"word":     pattern(`^[a-zA-Z]+$`),
		"code":     pattern(`^[-+]?[0-9]+(?:\.[0-9]+)?$`),
		"hex":      pattern(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`),
		"phone":    pattern(`^\+?[1-9]\d{7,14}$`),
		"text":     pattern("^[\\x00-\\x7F]*$"),
		"token":    pattern(`^[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]*$`),
		"kind":     rfM{"type": "string", "enum": []any{"a.b"}},
		"banned":   rfM{"type": "string", "not": rfM{"enum": []any{"root"}}},
		"level":    rfM{"type": "integer", "enum": []any{int64(3)}},
		"not_zero": rfM{"type": "integer", "not": rfM{"enum": []any{int64(0)}}},
		"flag":     rfM{"type": "boolean", "enum": []any{true}},
		"port":     rfM{"type": "integer", "minimum": 1, "maximum": 65535},
		// eq on a list is its length; unique by a field cannot be expressed.
		"labels": rfM{"type": "array", "items": str, "uniqueItems": true, "minItems": int64(2), "maxItems": int64(2)},
		"people": list,
		// omitempty lets an empty value through, which these rules refuse, but ascii accepts the empty string.
		"optional":  str,
		"opt_port":  rfM{"type": "integer"},
		"opt_ascii": pattern("^[\\x00-\\x7F]*$"),
		// A quoted number is compared as a number.
		"quoted": str,
		"raw":    rfM{"type": "string", "format": "byte"},
	}, requireProps(t, SchemaOf(validated{})))
}

type conditionalModel struct {
	Kind     string `json:"kind"`
	Email    string `json:"email,omitempty" validate:"required_without=Phone"`
	Phone    string `json:"phone,omitempty"`
	Street   string `json:"street,omitempty" validate:"required_with=City"`
	City     string `json:"city,omitempty"`
	Zip      string `json:"zip,omitempty" validate:"required_with_all=Street City"`
	Company  string `json:"company,omitempty" validate:"required_if=Kind business Count 2"`
	Reason   string `json:"reason,omitempty" validate:"required_unless=Kind personal"`
	Fax      string `json:"fax,omitempty" validate:"required_without_all=Email Phone"`
	Rush     string `json:"rush,omitempty" validate:"required_if=Express true"`
	Unknown  string `json:"unknown,omitempty" validate:"required_if=Nope x"`
	Optional string `json:"optional,omitempty"`
	Count    int    `json:"count"`
	Express  bool   `json:"express"`
}

func Test_OpenAPI_ConditionalRequiredRules(t *testing.T) {
	t.Parallel()
	build := func(version string) map[string]any {
		spec := rfSpec(t, func(app *fiber.App) { app.Post("/c", listUsers).Accepts(conditionalModel{}) }, Config{OpenAPIVersion: version})
		return requireMap(t, modelSchemas(t, spec)["conditionalModel"])
	}

	require.NotContains(t, build("3.0.3"), "allOf", "3.0 has no if/then")

	schema := build("3.1.0")
	require.NotContains(t, requireSlice(t, schema["required"]), "email")
	rules := requireSlice(t, schema["allOf"])
	require.Len(t, rules, 7, "the rule on a field that does not exist is left out")
	require.Contains(t, rules, map[string]any{"dependentRequired": map[string]any{"city": []any{"street"}}})
	require.Contains(t, rules, map[string]any{
		"if":   map[string]any{"not": map[string]any{"required": []any{"phone"}}},
		"then": map[string]any{"required": []any{"email"}},
	})
	require.Contains(t, rules, map[string]any{
		"if":   map[string]any{"required": []any{"street", "city"}},
		"then": map[string]any{"required": []any{"zip"}},
	})
	require.Contains(t, rules, map[string]any{
		"if": map[string]any{"allOf": []any{
			map[string]any{"required": []any{"kind"}, "properties": map[string]any{"kind": map[string]any{"const": "business"}}},
			map[string]any{"required": []any{"count"}, "properties": map[string]any{"count": map[string]any{"const": float64(2)}}},
		}},
		"then": map[string]any{"required": []any{"company"}},
	})
	require.Contains(t, rules, map[string]any{
		"if":   map[string]any{"not": map[string]any{"required": []any{"kind"}, "properties": map[string]any{"kind": map[string]any{"const": "personal"}}}},
		"then": map[string]any{"required": []any{"reason"}},
	})
	require.Contains(t, rules, map[string]any{
		"if":   map[string]any{"allOf": []any{map[string]any{"not": map[string]any{"required": []any{"email"}}}, map[string]any{"not": map[string]any{"required": []any{"phone"}}}}},
		"then": map[string]any{"required": []any{"fax"}},
	})
	require.Contains(t, rules, map[string]any{
		"if":   map[string]any{"allOf": []any{map[string]any{"required": []any{"express"}, "properties": map[string]any{"express": map[string]any{"const": true}}}}},
		"then": map[string]any{"required": []any{"rush"}},
	})
}

type commaModel struct {
	Plain   string `json:"plain" openapi:"description:Sorted, then trimmed: yes"`
	Split   string `json:"split" openapi:"description:Sorted, title:Name"`
	Flagged string `json:"flagged" openapi:"description:Old, deprecated"`
}

func Test_OpenAPI_TagValueCommaLimitation(t *testing.T) {
	t.Parallel()
	props := requireMap(t, SchemaOf(commaModel{})["properties"])
	prop := func(name string) map[string]any { return requireMap(t, props[name]) }
	require.Equal(t, "Sorted, then trimmed: yes", prop("plain")["description"], "a comma not followed by a directive stays in the value")
	require.Equal(t, "Sorted", prop("split")["description"])
	require.Equal(t, "Name", prop("split")["title"])
	require.Equal(t, "Old", prop("flagged")["description"])
	require.Equal(t, true, prop("flagged")["deprecated"])
}
