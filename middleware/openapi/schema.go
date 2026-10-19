package openapi

import (
	"encoding"
	"encoding/json"
	"math"
	"mime/multipart"
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

var (
	timeType          = reflect.TypeFor[time.Time]()
	fileHeaderType    = reflect.TypeFor[multipart.FileHeader]()
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

	// An uploaded file part is a binary string, which Swagger UI shows as a file picker.
	if t == fileHeaderType {
		return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "binary"}
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
			return reg.ref(t)
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

	// encoding/json always writes a non-omitempty field, so it is required; a form field is only
	// required when a validate rule says so, as a form has no omitempty.
	implicitRequired := reg.tagKey() == tagJSON

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

				tagInfo := parseJSONTag(&field, reg.tagKey())
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
				// A pointer is written as null when nil, which the schema has to allow.
				// An uploaded file is a part of a form and is never null.
				if field.Type.Kind() == reflect.Pointer && !holdsFileHeaders(field.Type) {
					fieldSchema = reg.nullable(fieldSchema)
				}
				fieldSchema = reg.withRefSiblings(fieldSchema)

				if _, ok := candidates[name]; !ok {
					order = append(order, name)
				}
				candidates[name] = append(candidates[name], fieldCandidate{
					schema:   fieldSchema,
					required: mustValidate || (implicitRequired && !tagInfo.omit && field.Type.Kind() != reflect.Pointer && !ref.optional),
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
		ref, isRef := referenceOf(prop)
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

// referenceOf returns the "$ref" a property schema points at, directly or, in 3.0, through a one-entry "allOf".
func referenceOf(schema map[string]any) (string, bool) {
	if target, ok := schema[schemaKeyRef].(string); ok {
		return target, true
	}
	if all, ok := schema["allOf"].([]any); ok && len(all) == 1 {
		if entry, isMap := all[0].(map[string]any); isMap {
			target, found := entry[schemaKeyRef].(string)
			return target, found
		}
	}
	return "", false
}

type jsonTagInfo struct {
	name     string
	omit     bool
	skip     bool
	asString bool
}

func parseJSONTag(field *reflect.StructField, key string) jsonTagInfo {
	tag := field.Tag.Get(key)
	if tag == "" {
		return jsonTagInfo{}
	}
	if tag == "-" {
		return jsonTagInfo{skip: true}
	}
	name, opts, _ := utils.CutByte(tag, ',')
	// Unusual JSON names are resolved against the running encoding/json.
	if key == tagJSON && !isPlainJSONTagName(name) {
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
			info.asString = key == tagJSON
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
			var enumSlice []any
			for v := range utils.SplitTrimSeq(val, '|') {
				// Convert to the field's type so no value is unsatisfiable.
				enumSlice = append(enumSlice, inferExampleValue(v, schema))
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
		if f, err := parseFloat(val); err == nil {
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
// whether the field is required. Rules it cannot express are ignored, among them gt and lt on a
// float, whose exclusive bound is spelled differently in each OpenAPI version.
//
// The validator's structure is followed: rules after "dive" constrain the elements, so they go to the
// items (or additionalProperties) schema and never make the field required; rules between "keys" and
// "endkeys" constrain map keys and are skipped; and after "omitempty" the rules that an empty value
// would break (lower bounds, len, oneof) are skipped, as the validator does not run them then.
func applyValidateTag(field *reflect.StructField, schema map[string]any) bool {
	tag := field.Tag.Get("validate")
	if tag == "" {
		return false
	}

	rules := validateTarget{
		schema: schema,
		top:    true,
		// A []byte is a base64 string, so its byte counts become character counts.
		bytes: isByteSlice(field.Type),
		// A quoted number is a string in the document while the validator still compares the number.
		numeric: quotedNumber(field),
		// A pointer is validated whenever it is not nil, empty or not.
		pointer: field.Type.Kind() == reflect.Pointer,
	}
	required := false
	inKeys := false
	for rule := range utils.SplitTrimSeq(tag, ',') {
		key, value, _ := utils.CutByte(rule, '=')
		switch key {
		case "keys":
			inKeys = true
			continue
		case "endkeys":
			inKeys = false
			continue
		}
		if inKeys {
			continue
		}
		if key == "dive" {
			rules = rules.dive()
			continue
		}
		if rules.schema == nil {
			continue
		}
		if key == "required" {
			required = required || rules.top
			continue
		}
		rules.apply(key, value)
	}
	return required
}

// validateTarget is the schema a run of validate rules constrains, and what is known about its Go type.
type validateTarget struct {
	schema map[string]any
	// bytes is set for a []byte field, numeric for a quoted number, pointer for a pointer field.
	bytes, numeric, pointer bool
	// omitted is set once "omitempty" has been seen at this level.
	omitted bool
	// top is set while the target is the field itself and not one of its elements.
	top bool
}

// dive moves to the schema of the elements, or to nothing when there is none a rule could be added to.
func (t validateTarget) dive() validateTarget {
	next := validateTarget{}
	for _, keyword := range [...]string{"items", "additionalProperties"} {
		child, ok := t.schema[keyword].(map[string]any)
		if !ok {
			continue
		}
		// A reference takes no sibling keys under 3.0, so its constraints are left out.
		if _, isRef := child[schemaKeyRef]; !isRef {
			next.schema = child
		}
		break
	}
	return next
}

// apply writes one rule onto the target.
func (t *validateTarget) apply(key, value string) {
	if key == "omitempty" {
		t.omitted = !t.pointer
		return
	}
	// An empty value is allowed after omitempty, so a rule that needs a non-empty one cannot be documented.
	if t.omitted {
		switch key {
		case "min", "gte", "gt", "len", "oneof":
			return
		}
	}
	switch key {
	case "min", "gte":
		t.bound(value, lowerBound)
	case "max", "lte":
		t.bound(value, upperBound)
	case "gt":
		t.exclusiveBound(value, lowerBound)
	case "lt":
		t.exclusiveBound(value, upperBound)
	case "len":
		t.bound(value, lowerBound)
		t.bound(value, upperBound)
	case "oneof":
		values := splitRuleParams(value)
		enum := make([]any, len(values))
		for i, v := range values {
			enum[i] = inferExampleValue(v, t.schema)
		}
		if len(enum) > 0 {
			t.schema["enum"] = enum
		}
	case "datetime":
		if value == time.RFC3339 {
			setFormat(t.schema, "date-time")
		}
	default:
		if format, ok := validateFormats[key]; ok {
			setFormat(t.schema, format)
		}
	}
}

// bound writes a limit under the keyword for the schema's type.
func (t *validateTarget) bound(value string, side boundSide) {
	switch {
	case t.numeric:
	case t.bytes:
		if n, ok := byteCount(value, 0); ok {
			setBound(t.schema, strconv.FormatInt(encodedLength(n), 10), side)
		}
	default:
		setBound(t.schema, value, side)
	}
}

// exclusiveBound writes a gt or lt limit one step inside the value.
func (t *validateTarget) exclusiveBound(value string, side boundSide) {
	switch {
	case t.numeric:
	case t.bytes:
		step := int64(-1)
		if side == lowerBound {
			step = 1
		}
		if n, ok := byteCount(value, step); ok {
			setBound(t.schema, strconv.FormatInt(encodedLength(n), 10), side)
		}
	default:
		setExclusiveBound(t.schema, value, side)
	}
}

// byteCount reads a byte count moved by step, or false when it is not a count.
func byteCount(value string, step int64) (int64, bool) {
	n, err := utils.ParseInt(value)
	n += step
	return n, err == nil && n >= 0
}

// encodedLength is the length of n bytes as padded base64.
func encodedLength(n int64) int64 {
	return 4 * ((n + 2) / 3)
}

// splitRuleParams splits a rule's parameter the way the validator does: on whitespace, except inside single
// quotes, which are dropped.
func splitRuleParams(value string) []string {
	var params []string
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }
	for i := 0; i < len(value); {
		if isSpace(value[i]) {
			i++
			continue
		}
		if value[i] == '\'' {
			if end := strings.IndexByte(value[i+1:], '\''); end >= 0 {
				params = append(params, value[i+1:i+1+end])
				i += end + 2
				continue
			}
		}
		end := i
		for end < len(value) && !isSpace(value[end]) {
			end++
		}
		params = append(params, strings.ReplaceAll(value[i:end], "'", ""))
		i = end
	}
	return params
}

// holdsFileHeaders reports whether t is, or is a pointer to or slice of, a multipart file header.
func holdsFileHeaders(t reflect.Type) bool {
	t = derefType(t)
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = derefType(t.Elem())
	}
	return t == fileHeaderType
}

// isByteSlice reports whether t is a slice of bytes, which JSON writes as a base64 string.
func isByteSlice(t reflect.Type) bool {
	t = derefType(t)
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}

// quotedNumber reports whether the field is a number or boolean written as a JSON string by the ",string" option.
func quotedNumber(field *reflect.StructField) bool {
	_, options, _ := utils.CutByte(field.Tag.Get("json"), ',')
	for option := range utils.SplitTrimSeq(options, ',') {
		if option != "string" {
			continue
		}
		switch derefType(field.Type).Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.Bool:
			return true
		default:
			return false
		}
	}
	return false
}

// parseFloat reads a finite float, as a document cannot carry NaN or infinity.
func parseFloat(value string) (float64, error) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err //nolint:wrapcheck // the caller only tests for failure
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, strconv.ErrRange
	}
	return f, nil
}

func setFormat(schema map[string]any, format string) {
	if _, ok := schema[schemaKeyFormat]; !ok {
		schema[schemaKeyFormat] = format
	}
}

// boundSide selects which end of a range a validate rule constrains.
type boundSide uint8

// setExclusiveBound writes a gt or lt limit as the equivalent inclusive one, which holds for
// integers and for lengths and counts. A float has no such equivalent and is left unbounded.
func setExclusiveBound(schema map[string]any, value string, side boundSide) {
	if schema[schemaKeyType] == schemaTypeNumber {
		return
	}
	n, err := utils.ParseInt(value)
	if err != nil {
		return
	}
	if side == lowerBound {
		n++
	} else {
		n--
	}
	setBound(schema, strconv.FormatInt(n, 10), side)
}

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
		f, err := parseFloat(value)
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
