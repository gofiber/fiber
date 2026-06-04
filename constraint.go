package fiber

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type regexMatcher interface {
	MatchString(s string) bool
}

var (
	regexMatcherType = reflect.TypeFor[regexMatcher]()
	stringType       = reflect.TypeFor[string]()
)

func isNilRegexMatcher(matcher regexMatcher) bool {
	if matcher == nil {
		return true
	}
	matcherValue := reflect.ValueOf(matcher)
	switch matcherValue.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return matcherValue.IsNil()
	default:
		return false
	}
}

func compileRegex(handler any, pattern string) regexMatcher {
	result := reflect.ValueOf(handler).Call([]reflect.Value{reflect.ValueOf(pattern)})
	matcher, ok := result[0].Interface().(regexMatcher)
	if !ok {
		panic("fiber: Config.RegexHandler return type must support MatchString(string) bool")
	}
	if isNilRegexMatcher(matcher) {
		panic("fiber: Config.RegexHandler must not return nil")
	}
	return matcher
}

func validateRegexHandler(handler any) any {
	if handler == nil {
		return regexp.MustCompile
	}
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()
	if handlerType.Kind() != reflect.Func || handlerValue.IsNil() {
		panic("fiber: Config.RegexHandler must be a non-nil function")
	}
	if handlerType.NumIn() != 1 || handlerType.In(0) != stringType || handlerType.NumOut() != 1 {
		panic("fiber: Config.RegexHandler must have signature func(string) T")
	}
	if !handlerType.Out(0).Implements(regexMatcherType) {
		panic("fiber: Config.RegexHandler return type must support MatchString(string) bool")
	}
	return handler
}

// ConstraintHandler is the interface that all constraints must implement.
// Built-in and custom constraints are treated uniformly through this interface.
type ConstraintHandler interface {
	// Name returns the constraint identifier used in route patterns (e.g. "int", "minLen", "regex").
	Name() string

	// Execute validates a request parameter value against the constraint.
	// param is the request parameter value to check.
	// data contains the pre-typed constraint data produced by Analyze() at registration time.
	Execute(param string, data []any) bool
}

// ConstraintAnalyzer is an optional interface that constraints can implement
// to preprocess data at route registration time. The returned values are stored
// in Constraint.Data and passed to Execute() on every request, avoiding repeated parsing.
type ConstraintAnalyzer interface {
	// Analyze preprocesses constraint data at route registration time.
	// Returns pre-typed values that will be stored in Constraint.Data.
	Analyze(args []string) ([]any, error)
}

// CustomConstraint is the legacy interface for user-defined constraints.
// It is kept for backward compatibility. CustomConstraint implementations
// are automatically wrapped to satisfy the ConstraintHandler interface.
type CustomConstraint interface {
	Name() string
	Execute(param string, args ...string) bool
}

type customConstraintWrapper struct {
	CustomConstraint
}

func (*customConstraintWrapper) Analyze(args []string) ([]any, error) {
	return stringArgsToAny(parseConstraintArgs(args)), nil
}

func (w *customConstraintWrapper) Execute(param string, data []any) bool {
	args := make([]string, len(data))
	for i, d := range data {
		if s, ok := d.(string); ok {
			args[i] = s
		}
	}
	return w.CustomConstraint.Execute(param, args...)
}

func stringArgsToAny(args []string) []any {
	raw := make([]any, len(args))
	for i, a := range args {
		raw[i] = a
	}
	return raw
}

func parseConstraintArgs(args []string) []string {
	if len(args) != 1 {
		return args
	}
	parsed := splitNonEscaped(args[0], paramConstraintDataSeparator)
	if len(parsed) == 1 {
		parsed[0] = RemoveEscapeChar(parsed[0])
	} else if len(parsed) == 2 {
		parsed[0] = RemoveEscapeChar(parsed[0])
		parsed[1] = RemoveEscapeChar(parsed[1])
	}
	return parsed
}

// builtinConstraints is the registry of all built-in constraint handlers.
var builtinConstraints = []ConstraintHandler{
	intConstraintType{},
	boolConstraintType{},
	floatConstraintType{},
	alphaConstraintType{},
	datetimeConstraintType{},
	guidConstraintType{},
	minLenConstraintType{},
	maxLenConstraintType{},
	lenConstraintType{},
	betweenLenConstraintType{},
	minConstraintType{},
	maxConstraintType{},
	rangeConstraintType{},
}

