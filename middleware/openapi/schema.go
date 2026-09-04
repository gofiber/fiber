package openapi

import (
	"encoding"
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/utils/v2"
)

// SchemaOf generates an OpenAPI JSON Schema from a Go value using reflection,
// suitable for the route helpers (ResponseWithExample, RequestBodyWithExample,
// ParameterWithExample) or for Config.Components.
//
// Supported types:
//   - Primitives: string, bool, int*, uint*, float*
//   - time.Time → {"type": "string", "format": "date-time"}
//   - []byte → {"type": "string", "format": "byte"}
//   - Slices/arrays → {"type": "array", "items": {...}}
//   - Maps with string keys → {"type": "object", "additionalProperties": {...}}
//   - Structs → {"type": "object", "properties": {...}, "required": [...]}
//   - Pointers → schema of the pointed-to type (not required)
//   - interface{}/any → {}
//   - json.Number → {"type": "number"}
//   - Types implementing json.Marshaler → {}
//   - Types implementing encoding.TextMarshaler → {"type": "string"}
//
// Embedded structs are flattened as encoding/json does, recursive types emit a
// bare {"type": "object"} where the cycle repeats, and fields with no JSON
// representation (chan, func, complex) are skipped.
//
// Struct field tags:
//   - `json:"name"` sets the property name; `json:"-"` skips the field
//   - `json:",omitempty"` and `json:",omitzero"` make the field optional
//   - `json:",string"` documents the field as a string
//   - `openapi:"description:text"` sets the property description
//   - `openapi:"example:value"` sets the property example
//   - `openapi:"format:fmt"` overrides the format (e.g. "email", "uuid")
//   - `openapi:"enum:a|b|c"` sets the enum values
//
// openapi directives are comma-separated; a value may contain commas and colons,
// but not a comma immediately followed by another directive key.
func SchemaOf(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	return typeSchema(t, nil)
}

