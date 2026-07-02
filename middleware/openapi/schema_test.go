package openapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// requireProps extracts the "properties" map from a schema, failing the test if
// the type assertion is unsuccessful.
func requireProps(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok, "expected properties to be map[string]any")
	return props
}

// requireRequired extracts the "required" slice from a schema, failing the test
// if the type assertion is unsuccessful.
func requireRequired(t *testing.T, schema map[string]any) []string {
	t.Helper()
	req, ok := schema["required"].([]string)
	require.True(t, ok, "expected required to be []string")
	return req
}

// requireProp extracts a single property schema, failing the test if the type
// assertion is unsuccessful.
func requireProp(t *testing.T, props map[string]any, name string) map[string]any {
	t.Helper()
	prop, ok := props[name].(map[string]any)
	require.True(t, ok, "expected property %q to be map[string]any", name)
	return prop
}

func Test_SchemaOf_SimpleStruct(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}

	schema := SchemaOf(User{})
	require.Equal(t, "object", schema["type"])

	props := requireProps(t, schema)
	require.Equal(t, map[string]any{"type": "integer"}, props["id"])
	require.Equal(t, map[string]any{"type": "string"}, props["name"])

	required := requireRequired(t, schema)
	require.Contains(t, required, "id")
	require.Contains(t, required, "name")
}

func Test_SchemaOf_AllPrimitiveTypes(t *testing.T) {
	t.Parallel()

	type Primitives struct {
		S   string  `json:"s"`
		I64 int64   `json:"i64"`
		U64 uint64  `json:"u64"`
		F64 float64 `json:"f64"`
		I   int     `json:"i"`
		U   uint    `json:"u"`
		F32 float32 `json:"f32"`
		I32 int32   `json:"i32"`
		U32 uint32  `json:"u32"`
		I16 int16   `json:"i16"`
		U16 uint16  `json:"u16"`
		I8  int8    `json:"i8"`
		U8  uint8   `json:"u8"`
		B   bool    `json:"b"`
	}

	schema := SchemaOf(Primitives{})
	props := requireProps(t, schema)

	require.Equal(t, "string", requireProp(t, props, "s")["type"])
	require.Equal(t, "boolean", requireProp(t, props, "b")["type"])
	require.Equal(t, "integer", requireProp(t, props, "i")["type"])
	require.Equal(t, "integer", requireProp(t, props, "i8")["type"])
	require.Equal(t, "integer", requireProp(t, props, "i16")["type"])
	require.Equal(t, "integer", requireProp(t, props, "i32")["type"])
	require.Equal(t, "integer", requireProp(t, props, "i64")["type"])
	require.Equal(t, "integer", requireProp(t, props, "u")["type"])
	require.Equal(t, "integer", requireProp(t, props, "u8")["type"])
	require.Equal(t, "integer", requireProp(t, props, "u16")["type"])
	require.Equal(t, "integer", requireProp(t, props, "u32")["type"])
	require.Equal(t, "integer", requireProp(t, props, "u64")["type"])
	require.Equal(t, "number", requireProp(t, props, "f32")["type"])
	require.Equal(t, "number", requireProp(t, props, "f64")["type"])
}

func Test_SchemaOf_PointerField(t *testing.T) {
	t.Parallel()

	type WithPointer struct {
		Name *string `json:"name"`
		Age  int     `json:"age"`
	}

	schema := SchemaOf(WithPointer{})
	props := requireProps(t, schema)

	// Pointer field schema should resolve to the underlying type
	require.Equal(t, "string", requireProp(t, props, "name")["type"])

	// Pointer fields should not be in required
	required := requireRequired(t, schema)
	require.NotContains(t, required, "name")
	require.Contains(t, required, "age")
}

func Test_SchemaOf_OmitemptyNotRequired(t *testing.T) {
	t.Parallel()

	type WithOmit struct {
		Name string `json:"name,omitempty"`
		ID   int    `json:"id"`
	}

	schema := SchemaOf(WithOmit{})
	required := requireRequired(t, schema)
	require.Contains(t, required, "id")
	require.NotContains(t, required, "name")
}

func Test_SchemaOf_JSONDash_SkipsField(t *testing.T) {
	t.Parallel()

	type WithSkip struct {
		Public  string `json:"public"`
		private string //nolint:unused // testing unexported field
		Skipped string `json:"-"`
	}

	schema := SchemaOf(WithSkip{})
	props := requireProps(t, schema)
	require.Contains(t, props, "public")
	require.NotContains(t, props, "Skipped")
	require.NotContains(t, props, "private")
}

