package openapi

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// Route patterns may constrain a parameter (":id<int>", ":n<range(1,10)>"), and
// those constraints describe the accepted values precisely enough to type the
// generated schema. The grammar below mirrors path.go: constraints inside the
// "<...>" span are separated by non-escaped ';', a constraint's arguments run
// from its first non-escaped '(' to its last ')', and arguments are separated by
// non-escaped ','.
const (
	constraintSeparator     = ';'
	constraintArgsStart     = '('
	constraintArgsEnd       = ')'
	constraintArgsSeparator = ','
	constraintEscapeChar    = '\\'
)

// pathParamSchema derives a parameter schema from a route pattern's constraint
// span. An empty or unrecognized span yields the default string schema, so an
// unknown or custom constraint never produces a wrong type.
func pathParamSchema(rawConstraints string) map[string]any {
	schema := map[string]any{}
	for _, entry := range splitNonEscaped(rawConstraints, constraintSeparator) {
		parsed := splitConstraintEntry(entry)
		if parsed.name == "" {
			continue
		}
		applyConstraintToSchema(schema, parsed.name, parsed.args)
	}
	if _, ok := schema[schemaKeyType]; !ok {
		schema[schemaKeyType] = schemaTypeString
	}
	return schema
}

// applyConstraintToSchema merges one constraint into schema. Keywords already
// present are never overwritten, so in a chain like "<int;min(5)>" the first
// constraint that pins the type keeps it.
func applyConstraintToSchema(schema map[string]any, name string, args []string) {
	switch resolveConstraintName(name) {
	case fiber.ConstraintInt:
		setSchemaType(schema, schemaTypeInteger)
	case fiber.ConstraintBool:
		setSchemaType(schema, schemaTypeBoolean)
	case fiber.ConstraintFloat:
		setSchemaType(schema, schemaTypeNumber)
	case fiber.ConstraintAlpha:
		// The runtime check accepts any Unicode letter. A "^[A-Za-z]+$" pattern
		// would document the endpoint as stricter than it is, and "\p{L}" needs
		// a regex flavor OpenAPI tooling cannot be assumed to have, so the type
		// alone is the honest description.
		setSchemaType(schema, schemaTypeString)
	case fiber.ConstraintGUID:
		setSchemaType(schema, schemaTypeString)
		setSchemaKey(schema, schemaKeyFormat, "uuid")
	case fiber.ConstraintDatetime:
		setSchemaType(schema, schemaTypeString)
		if format := datetimeLayoutFormat(args); format != "" {
			setSchemaKey(schema, schemaKeyFormat, format)
		}
	case fiber.ConstraintRegex:
		setSchemaType(schema, schemaTypeString)
		if len(args) > 0 && args[0] != "" {
			setSchemaKey(schema, "pattern", args[0])
		}
	case fiber.ConstraintMinLen:
		setSchemaType(schema, schemaTypeString)
		setIntSchemaKey(schema, "minLength", args, 0)
	case fiber.ConstraintMaxLen:
		setSchemaType(schema, schemaTypeString)
		setIntSchemaKey(schema, "maxLength", args, 0)
	case fiber.ConstraintLen:
		setSchemaType(schema, schemaTypeString)
		setIntSchemaKey(schema, "minLength", args, 0)
		setIntSchemaKey(schema, "maxLength", args, 0)
	case fiber.ConstraintBetweenLen:
		setSchemaType(schema, schemaTypeString)
		setIntSchemaKey(schema, "minLength", args, 0)
		setIntSchemaKey(schema, "maxLength", args, 1)
	case fiber.ConstraintMin:
		setSchemaType(schema, schemaTypeInteger)
		setIntSchemaKey(schema, "minimum", args, 0)
	case fiber.ConstraintMax:
		setSchemaType(schema, schemaTypeInteger)
		setIntSchemaKey(schema, "maximum", args, 0)
	case fiber.ConstraintRange:
		setSchemaType(schema, schemaTypeInteger)
		setIntSchemaKey(schema, "minimum", args, 0)
		setIntSchemaKey(schema, "maximum", args, 1)
	default:
		// A custom or unknown constraint carries no portable schema meaning.
	}
}

// resolveConstraintName folds the lowercase aliases the router accepts onto the
// canonical constraint names, mirroring path.go's resolveConstraintName.
func resolveConstraintName(name string) string {
	switch strings.ToLower(name) {
	case fiber.ConstraintMinLenLower:
		return fiber.ConstraintMinLen
	case fiber.ConstraintMaxLenLower:
		return fiber.ConstraintMaxLen
	case fiber.ConstraintBetweenLenLower:
		return fiber.ConstraintBetweenLen
	default:
		return utilsstrings.ToLower(name)
	}
}

// datetimeLayoutFormat maps the two Go time layouts that have an exact OpenAPI
// format to it. Any other layout is left as a plain string, because "format"
// would then claim a shape the route does not actually accept.
func datetimeLayoutFormat(args []string) string {
	if len(args) == 0 {
		return ""
	}
	switch args[0] {
	case time.RFC3339, time.RFC3339Nano:
		return "date-time"
	case time.DateOnly:
		return "date"
	case time.TimeOnly:
		return "time"
	default:
		return ""
	}
}

func setSchemaType(schema map[string]any, typ string) {
	setSchemaKey(schema, schemaKeyType, typ)
}

func setSchemaKey(schema map[string]any, key string, value any) {
	if _, ok := schema[key]; !ok {
		schema[key] = value
	}
}

// setIntSchemaKey sets key from args[idx] when that argument is a plain integer.
// A non-numeric argument is skipped rather than guessed at.
func setIntSchemaKey(schema map[string]any, key string, args []string, idx int) {
	if idx >= len(args) {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(args[idx]))
	if err != nil {
		return
	}
	setSchemaKey(schema, key, n)
}

// parsedConstraint is one entry of a "<...>" span, split into its name and its
// argument list.
type parsedConstraint struct {
	name string
	args []string
}

// splitConstraintEntry separates a constraint's name from its arguments,
// matching path.go: the argument list runs from the first non-escaped '(' to the
// last ')'.
func splitConstraintEntry(entry string) parsedConstraint {
	entry = strings.TrimSpace(entry)
	start := indexNonEscaped(entry, constraintArgsStart)
	end := strings.LastIndexByte(entry, constraintArgsEnd)
	if start == -1 || end == -1 || end < start {
		return parsedConstraint{name: entry}
	}
	return parsedConstraint{
		name: entry[:start],
		args: splitNonEscaped(entry[start+1:end], constraintArgsSeparator),
	}
}

// splitNonEscaped splits s on every occurrence of sep that is not preceded by a
// backslash, mirroring path.go's splitNonEscaped.
func splitNonEscaped(s string, sep byte) []string {
	var result []string
	for {
		i := indexNonEscaped(s, sep)
		if i == -1 {
			return append(result, s)
		}
		result = append(result, s[:i])
		s = s[i+1:]
	}
}

// indexNonEscaped returns the index of the first char that is not preceded by a
// backslash, or -1.
func indexNonEscaped(s string, char byte) int {
	for i := range len(s) {
		if s[i] == char && (i == 0 || s[i-1] != constraintEscapeChar) {
			return i
		}
	}
	return -1
}