var (
	timeType          = reflect.TypeFor[time.Time]()
	jsonNumberType    = reflect.TypeFor[json.Number]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// implementsMarshaler reports whether t (or *t) implements the interface, in
// which case encoding/json bypasses ordinary field reflection.
func implementsMarshaler(t, iface reflect.Type) bool {
	return t.Implements(iface) || reflect.PointerTo(t).Implements(iface)
}

// maxPointerDepth bounds pointer dereferencing so a self-referential pointer
// type cannot spin forever.
const maxPointerDepth = 32

// markVisited records t as being expanded, allocating the set on first use, and
// returns it so callers can pass it down the recursion.
func markVisited(visited map[reflect.Type]bool, t reflect.Type) map[reflect.Type]bool {
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	visited[t] = true
	return visited
}

// typeSchema builds the schema for a single type. visited tracks the composite
// types currently on the recursion stack so that cyclic types terminate.
func typeSchema(t reflect.Type, visited map[reflect.Type]bool) map[string]any {
	// A self-referential pointer type (type P *P) never stops being a pointer,
	// so bound the walk instead of dereferencing forever.
	for range maxPointerDepth {
		if t.Kind() != reflect.Pointer {
			break
		}
		t = t.Elem()
	}
	if t.Kind() == reflect.Pointer {
		return map[string]any{}
	}

	if t == timeType {
		return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "date-time"}
	}

	// json.Number is a string kind but marshals as a bare JSON number.
	if t == jsonNumberType {
		return map[string]any{schemaKeyType: schemaTypeNumber}
	}

	// Custom JSON marshaling produces output field reflection cannot predict,
	// so accept any value.
	if implementsMarshaler(t, jsonMarshalerType) {
		return map[string]any{}
	}
	// A value-receiver text marshaler always yields a string; when only *T
	// implements it, encoding/json may fall back and the shape is unknowable.
	if t.Implements(textMarshalerType) {
		return map[string]any{schemaKeyType: schemaTypeString}
	}
	if reflect.PointerTo(t).Implements(textMarshalerType) {
		return map[string]any{}
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{schemaKeyType: schemaTypeString}
	case reflect.Bool:
		return map[string]any{schemaKeyType: schemaTypeBoolean}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		// encoding/json writes uintptr as a bare number, so omitting it here
		// would drop a field that does appear on the wire.
		reflect.Uintptr:
		return map[string]any{schemaKeyType: schemaTypeInteger}
	case reflect.Float32, reflect.Float64:
		return map[string]any{schemaKeyType: schemaTypeNumber}
	case reflect.Slice, reflect.Array:
		// Go marshals []byte (a slice of uint8) as a base64-encoded string.
		// Fixed-size byte arrays are still marshaled as arrays of numbers.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "byte"}
		}
		// A recursive element type (type L []L) would expand forever.
		if visited[t] {
			return map[string]any{schemaKeyType: "array"}
		}
		visited = markVisited(visited, t)
		items := typeSchema(t.Elem(), visited)
		delete(visited, t)
		if items == nil {
			// With no JSON representation for the element there is none for the
			// slice: encoding/json fails outright rather than emitting one.
			return nil
		}
		return map[string]any{schemaKeyType: "array", "items": items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		// A recursive element type (type M map[string]M) would expand forever.
		if visited[t] {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		visited = markVisited(visited, t)
		additional := typeSchema(t.Elem(), visited)
		delete(visited, t)
		if additional == nil {
			// See the slice branch: an unmarshalable element makes the whole
			// map unmarshalable.
			return nil
		}
		return map[string]any{schemaKeyType: schemaTypeObject, "additionalProperties": additional}
	case reflect.Struct:
		return structSchema(t, visited)
	case reflect.Interface:
		// An interface value (e.g. any) accepts any JSON value.
		return map[string]any{}
	default:
		// Unsupported kinds (chan, func, complex, uintptr, unsafe.Pointer) have
		// no JSON representation.
		return nil
	}
}

func structSchema(t reflect.Type, visited map[reflect.Type]bool) map[string]any {
	// Break reference cycles: if this struct type is already being expanded
	// further up the stack, emit a bare object instead of recursing forever.
	if visited[t] {
		return map[string]any{schemaKeyType: schemaTypeObject}
	}
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	visited[t] = true
	defer delete(visited, t)

	properties := make(map[string]any)
	var required []string

	// Resolved level by level like encoding/json: a name is taken at its
	// shallowest depth, where one tagged field wins or the name is dropped.
	type fieldCandidate struct {
		schema   map[string]any
		required bool
		tagged   bool
	}
	type embedRef struct {
		t reflect.Type
		// optional marks fields reached through a pointer or omitempty embed:
		// not guaranteed present, so never required on the parent.
		optional bool
	}

	level := []embedRef{{t: t}}
	// expanded tracks types already flattened shallower, which could otherwise
	// recurse forever. Same-level duplicates must still collide and drop.
	expanded := map[reflect.Type]bool{t: true}
	dropped := make(map[string]bool)

	for len(level) > 0 {
		var nextLevel []embedRef
		candidates := make(map[string][]fieldCandidate)
		var order []string

		for _, ref := range level {
			for i := range ref.t.NumField() {
				field := ref.t.Field(i)

				tagInfo := parseJSONTag(&field)
				if tagInfo.skip {
					continue
				}
				name := tagInfo.name

				embeddedType := field.Type
				// Bounded like typeSchema: a self-referential pointer type
				// (type P *P) never stops being a pointer.
				for range maxPointerDepth {
					if embeddedType.Kind() != reflect.Pointer {
						break
					}
					embeddedType = embeddedType.Elem()
				}
				isEmbeddedStruct := field.Anonymous && embeddedType.Kind() == reflect.Struct && embeddedType != timeType && name == ""

				// encoding/json ignores unexported fields but still promotes
				// those of an embedded unexported struct.
				if !field.IsExported() && !isEmbeddedStruct {
					continue
				}

				if isEmbeddedStruct {
					if expanded[embeddedType] {
						continue
					}
					nextLevel = append(nextLevel, embedRef{
						t:        embeddedType,
						optional: ref.optional || tagInfo.omit || field.Type.Kind() == reflect.Pointer,
					})
					continue
				}

				if name == "" {
					name = field.Name
				}

				fieldSchema := typeSchema(field.Type, visited)
				if fieldSchema == nil {
					// The field type has no JSON representation; skip it
					// entirely rather than emitting a meaningless empty schema.
					continue
				}

				// The ",string" option makes encoding/json wrap the value in a
				// JSON string, so the documented type must be string as well.
				if tagInfo.asString {
					switch fieldSchema[schemaKeyType] {
					case schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
						fieldSchema[schemaKeyType] = schemaTypeString
					default:
					}
				}

				applyOpenAPITag(&field, fieldSchema)

				if _, ok := candidates[name]; !ok {
					order = append(order, name)
				}
				candidates[name] = append(candidates[name], fieldCandidate{
					schema:   fieldSchema,
					required: !tagInfo.omit && field.Type.Kind() != reflect.Pointer && !ref.optional,
					tagged:   tagInfo.name != "",
				})
			}
		}

		for _, name := range order {
			if dropped[name] {
				continue
			}
			if _, exists := properties[name]; exists {
				continue
			}
			cands := candidates[name]
			chosen := 0
			if len(cands) > 1 {
				// Exactly one json-tagged candidate dominates; otherwise the
				// name is ambiguous at this depth and dropped for good.
				taggedIdx, taggedCount := -1, 0
				for i := range cands {
					if cands[i].tagged {
						taggedIdx = i
						taggedCount++
					}
				}
				if taggedCount != 1 {
					dropped[name] = true
					continue
				}
				chosen = taggedIdx
			}
			properties[name] = cands[chosen].schema
			if cands[chosen].required {
				required = append(required, name)
			}
		}

		for _, ref := range nextLevel {
			expanded[ref.t] = true
		}
		level = nextLevel
	}

	schema := map[string]any{
		schemaKeyType: schemaTypeObject,
		"properties":  properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// jsonTagInfo carries the parsed pieces of a field's json tag.
type jsonTagInfo struct {
	name     string
	omit     bool
	skip     bool
	asString bool
}

func parseJSONTag(field *reflect.StructField) jsonTagInfo {
	tag := field.Tag.Get("json")
	if tag == "" {
		return jsonTagInfo{}
	}
	if tag == "-" {
		return jsonTagInfo{skip: true}
	}
	name, opts, _ := strings.Cut(tag, ",")
	// An unusual name is resolved against the running encoding/json rather than
	// assumed, so the schema matches the wire format on every toolchain.
	if !isPlainJSONTagName(name) {
		name = effectiveJSONTagName(name)
	}
	info := jsonTagInfo{name: name}
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")
		switch opt {
		case "omitempty", "omitzero":
			info.omit = true
		case "string":
			info.asString = true
		default:
		}
	}
	return info
}

// isPlainJSONTagName reports whether every encoding/json release has taken name
// as written: letters, digits and the punctuation isValidTag has always allowed.
// Anything else is left to effectiveJSONTagName.
func isPlainJSONTagName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
			// Reserved punctuation is allowed inside a tag name.
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}

// effectiveJSONTagName returns the property name encoding/json actually gives a
// field tagged with name, or "" when the tag is ignored and the Go field name is
// used instead. The rules for unusual names changed in Go 1.27, so the answer is
// read from the toolchain in use rather than reimplemented here.
func effectiveJSONTagName(name string) string {
	// Quoted, not spliced: a struct tag value is an unquoted Go string literal,
	// so a name carrying a backslash or a quote has to be re-escaped or it
	// round-trips through StructTag.Get as something else entirely.
	probe := reflect.StructOf([]reflect.StructField{{
		Name: "Probe",
		Type: reflect.TypeFor[string](),
		Tag:  reflect.StructTag("json:" + strconv.Quote(name)),
	}})

	encoded, err := json.Marshal(reflect.New(probe).Elem().Interface())
	if err != nil {
		return ""
	}

	var decoded map[string]string
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return ""
	}
	for key := range decoded {
		if key == "Probe" {
			// The tag was ignored; the caller falls back to the field name.
			return ""
		}
		return key
	}
	return ""
}