func Test_SchemaOf_SliceAndArray(t *testing.T) {
	t.Parallel()

	type WithSlice struct {
		Tags  []string `json:"tags"`
		Codes [3]int   `json:"codes"`
	}

	schema := SchemaOf(WithSlice{})
	props := requireProps(t, schema)

	tagsSchema := requireProp(t, props, "tags")
	require.Equal(t, "array", tagsSchema["type"])
	require.Equal(t, map[string]any{"type": "string"}, tagsSchema["items"])

	codesSchema := requireProp(t, props, "codes")
	require.Equal(t, "array", codesSchema["type"])
	require.Equal(t, map[string]any{"type": "integer"}, codesSchema["items"])
}

func Test_SchemaOf_MapField(t *testing.T) {
	t.Parallel()

	type WithMap struct {
		Meta map[string]int `json:"meta"`
	}

	schema := SchemaOf(WithMap{})
	props := requireProps(t, schema)

	metaSchema := requireProp(t, props, "meta")
	require.Equal(t, "object", metaSchema["type"])
	require.Equal(t, map[string]any{"type": "integer"}, metaSchema["additionalProperties"])
}

func Test_SchemaOf_NestedStruct(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}
	type Person struct {
		Address Address `json:"address"`
		Name    string  `json:"name"`
	}

	schema := SchemaOf(Person{})
	props := requireProps(t, schema)

	addrSchema := requireProp(t, props, "address")
	require.Equal(t, "object", addrSchema["type"])
	addrProps := requireProps(t, addrSchema)
	require.Equal(t, map[string]any{"type": "string"}, addrProps["street"])
	require.Equal(t, map[string]any{"type": "string"}, addrProps["city"])
}

func Test_SchemaOf_EmbeddedStruct(t *testing.T) {
	t.Parallel()

	type Base struct {
		ID int `json:"id"`
	}
	type Extended struct {
		Name string `json:"name"`
		Base
	}

	schema := SchemaOf(Extended{})
	props := requireProps(t, schema)

	// Embedded fields should be flattened
	require.Contains(t, props, "id")
	require.Contains(t, props, "name")
}

func Test_SchemaOf_TimeField(t *testing.T) {
	t.Parallel()

	type Event struct {
		At   time.Time `json:"at"`
		Name string    `json:"name"`
	}

	schema := SchemaOf(Event{})
	props := requireProps(t, schema)

	atSchema := requireProp(t, props, "at")
	require.Equal(t, "string", atSchema["type"])
	require.Equal(t, "date-time", atSchema["format"])
}

func Test_SchemaOf_OpenAPITags(t *testing.T) {
	t.Parallel()

	type Product struct {
		Name   string  `json:"name" openapi:"description:Product name,example:Widget"`
		Email  string  `json:"email" openapi:"format:email"`
		Status string  `json:"status" openapi:"enum:active|inactive|pending"`
		Price  float64 `json:"price" openapi:"example:9.99"`
	}

	schema := SchemaOf(Product{})
	props := requireProps(t, schema)

	nameSchema := requireProp(t, props, "name")
	require.Equal(t, "string", nameSchema["type"])
	require.Equal(t, "Product name", nameSchema["description"])
	require.Equal(t, "Widget", nameSchema["example"])

	priceSchema := requireProp(t, props, "price")
	require.Equal(t, "number", priceSchema["type"])
	require.InEpsilon(t, 9.99, priceSchema["example"], 0.001)

	emailSchema := requireProp(t, props, "email")
	require.Equal(t, "email", emailSchema["format"])

	statusSchema := requireProp(t, props, "status")
	require.Equal(t, []any{"active", "inactive", "pending"}, statusSchema["enum"])
}

func Test_SchemaOf_Nil(t *testing.T) {
	t.Parallel()
	require.Nil(t, SchemaOf(nil))
}

func Test_SchemaOf_Pointer(t *testing.T) {
	t.Parallel()

	type Simple struct {
		Name string `json:"name"`
	}

	schema := SchemaOf(&Simple{})
	require.Equal(t, "object", schema["type"])
	props := requireProps(t, schema)
	require.Contains(t, props, "name")
}

func Test_SchemaOf_NoJSONTag(t *testing.T) {
	t.Parallel()

	type NoTag struct {
		FieldName string
	}

	schema := SchemaOf(NoTag{})
	props := requireProps(t, schema)
	// Without a json tag, the Go field name is used
	require.Contains(t, props, "FieldName")
}

func Test_SchemaOf_MapWithNonStringKey(t *testing.T) {
	t.Parallel()

	type WithIntKey struct {
		Data map[int]string `json:"data"`
	}

	schema := SchemaOf(WithIntKey{})
	props := requireProps(t, schema)
	dataSchema := requireProp(t, props, "data")
	require.Equal(t, "object", dataSchema["type"])
	// Non-string key maps don't get additionalProperties
	require.Nil(t, dataSchema["additionalProperties"])
}

