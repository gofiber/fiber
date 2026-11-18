package openapi

import (
	"encoding/xml"
	"reflect"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

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

	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	status, ok := props["status"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "open", status["default"])
	require.Equal(t, "^[a-z]+$", status["pattern"])
	require.Equal(t, "Status", status["title"])
	require.InDelta(t, 1, status["x-order"], 0)
	require.Equal(t, "main", status["x-group"])

	qty, ok := props["qty"].(map[string]any)
	require.True(t, ok)
	require.InDelta(t, 5, qty["multipleOf"], 0)
	require.InDelta(t, 9.5, requireMap(t, props["price"])["default"], 0)
	require.Equal(t, true, requireMap(t, props["tags"])["uniqueItems"])
	comment := requireMap(t, props["comment"])
	require.Equal(t, []any{float64(1), float64(2)}, comment["x-flags"])
	require.Equal(t, true, comment["x-ok"])
}

func Test_OpenAPI_NullableTagIsSpelledPerVersion(t *testing.T) {
	t.Parallel()
	build := func() *fiber.App {
		app := fiber.New()
		app.Get("/a", listUsers).Returns(fiber.StatusOK, directiveModel{})
		return app
	}
	qty := func(spec map[string]any) map[string]any {
		return requireMap(t, requireMap(t, requireMap(t, modelSchemas(t, spec)["directiveModel"])["properties"])["qty"])
	}

	require.Equal(t, []any{"integer", "null"}, qty(modelSpec(t, build(), Config{OpenAPIVersion: "3.1.0"}))["type"])
	old := qty(modelSpec(t, build(), Config{OpenAPIVersion: "3.0.3"}))
	require.Equal(t, true, old["nullable"])
	require.Equal(t, "integer", old["type"])
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
	app := fiber.New()
	app.Get("/invoices", listUsers).Returns(fiber.StatusOK, invoice{})
	spec := modelSpec(t, app, Config{TypeSchemas: map[reflect.Type]any{
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
		app := fiber.New()
		app.Post("/homes", listUsers).Accepts(household{})
		spec := modelSpec(t, app, Config{OpenAPIVersion: version})
		schemas := modelSchemas(t, spec)
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
	oneOf, ok := inline["oneOf"].([]any)
	require.True(t, ok)
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
	app := fiber.New()
	app.Get("/a", listUsers).Returns(fiber.StatusOK, cat{})
	app.Get("/b", listUsers).Returns(fiber.StatusOK, dog{})
	app.Get("/c", listUsers).Returns(fiber.StatusOK, directiveModel{})
	spec := modelSpec(t, app, Config{SchemaName: func(typ reflect.Type) string {
		switch typ {
		case reflect.TypeFor[cat](), reflect.TypeFor[dog]():
			return "pets/animal v1" // sanitized, and the second one is numbered
		default:
			return ""
		}
	}})
	schemas := modelSchemas(t, spec)
	require.Contains(t, schemas, "pets_animal_v1")
	require.Contains(t, schemas, "directiveModel", "an empty name keeps the default")
	require.Len(t, schemas, 3)
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
	app := fiber.New()
	app.Get("/pets", listUsers).Returns(fiber.StatusOK, xmlPet{}, fiber.MIMEApplicationXML)
	app.Get("/json-pets", listUsers).Returns(fiber.StatusOK, xmlPet{})
	spec := modelSpec(t, app)
	schemas := modelSchemas(t, spec)

	xmlSchema := requireMap(t, schemas["xmlPet_xml"])
	require.Equal(t, map[string]any{"name": "pet"}, xmlSchema["xml"])
	props := requireMap(t, xmlSchema["properties"])
	require.Equal(t, map[string]any{"attribute": true}, requireMap(t, props["id"])["xml"])
	require.Contains(t, props, "name")
	require.Contains(t, props, "nick")
	require.NotContains(t, props, "Skip")
	require.NotContains(t, props, "Extra", "a catch-all is not an element")
	require.NotContains(t, props, "Hidden", "a comment is not an element")
	require.NotContains(t, props, "JSONOne")

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

func Test_OpenAPI_TypeSchemasValidatesItsKeys(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() { New(Config{TypeSchemas: map[reflect.Type]any{nil: map[string]any{}}}) })
	require.NotPanics(t, func() { New(Config{TypeSchemas: map[reflect.Type]any{reflect.TypeFor[duration](): cat{}}}) })
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
	props := requireMap(t, SchemaOf(validated{})["properties"])
	prop := func(name string) map[string]any { return requireMap(t, props[name]) }

	require.Equal(t, `^[a-zA-Z0-9]+$`, prop("handle")["pattern"], "the first pattern rule keeps the pattern")
	require.Equal(t, `^[a-zA-Z]+$`, prop("word")["pattern"])
	require.Equal(t, `^[-+]?[0-9]+(?:\.[0-9]+)?$`, prop("code")["pattern"])
	require.Equal(t, `^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`, prop("hex")["pattern"])
	require.Equal(t, `^\+?[1-9]\d{7,14}$`, prop("phone")["pattern"])
	require.Equal(t, "^[\\x00-\\x7F]*$", prop("text")["pattern"])
	require.Contains(t, prop("token"), "pattern")

	require.Equal(t, []any{"a.b"}, prop("kind")["enum"])
	require.Equal(t, map[string]any{"enum": []any{"root"}}, prop("banned")["not"])
	require.Equal(t, []any{int64(3)}, prop("level")["enum"])
	require.Equal(t, map[string]any{"enum": []any{int64(0)}}, prop("not_zero")["not"])
	require.Equal(t, []any{true}, prop("flag")["enum"])
	require.InDelta(t, 1, prop("port")["minimum"], 0)
	require.InDelta(t, 65535, prop("port")["maximum"], 0)

	labels := prop("labels")
	require.Equal(t, true, labels["uniqueItems"])
	require.InDelta(t, 2, labels["minItems"], 0)
	require.InDelta(t, 2, labels["maxItems"], 0, "eq on a list is its length")
	require.NotContains(t, prop("people"), "uniqueItems", "unique by a field cannot be expressed")

	optional := prop("optional")
	require.NotContains(t, optional, "pattern", "omitempty lets an empty value through, which these rules refuse")
	require.NotContains(t, optional, "enum")
	require.NotContains(t, prop("opt_port"), "minimum")
	require.Contains(t, prop("opt_ascii"), "pattern", "ascii accepts the empty string")

	require.NotContains(t, prop("quoted"), "enum", "a quoted number is compared as a number")
	require.NotContains(t, prop("raw"), "minItems")
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
		app := fiber.New()
		app.Post("/c", listUsers).Accepts(conditionalModel{})
		return requireMap(t, modelSchemas(t, modelSpec(t, app, Config{OpenAPIVersion: version}))["conditionalModel"])
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
