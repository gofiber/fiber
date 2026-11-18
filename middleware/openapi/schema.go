package openapi

import (
	"encoding"
	"encoding/json"
	"encoding/xml"
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
	xmlNameType       = reflect.TypeFor[xml.Name]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// openapiDirectiveRe finds each directive's start; a value runs to the next one.
var openapiDirectiveRe = regexp.MustCompile(`(?:^|,)\s*(description|example|format|enum|default|pattern|title|multipleOf|x-[A-Za-z0-9_.-]+):|(?:^|,)\s*(readOnly|readonly|writeOnly|writeonly|deprecated|uniqueItems|uniqueitems|nullable)\s*`)

// validatePatterns are the validator's string rules that are a regular expression, with its expressions.
var validatePatterns = map[string]string{
	"alpha":       `^[a-zA-Z]+$`,
	"alphanum":    `^[a-zA-Z0-9]+$`,
	"numeric":     `^[-+]?[0-9]+(?:\.[0-9]+)?$`,
	"number":      `^[0-9]+$`,
	"hexadecimal": `^(0[xX])?[0-9a-fA-F]+$`,
	"hexcolor":    `^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`,
	"e164":        `^\+?[1-9]\d{7,14}$`,
	"ascii":       `^[\x00-\x7F]*$`,
	"jwt":         `^[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]*$`,
}

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

var boundKeywords = map[string][2]string{
	schemaTypeString:  {"minLength", "maxLength"},
	schemaTypeArray:   {"minItems", "maxItems"},
	schemaTypeObject:  {"minProperties", "maxProperties"},
	schemaTypeInteger: {"minimum", "maximum"},
	schemaTypeNumber:  {"minimum", "maximum"},
}

// SchemaOf generates an OpenAPI JSON Schema from a Go value by reflection.
// Embedded structs are flattened as in encoding/json; fields with no JSON form are skipped.
//
// Recognized tags: `json`, `openapi` (description, example, format, enum with "|",
// readOnly, writeOnly, deprecated) and `validate` (required, min, max, len, gte, lte,
// oneof, common formats). Nested structs are inlined here, unlike in the route helpers.
func SchemaOf(v any) map[string]any {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil
	}
	return typeSchema(t, nil, nil)
}

func implementsMarshaler(t, iface reflect.Type) bool {
	return t.Implements(iface) || reflect.PointerTo(t).Implements(iface)
}

func markVisited(visited map[reflect.Type]bool, t reflect.Type) map[reflect.Type]bool {
	if visited == nil {
		visited = make(map[reflect.Type]bool)
	}
	visited[t] = true
	return visited
}

// derefType strips pointers; a self-referential pointer type stays a pointer.
func derefType(t reflect.Type) reflect.Type {
	for range maxPointerDepth {
		if t.Kind() != reflect.Pointer {
			break
		}
		t = t.Elem()
	}
	return t
}

func typeSchema(t reflect.Type, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	t = derefType(t)
	if t.Kind() == reflect.Pointer {
		return map[string]any{}
	}

	// A type that describes itself: a named struct becomes a component, anything else is inline.
	if hasCustomSchema(t, reg) {
		if reg != nil && t.Kind() == reflect.Struct && t.Name() != "" && !reg.configured(t) {
			return reg.ref(t)
		}
		if visited[t] {
			return map[string]any{}
		}
		visited = markVisited(visited, t)
		defer delete(visited, t)
		return customSchema(t, visited, reg)
	}

	if t == timeType {
		return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "date-time"}
	}

	if t == fileHeaderType {
		return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "binary"}
	}

	if t == jsonNumberType {
		return map[string]any{schemaKeyType: schemaTypeNumber}
	}

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
		reflect.Uintptr:
		return map[string]any{schemaKeyType: schemaTypeInteger}
	case reflect.Float32, reflect.Float64:
		return map[string]any{schemaKeyType: schemaTypeNumber}
	case reflect.Slice, reflect.Array:
		// []byte marshals as base64; byte arrays marshal as number arrays.
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			return map[string]any{schemaKeyType: schemaTypeString, schemaKeyFormat: "byte"}
		}
		if visited[t] {
			return map[string]any{schemaKeyType: schemaTypeArray}
		}
		visited = markVisited(visited, t)
		items := typeSchema(t.Elem(), visited, reg)
		delete(visited, t)
		if items == nil {
			return nil
		}
		return map[string]any{schemaKeyType: schemaTypeArray, schemaKeyItems: items}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		if visited[t] {
			return map[string]any{schemaKeyType: schemaTypeObject}
		}
		visited = markVisited(visited, t)
		additional := typeSchema(t.Elem(), visited, reg)
		delete(visited, t)
		if additional == nil {
			return nil
		}
		return map[string]any{schemaKeyType: schemaTypeObject, schemaKeyAdditional: additional}
	case reflect.Struct:
		if reg != nil && t.Name() != "" {
			return reg.ref(t)
		}
		return structSchema(t, visited, reg)
	case reflect.Interface:
		return map[string]any{}
	default:
		return nil
	}
}