// openapiDirectiveRe locates each directive's start. Its value runs from the
// colon to the next directive, so values may contain commas and colons.
var openapiDirectiveRe = regexp.MustCompile(`(?:^|,)\s*(description|example|format|enum):`)

func applyOpenAPITag(field *reflect.StructField, schema map[string]any) {
	tag := field.Tag.Get("openapi")
	if tag == "" {
		return
	}

	locs := openapiDirectiveRe.FindAllStringSubmatchIndex(tag, -1)
	for i, loc := range locs {
		key := tag[loc[2]:loc[3]]
		valStart := loc[1]
		valEnd := len(tag)
		if i+1 < len(locs) {
			valEnd = locs[i+1][0]
		}
		val := utils.TrimSpace(tag[valStart:valEnd])

		switch key {
		case "description":
			schema["description"] = val
		case "example":
			schema["example"] = inferExampleValue(val, schema)
		case "format":
			schema["format"] = val
		case "enum":
			values := strings.Split(val, "|")
			enumSlice := make([]any, len(values))
			for j, v := range values {
				// Convert each value to the field's type so an integer field
				// does not end up with a string-only enum no value can satisfy.
				enumSlice[j] = inferExampleValue(utils.TrimSpace(v), schema)
			}
			schema["enum"] = enumSlice
		default:
			// Unreachable: the regexp only matches the keys handled above.
		}
	}
}

func inferExampleValue(val string, schema map[string]any) any {
	schemaType, ok := schema[schemaKeyType].(string)
	if !ok {
		return val
	}
	switch schemaType {
	case schemaTypeInteger:
		if n, err := utils.ParseInt(val); err == nil {
			return n
		}
	case schemaTypeNumber:
		if f, err := utils.ParseFloat64(val); err == nil {
			return f
		}
	case schemaTypeBoolean:
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	default:
		// String and other types use the raw string value.
	}
	return val
}
