package openapi

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/utils/v2"
)

// SchemaOf generates an OpenAPI JSON Schema from a Go value using reflection.
// It inspects struct fields, their types, and json tags to produce a schema
// suitable for use with route helpers like ResponseWithExample, RequestBodyWithExample,
// and ParameterWithExample, or for inclusion in Config.Components.
//
// Supported types:
//   - Primitives: string, bool, int*, uint*, float*
//   - time.Time → {"type": "string", "format": "date-time"}
//   - []byte → {"type": "string", "format": "byte"} (Go marshals it as base64)
//   - Slices/arrays → {"type": "array", "items": {...}}
//   - Maps with string keys → {"type": "object", "additionalProperties": {...}}
//   - Structs → {"type": "object", "properties": {...}, "required": [...]}
//   - Pointers → schema of the pointed-to type (nullable fields are not required)
//   - interface{}/any → {} (accepts any value)
//
// Embedded structs and embedded pointers to structs are flattened into the
// parent object (matching encoding/json). Self-referential or mutually
// recursive structs are handled by emitting a bare {"type": "object"} at the
// point the cycle repeats, so reflection never recurses forever. Fields whose
// type has no JSON representation (chan, func, complex, ...) are skipped.
//
// Struct field tags:
//   - `json:"name"` sets the property name; `json:"-"` skips the field
//   - `json:",omitempty"` makes the field optional (not added to required)
//   - `openapi:"description:text"` sets the property description
//   - `openapi:"example:value"` sets the property example
//   - `openapi:"format:fmt"` overrides the format (e.g., "email", "uuid")
//   - `openapi:"enum:a|b|c"` sets the enum values
//
// openapi directives are comma-separated and a value may contain commas and
// colons; the only limitation is that a value cannot contain a comma immediately
// followed by another directive key (e.g. ",description:").
//
// Example:
//
//	type User struct {
//	    ID    int    `json:"id"`
//	    Name  string `json:"name"`
//	    Email string `json:"email" openapi:"format:email,description:User email"`
//	}
//	schema := openapi.SchemaOf(User{})
//	// Returns: map[string]any{
//	//   "type": "object",
//	//   "properties": map[string]any{
//	//     "id":    map[string]any{"type": "integer"},
//	//     "name":  map[string]any{"type": "string"},
//	//     "email": map[string]any{"type": "string", "format": "email", "description": "User email"},
//	//   },
//	//   "required": []string{"id", "name", "email"},
//	// }
func SchemaOf(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	return typeSchema(t, nil)
}

var timeType = reflect.TypeFor[time.Time]()

// typeSchema builds the schema for a single type. visited tracks the struct
// types currently on the recursion stack so that cyclic types terminate.
func typeSchema(t reflect.Type, visited map[reflect.Type]bool) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == timeType {
		return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "date-time"}
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]any{schemaKeyType: schemaTypeString}
	case reflect.Bool:
		return map[string]any{schemaKeyType: schemaTypeBoolean}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{schemaKeyType: schemaTypeInteger}
	case reflect.Float32, reflect.Float64:
		return map[string]any{schemaKeyType: schemaTypeNumber}
	case reflect.Slice, reflect.Array:
		// Go marshals []byte (a slice of uint8) as a base64-encoded string.
		// Fixed-size byte arrays are still marshaled as arrays of numbers.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "byte"}
		}
		items := typeSchema(t.Elem(), visited)
		if items == nil {
			items = map[string]any{}
		}
		return map[string]any{schemaKeyType: "array", "items": items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		additional := typeSchema(t.Elem(), visited)
		if additional == nil {
			additional = map[string]any{}
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

	// Fields are resolved level by level over the embedding tree, matching
	// encoding/json: a name is taken at the shallowest depth where it appears;
	// among candidates at that depth exactly one json-tagged field wins,
	// otherwise the name is ambiguous and dropped entirely (deeper fields do
	// not resurrect it).
	type fieldCandidate struct {
		schema   map[string]any
		required bool
		tagged   bool
	}
	type embedRef struct {
		t reflect.Type
		// optional marks fields reached through a pointer embed or an
		// omitempty embed: they are not guaranteed to be present and must not
		// be marked required on the parent.
		optional bool
	}

	level := []embedRef{{t: t}}
	// expanded tracks struct types flattened at shallower levels: re-expanding
	// them deeper could recurse forever (embedding cycles) and their fields
	// would lose to the shallower ones anyway. Same-level duplicates are NOT
	// deduplicated — their fields must collide and be dropped like
	// encoding/json does.
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
				for embeddedType.Kind() == reflect.Pointer {
					embeddedType = embeddedType.Elem()
				}
				isEmbeddedStruct := field.Anonymous && embeddedType.Kind() == reflect.Struct && embeddedType != timeType && name == ""

				// encoding/json ignores unexported fields, but it still
				// promotes the exported fields of an embedded unexported
				// struct type.
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
	parts := strings.Split(tag, ",")
	info := jsonTagInfo{name: parts[0]}
	for _, opt := range parts[1:] {
		switch opt {
		case "omitempty":
			info.omit = true
		case "string":
			info.asString = true
		default:
		}
	}
	return info
}

// openapiDirectiveRe locates the start of each recognized openapi tag directive.
// A directive begins at the start of the tag or after a comma. Everything from a
// directive's colon up to the next directive (or the end of the tag) is its
// value, so values may freely contain commas and colons.
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