func Test_SchemaOf_NoRequiredWhenAllOmitempty(t *testing.T) {
	t.Parallel()

	type AllOptional struct {
		A string `json:"a,omitempty"`
		B int    `json:"b,omitempty"`
	}

	schema := SchemaOf(AllOptional{})
	_, hasRequired := schema["required"]
	require.False(t, hasRequired)
}

func Test_SchemaOf_SliceOfStructs(t *testing.T) {
	t.Parallel()

	type Item struct {
		Name string `json:"name"`
	}
	type Container struct {
		Items []Item `json:"items"`
	}

	schema := SchemaOf(Container{})
	props := requireProps(t, schema)
	itemsSchema := requireProp(t, props, "items")
	require.Equal(t, "array", itemsSchema["type"])
	items := requireMap(t, itemsSchema["items"])
	require.Equal(t, "object", items["type"])
	itemProps := requireProps(t, items)
	require.Contains(t, itemProps, "name")
}

func Test_SchemaOf_BooleanExample(t *testing.T) {
	t.Parallel()

	type Flags struct {
		Active bool `json:"active" openapi:"example:true"`
	}

	schema := SchemaOf(Flags{})
	props := requireProps(t, schema)
	activeSchema := requireProp(t, props, "active")
	require.Equal(t, true, activeSchema["example"])
}

func Test_SchemaOf_IntegerExample(t *testing.T) {
	t.Parallel()

	type Counter struct {
		Count int `json:"count" openapi:"example:42"`
	}

	schema := SchemaOf(Counter{})
	props := requireProps(t, schema)
	countSchema := requireProp(t, props, "count")
	require.Equal(t, int64(42), countSchema["example"])
}

func Test_SchemaOf_PlainType(t *testing.T) {
	t.Parallel()

	// Non-struct types should return their schema directly
	require.Equal(t, map[string]any{"type": "string"}, SchemaOf("hello"))
	require.Equal(t, map[string]any{"type": "integer"}, SchemaOf(42))
	require.Equal(t, map[string]any{"type": "boolean"}, SchemaOf(true))
	require.Equal(t, map[string]any{"type": "number"}, SchemaOf(3.14))
}

func Test_SchemaOf_RecursiveStruct(t *testing.T) {
	t.Parallel()

	type Node struct {
		Next  *Node `json:"next"`
		Value int   `json:"value"`
	}

	// Must not stack-overflow on a self-referential type.
	schema := SchemaOf(Node{})
	props := requireProps(t, schema)
	require.Equal(t, map[string]any{"type": "integer"}, requireProp(t, props, "value"))
	// The cyclic reference is broken with a bare object schema.
	require.Equal(t, map[string]any{"type": "object"}, requireProp(t, props, "next"))
}

func Test_SchemaOf_MutuallyRecursiveStructs(t *testing.T) {
	t.Parallel()

	type B struct{}
	type A struct {
		B *B `json:"b"`
	}
	// Reusing the same type in sibling positions must still fully expand.
	type Pair struct {
		First  A `json:"first"`
		Second A `json:"second"`
	}

	schema := SchemaOf(Pair{})
	props := requireProps(t, schema)
	first := requireProp(t, props, "first")
	second := requireProp(t, props, "second")
	require.Equal(t, "object", first["type"])
	require.Equal(t, "object", second["type"])
	require.Contains(t, requireProps(t, first), "b")
	require.Contains(t, requireProps(t, second), "b")
}

func Test_SchemaOf_ByteSlice(t *testing.T) {
	t.Parallel()

	type WithBytes struct {
		Data    []byte  `json:"data"`
		FixedID [4]byte `json:"fixed_id"`
	}

	schema := SchemaOf(WithBytes{})
	props := requireProps(t, schema)
	// []byte marshals to a base64 string.
	require.Equal(t, map[string]any{"type": "string", "format": "byte"}, requireProp(t, props, "data"))
	// Fixed-size byte arrays marshal to arrays of numbers.
	require.Equal(t, "array", requireProp(t, props, "fixed_id")["type"])
}

func Test_SchemaOf_EmbeddedPointerStruct(t *testing.T) {
	t.Parallel()

	type Inner struct {
		ID int `json:"id"`
	}
	type Outer struct {
		*Inner
		Name string `json:"name"`
	}

	schema := SchemaOf(Outer{})
	props := requireProps(t, schema)
	// Embedded pointer fields are flattened into the parent.
	require.Equal(t, map[string]any{"type": "integer"}, requireProp(t, props, "id"))
	require.Equal(t, map[string]any{"type": "string"}, requireProp(t, props, "name"))
	require.NotContains(t, props, "Inner")

	// The embedded pointer may be nil, so its fields are not required; only the
	// direct non-pointer field is.
	required := requireRequired(t, schema)
	require.Contains(t, required, "name")
	require.NotContains(t, required, "id")
}

