package openapi

import (
	"encoding"
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/utils/v2"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

var (
	timeType          = reflect.TypeFor[time.Time]()
	jsonNumberType    = reflect.TypeFor[json.Number]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// openapiDirectiveRe locates each directive's start; a value runs to the next
// directive, so it may contain commas and colons.
var openapiDirectiveRe = regexp.MustCompile(`(?:^|,)\s*(description|example|format|enum):|(?:^|,)\s*(readOnly|readonly|writeOnly|writeonly|deprecated)\s*`)

// validateFormats maps validator format rules to JSON Schema formats.
var validateFormats = map[string]string{
	formatEmail:        formatEmail,
	formatUUID:         formatUUID,
	"uuid3":            formatUUID,
	"uuid4":            formatUUID,
	"uuid5":            formatUUID,
	"url":              formatURI,
	formatURI:          formatURI,
	formatIPv4:         formatIPv4,
	formatIPv6:         formatIPv6,
	formatHostname:     formatHostname,
	"hostname_rfc1123": formatHostname,
	"fqdn":             formatHostname,
	"base64":           formatByte,
}

// boundKeywords lists the lower and upper limit keywords for each schema type.
var boundKeywords = map[string][2]string{
	schemaTypeString:  {"minLength", "maxLength"},
	schemaTypeArray:   {"minItems", "maxItems"},
	schemaTypeObject:  {"minProperties", "maxProperties"},
	schemaTypeInteger: {"minimum", "maximum"},
	schemaTypeNumber:  {"minimum", "maximum"},
}

// maxPointerDepth stops a self-referential pointer type from looping forever.
const maxPointerDepth = 32

const (
	formatEmail    = "email"
	formatUUID     = "uuid"
	formatURI      = "uri"
	formatIPv4     = "ipv4"
	formatIPv6     = "ipv6"
	formatHostname = "hostname"
	formatByte     = "byte"
)

const (
	lowerBound boundSide = iota
	upperBound
)

// SchemaOf generates an OpenAPI JSON Schema from a Go value by reflection.
// Embedded structs are flattened as encoding/json does, and fields with no
// JSON representation (chan, func, complex) are skipped.
//
// Recognized field tags are `json` (name, "-", omitempty, omitzero, string),
// `openapi` (description, example, format, enum with "|" separators, readOnly,
// writeOnly, deprecated) and `validate` (required, min, max, len, gte, lte,
// oneof and common format rules). A value in an openapi tag may contain commas
// and colons, but not a comma followed by another directive key.
//
// Nested structs are inlined here, unlike values passed to the route helpers,
// which emit named structs once under components.schemas.
func SchemaOf(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	return typeSchema(t, nil, nil)
}

// implementsMarshaler reports whether t or *t implements iface.
func implementsMarshaler(t, iface reflect.Type) bool {
	return t.Implements(iface) || reflect.PointerTo(t).Implements(iface)
}

// markVisited records t as being expanded and returns the (possibly new) set.
func markVisited(visited map[reflect.Type]bool, t reflect.Type) map[reflect.Type]bool {
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	visited[t] = true
	return visited
}

// derefType strips pointer indirections; a self-referential type (type P *P)
// stays a pointer after maxPointerDepth steps.
func derefType(t reflect.Type) reflect.Type {
	for range maxPointerDepth {
		if t.Kind() != reflect.Pointer {
			break
		}
		t = t.Elem()
	}
	return t
}

// typeSchema builds the schema for t; visited holds the types being expanded so
// cyclic types terminate.
func typeSchema(t reflect.Type, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	t = derefType(t)
	if t.Kind() == reflect.Pointer {
		return map[string]any{}
	}

	if t == timeType {
		return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "date-time"}
	}

	// json.Number is a string kind but marshals as a JSON number.
	if t == jsonNumberType {
		return map[string]any{schemaKeyType: schemaTypeNumber}
	}

	// Custom marshaling output cannot be predicted, so accept any value.
	if implementsMarshaler(t, jsonMarshalerType) {
		return map[string]any{}
	}
	// Only a value-receiver text marshaler is certain to yield a string.
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
		// encoding/json writes uintptr as a number.
		reflect.Uintptr:
		return map[string]any{schemaKeyType: schemaTypeInteger}
	case reflect.Float32, reflect.Float64:
		return map[string]any{schemaKeyType: schemaTypeNumber}
	case reflect.Slice, reflect.Array:
		// []byte marshals as base64; byte arrays marshal as number arrays.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "byte"}
		}
		// Recursive element types (type L []L) would expand forever.
		if visited[t] {
			return map[string]any{schemaKeyType: schemaTypeArray}
		}
		visited = markVisited(visited, t)
		items := typeSchema(t.Elem(), visited, reg)
		delete(visited, t)
		if items == nil {
			// encoding/json fails on an unmarshalable element.
			return nil
		}
		return map[string]any{schemaKeyType: schemaTypeArray, "items": items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		// Recursive element types (type M map[string]M) would expand forever.
		if visited[t] {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		visited = markVisited(visited, t)
		additional := typeSchema(t.Elem(), visited, reg)
		delete(visited, t)
		if additional == nil {
			// Same as the slice branch.
			return nil
		}
		return map[string]any{schemaKeyType: schemaTypeObject, "additionalProperties": additional}
	case reflect.Struct:
		// Named types go under components when a registry is present.
		if reg != nil && t.Name() != "" {
			return reg.ref(t, visited)
		}
		return structSchema(t, visited, reg)
	case reflect.Interface:
		return map[string]any{}
	default:
		// chan, func, complex and unsafe.Pointer have no JSON representation.
		return nil
	}
}

func structSchema(t reflect.Type, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	// Break reference cycles with a bare object.
	if visited[t] {
		return map[string]any{schemaKeyType: schemaTypeObject}
	}
	visited = markVisited(visited, t)
	defer delete(visited, t)

	properties := make(map[string]any)
	var required []string

	// Names resolve level by level like encoding/json: the shallowest depth
	// wins, and there one tagged field wins or the name is dropped.
	type fieldCandidate struct {
		schema   map[string]any
		required bool
		tagged   bool
	}
	type embedRef struct {
		t reflect.Type
		// optional: reached through a pointer or omitempty embed, so never required.
		optional bool
	}

	level := []embedRef{{t: t}}
	// expanded holds types flattened at a shallower level; same-level duplicates
	// must still collide.
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

				embeddedType := derefType(field.Type)
				isEmbeddedStruct := field.Anonymous && embeddedType.Kind() == reflect.Struct && embeddedType != timeType && name == ""

				// encoding/json still promotes fields of an embedded unexported struct.
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

				fieldSchema := typeSchema(field.Type, visited, reg)
				if fieldSchema == nil {
					continue
				}

				// The ",string" option wraps the value in a JSON string.
				if tagInfo.asString {
					switch fieldSchema[schemaKeyType] {
					case schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
						fieldSchema[schemaKeyType] = schemaTypeString
					default:
					}
				}

				applyOpenAPITag(&field, fieldSchema)
				// A validate rule outranks omitempty.
				mustValidate := applyValidateTag(&field, fieldSchema)

				if _, ok := candidates[name]; !ok {
					order = append(order, name)
				}
				candidates[name] = append(candidates[name], fieldCandidate{
					schema:   fieldSchema,
					required: mustValidate || (!tagInfo.omit && field.Type.Kind() != reflect.Pointer && !ref.optional),
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
				// One json-tagged candidate wins; otherwise the name is dropped.
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
	if example := structExample(properties, reg); len(example) > 0 {
		schema["example"] = example
	}
	return schema
}

// structExample builds an example object from the properties that have one,
// following references to registered types.
func structExample(properties map[string]any, reg *schemaRegistry) map[string]any {
	example := make(map[string]any)
	for name, raw := range properties {
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if value, found := prop["example"]; found {
			example[name] = value
			continue
		}
		ref, isRef := prop[schemaKeyRef].(string)
		if !isRef || reg == nil {
			continue
		}
		if target := reg.schemas[strings.TrimPrefix(ref, componentsSchemasRef)]; target != nil {
			if value, found := target["example"]; found {
				example[name] = value
			}
		}
	}
	return example
}

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
	name, opts, _ := utils.CutByte(tag, ',')
	// Unusual names are resolved against the running encoding/json.
	if !isPlainJSONTagName(name) {
		name = effectiveJSONTagName(name)
	}
	info := jsonTagInfo{name: name}
	for opts != "" {
		var opt string
		opt, opts, _ = utils.CutByte(opts, ',')
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

// isPlainJSONTagName reports whether every encoding/json release takes name
// as written.
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

// effectiveJSONTagName returns the property name encoding/json gives a field
// tagged with name, or "" when the tag is ignored. Rules for unusual names
// changed in Go 1.27, so it asks the toolchain instead of reimplementing them.
func effectiveJSONTagName(name string) string {
	// Quote the name so backslashes and quotes survive StructTag.Get.
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
			// Tag ignored; the caller uses the field name.
			return ""
		}
		return key
	}
	return ""
}

func applyOpenAPITag(field *reflect.StructField, schema map[string]any) {
	tag := field.Tag.Get("openapi")
	if tag == "" {
		return
	}

	// A flag counts only when a comma or the end follows it, so "deprecated" in
	// "description:Old, deprecated field" stays part of the value.
	locs := openapiDirectiveRe.FindAllStringSubmatchIndex(tag, -1)
	locs = slices.DeleteFunc(locs, func(loc []int) bool {
		return loc[2] < 0 && loc[1] < len(tag) && tag[loc[1]] != ','
	})
	for i, loc := range locs {
		if loc[2] < 0 {
			switch utilsstrings.ToLower(tag[loc[4]:loc[5]]) {
			case "readonly":
				schema["readOnly"] = true
			case "writeonly":
				schema["writeOnly"] = true
			case "deprecated":
				schema["deprecated"] = true
			default:
				// Unreachable: the regexp only matches the flags handled above.
			}
			continue
		}
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
				// Convert to the field's type so no value is unsatisfiable.
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

// applyValidateTag translates validate rules into schema constraints and reports
// whether the field is required. Rules it cannot express are ignored.
func applyValidateTag(field *reflect.StructField, schema map[string]any) bool {
	tag := field.Tag.Get("validate")
	if tag == "" {
		return false
	}
	required := false
	for rule := range strings.SplitSeq(tag, ",") {
		key, value, _ := utils.CutByte(utils.TrimSpace(rule), '=')
		switch key {
		case "required":
			required = true
		case "min", "gte":
			setBound(schema, value, lowerBound)
		case "max", "lte":
			setBound(schema, value, upperBound)
		case "len":
			setBound(schema, value, lowerBound)
			setBound(schema, value, upperBound)
		case "oneof":
			values := strings.Fields(value)
			enum := make([]any, len(values))
			for i, v := range values {
				enum[i] = inferExampleValue(utils.Trim(v, '\''), schema)
			}
			if len(enum) > 0 {
				schema["enum"] = enum
			}
		case "datetime":
			if value == time.RFC3339 {
				setFormat(schema, "date-time")
			}
		default:
			if format, ok := validateFormats[key]; ok {
				setFormat(schema, format)
			}
		}
	}
	return required
}

func setFormat(schema map[string]any, format string) {
	if _, ok := schema[schemaKeyFormat]; !ok {
		schema[schemaKeyFormat] = format
	}
}

// boundSide selects which end of a range a validate rule constrains.
type boundSide uint8

// setBound writes a validate rule's limit under the keyword for the schema's
// type, or nothing if there is none or the value does not parse.
func setBound(schema map[string]any, value string, side boundSide) {
	schemaType, ok := schema[schemaKeyType].(string)
	if !ok {
		return
	}
	keywords, ok := boundKeywords[schemaType]
	if !ok {
		return
	}
	key := keywords[side]
	var bound any
	switch schemaType {
	case schemaTypeInteger:
		n, err := utils.ParseInt(value)
		if err != nil {
			return
		}
		bound = n
	case schemaTypeNumber:
		f, err := utils.ParseFloat64(value)
		if err != nil {
			return
		}
		bound = f
	default:
	}
	if bound == nil {
		n, err := utils.ParseInt(value)
		if err != nil || n < 0 {
			return
		}
		bound = n
	}
	schema[key] = bound
}