func structSchema(t reflect.Type, visited map[reflect.Type]bool, reg *schemaRegistry) map[string]any {
	if visited[t] {
		return map[string]any{schemaKeyType: schemaTypeObject}
	}
	visited = markVisited(visited, t)
	defer delete(visited, t)

	properties := make(map[string]any)
	objectExample := make(map[string]any)
	var required []string

	// Names resolve like encoding/json: the shallowest depth wins.
	type fieldCandidate struct {
		// example is what the property adds to the object's example, read before a wrapper hides its $ref.
		example    any
		schema     map[string]any
		hasExample bool
		required   bool
		tagged     bool
	}
	type embedRef struct {
		t        reflect.Type
		optional bool
	}

	// A non-omitempty JSON or XML field is always written, so required; a form field needs a validate rule.
	implicitRequired := reg.tagKey() == tagJSON || reg.tagKey() == tagXML
	// The object's own title, description and extensions, from a blank field tagged openapi, and its XML name.
	objectMeta := map[string]any{}

	level := []embedRef{{t: t}}
	expanded := map[reflect.Type]bool{t: true}
	dropped := make(map[string]bool)

	for len(level) > 0 {
		var nextLevel []embedRef
		candidates := make(map[string][]fieldCandidate)
		var order []string

		for _, ref := range level {
			for i := range ref.t.NumField() {
				field := ref.t.Field(i)

				if ref.t == t && field.Name == "_" {
					applyOpenAPITag(&field, objectMeta)
					continue
				}
				if ref.t == t && field.Type == xmlNameType && reg.tagKey() == tagXML {
					if name, _, _ := utils.CutByte(field.Tag.Get(tagXML), ','); name != "" && name != "-" {
						objectMeta["xml"] = map[string]any{keyNameField: name}
					}
					continue
				}

				if reg != nil && reg.request && field.Tag.Get(reg.tagKey()) == "" && taggedForAnotherLocation(&field, reg.tagKey()) {
					continue // a parameter, not a part of the body
				}

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

				if tagInfo.asString {
					switch fieldSchema[schemaKeyType] {
					case schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
						fieldSchema[schemaKeyType] = schemaTypeString
					default:
					}
				}

				nullable := applyOpenAPITag(&field, fieldSchema)
				mustValidate := applyValidateTag(&field, fieldSchema)
				applyXMLTag(fieldSchema, &tagInfo, reg)
				fieldExample, hasExample := propertyExample(fieldSchema, reg)
				// A nil pointer is written as null; an uploaded file part is never null.
				if nullable || (field.Type.Kind() == reflect.Pointer && !holdsFileHeaders(field.Type)) {
					fieldSchema = reg.nullable(fieldSchema)
				}
				fieldSchema = reg.withRefSiblings(fieldSchema)

				if _, ok := candidates[name]; !ok {
					order = append(order, name)
				}
				candidates[name] = append(candidates[name], fieldCandidate{
					schema:     fieldSchema,
					example:    fieldExample,
					hasExample: hasExample,
					required:   mustValidate || (implicitRequired && !tagInfo.omit && field.Type.Kind() != reflect.Pointer && !ref.optional),
					tagged:     tagInfo.name != "",
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
			if cands[chosen].hasExample {
				objectExample[name] = cands[chosen].example
			}
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
	if len(objectExample) > 0 {
		schema["example"] = objectExample
	}
	applyObjectMeta(schema, objectMeta)
	return schema
}

// applyObjectMeta adds what the blank field of a struct (and its XML name) says about the object itself. An
// example given there is JSON and replaces the one assembled from the fields.
func applyObjectMeta(schema, meta map[string]any) {
	for key, value := range meta {
		if key == "example" {
			if text, isText := value.(string); isText {
				var decoded any
				if json.Unmarshal([]byte(text), &decoded) == nil {
					schema[key] = decoded
				}
			}
			continue
		}
		schema[key] = value
	}
}

// applyXMLTag adds the XML object an xml tag implies: an attribute, or a wrapped list with its item name.
func applyXMLTag(fieldSchema map[string]any, info *jsonTagInfo, reg *schemaRegistry) {
	if reg.tagKey() != tagXML {
		return
	}
	switch {
	case info.attr:
		fieldSchema["xml"] = map[string]any{"attribute": true}
	case info.itemName != "" && fieldSchema[schemaKeyType] == schemaTypeArray:
		fieldSchema["xml"] = map[string]any{"wrapped": true}
		if items, isMap := fieldSchema[schemaKeyItems].(map[string]any); isMap {
			items["xml"] = map[string]any{keyNameField: info.itemName}
			fieldSchema[schemaKeyItems] = reg.withRefSiblings(items)
		}
	}
}

// propertyExample returns what a property adds to its object's example, read before wrapping hides its $ref.
func propertyExample(prop map[string]any, reg *schemaRegistry) (any, bool) {
	if value, found := prop["example"]; found {
		return value, true
	}
	ref, isRef := prop[schemaKeyRef].(string)
	if !isRef || reg == nil {
		return nil, false
	}
	value, found := reg.schemas[strings.TrimPrefix(ref, componentsSchemasRef)]["example"]
	return value, found
}

type jsonTagInfo struct {
	name     string
	itemName string // the element name of a list an xml tag wraps ("items>item")
	omit     bool
	skip     bool
	asString bool
	attr     bool // an xml attribute
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
	// Unusual names are resolved against the running encoding/json.
	if key == tagJSON && name != "" && !isPlainJSONTagName(name) {
		name = effectiveJSONTagName(name)
	}
	info := jsonTagInfo{name: name}
	if key == tagXML {
		// "wrapper>item" names the element of a list and the element wrapped around its items.
		if first, _, found := strings.Cut(name, ">"); found {
			info.name, info.itemName = first, name[strings.LastIndexByte(name, '>')+1:]
		}
	}
	for opts != "" {
		var opt string
		opt, opts, _ = utils.CutByte(opts, ',')
		switch opt {
		case "omitempty", "omitzero":
			info.omit = true
		case "string":
			info.asString = key == tagJSON
		case "attr":
			info.attr = key == tagXML
		case "any", "comment", "innerxml":
			// Not an element of its own: the content XML collects, or a comment.
			info.skip = key == tagXML
		default:
		}
	}
	return info
}

// isPlainJSONTagName reports whether every encoding/json release takes name as written.
func isPlainJSONTagName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}
	return true
}

// effectiveJSONTagName returns the property name encoding/json gives a field tagged name, or "" if ignored.
// Rules for unusual names changed in Go 1.27, so it asks the toolchain.
func effectiveJSONTagName(name string) string {
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
			return ""
		}
		return key
	}
	return ""
}

// applyOpenAPITag applies the openapi tag of field to its schema, and reports whether it asked for null.
func applyOpenAPITag(field *reflect.StructField, schema map[string]any) bool {
	tag := field.Tag.Get("openapi")
	if tag == "" {
		return false
	}
	nullable := false

	// A flag counts only when a comma or the end follows it.
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
			case "uniqueitems":
				schema["uniqueItems"] = true
			case "nullable":
				nullable = true
			default:
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
		case schemaKeyEnum:
			var enumSlice []any
			for v := range utils.SplitTrimSeq(val, '|') {
				enumSlice = append(enumSlice, inferExampleValue(v, schema))
			}
			schema[schemaKeyEnum] = enumSlice
		case keyDefault:
			schema[keyDefault] = inferExampleValue(val, schema)
		case "pattern":
			schema["pattern"] = val
		case "title":
			schema["title"] = val
		case "multipleOf":
			if f, err := parseFloat(val); err == nil && f > 0 {
				schema["multipleOf"] = f
			}
		default:
			// An "x-" extension: a value that is JSON (5, true, {"a":1}) is kept as such, anything else as text.
			var decoded any
			if err := json.Unmarshal([]byte(val), &decoded); err == nil {
				schema[key] = decoded
			} else {
				schema[key] = val
			}
		}
	}
	return nullable
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
	}
	return val
}

