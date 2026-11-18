package openapi

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/gofiber/utils/v2"
)

// conditionalRule is a validate rule that makes a field required depending on other fields of the struct.
type conditionalRule struct {
	name string   // the rule: required_with, required_if, ...
	args []string // the Go field names, or the field name and value pairs
}

// fieldConditions is the conditional rules of the property a field became.
type fieldConditions struct {
	property string
	rules    []conditionalRule
}

// conditionalRules reads the required_with, required_with_all, required_without, required_without_all, required_if
// and required_unless rules of a field's validate tag; the rules after "dive" belong to its elements.
func conditionalRules(field *reflect.StructField) []conditionalRule {
	tag := field.Tag.Get("validate")
	if tag == "" {
		return nil
	}
	var rules []conditionalRule
	for rule := range utils.SplitTrimSeq(tag, ',') {
		key, value, _ := utils.CutByte(rule, '=')
		if key == "dive" || key == "keys" {
			break
		}
		switch key {
		case "required_with", "required_with_all", "required_without", "required_without_all", "required_if", "required_unless":
			rules = append(rules, conditionalRule{name: key, args: strings.Fields(value)})
		default:
		}
	}
	return rules
}

// conditionalSchemas turns the conditional rules into allOf entries (JSON Schema 2020-12, so 3.1+): a rule whose
// fields are not properties of the object is left out.
func conditionalSchemas(conditions []fieldConditions, goNames map[string]string, properties map[string]any) []any {
	var out []any
	for i := range conditions {
		for _, rule := range conditions[i].rules {
			if entry := conditionalSchema(conditions[i].property, rule, goNames, properties); entry != nil {
				out = append(out, entry)
			}
		}
	}
	return out
}

func conditionalSchema(property string, rule conditionalRule, goNames map[string]string, properties map[string]any) map[string]any {
	present := func(name string) map[string]any { return map[string]any{schemaKeyRequired: []any{name}} }
	absent := func(name string) map[string]any { return map[string]any{schemaKeyNot: present(name)} }
	then := present(property)
	when := func(condition map[string]any) map[string]any { return map[string]any{"if": condition, "then": then} }

	names := make([]string, 0, len(rule.args))
	pairs := strings.HasSuffix(rule.name, "_if") || strings.HasSuffix(rule.name, "_unless")
	values := make([]any, 0, len(rule.args)/2)
	for i := 0; i < len(rule.args); i++ {
		name, ok := goNames[rule.args[i]]
		if !ok {
			return nil
		}
		names = append(names, name)
		if pairs {
			if i+1 >= len(rule.args) {
				return nil
			}
			i++
			values = append(values, constantFor(properties[name], rule.args[i]))
		}
	}
	if len(names) == 0 {
		return nil
	}
	anyOf := func(list []any) map[string]any {
		if len(list) == 1 {
			return list[0].(map[string]any) //nolint:errcheck,forcetypeassert // built just above
		}
		return map[string]any{schemaKeyAnyOf: list}
	}
	each := func(build func(i int) any) []any {
		list := make([]any, len(names))
		for i := range names {
			list[i] = build(i)
		}
		return list
	}
	equals := func(i int) any {
		return map[string]any{
			schemaKeyRequired:   []any{names[i]},
			schemaKeyProperties: map[string]any{names[i]: map[string]any{"const": values[i]}},
		}
	}

	switch rule.name {
	case "required_with":
		dependent := make(map[string]any, len(names))
		for _, name := range names {
			dependent[name] = []any{property}
		}
		return map[string]any{"dependentRequired": dependent}
	case "required_with_all":
		return when(map[string]any{schemaKeyRequired: toAny(names)})
	case "required_without":
		return when(anyOf(each(func(i int) any { return absent(names[i]) })))
	case "required_without_all":
		return when(map[string]any{schemaKeyAllOf: each(func(i int) any { return absent(names[i]) })})
	case "required_if":
		return when(map[string]any{schemaKeyAllOf: each(equals)})
	case "required_unless":
		return when(map[string]any{schemaKeyNot: anyOf(each(equals))})
	default:
		return nil
	}
}

func toAny(list []string) []any {
	out := make([]any, len(list))
	for i := range list {
		out[i] = list[i]
	}
	return out
}

// constantFor reads a rule's text as a value of the property's type.
func constantFor(schema any, text string) any {
	typed, ok := schema.(map[string]any)
	if !ok {
		return text
	}
	switch typed[schemaKeyType] {
	case schemaTypeInteger:
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			return n
		}
	case schemaTypeNumber:
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return f
		}
	case schemaTypeBoolean:
		if b, err := strconv.ParseBool(text); err == nil {
			return b
		}
	default:
	}
	return text
}