// findConstraintHandler looks up a constraint handler by name from the merged
// list of custom and built-in constraints. Custom constraints take priority.
func findConstraintHandler(name string, regexHandler any, customs []CustomConstraint) ConstraintHandler {
	for _, cc := range customs {
		if cc.Name() == name {
			return &customConstraintWrapper{CustomConstraint: cc}
		}
	}
	if name == ConstraintRegex {
		return regexConstraintType{regexHandler: regexHandler}
	}
	for _, bc := range builtinConstraints {
		if bc.Name() == name {
			return bc
		}
	}
	return nil
}

// newConstraint creates a Constraint with the given handler and data,
// calling Analyze() if the handler implements ConstraintAnalyzer.
func newConstraint(handler ConstraintHandler, args []string) *Constraint {
	c := &Constraint{
		Name:    handler.Name(),
		handler: handler,
	}
	if analyser, ok := handler.(ConstraintAnalyzer); ok {
		if typed, err := analyser.Analyze(args); err == nil {
			c.Data = typed
		} else {
			// Store raw strings as fallback for invalid data.
			c.Data = stringArgsToAny(args)
		}
	} else {
		c.Data = stringArgsToAny(args)
	}
	return c
}

// matchConstraint validates a parameter against this constraint.
func (c *Constraint) matchConstraint(param string) bool {
	handler := c.handler
	data := c.Data
	if handler == nil {
		handler = findConstraintHandler(resolveConstraintName(c.Name), nil, nil)
		if handler == nil {
			return true
		}
		if analyser, ok := handler.(ConstraintAnalyzer); ok {
			// Convert raw string data to typed data.
			rawArgs := make([]string, len(data))
			for i, d := range data {
				if s, ok := d.(string); ok {
					rawArgs[i] = s
				}
			}
			if typed, err := analyser.Analyze(rawArgs); err == nil {
				data = typed
			}
		}
	}
	return handler.Execute(param, data)
}

// --- Built-in constraint types ---

type intConstraintType struct{}

func (intConstraintType) Name() string { return ConstraintInt }
func (intConstraintType) Execute(param string, _ []any) bool {
	_, err := strconv.Atoi(param)
	return err == nil
}

type boolConstraintType struct{}

func (boolConstraintType) Name() string { return ConstraintBool }
func (boolConstraintType) Execute(param string, _ []any) bool {
	_, err := strconv.ParseBool(param)
	return err == nil
}

type floatConstraintType struct{}

func (floatConstraintType) Name() string { return ConstraintFloat }
func (floatConstraintType) Execute(param string, _ []any) bool {
	_, err := strconv.ParseFloat(param, 64)
	return err == nil
}

type alphaConstraintType struct{}

func (alphaConstraintType) Name() string { return ConstraintAlpha }
func (alphaConstraintType) Execute(param string, _ []any) bool {
	for _, c := range param {
		if !unicode.IsLetter(c) {
			return false
		}
	}
	return param != ""
}

type guidConstraintType struct{}

func (guidConstraintType) Name() string { return ConstraintGUID }
func (guidConstraintType) Execute(param string, _ []any) bool {
	_, err := uuid.Parse(param)
	return err == nil
}

type datetimeConstraintType struct{}

func (datetimeConstraintType) Name() string { return ConstraintDatetime }
func (datetimeConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) == 0 {
		return nil, errors.New("datetime constraint requires a layout argument")
	}
	return []any{args[0]}, nil
}

func (datetimeConstraintType) Execute(param string, data []any) bool {
	if len(data) == 0 {
		return false
	}
	layout, ok := data[0].(string)
	if !ok || layout == "" {
		return false
	}
	_, err := time.Parse(layout, param)
	return err == nil
}

type minLenConstraintType struct{}

func (minLenConstraintType) Name() string { return ConstraintMinLen }
func (minLenConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) == 0 {
		return nil, errors.New("minLen constraint requires an argument")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	return []any{n}, nil
}

func (minLenConstraintType) Execute(param string, data []any) bool {
	if len(data) == 0 {
		return false
	}
	limit, ok := data[0].(int)
	if !ok {
		return false
	}
	return len(param) >= limit
}

type maxLenConstraintType struct{}

func (maxLenConstraintType) Name() string { return ConstraintMaxLen }
func (maxLenConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) == 0 {
		return nil, errors.New("maxLen constraint requires an argument")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	return []any{n}, nil
}