func Test_SchemaOf_AnyField(t *testing.T) {
	t.Parallel()

	type WithAny struct {
		Meta any `json:"meta"`
	}

	schema := SchemaOf(WithAny{})
	props := requireProps(t, schema)
	// interface{}/any accepts any JSON value -> empty schema.
	require.Equal(t, map[string]any{}, requireProp(t, props, "meta"))
}

func Test_SchemaOf_UnsupportedFieldSkipped(t *testing.T) {
	t.Parallel()

	type WithChan struct {
		Ch   chan int    `json:"ch"`
		Fn   func() bool `json:"fn"`
		Name string      `json:"name"`
	}

	schema := SchemaOf(WithChan{})
	props := requireProps(t, schema)
	require.Contains(t, props, "name")
	// Non-serializable fields are skipped entirely.
	require.NotContains(t, props, "ch")
	require.NotContains(t, props, "fn")
}

func Test_SchemaOf_OpenAPITagWithComma(t *testing.T) {
	t.Parallel()

	type Product struct {
		Status string `json:"status" openapi:"enum:active|inactive,description:Status, including a comma"`
	}

	schema := SchemaOf(Product{})
	props := requireProps(t, schema)
	status := requireProp(t, props, "status")
	require.Equal(t, "Status, including a comma", status["description"])
	require.Equal(t, []any{"active", "inactive"}, status["enum"])
}

// Test_SchemaOf_EmbeddedFieldDoesNotShadowParent verifies that a field declared
// on the parent struct wins over a promoted embedded field of the same name,
// matching encoding/json semantics, and that required entries are not
// duplicated.
func Test_SchemaOf_EmbeddedFieldDoesNotShadowParent(t *testing.T) {
	t.Parallel()

	type Inner struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	}
	type Outer struct { //nolint:govet // fieldalignment: the embedded struct must follow the parent field to exercise shadowing
		ID int `json:"id"`
		Inner
	}

	schema := SchemaOf(Outer{})
	props := requireProps(t, schema)

	// The parent's int field shadows the embedded string field regardless of
	// declaration order.
	id := requireProp(t, props, "id")
	require.Equal(t, "integer", id[schemaKeyType])
	require.Contains(t, props, "note")

	required, ok := schema["required"].([]string)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"id", "note"}, required)
}

// Test_SchemaOf_StringOption verifies the json ",string" option is reflected
// as a string type, matching encoding/json's wire format.
func Test_SchemaOf_StringOption(t *testing.T) {
	t.Parallel()

	type User struct {
		Name   string  `json:"name"`
		ID     int64   `json:"id,string"`
		Score  float64 `json:"score,string"`
		Active bool    `json:"active,string"`
	}

	schema := SchemaOf(User{})
	props := requireProps(t, schema)
	require.Equal(t, "string", requireProp(t, props, "id")[schemaKeyType])
	require.Equal(t, "string", requireProp(t, props, "score")[schemaKeyType])
	require.Equal(t, "string", requireProp(t, props, "active")[schemaKeyType])
	require.Equal(t, "string", requireProp(t, props, "name")[schemaKeyType])
}

// Test_SchemaOf_ConflictingEmbeddedFieldsDropped verifies a field promoted by
// two embedded structs at the same depth is dropped entirely, matching
// encoding/json's ambiguity rule.
func Test_SchemaOf_ConflictingEmbeddedFieldsDropped(t *testing.T) {
	t.Parallel()

	type B1 struct {
		X int `json:"x"`
	}
	type B2 struct {
		X string `json:"x"`
	}
	type T struct { //nolint:govet // fieldalignment: embed order mirrors the documented scenario
		Y string `json:"y"`
		B1
		B2 //nolint:govet // structtag: the duplicate json tag is the ambiguity under test
	}

	schema := SchemaOf(T{})
	props := requireProps(t, schema)
	require.NotContains(t, props, "x")
	require.Contains(t, props, "y")

	required, ok := schema["required"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"y"}, required)
}

// Test_SchemaOf_EmbeddedRequiredDeterministic verifies the required list
// derived from embedded structs is stable across invocations.
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

	first, ok := SchemaOf(T{})["required"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"a", "b", "c", "d", "e"}, first)
	for range 10 {
		next, ok := SchemaOf(T{})["required"].([]string)
		require.True(t, ok)
		require.Equal(t, first, next)
	}
}