// applyValidateTag translates validate rules into schema constraints and reports whether the field is required.
// It follows the validator: rules after "dive" go to the elements, "keys"…"endkeys" is skipped, and
// after "omitempty" the rules an empty value would break are skipped.
func applyValidateTag(field *reflect.StructField, schema map[string]any) bool {
	tag := field.Tag.Get("validate")
	if tag == "" {
		return false
	}

	rules := validateTarget{
		schema:  schema,
		top:     true,
		bytes:   isByteSlice(field.Type),
		numeric: quotedNumber(field),
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

// validateTarget is the schema a run of validate rules constrains.
type validateTarget struct {
	schema map[string]any
	// bytes: []byte field; numeric: quoted number; pointer: pointer field.
	bytes, numeric, pointer bool
	omitted                 bool
	top                     bool
}

func (t validateTarget) dive() validateTarget {
	next := validateTarget{}
	for _, keyword := range [...]string{"items", schemaKeyAdditional} {
		child, ok := t.schema[keyword].(map[string]any)
		if !ok {
			continue
		}
		// A $ref takes no sibling keys under 3.0.
		if _, isRef := child[schemaKeyRef]; !isRef {
			next.schema = child
		}
		break
	}
	return next
}

func (t *validateTarget) apply(key, value string) {
	if key == "omitempty" {
		t.omitted = !t.pointer
		return
	}
	// An empty value is allowed after omitempty.
	if t.omitted {
		switch key {
		case "min", "gte", "gt", "len", "oneof", "eq", "port", "startswith", "endswith", ruleContains:
			return
		}
		if _, isPattern := validatePatterns[key]; isPattern && key != "ascii" {
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
			t.schema[schemaKeyEnum] = enum
		}
	case "datetime":
		if value == time.RFC3339 {
			setFormat(t.schema, "date-time")
		}
	case "eq":
		t.equal(value)
	case "ne":
		if enum := t.single(value); enum != nil {
			t.schema["not"] = map[string]any{schemaKeyEnum: enum}
		}
	case "unique":
		// A parameter names the field of the elements that must differ, which a schema cannot say.
		if value == "" && t.schema[schemaKeyType] == schemaTypeArray {
			t.schema["uniqueItems"] = true
		}
	case "port":
		if t.schema[schemaKeyType] == schemaTypeInteger {
			t.schema["minimum"], t.schema["maximum"] = 1, 65535
		}
	case "startswith":
		t.pattern("^" + regexp.QuoteMeta(value))
	case "endswith":
		t.pattern(regexp.QuoteMeta(value) + "$")
	case ruleContains:
		t.pattern(regexp.QuoteMeta(value))
	default:
		if format, ok := validateFormats[key]; ok {
			setFormat(t.schema, format)
		} else if expression, ok := validatePatterns[key]; ok {
			t.pattern(expression)
		}
	}
}

// pattern adds a regular expression to a string, unless it has one: a schema holds a single pattern.
func (t *validateTarget) pattern(expression string) {
	if t.schema[schemaKeyType] != schemaTypeString || t.bytes || t.numeric {
		return
	}
	if _, has := t.schema["pattern"]; !has {
		t.schema["pattern"] = expression
	}
}

// single is the one-value enum a scalar must equal, or nil for a schema that is not a scalar.
func (t *validateTarget) single(value string) []any {
	switch t.schema[schemaKeyType] {
	case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
		if (t.bytes || t.numeric) && t.schema[schemaKeyType] != schemaTypeString {
			return nil
		}
		return []any{inferExampleValue(value, t.schema)}
	default:
		return nil
	}
}

// equal is the validator's eq: a scalar equals the value, and a list or map has that length.
func (t *validateTarget) equal(value string) {
	switch t.schema[schemaKeyType] {
	case schemaTypeArray, schemaTypeObject:
		t.bound(value, lowerBound)
		t.bound(value, upperBound)
	default:
		if t.bytes || t.numeric {
			return
		}
		if enum := t.single(value); enum != nil {
			t.schema[schemaKeyEnum] = enum
		}
	}
}

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

func byteCount(value string, step int64) (int64, bool) {
	n, err := utils.ParseInt(value)
	n += step
	return n, err == nil && n >= 0
}

func encodedLength(n int64) int64 {
	return 4 * ((n + 2) / 3)
}

// splitRuleParams splits on whitespace except inside single quotes, as the validator does.
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

func holdsFileHeaders(t reflect.Type) bool {
	t = derefType(t)
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = derefType(t.Elem())
	}
	return t == fileHeaderType
}

func isByteSlice(t reflect.Type) bool {
	t = derefType(t)
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8
}

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

type boundSide uint8

// setExclusiveBound writes gt/lt as the inclusive bound; floats have no equivalent and stay unbounded.
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