func (maxLenConstraintType) Execute(param string, data []any) bool {
	if len(data) == 0 {
		return false
	}
	limit, ok := data[0].(int)
	if !ok {
		return false
	}
	return len(param) <= limit
}

type lenConstraintType struct{}

func (lenConstraintType) Name() string { return ConstraintLen }
func (lenConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) == 0 {
		return nil, errors.New("len constraint requires an argument")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	return []any{n}, nil
}

func (lenConstraintType) Execute(param string, data []any) bool {
	if len(data) == 0 {
		return false
	}
	limit, ok := data[0].(int)
	if !ok {
		return false
	}
	return len(param) == limit
}

type betweenLenConstraintType struct{}

func (betweenLenConstraintType) Name() string { return ConstraintBetweenLen }
func (betweenLenConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) < 2 {
		return nil, errors.New("betweenLen constraint requires two arguments")
	}
	lo, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	hi, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	return []any{lo, hi}, nil
}

func (betweenLenConstraintType) Execute(param string, data []any) bool {
	if len(data) < 2 {
		return false
	}
	lo, ok := data[0].(int)
	if !ok {
		return false
	}
	hi, ok := data[1].(int)
	if !ok {
		return false
	}
	length := len(param)
	return length >= lo && length <= hi
}

type minConstraintType struct{}

func (minConstraintType) Name() string { return ConstraintMin }
func (minConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) == 0 {
		return nil, errors.New("min constraint requires an argument")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	return []any{n}, nil
}

func (minConstraintType) Execute(param string, data []any) bool {
	if len(data) == 0 {
		return false
	}
	limit, ok := data[0].(int)
	if !ok {
		return false
	}
	num, err := strconv.Atoi(param)
	return err == nil && num >= limit
}

type maxConstraintType struct{}

func (maxConstraintType) Name() string { return ConstraintMax }
func (maxConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) == 0 {
		return nil, errors.New("max constraint requires an argument")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	return []any{n}, nil
}

func (maxConstraintType) Execute(param string, data []any) bool {
	if len(data) == 0 {
		return false
	}
	limit, ok := data[0].(int)
	if !ok {
		return false
	}
	num, err := strconv.Atoi(param)
	return err == nil && num <= limit
}

type rangeConstraintType struct{}

func (rangeConstraintType) Name() string { return ConstraintRange }
func (rangeConstraintType) Analyze(args []string) ([]any, error) {
	args = parseConstraintArgs(args)
	if len(args) < 2 {
		return nil, errors.New("range constraint requires two arguments")
	}
	lo, err := strconv.Atoi(args[0])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	hi, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, fmt.Errorf("parse constraint arg: %w", err)
	}
	return []any{lo, hi}, nil
}

func (rangeConstraintType) Execute(param string, data []any) bool {
	if len(data) < 2 {
		return false
	}
	lo, ok := data[0].(int)
	if !ok {
		return false
	}
	hi, ok := data[1].(int)
	if !ok {
		return false
	}
	num, err := strconv.Atoi(param)
	return err == nil && num >= lo && num <= hi
}

type regexConstraintType struct {
	regexHandler any
}

func (regexConstraintType) Name() string { return ConstraintRegex }
func (r regexConstraintType) Analyze(args []string) ([]any, error) {
	if len(args) == 0 {
		return nil, errors.New("regex constraint requires a pattern argument")
	}
	if r.regexHandler == nil {
		re, err := regexp.Compile(args[0])
		if err != nil {
			return nil, fmt.Errorf("parse constraint arg: %w", err)
		}
		return []any{re}, nil
	}
	matcher := compileRegex(r.regexHandler, args[0])
	return []any{matcher}, nil
}

func (regexConstraintType) Execute(param string, data []any) bool {
	if len(data) == 0 {
		return false
	}
	matcher, ok := data[0].(regexMatcher)
	if !ok || matcher == nil {
		return false
	}
	return matcher.MatchString(param)
}

// resolveConstraintName handles case-insensitive and alias matching for constraint names.
func resolveConstraintName(name string) string {
	switch strings.ToLower(name) {
	case "minlen":
		return ConstraintMinLen
	case "maxlen":
		return ConstraintMaxLen
	case "betweenlen":
		return ConstraintBetweenLen
	default:
		return name
	}
}
